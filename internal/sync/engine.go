// ABOUTME: Core sync orchestration: read-mask-write pipeline with incremental message append.
// ABOUTME: Composes reader, secrets, and store into an idempotent sync engine with watermark tracking.
package sync

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/clkao/agentstrove/internal/config"
	"github.com/clkao/agentstrove/internal/gitlinks"
	"github.com/clkao/agentstrove/internal/reader"
	"github.com/clkao/agentstrove/internal/secrets"
	"github.com/clkao/agentstrove/internal/store"
)

// SyncVersion is incremented when the sync logic changes in a way that requires
// a full re-sync of all sessions.
const SyncVersion = 1

// SyncResult reports what happened during a single RunOnce invocation.
type SyncResult struct {
	SessionsSynced  int
	SessionsSkipped int
	SecretsDetected int
	Errors          map[string]error
}

// Engine orchestrates the read-mask-write sync pipeline.
type Engine struct {
	reader    *reader.Reader
	store     store.Store
	config    *config.Config
	state     *SyncState
	statePath string
}

// NewEngine creates a sync engine. It loads the watermark state from the config
// DataDir, or initializes empty state if no file exists.
func NewEngine(cfg *config.Config, r *reader.Reader, st store.Store) (*Engine, error) {
	statePath := filepath.Join(cfg.DataDir, "sync-state.json")

	state, err := LoadSyncState(statePath)
	if err != nil {
		return nil, fmt.Errorf("load sync state: %w", err)
	}

	return &Engine{
		reader:    r,
		store:     st,
		config:    cfg,
		state:     state,
		statePath: statePath,
	}, nil
}

// ForceResync resets the watermark state so the next RunOnce will re-sync all sessions.
func (e *Engine) ForceResync() {
	e.state.ResetForResync(SyncVersion)
}

// RunOnce executes one sync cycle: reads changed sessions, masks secrets, writes
// to the store, and advances the watermark. Returns a result summarizing what happened.
func (e *Engine) RunOnce(ctx context.Context) (*SyncResult, error) {
	result := &SyncResult{Errors: make(map[string]error)}

	// Full resync if the sync version has changed.
	if e.state.NeedsFullResync(SyncVersion) {
		e.state.ResetForResync(SyncVersion)
	}

	sessions, err := e.reader.ReadSessionsSince("")
	if err != nil {
		return nil, fmt.Errorf("read sessions: %w", err)
	}

	userID, userName := e.config.ResolvedUserIdentity()

	for _, sess := range sessions {
		if !e.state.IsSessionChanged(sess.ID, sess.FileHash) {
			result.SessionsSkipped++
			continue
		}

		if err := e.syncSession(ctx, sess, userID, userName, result); err != nil {
			result.Errors[sess.ID] = err
			continue
		}

		result.SessionsSynced++
	}

	if err := e.state.Save(e.statePath); err != nil {
		return result, fmt.Errorf("save sync state: %w", err)
	}

	return result, nil
}

// syncSession handles one session: reads only new messages and tool calls
// (ordinal > lastOrdinal), masks secrets, converts to store types, and writes.
// The session row is always re-written to refresh metadata.
func (e *Engine) syncSession(ctx context.Context, sess reader.Session, userID, userName string, result *SyncResult) error {
	lastOrdinal := e.state.GetLastOrdinal(sess.ID)

	allMsgs, err := e.reader.ReadMessagesForSession(sess.ID)
	if err != nil {
		return fmt.Errorf("read messages for %s: %w", sess.ID, err)
	}

	allTCs, err := e.reader.ReadToolCallsForSession(sess.ID)
	if err != nil {
		return fmt.Errorf("read tool calls for %s: %w", sess.ID, err)
	}

	// Filter to only messages and tool calls not yet synced.
	var newMsgs []reader.Message
	for _, m := range allMsgs {
		if m.Ordinal > lastOrdinal {
			newMsgs = append(newMsgs, m)
		}
	}

	var newTCs []reader.ToolCall
	for _, tc := range allTCs {
		if tc.MessageOrdinal > lastOrdinal {
			newTCs = append(newTCs, tc)
		}
	}

	projectID, projectName := config.ResolveProjectIdentity(sess.Project)

	storeSession := store.Session{
		OrgID:            "",
		ID:               sess.ID,
		UserID:           userID,
		UserName:         userName,
		ProjectID:        projectID,
		ProjectName:      projectName,
		ProjectPath:      sanitizeUTF8(sess.Project),
		Machine:          sanitizeUTF8(sess.Machine),
		AgentType:        sanitizeUTF8(sess.Agent),
		FirstMessage:     sanitizeUTF8(sess.FirstMessage),
		StartedAt:        parseTimestamp(sess.StartedAt),
		EndedAt:          parseTimestamp(sess.EndedAt),
		MessageCount:     sess.MessageCount,
		UserMessageCount: sess.UserMessageCount,
		ParentSessionID:  sess.ParentSessionID,
		RelationshipType: sess.RelationshipType,
		SourceCreatedAt:  sess.CreatedAt,
	}

	storeMsgs := make([]store.Message, len(newMsgs))
	for i, m := range newMsgs {
		masked := secrets.MaskSecrets(sanitizeUTF8(m.Content))
		result.SecretsDetected += masked.SecretCount
		e.state.TotalMasked += int64(masked.SecretCount)

		storeMsgs[i] = store.Message{
			OrgID:         "",
			SessionID:     m.SessionID,
			Ordinal:       m.Ordinal,
			Role:          m.Role,
			Content:       masked.Masked,
			Timestamp:     parseTimestamp(m.Timestamp),
			HasThinking:   m.HasThinking,
			HasToolUse:    m.HasToolUse,
			ContentLength: m.ContentLength,
		}
	}

	storeTCs := make([]store.ToolCall, len(newTCs))
	for i, tc := range newTCs {
		maskedInput := secrets.MaskSecrets(sanitizeUTF8(tc.InputJSON))
		result.SecretsDetected += maskedInput.SecretCount
		e.state.TotalMasked += int64(maskedInput.SecretCount)

		maskedResult := secrets.MaskSecrets(sanitizeUTF8(tc.ResultContent))
		result.SecretsDetected += maskedResult.SecretCount
		e.state.TotalMasked += int64(maskedResult.SecretCount)

		var rcl *int
		if tc.ResultContentLength > 0 {
			v := tc.ResultContentLength
			rcl = &v
		}

		storeTCs[i] = store.ToolCall{
			OrgID:               "",
			MessageOrdinal:      tc.MessageOrdinal,
			SessionID:           tc.SessionID,
			ToolName:            tc.ToolName,
			Category:            tc.Category,
			ToolUseID:           tc.ToolUseID,
			InputJSON:           maskedInput.Masked,
			SkillName:           tc.SkillName,
			ResultContentLength: rcl,
			ResultContent:       maskedResult.Masked,
			SubagentSessionID:   tc.SubagentSessionID,
		}
	}

	if err := e.store.WriteSession(ctx, "", storeSession, storeMsgs, storeTCs); err != nil {
		return fmt.Errorf("write session %s: %w", sess.ID, err)
	}

	// Extract git links from new tool calls (non-fatal on error).
	links := gitlinks.ExtractGitLinks(storeTCs, storeMsgs)
	if len(links) > 0 {
		// Set OrgID on each link before writing.
		for i := range links {
			links[i].OrgID = ""
			links[i].UserID = userID
		}
		if err := e.store.WriteGitLinks(ctx, "", links); err != nil {
			log.Printf("sync: git link write error for %s: %v", sess.ID, err)
		}
	}

	// Compute max ordinal of new messages for watermark advance.
	maxOrdinal := lastOrdinal
	for _, m := range newMsgs {
		if m.Ordinal > maxOrdinal {
			maxOrdinal = m.Ordinal
		}
	}

	e.state.MarkSynced(sess.ID, sess.FileHash, maxOrdinal, sess.CreatedAt)
	e.state.TotalSynced++

	return nil
}

// sanitizeUTF8 replaces invalid UTF-8 sequences with the Unicode replacement character.
// ClickHouse String columns require valid UTF-8.
func sanitizeUTF8(s string) string {
	return strings.ToValidUTF8(s, "\uFFFD")
}

// parseTimestamp converts an ISO 8601 string to *time.Time. Returns nil for empty strings.
func parseTimestamp(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}
