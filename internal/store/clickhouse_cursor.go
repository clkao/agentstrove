// ABOUTME: Cursor encoding and decoding for paginated ClickHouse queries.
// ABOUTME: Cursors are base64-encoded composite keys of timestamp and session ID.
package store

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// encodeCursor creates a cursor string from a timestamp and session ID.
func encodeCursor(t *time.Time, id string) string {
	ts := ""
	if t != nil {
		ts = t.Format(time.RFC3339Nano)
	}
	return base64.StdEncoding.EncodeToString([]byte(ts + "|" + id))
}

// decodeCursor parses a cursor string back into a timestamp string and session ID.
func decodeCursor(cursor string) (string, string, error) {
	data, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(string(data), "|", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed cursor")
	}
	return parts[0], parts[1], nil
}
