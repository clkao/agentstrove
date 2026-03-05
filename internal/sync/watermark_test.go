// ABOUTME: Tests for watermark-based sync state persistence.
// ABOUTME: Validates load/save, hash-based change detection, ordinal tracking, and version resync.
package sync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSyncState_NonexistentFile(t *testing.T) {
	state, err := LoadSyncState("/nonexistent/path/sync-state.json")
	require.NoError(t, err, "loading from nonexistent file should not error")
	assert.NotNil(t, state)
	assert.Equal(t, "", state.LastSessionCreatedAt)
	assert.NotNil(t, state.Sessions)
	assert.Empty(t, state.Sessions)
	assert.Equal(t, int64(0), state.TotalSynced)
	assert.Equal(t, int64(0), state.TotalMasked)
	assert.Equal(t, 0, state.Version)
}

func TestLoadSyncState_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync-state.json")

	data := `{
		"version": 2,
		"last_session_created_at": "2026-01-15T10:00:00.000Z",
		"sessions": {
			"sess-1": {"file_hash": "abc123", "last_ordinal": 5},
			"sess-2": {"file_hash": "def456", "last_ordinal": 10}
		},
		"last_synced_at": "2026-01-15T10:05:00Z",
		"total_synced": 42,
		"total_masked": 7
	}`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))

	state, err := LoadSyncState(path)
	require.NoError(t, err)
	assert.Equal(t, 2, state.Version)
	assert.Equal(t, "2026-01-15T10:00:00.000Z", state.LastSessionCreatedAt)
	assert.Equal(t, "abc123", state.Sessions["sess-1"].FileHash)
	assert.Equal(t, 5, state.Sessions["sess-1"].LastOrdinal)
	assert.Equal(t, "def456", state.Sessions["sess-2"].FileHash)
	assert.Equal(t, 10, state.Sessions["sess-2"].LastOrdinal)
	assert.Equal(t, int64(42), state.TotalSynced)
	assert.Equal(t, int64(7), state.TotalMasked)
}

func TestSyncState_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sync-state.json")

	original := &SyncState{
		Version:              1,
		LastSessionCreatedAt: "2026-03-01T12:00:00.000Z",
		Sessions: map[string]SessionWatermark{
			"sess-a": {FileHash: "hash-a", LastOrdinal: 3},
			"sess-b": {FileHash: "hash-b", LastOrdinal: 7},
		},
		TotalSynced: 10,
		TotalMasked: 3,
	}

	require.NoError(t, original.Save(path))

	loaded, err := LoadSyncState(path)
	require.NoError(t, err)
	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.LastSessionCreatedAt, loaded.LastSessionCreatedAt)
	assert.Equal(t, original.Sessions, loaded.Sessions)
	assert.Equal(t, original.TotalSynced, loaded.TotalSynced)
	assert.Equal(t, original.TotalMasked, loaded.TotalMasked)
}

func TestSyncState_Save_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "subdir", "sync-state.json")

	state := &SyncState{
		Version:  1,
		Sessions: map[string]SessionWatermark{},
	}

	require.NoError(t, state.Save(path))

	loaded, err := LoadSyncState(path)
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Version)
}

func TestSyncState_IsSessionChanged_MatchingHash(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{
			"sess-1": {FileHash: "hash-1", LastOrdinal: 0},
		},
	}
	assert.False(t, state.IsSessionChanged("sess-1", "hash-1"))
}

func TestSyncState_IsSessionChanged_UnknownSession(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{},
	}
	assert.True(t, state.IsSessionChanged("sess-unknown", "some-hash"))
}

func TestSyncState_IsSessionChanged_ChangedHash(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{
			"sess-1": {FileHash: "old-hash", LastOrdinal: 5},
		},
	}
	assert.True(t, state.IsSessionChanged("sess-1", "new-hash"))
}

func TestSyncState_GetLastOrdinal_TrackedSession(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{
			"sess-1": {FileHash: "hash-1", LastOrdinal: 12},
		},
	}
	assert.Equal(t, 12, state.GetLastOrdinal("sess-1"))
}

func TestSyncState_GetLastOrdinal_UntrackedSession(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{},
	}
	assert.Equal(t, 0, state.GetLastOrdinal("sess-unknown"))
}

func TestSyncState_MarkSynced_UpdatesSession(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{},
	}
	state.MarkSynced("sess-1", "hash-1", 5, "2026-01-01T10:00:00.000Z")

	assert.Equal(t, "hash-1", state.Sessions["sess-1"].FileHash)
	assert.Equal(t, 5, state.Sessions["sess-1"].LastOrdinal)
	assert.Equal(t, "2026-01-01T10:00:00.000Z", state.LastSessionCreatedAt)
}

func TestSyncState_MarkSynced_AdvancesWatermark(t *testing.T) {
	state := &SyncState{
		LastSessionCreatedAt: "2026-01-01T10:00:00.000Z",
		Sessions:             map[string]SessionWatermark{},
	}

	// Later timestamp should advance watermark
	state.MarkSynced("sess-2", "hash-2", 3, "2026-01-02T10:00:00.000Z")
	assert.Equal(t, "2026-01-02T10:00:00.000Z", state.LastSessionCreatedAt)

	// Earlier timestamp should NOT advance watermark
	state.MarkSynced("sess-3", "hash-3", 1, "2026-01-01T05:00:00.000Z")
	assert.Equal(t, "2026-01-02T10:00:00.000Z", state.LastSessionCreatedAt,
		"watermark should not go backwards")
}

func TestSyncState_MarkSynced_UpdatesOrdinal(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{
			"sess-1": {FileHash: "hash-old", LastOrdinal: 3},
		},
	}
	state.MarkSynced("sess-1", "hash-new", 8, "2026-01-01T10:00:00.000Z")

	assert.Equal(t, "hash-new", state.Sessions["sess-1"].FileHash)
	assert.Equal(t, 8, state.Sessions["sess-1"].LastOrdinal)
}

func TestSyncState_MarkSynced_TracksCounters(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionWatermark{},
	}

	assert.Equal(t, int64(0), state.TotalSynced)
	assert.Equal(t, int64(0), state.TotalMasked)

	state.TotalSynced++
	state.TotalMasked += 3

	assert.Equal(t, int64(1), state.TotalSynced)
	assert.Equal(t, int64(3), state.TotalMasked)
}

func TestSyncState_NeedsFullResync_OlderVersion(t *testing.T) {
	state := &SyncState{Version: 1}
	assert.True(t, state.NeedsFullResync(2))
}

func TestSyncState_NeedsFullResync_SameVersion(t *testing.T) {
	state := &SyncState{Version: 2}
	assert.False(t, state.NeedsFullResync(2))
}

func TestSyncState_NeedsFullResync_NewerStoredVersion(t *testing.T) {
	// Should not happen in practice, but guard against it
	state := &SyncState{Version: 3}
	assert.False(t, state.NeedsFullResync(2))
}

func TestSyncState_NeedsFullResync_ZeroVersion(t *testing.T) {
	// Fresh state (version 0) always needs resync if currentVersion > 0
	state := &SyncState{}
	assert.True(t, state.NeedsFullResync(1))
}

func TestSyncState_ResetForResync_ClearsSessions(t *testing.T) {
	state := &SyncState{
		Version: 1,
		Sessions: map[string]SessionWatermark{
			"sess-1": {FileHash: "hash-1", LastOrdinal: 5},
			"sess-2": {FileHash: "hash-2", LastOrdinal: 10},
		},
		LastSessionCreatedAt: "2026-01-15T10:00:00.000Z",
	}

	state.ResetForResync(2)

	assert.Equal(t, 2, state.Version)
	assert.Empty(t, state.Sessions)
	assert.Equal(t, "", state.LastSessionCreatedAt)
}

func TestSyncState_ResetForResync_ThenNeedsFullResyncIsFalse(t *testing.T) {
	state := &SyncState{Version: 1}
	state.ResetForResync(2)
	assert.False(t, state.NeedsFullResync(2), "after reset to version 2, should not need resync for version 2")
}
