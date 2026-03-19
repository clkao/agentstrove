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

	"github.com/clkao/agentlore/internal/config"
	"github.com/clkao/agentlore/internal/gitlinks"
	"github.com/clkao/agentlore/internal/reader"
	"github.com/clkao/agentlore/internal/secrets"
	"github.com/clkao/agentlore/internal/store"
)

// SyncVersion is incremented when the sync logic changes in a way that requires
// a full re-sync of all sessions.
const SyncVersion = 4

const syncChunkSize = 50

// preparedSession holds the transformed data for one session, ready for batch writing.
type preparedSession struct {
	readerSession reader.Session
	storeSession  store.Session
	messages      []store.Message
	toolCalls     []store.ToolCall
	gitLinks      []store.GitLink
	maxOrdinal    int
	secretCount   int
	err           error
}

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

	if e.state.NeedsFullResync(SyncVersion) {
		e.state.ResetForResync(SyncVersion)
	}

	sessions, err := e.reader.ReadSessionsSince("")
	if err != nil {
		return nil, fmt.Errorf("read sessions: %w", err)
	}

	// Classify: split into changed vs unchanged
	var changed []reader.Session
	for _, sess := range sessions {
		if e.state.IsSessionChanged(sess.ID, sess.FileHash) {
			changed = append(changed, sess)
		} else {
			result.SessionsSkipped++
		}
	}

	log.Printf("sync: %d changed, %d unchanged out of %d sessions", len(changed), result.SessionsSkipped, len(sessions))

	if len(changed) == 0 {
		if err := e.syncUserActions(ctx); err != nil {
			log.Printf("sync: user actions: %v", err)
		}
		if err := e.state.Save(e.statePath); err != nil {
			return result, fmt.Errorf("save sync state: %w", err)
		}
		return result, nil
	}

	userID, userName := e.config.ResolvedUserIdentity()
	projectCache := make(map[string][2]string)

	// Process in chunks
	for i := 0; i < len(changed); i += syncChunkSize {
		end := i + syncChunkSize
		if end > len(changed) {
			end = len(changed)
		}
		chunk := changed[i:end]

		if err := e.syncChunk(ctx, chunk, userID, userName, projectCache, result); err != nil {
			return result, fmt.Errorf("sync chunk: %w", err)
		}

		// Save watermark after each chunk for partial progress
		if err := e.state.Save(e.statePath); err != nil {
			return result, fmt.Errorf("save sync state: %w", err)
		}

		log.Printf("sync: %d/%d sessions synced", result.SessionsSynced, len(changed))
	}

	if err := e.syncUserActions(ctx); err != nil {
		log.Printf("sync: user actions: %v", err)
	}

	return result, nil
}

// syncChunk writes a batch of prepared sessions to the store in a single WriteBatch call,
// then advances watermarks for successfully written sessions.
func (e *Engine) syncChunk(ctx context.Context, chunk []reader.Session, userID, userName string, projectCache map[string][2]string, result *SyncResult) error {
	// Prepare all sessions in chunk
	prepared := make([]preparedSession, len(chunk))
	for i, sess := range chunk {
		prepared[i] = e.prepareSession(sess, userID, userName, projectCache)
	}

	// Collect successful preparations into flat slices
	var allSessions []store.Session
	var allMessages []store.Message
	var allToolCalls []store.ToolCall
	var allGitLinks []store.GitLink

	for _, p := range prepared {
		if p.err != nil {
			result.Errors[p.readerSession.ID] = p.err
			continue
		}
		allSessions = append(allSessions, p.storeSession)
		allMessages = append(allMessages, p.messages...)
		allToolCalls = append(allToolCalls, p.toolCalls...)
		allGitLinks = append(allGitLinks, p.gitLinks...)
	}

	// Batch write
	if len(allSessions) > 0 {
		if err := e.store.WriteBatch(ctx, "", allSessions, allMessages, allToolCalls); err != nil {
			// Batch failed — record error for all sessions in this chunk
			for _, p := range prepared {
				if p.err == nil {
					result.Errors[p.readerSession.ID] = fmt.Errorf("batch write: %w", err)
				}
			}
			return nil // don't abort the whole sync, just skip this chunk
		}
	}

	// Git links (already accepts cross-session slices)
	if len(allGitLinks) > 0 {
		if err := e.store.WriteGitLinks(ctx, "", allGitLinks); err != nil {
			log.Printf("sync: batch git link write error: %v", err)
		}
	}

	// Advance watermarks only for successfully written sessions
	for _, p := range prepared {
		if p.err != nil {
			continue
		}
		if _, hasErr := result.Errors[p.readerSession.ID]; hasErr {
			continue
		}
		e.state.MarkSynced(p.readerSession.ID, p.readerSession.FileHash, p.maxOrdinal, p.readerSession.CreatedAt)
		e.state.TotalSynced++
		e.state.TotalMasked += int64(p.secretCount)
		result.SecretsDetected += p.secretCount
		result.SessionsSynced++
	}

	return nil
}

// prepareSession reads and transforms one session's data for batch writing.
// Does not write to the store or mutate sync state.
func (e *Engine) prepareSession(sess reader.Session, userID, userName string, projectCache map[string][2]string) preparedSession {
	lastOrdinal := e.state.GetLastOrdinal(sess.ID)

	allMsgs, err := e.reader.ReadMessagesForSession(sess.ID)
	if err != nil {
		return preparedSession{readerSession: sess, err: fmt.Errorf("read messages for %s: %w", sess.ID, err)}
	}

	allTCs, err := e.reader.ReadToolCallsForSession(sess.ID)
	if err != nil {
		return preparedSession{readerSession: sess, err: fmt.Errorf("read tool calls for %s: %w", sess.ID, err)}
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

	var projectID, projectName string
	if cached, ok := projectCache[sess.Project]; ok {
		projectID, projectName = cached[0], cached[1]
	} else {
		projectID, projectName = config.ResolveProjectIdentity(sess.Project)
		projectCache[sess.Project] = [2]string{projectID, projectName}
	}

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
		DisplayName:      sanitizeUTF8(sess.DisplayName),
		TotalOutputTokens: sess.TotalOutputTokens,
		PeakContextTokens: sess.PeakContextTokens,
		SourceCreatedAt:  sess.CreatedAt,
	}

	secretCount := 0

	storeMsgs := make([]store.Message, len(newMsgs))
	for i, m := range newMsgs {
		masked := secrets.MaskSecrets(sanitizeUTF8(m.Content))
		secretCount += masked.SecretCount
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
			Model:         sanitizeUTF8(m.Model),
			TokenUsage:    sanitizeUTF8(m.TokenUsage),
			ContextTokens: m.ContextTokens,
			OutputTokens:  m.OutputTokens,
		}
	}

	storeTCs := make([]store.ToolCall, len(newTCs))
	for i, tc := range newTCs {
		maskedInput := secrets.MaskSecrets(sanitizeUTF8(tc.InputJSON))
		secretCount += maskedInput.SecretCount
		maskedResult := secrets.MaskSecrets(sanitizeUTF8(tc.ResultContent))
		secretCount += maskedResult.SecretCount

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

	// Extract git links
	links := gitlinks.ExtractGitLinks(storeTCs, storeMsgs)
	for i := range links {
		links[i].OrgID = ""
		links[i].UserID = userID
	}

	maxOrdinal := lastOrdinal
	for _, m := range newMsgs {
		if m.Ordinal > maxOrdinal {
			maxOrdinal = m.Ordinal
		}
	}

	return preparedSession{
		readerSession: sess,
		storeSession:  storeSession,
		messages:      storeMsgs,
		toolCalls:     storeTCs,
		gitLinks:      links,
		maxOrdinal:    maxOrdinal,
		secretCount:   secretCount,
	}
}

// syncUserActions reads stars, pins, and deleted sessions from the reader and
// writes them to the store. This is a full-replace sync each cycle.
func (e *Engine) syncUserActions(ctx context.Context) error {
	userID, _ := e.config.ResolvedUserIdentity()

	// Stars
	starIDs, err := e.reader.ReadStarredSessionIDs()
	if err != nil {
		return fmt.Errorf("read starred sessions: %w", err)
	}
	if len(starIDs) > 0 {
		now := time.Now().UTC()
		stars := make([]store.SessionStar, len(starIDs))
		for i, id := range starIDs {
			stars[i] = store.SessionStar{
				SessionID: id,
				UserID:    userID,
				CreatedAt: now,
			}
		}
		if err := e.store.WriteSessionStars(ctx, "", stars); err != nil {
			return fmt.Errorf("write session stars: %w", err)
		}
		log.Printf("sync: %d starred sessions", len(stars))
	}

	// Pins
	readerPins, err := e.reader.ReadPinnedMessages()
	if err != nil {
		return fmt.Errorf("read pinned messages: %w", err)
	}
	if len(readerPins) > 0 {
		pins := make([]store.MessagePin, len(readerPins))
		for i, p := range readerPins {
			pins[i] = store.MessagePin{
				SessionID:      p.SessionID,
				MessageOrdinal: p.MessageOrdinal,
				UserID:         userID,
				Note:           sanitizeUTF8(p.Note),
				CreatedAt:      parseTimestampOrNow(p.CreatedAt),
			}
		}
		if err := e.store.WriteMessagePins(ctx, "", pins); err != nil {
			return fmt.Errorf("write message pins: %w", err)
		}
		log.Printf("sync: %d pinned messages", len(pins))
	}

	// Deletes
	deleteIDs, err := e.reader.ReadDeletedSessionIDs()
	if err != nil {
		return fmt.Errorf("read deleted sessions: %w", err)
	}
	if len(deleteIDs) > 0 {
		now := time.Now().UTC()
		deletes := make([]store.SessionDelete, len(deleteIDs))
		for i, id := range deleteIDs {
			deletes[i] = store.SessionDelete{
				SessionID: id,
				UserID:    userID,
				CreatedAt: now,
			}
		}
		if err := e.store.WriteSessionDeletes(ctx, "", deletes); err != nil {
			return fmt.Errorf("write session deletes: %w", err)
		}
		log.Printf("sync: %d deleted sessions", len(deletes))
	}

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

// parseTimestampOrNow converts an ISO 8601 string to time.Time, falling back to now.
func parseTimestampOrNow(s string) time.Time {
	if t := parseTimestamp(s); t != nil {
		return *t
	}
	return time.Now().UTC()
}
