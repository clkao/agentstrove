// ABOUTME: Write operations for the ClickHouse store.
// ABOUTME: Implements WriteSession, WriteBatch, and WriteGitLinks with batch inserts.
package store

import (
	"context"
	"fmt"
	"time"
)

// WriteSession inserts or replaces a session row and appends new messages and tool calls.
// ReplacingMergeTree deduplicates by keeping the highest _version.
func (s *ClickHouseStore) WriteSession(ctx context.Context, orgID string, session Session, messages []Message, toolCalls []ToolCall) error {
	version := uint64(time.Now().UnixMilli())

	// Insert session row
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO sessions")
	if err != nil {
		return fmt.Errorf("prepare session batch: %w", err)
	}
	if err := batch.Append(
		orgID,
		session.ID,
		session.UserID,
		session.UserName,
		session.ProjectID,
		session.ProjectName,
		session.ProjectPath,
		session.AgentType,
		session.FirstMessage,
		session.StartedAt,
		session.EndedAt,
		uint32(session.MessageCount),
		uint32(session.UserMessageCount),
		session.ParentSessionID,
		session.RelationshipType,
		session.Machine,
		session.SourceCreatedAt,
		session.DisplayName,
		uint32(session.TotalOutputTokens),
		uint32(session.PeakContextTokens),
		version,
	); err != nil {
		return fmt.Errorf("append session: %w", err)
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send session: %w", err)
	}

	// Insert messages
	if len(messages) > 0 {
		msgBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO messages")
		if err != nil {
			return fmt.Errorf("prepare messages batch: %w", err)
		}
		for _, m := range messages {
			if err := msgBatch.Append(
				orgID,
				m.SessionID,
				uint32(m.Ordinal),
				m.Role,
				m.Content,
				m.Timestamp,
				m.HasThinking,
				m.HasToolUse,
				uint32(m.ContentLength),
				m.Model,
				m.TokenUsage,
				uint32(m.ContextTokens),
				uint32(m.OutputTokens),
				version,
			); err != nil {
				return fmt.Errorf("append message ordinal %d: %w", m.Ordinal, err)
			}
		}
		if err := msgBatch.Send(); err != nil {
			return fmt.Errorf("send messages: %w", err)
		}
	}

	// Insert tool calls
	if len(toolCalls) > 0 {
		tcBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO tool_calls")
		if err != nil {
			return fmt.Errorf("prepare tool_calls batch: %w", err)
		}
		for _, tc := range toolCalls {
			var resultLen *uint32
			if tc.ResultContentLength != nil {
				v := uint32(*tc.ResultContentLength)
				resultLen = &v
			}
			if err := tcBatch.Append(
				orgID,
				tc.SessionID,
				uint32(tc.MessageOrdinal),
				tc.ToolUseID,
				tc.ToolName,
				tc.Category,
				tc.InputJSON,
				tc.SkillName,
				tc.ResultContent,
				resultLen,
				tc.SubagentSessionID,
				version,
			); err != nil {
				return fmt.Errorf("append tool_call: %w", err)
			}
		}
		if err := tcBatch.Send(); err != nil {
			return fmt.Errorf("send tool_calls: %w", err)
		}
	}

	return nil
}

// WriteBatch inserts multiple sessions with their messages and tool calls
// in combined batches — one PrepareBatch+Send per table.
func (s *ClickHouseStore) WriteBatch(ctx context.Context, orgID string, sessions []Session, messages []Message, toolCalls []ToolCall) error {
	if len(sessions) == 0 {
		return nil
	}
	version := uint64(time.Now().UnixMilli())

	// Sessions batch
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO sessions")
	if err != nil {
		return fmt.Errorf("prepare session batch: %w", err)
	}
	for _, session := range sessions {
		if err := batch.Append(
			orgID,
			session.ID,
			session.UserID,
			session.UserName,
			session.ProjectID,
			session.ProjectName,
			session.ProjectPath,
			session.AgentType,
			session.FirstMessage,
			session.StartedAt,
			session.EndedAt,
			uint32(session.MessageCount),
			uint32(session.UserMessageCount),
			session.ParentSessionID,
			session.RelationshipType,
			session.Machine,
			session.SourceCreatedAt,
			session.DisplayName,
			uint32(session.TotalOutputTokens),
			uint32(session.PeakContextTokens),
			version,
		); err != nil {
			return fmt.Errorf("append session %s: %w", session.ID, err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send sessions: %w", err)
	}

	// Messages batch
	if len(messages) > 0 {
		msgBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO messages")
		if err != nil {
			return fmt.Errorf("prepare messages batch: %w", err)
		}
		for _, m := range messages {
			if err := msgBatch.Append(
				orgID,
				m.SessionID,
				uint32(m.Ordinal),
				m.Role,
				m.Content,
				m.Timestamp,
				m.HasThinking,
				m.HasToolUse,
				uint32(m.ContentLength),
				m.Model,
				m.TokenUsage,
				uint32(m.ContextTokens),
				uint32(m.OutputTokens),
				version,
			); err != nil {
				return fmt.Errorf("append message: %w", err)
			}
		}
		if err := msgBatch.Send(); err != nil {
			return fmt.Errorf("send messages: %w", err)
		}
	}

	// Tool calls batch
	if len(toolCalls) > 0 {
		tcBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO tool_calls")
		if err != nil {
			return fmt.Errorf("prepare tool_calls batch: %w", err)
		}
		for _, tc := range toolCalls {
			var resultLen *uint32
			if tc.ResultContentLength != nil {
				v := uint32(*tc.ResultContentLength)
				resultLen = &v
			}
			if err := tcBatch.Append(
				orgID,
				tc.SessionID,
				uint32(tc.MessageOrdinal),
				tc.ToolUseID,
				tc.ToolName,
				tc.Category,
				tc.InputJSON,
				tc.SkillName,
				tc.ResultContent,
				resultLen,
				tc.SubagentSessionID,
				version,
			); err != nil {
				return fmt.Errorf("append tool_call: %w", err)
			}
		}
		if err := tcBatch.Send(); err != nil {
			return fmt.Errorf("send tool_calls: %w", err)
		}
	}

	return nil
}

// WriteSessionStars inserts session star records into the session_stars table.
func (s *ClickHouseStore) WriteSessionStars(ctx context.Context, orgID string, stars []SessionStar) error {
	if len(stars) == 0 {
		return nil
	}
	version := uint64(time.Now().UnixMilli())
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO session_stars")
	if err != nil {
		return fmt.Errorf("prepare session_stars batch: %w", err)
	}
	for _, star := range stars {
		if err := batch.Append(orgID, star.SessionID, star.UserID, star.CreatedAt, version); err != nil {
			return fmt.Errorf("append session_star: %w", err)
		}
	}
	return batch.Send()
}

// WriteMessagePins inserts message pin records into the message_pins table.
func (s *ClickHouseStore) WriteMessagePins(ctx context.Context, orgID string, pins []MessagePin) error {
	if len(pins) == 0 {
		return nil
	}
	version := uint64(time.Now().UnixMilli())
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO message_pins")
	if err != nil {
		return fmt.Errorf("prepare message_pins batch: %w", err)
	}
	for _, pin := range pins {
		if err := batch.Append(orgID, pin.SessionID, uint32(pin.MessageOrdinal), pin.UserID, pin.Note, pin.CreatedAt, version); err != nil {
			return fmt.Errorf("append message_pin: %w", err)
		}
	}
	return batch.Send()
}

// WriteSessionDeletes inserts session delete records into the session_deletes table.
func (s *ClickHouseStore) WriteSessionDeletes(ctx context.Context, orgID string, deletes []SessionDelete) error {
	if len(deletes) == 0 {
		return nil
	}
	version := uint64(time.Now().UnixMilli())
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO session_deletes")
	if err != nil {
		return fmt.Errorf("prepare session_deletes batch: %w", err)
	}
	for _, del := range deletes {
		if err := batch.Append(orgID, del.SessionID, del.UserID, del.CreatedAt, version); err != nil {
			return fmt.Errorf("append session_delete: %w", err)
		}
	}
	return batch.Send()
}

// WriteGitLinks inserts git link records into the git_links table.
func (s *ClickHouseStore) WriteGitLinks(ctx context.Context, orgID string, links []GitLink) error {
	if len(links) == 0 {
		return nil
	}
	version := uint64(time.Now().UnixMilli())
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO git_links")
	if err != nil {
		return fmt.Errorf("prepare git_links batch: %w", err)
	}
	now := time.Now().UTC()
	for _, link := range links {
		if err := batch.Append(
			orgID,
			link.SessionID,
			link.UserID,
			uint32(link.MessageOrdinal),
			link.CommitSHA,
			link.PRURL,
			link.LinkType,
			link.Confidence,
			now,
			version,
		); err != nil {
			return fmt.Errorf("append git_link: %w", err)
		}
	}
	if err := batch.Send(); err != nil {
		return fmt.Errorf("send git_links: %w", err)
	}
	return nil
}
