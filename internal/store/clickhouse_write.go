// ABOUTME: Write operations for the ClickHouse store.
// ABOUTME: Implements WriteSession and WriteGitLinks with batch inserts.
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
