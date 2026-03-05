// ABOUTME: Tests for the sync engine pure functions and sync version logic.
// ABOUTME: Uses a fake store to verify session mapping; reader tests require CGO (skipped).
package sync

import (
	"context"
	"testing"
	"time"

	"github.com/clkao/agentstrove/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeStore records calls made by the engine for assertion in tests.
type fakeStore struct {
	sessions       []store.Session
	messages       []store.Message
	toolCalls      []store.ToolCall
	gitLinks       []store.GitLink
	batchCallCount int
	writeCallCount int
}

func (f *fakeStore) EnsureSchema(_ context.Context) error { return nil }

func (f *fakeStore) WriteSession(_ context.Context, _ string, sess store.Session, msgs []store.Message, tcs []store.ToolCall) error {
	f.writeCallCount++
	f.sessions = append(f.sessions, sess)
	f.messages = append(f.messages, msgs...)
	f.toolCalls = append(f.toolCalls, tcs...)
	return nil
}

func (f *fakeStore) WriteBatch(_ context.Context, _ string, sessions []store.Session, msgs []store.Message, tcs []store.ToolCall) error {
	f.batchCallCount++
	f.sessions = append(f.sessions, sessions...)
	f.messages = append(f.messages, msgs...)
	f.toolCalls = append(f.toolCalls, tcs...)
	return nil
}

func (f *fakeStore) WriteGitLinks(_ context.Context, _ string, links []store.GitLink) error {
	f.gitLinks = append(f.gitLinks, links...)
	return nil
}

func (f *fakeStore) Close() error { return nil }

func TestSanitizeUTF8(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"hello", "hello"},
		{"", ""},
		{"valid utf8: \u00e9", "valid utf8: \u00e9"},
		// Invalid UTF-8 byte sequences replaced with replacement chars.
		// \xff alone is one invalid sequence → one replacement char.
		{"bad\xffseq", "bad\uFFFDseq"},
		// \xff\xfe is a single maximal invalid sequence → one replacement char.
		{"bad\xff\xfeseq", "bad\uFFFDseq"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, sanitizeUTF8(tt.in), "input: %q", tt.in)
	}
}

func TestParseTimestamp(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		assert.Nil(t, parseTimestamp(""))
	})

	t.Run("RFC3339", func(t *testing.T) {
		ts := parseTimestamp("2024-01-15T10:30:00Z")
		require.NotNil(t, ts)
		assert.Equal(t, 2024, ts.Year())
		assert.Equal(t, time.January, ts.Month())
		assert.Equal(t, 15, ts.Day())
	})

	t.Run("millisecond format", func(t *testing.T) {
		ts := parseTimestamp("2024-03-20T14:22:05.000Z")
		require.NotNil(t, ts)
		assert.Equal(t, 2024, ts.Year())
		assert.Equal(t, time.March, ts.Month())
	})

	t.Run("unparseable returns nil", func(t *testing.T) {
		assert.Nil(t, parseTimestamp("not-a-date"))
	})
}

func TestSyncVersionResetsState(t *testing.T) {
	// Populate state with a prior session.
	state := &SyncState{
		Version: 0, // older than SyncVersion (1)
		Sessions: map[string]SessionWatermark{
			"sess-abc": {FileHash: "oldhash", LastOrdinal: 10},
		},
	}
	assert.True(t, state.NeedsFullResync(SyncVersion))

	state.ResetForResync(SyncVersion)

	assert.Equal(t, SyncVersion, state.Version)
	assert.Empty(t, state.Sessions)
	assert.False(t, state.NeedsFullResync(SyncVersion))
}

func TestSyncVersionNoResetWhenCurrent(t *testing.T) {
	state := &SyncState{
		Version: SyncVersion,
		Sessions: map[string]SessionWatermark{
			"sess-abc": {FileHash: "hash", LastOrdinal: 5},
		},
	}
	assert.False(t, state.NeedsFullResync(SyncVersion))
}

func TestFakeStoreImplementsInterface(t *testing.T) {
	// Compile-time check that fakeStore satisfies store.Store.
	var _ store.Store = &fakeStore{}
}
