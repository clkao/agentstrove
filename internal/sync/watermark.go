// ABOUTME: Watermark state persistence for idempotent sync.
// ABOUTME: Tracks which sessions have been synced via per-session hash and last ordinal.
package sync

import (
	"encoding/json"
	"os"
	"time"
)

// SessionWatermark holds the sync position for a single session.
type SessionWatermark struct {
	FileHash    string `json:"file_hash"`
	LastOrdinal int    `json:"last_ordinal"`
}

// SyncState tracks the sync position and per-session watermarks for idempotency.
type SyncState struct {
	Version              int                         `json:"version"`
	LastSessionCreatedAt string                      `json:"last_session_created_at"`
	Sessions             map[string]SessionWatermark `json:"sessions"`
	LastSyncedAt         time.Time                   `json:"last_synced_at"`
	TotalSynced          int64                       `json:"total_synced"`
	TotalMasked          int64                       `json:"total_masked"`
}

// LoadSyncState reads sync state from a JSON file. Returns an empty state if
// the file does not exist.
func LoadSyncState(path string) (*SyncState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncState{Sessions: make(map[string]SessionWatermark)}, nil
		}
		return nil, err
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	if state.Sessions == nil {
		state.Sessions = make(map[string]SessionWatermark)
	}
	return &state, nil
}

// Save writes the sync state to a JSON file.
func (s *SyncState) Save(path string) error {
	s.LastSyncedAt = time.Now().UTC()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// IsSessionChanged returns true if the session's file hash differs from the stored hash,
// or if the session has not been tracked. Used to detect sessions that need re-syncing.
func (s *SyncState) IsSessionChanged(sessionID, fileHash string) bool {
	stored, ok := s.Sessions[sessionID]
	return !ok || stored.FileHash != fileHash
}

// GetLastOrdinal returns the last synced ordinal for a session, or 0 if not tracked.
func (s *SyncState) GetLastOrdinal(sessionID string) int {
	return s.Sessions[sessionID].LastOrdinal
}

// MarkSynced records a session as synced with its file hash and last ordinal,
// and advances the watermark if createdAt is later than the current position.
func (s *SyncState) MarkSynced(sessionID, fileHash string, lastOrdinal int, createdAt string) {
	s.Sessions[sessionID] = SessionWatermark{
		FileHash:    fileHash,
		LastOrdinal: lastOrdinal,
	}
	if createdAt > s.LastSessionCreatedAt {
		s.LastSessionCreatedAt = createdAt
	}
}

// NeedsFullResync returns true if the stored version is older than currentVersion,
// indicating all sessions must be re-synced from scratch.
func (s *SyncState) NeedsFullResync(currentVersion int) bool {
	return s.Version < currentVersion
}

// ResetForResync clears all session watermarks and sets the new version,
// forcing a full re-sync on the next sync run.
func (s *SyncState) ResetForResync(newVersion int) {
	s.Version = newVersion
	s.Sessions = make(map[string]SessionWatermark)
	s.LastSessionCreatedAt = ""
}
