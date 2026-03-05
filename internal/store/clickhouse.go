// ABOUTME: ClickHouse connection management, schema bootstrap, and lifecycle.
// ABOUTME: Defines the ClickHouseStore struct and constructors; write/read/search in separate files.
package store

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"net"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
)

//go:embed schema.sql
var schemaSQL string

// ClickHouseStore implements Store and ReadStore using ClickHouse native protocol.
type ClickHouseStore struct {
	conn     clickhouse.Conn
	database string
}

// ConnectOptions holds parameters for connecting to ClickHouse.
type ConnectOptions struct {
	Addr     string
	Database string
	User     string
	Password string
	Secure   bool
}

// NewClickHouseStore opens a native-protocol connection to ClickHouse with default credentials.
func NewClickHouseStore(addr, database string) (*ClickHouseStore, error) {
	return NewClickHouseStoreFromOptions(ConnectOptions{Addr: addr, Database: database})
}

// NewClickHouseStoreWithAuth opens a native-protocol connection to ClickHouse with explicit credentials.
func NewClickHouseStoreWithAuth(addr, database, user, password string) (*ClickHouseStore, error) {
	return NewClickHouseStoreFromOptions(ConnectOptions{
		Addr: addr, Database: database, User: user, Password: password,
	})
}

// NewClickHouseStoreFromOptions opens a native-protocol connection using the given options.
// Bootstraps by connecting to "default" first to CREATE DATABASE IF NOT EXISTS,
// since hosted ClickHouse validates the database during connection handshake.
func NewClickHouseStoreFromOptions(opts ConnectOptions) (*ClickHouseStore, error) {
	user := opts.User
	if user == "" {
		user = "default"
	}

	// Port 8443 is the standard ClickHouse HTTPS port (HTTP protocol).
	// All other ports use native protocol.
	proto := clickhouse.Native
	if _, port, _ := net.SplitHostPort(opts.Addr); port == "8443" {
		proto = clickhouse.HTTP
	}

	mkOpts := func(db string) *clickhouse.Options {
		o := &clickhouse.Options{
			Addr:     []string{opts.Addr},
			Auth:     clickhouse.Auth{Database: db, Username: user, Password: opts.Password},
			Protocol: proto,
		}
		if opts.Secure || proto == clickhouse.HTTP {
			o.TLS = &tls.Config{}
		}
		return o
	}

	// Bootstrap: connect to "default" to ensure working database exists.
	bootstrapConn, err := clickhouse.Open(mkOpts("default"))
	if err != nil {
		return nil, fmt.Errorf("open clickhouse (bootstrap): %w", err)
	}
	if err := bootstrapConn.Exec(context.Background(), "CREATE DATABASE IF NOT EXISTS "+opts.Database); err != nil {
		_ = bootstrapConn.Close()
		return nil, fmt.Errorf("create database %s: %w", opts.Database, err)
	}
	_ = bootstrapConn.Close()

	conn, err := clickhouse.Open(mkOpts(opts.Database))
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}
	return &ClickHouseStore{conn: conn, database: opts.Database}, nil
}


// ResetDatabase drops and recreates the database, then re-creates the schema.
// ClickHouse allows cross-database DDL from any connection.
func (s *ClickHouseStore) ResetDatabase(ctx context.Context) error {
	if err := s.conn.Exec(ctx, "DROP DATABASE IF EXISTS "+s.database); err != nil {
		return fmt.Errorf("drop database: %w", err)
	}
	if err := s.conn.Exec(ctx, "CREATE DATABASE "+s.database); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	return s.EnsureSchema(ctx)
}

// EnsureSchema creates the tables defined in the embedded DDL file.
// Statements are split on ";\n" and executed one by one. Lines starting
// with -- are stripped before execution.
func (s *ClickHouseStore) EnsureSchema(ctx context.Context) error {
	stmts := strings.Split(schemaSQL, ";\n")
	for _, stmt := range stmts {
		stmt = stripSQLComments(stmt)
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if err := s.conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("ensure schema (%s...): %w", truncate(stmt, 60), err)
		}
	}
	return nil
}

// stripSQLComments removes lines that start with -- from a SQL statement.
func stripSQLComments(sql string) string {
	var lines []string
	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "--") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// Close releases the ClickHouse connection.
func (s *ClickHouseStore) Close() error {
	return s.conn.Close()
}

// truncate returns the first n bytes of s for error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Interface compliance assertions.
var _ Store = (*ClickHouseStore)(nil)
var _ ReadStore = (*ClickHouseStore)(nil)
