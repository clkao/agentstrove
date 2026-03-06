// ABOUTME: CLI entry point with subcommands for sync daemon and API server.
// ABOUTME: Supports "sync" for one-shot sync, "daemon" for the watcher, and "serve" for the HTTP API server.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/clkao/agentlore/internal/api"
	"github.com/clkao/agentlore/internal/config"
	"github.com/clkao/agentlore/internal/reader"
	"github.com/clkao/agentlore/internal/store"
	astSync "github.com/clkao/agentlore/internal/sync"
)

var (
	version   = "0.1.0-dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	args := os.Args[1:]

	// No subcommand or first arg is a flag → show usage
	if len(args) == 0 || args[0][0] == '-' {
		printUsage()
		return 1
	}

	subcmd := args[0]
	args = args[1:]

	// Parse flags from remaining args
	configPath := defaultConfigPath()
	port := 0
	force := false
	resetDB := false
	for i, a := range args {
		if a == "-config" || a == "--config" {
			if i+1 < len(args) {
				configPath = args[i+1]
			}
		}
		if a == "-port" || a == "--port" {
			if i+1 < len(args) {
				if _, err := fmt.Sscanf(args[i+1], "%d", &port); err != nil {
					fmt.Fprintf(os.Stderr, "invalid port: %s\n", args[i+1])
					return 1
				}
			}
		}
		if a == "-force" || a == "--force" {
			force = true
		}
		if a == "-reset-db" || a == "--reset-db" {
			resetDB = true
		}
	}

	switch subcmd {
	case "version":
		fmt.Printf("agentlore %s (commit=%s, built=%s)\n", version, commit, buildDate)
		return 0
	case "sync":
		return runSync(configPath, force, resetDB)
	case "daemon":
		return runDaemon(configPath)
	case "serve":
		return runServe(configPath, port)
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n\n", subcmd)
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "agentlore %s\n\n", version)
	fmt.Fprintf(os.Stderr, "Usage: agentlore <command> [flags]\n\n")
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  sync      Sync local agentsview data to ClickHouse (one-shot)\n")
	fmt.Fprintf(os.Stderr, "  daemon    Watch agentsview for changes and sync continuously\n")
	fmt.Fprintf(os.Stderr, "  serve     Start the HTTP API server for the conversation browser\n")
	fmt.Fprintf(os.Stderr, "  version   Show version information\n")
	fmt.Fprintf(os.Stderr, "\nFlags:\n")
	fmt.Fprintf(os.Stderr, "  --config path   Config file (default: %s)\n", defaultConfigPath())
	fmt.Fprintf(os.Stderr, "  --port N        Server port (default: 9090, serve only)\n")
	fmt.Fprintf(os.Stderr, "  --force         Force full resync of all sessions (sync only)\n")
	fmt.Fprintf(os.Stderr, "  --reset-db      Drop and recreate the ClickHouse database (sync only)\n")
}

func defaultConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "agentlore", "config.json")
}

func loadConfig(configPath string) (*config.Config, bool) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config from %s: %v\n", configPath, err)
		return nil, false
	}

	// Apply defaults for unset fields
	if cfg.AgentsviewDBPath == "" {
		cfg.AgentsviewDBPath = config.DefaultAgentsviewDBPath()
	}
	if cfg.DataDir == "" {
		cfg.DataDir = config.DefaultDataDir()
	}

	return cfg, true
}

func clickhouseAddr(cfg *config.Config) string {
	if cfg.ClickHouseAddr != "" {
		return cfg.ClickHouseAddr
	}
	return "localhost:9000"
}

func clickhouseDatabase(cfg *config.Config) string {
	if cfg.ClickHouseDatabase != "" {
		return cfg.ClickHouseDatabase
	}
	return "agentlore"
}

func openStore(cfg *config.Config) (*store.ClickHouseStore, error) {
	return store.NewClickHouseStoreFromOptions(store.ConnectOptions{
		Addr:     clickhouseAddr(cfg),
		Database: clickhouseDatabase(cfg),
		User:     cfg.ClickHouseUser,
		Password: cfg.ClickHousePassword,
		Secure:   cfg.ClickHouseSecure,
	})
}

func validateSyncConfig(cfg *config.Config, configPath string) bool {
	if cfg.AgentsviewDBPath == "" {
		fmt.Fprintf(os.Stderr, "Error: agentsview_db_path is required for sync\n")
		fmt.Fprintf(os.Stderr, "\nSet agentsview_db_path in %s or ensure ~/.claude/agentsview/sessions.db exists.\n", configPath)
		return false
	}

	if cfg.DataDir == "" {
		fmt.Fprintf(os.Stderr, "Error: could not determine data_dir\n")
		return false
	}

	return true
}

func runSync(configPath string, force, resetDB bool) int {
	cfg, ok := loadConfig(configPath)
	if !ok {
		return 1
	}
	if !validateSyncConfig(cfg, configPath) {
		return 1
	}

	userID, userName := cfg.ResolvedUserIdentity()
	log.Printf("agentlore sync starting")
	log.Printf("  user: %s <%s>", userName, userID)
	log.Printf("  agentsview db: %s", cfg.AgentsviewDBPath)
	log.Printf("  clickhouse: %s", clickhouseAddr(cfg))

	r, err := reader.NewReader(cfg.AgentsviewDBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating reader: %v\n", err)
		return 1
	}
	defer func() { _ = r.Close() }()

	s, err := openStore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating store: %v\n", err)
		return 1
	}
	defer func() { _ = s.Close() }()

	if resetDB {
		log.Printf("agentlore sync: --reset-db specified, dropping and recreating database")
		if err := s.ResetDatabase(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "Error resetting database: %v\n", err)
			return 1
		}
		force = true // reset-db implies force resync
	} else {
		if err := s.EnsureSchema(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "Error ensuring schema: %v\n", err)
			return 1
		}
	}

	engine, err := astSync.NewEngine(cfg, r, s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating engine: %v\n", err)
		return 1
	}

	if force {
		log.Printf("agentlore sync: --force specified, resetting watermark")
		engine.ForceResync()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		cancel()
	}()

	result, err := engine.RunOnce(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sync error: %v\n", err)
		return 1
	}

	log.Printf("agentlore sync done: %d synced, %d skipped, %d secrets, %d errors",
		result.SessionsSynced, result.SessionsSkipped, result.SecretsDetected, len(result.Errors))

	if len(result.Errors) > 0 {
		shown := 0
		for sessionID, err := range result.Errors {
			log.Printf("  error [%s]: %v", sessionID, err)
			shown++
			if shown >= 5 {
				log.Printf("  ... and %d more errors", len(result.Errors)-shown)
				break
			}
		}
	}

	return 0
}

func runDaemon(configPath string) int {
	cfg, ok := loadConfig(configPath)
	if !ok {
		return 1
	}
	if !validateSyncConfig(cfg, configPath) {
		return 1
	}

	userID, userName := cfg.ResolvedUserIdentity()
	log.Printf("agentlore daemon starting")
	log.Printf("  user: %s <%s>", userName, userID)
	log.Printf("  agentsview db: %s", cfg.AgentsviewDBPath)
	log.Printf("  clickhouse: %s", clickhouseAddr(cfg))

	d, err := astSync.NewDaemon(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating daemon: %v\n", err)
		return 1
	}
	defer func() { _ = d.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		cancel()
	}()

	if err := d.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		return 1
	}

	status := d.Status()
	log.Printf("agentlore daemon stopped")
	log.Printf("  total sessions synced: %d", status.TotalSessionsSynced)
	log.Printf("  total secrets detected: %d", status.TotalSecretsDetected)

	return 0
}

func runServe(configPath string, portOverride int) int {
	cfg, ok := loadConfig(configPath)
	if !ok {
		return 1
	}

	log.Printf("agentlore serve starting")
	log.Printf("  clickhouse: %s", clickhouseAddr(cfg))

	s, err := openStore(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating store: %v\n", err)
		return 1
	}
	defer func() { _ = s.Close() }()

	apiServer := api.New(s)

	port := portOverride
	if port == 0 {
		port = cfg.ServerPort
	}
	if port == 0 {
		port = 9090
	}
	addr := fmt.Sprintf(":%d", port)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: apiServer,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	httpDone := make(chan error, 1)
	go func() {
		log.Printf("agentlore server listening on %s", addr)
		if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
			httpDone <- err
		}
		close(httpDone)
	}()

	select {
	case sig := <-sigCh:
		log.Printf("received signal %v, shutting down...", sig)
	case err := <-httpDone:
		if err != nil {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Printf("agentlore server stopped")

	return 0
}
