// ABOUTME: Read operations for the ClickHouse store.
// ABOUTME: Implements all ReadStore query methods: sessions, messages, tool calls, users, projects, agents, and git links.
package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// sessionRow is the scan target for session queries.
type sessionRow struct {
	OrgID            string     `ch:"org_id"`
	ID               string     `ch:"id"`
	UserID           string     `ch:"user_id"`
	UserName         string     `ch:"user_name"`
	ProjectID        string     `ch:"project_id"`
	ProjectName      string     `ch:"project_name"`
	ProjectPath      string     `ch:"project_path"`
	Machine          string     `ch:"machine"`
	AgentType        string     `ch:"agent_type"`
	FirstMessage     string     `ch:"first_message"`
	StartedAt        *time.Time `ch:"started_at"`
	EndedAt          *time.Time `ch:"ended_at"`
	MessageCount     uint32     `ch:"message_count"`
	UserMessageCount uint32     `ch:"user_message_count"`
	ParentSessionID  string     `ch:"parent_session_id"`
	RelationshipType string     `ch:"relationship_type"`
	SourceCreatedAt  string     `ch:"source_created_at"`
	CommitCount      uint64     `ch:"commit_count"`
}

func sessionRowToSession(r sessionRow) Session {
	return Session{
		OrgID:            r.OrgID,
		ID:               r.ID,
		UserID:           r.UserID,
		UserName:         r.UserName,
		ProjectID:        r.ProjectID,
		ProjectName:      r.ProjectName,
		ProjectPath:      r.ProjectPath,
		Machine:          r.Machine,
		AgentType:        r.AgentType,
		FirstMessage:     r.FirstMessage,
		StartedAt:        r.StartedAt,
		EndedAt:          r.EndedAt,
		MessageCount:     int(r.MessageCount),
		UserMessageCount: int(r.UserMessageCount),
		ParentSessionID:  r.ParentSessionID,
		RelationshipType: r.RelationshipType,
		SourceCreatedAt:  r.SourceCreatedAt,
		CommitCount:      int(r.CommitCount),
	}
}

// sessionSelectCols selects all session fields plus commit_count via LEFT JOIN.
// Queries using this constant must JOIN git link counts as "glc" via sessionGitLinkJoin.
const sessionSelectCols = `s.org_id, s.id, s.user_id, s.user_name,
	s.project_id, s.project_name, s.project_path, s.machine,
	s.agent_type, s.first_message, s.started_at, s.ended_at,
	s.message_count, s.user_message_count,
	s.parent_session_id, s.relationship_type, s.source_created_at,
	ifNull(glc.commit_count, 0) AS commit_count`

// sessionGitLinkJoin is the LEFT JOIN fragment that provides per-session commit counts.
// The caller must supply the orgID as a query argument.
const sessionGitLinkJoin = `LEFT JOIN (
	SELECT session_id, count() AS commit_count
	FROM git_links FINAL
	WHERE org_id = ?
	GROUP BY session_id
) AS glc ON glc.session_id = s.id`

// ListSessions returns a cursor-paginated page of browsable sessions.
// The cursor encodes base64(started_at_rfc3339 + "|" + id).
func (s *ClickHouseStore) ListSessions(ctx context.Context, orgID string, filter SessionFilter) (*SessionPage, error) {
	var baseWhere []string
	var baseArgs []interface{}

	baseWhere = append(baseWhere, "s.org_id = ?")
	baseArgs = append(baseArgs, orgID)

	if !filter.IncludeSubagents {
		baseWhere = append(baseWhere, "s.parent_session_id = ''")
	}
	baseWhere = append(baseWhere, "s.user_message_count > 0")

	if filter.UserID != "" {
		baseWhere = append(baseWhere, "s.user_id = ?")
		baseArgs = append(baseArgs, filter.UserID)
	}
	if filter.ProjectID != "" {
		baseWhere = append(baseWhere, "s.project_id = ?")
		baseArgs = append(baseArgs, filter.ProjectID)
	}
	if filter.ProjectName != "" {
		baseWhere = append(baseWhere, "s.project_name = ?")
		baseArgs = append(baseArgs, filter.ProjectName)
	}
	if filter.AgentType != "" {
		baseWhere = append(baseWhere, "s.agent_type = ?")
		baseArgs = append(baseArgs, filter.AgentType)
	}
	if filter.DateFrom != "" {
		baseWhere = append(baseWhere, "s.started_at >= ?")
		baseArgs = append(baseArgs, filter.DateFrom)
	}
	if filter.DateTo != "" {
		// include the whole day of date_to
		baseWhere = append(baseWhere, "s.started_at < toDate(?) + 1")
		baseArgs = append(baseArgs, filter.DateTo)
	}

	// Count query uses base filters only (no cursor)
	countWhere := chWhereClause(baseWhere)
	countArgs := append([]interface{}{}, baseArgs...)
	var countRows []struct {
		Total uint64 `ch:"total"`
	}
	countQ := fmt.Sprintf("SELECT count() AS total FROM sessions AS s FINAL %s", countWhere)
	if err := s.conn.Select(ctx, &countRows, countQ, countArgs...); err != nil {
		return nil, fmt.Errorf("count sessions: %w", err)
	}
	var total int64
	if len(countRows) > 0 {
		total = int64(countRows[0].Total)
	}

	// Data query adds cursor condition
	dataWhere := append([]string{}, baseWhere...)
	dataArgs := append([]interface{}{}, baseArgs...)

	if filter.Cursor != "" {
		cursorAt, cursorID, err := decodeCursor(filter.Cursor)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
		}
		// Parse the RFC3339 timestamp from the cursor so we can pass a time.Time
		// (ClickHouse cannot implicitly cast RFC3339 strings to DateTime64)
		cursorTime, parseErr := time.Parse(time.RFC3339Nano, cursorAt)
		if parseErr != nil {
			cursorTime, parseErr = time.Parse(time.RFC3339, cursorAt)
			if parseErr != nil {
				return nil, fmt.Errorf("%w: bad timestamp: %v", ErrInvalidCursor, parseErr)
			}
		}
		dataWhere = append(dataWhere, "(s.started_at < ? OR (s.started_at = ? AND s.id < ?))")
		dataArgs = append(dataArgs, cursorTime, cursorTime, cursorID)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	dataQ := fmt.Sprintf(`SELECT %s
		FROM sessions AS s FINAL
		%s
		%s
		ORDER BY s.started_at DESC, s.id DESC
		LIMIT ?`,
		sessionSelectCols, sessionGitLinkJoin, chWhereClause(dataWhere))
	// orgID for git_links join subquery must come before WHERE args
	joinArgs := []interface{}{orgID}
	joinArgs = append(joinArgs, dataArgs...)
	joinArgs = append(joinArgs, limit+1)

	var rows []sessionRow
	if err := s.conn.Select(ctx, &rows, dataQ, joinArgs...); err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	sessions := make([]Session, 0, len(rows))
	for _, r := range rows {
		sessions = append(sessions, sessionRowToSession(r))
	}

	var nextCursor string
	if len(sessions) > limit {
		sessions = sessions[:limit]
		last := sessions[limit-1]
		nextCursor = encodeCursor(last.StartedAt, last.ID)
	}

	return &SessionPage{
		Sessions:   sessions,
		NextCursor: nextCursor,
		Total:      total,
	}, nil
}

// GetSession returns a single session by ID or an error containing "not found".
func (s *ClickHouseStore) GetSession(ctx context.Context, orgID string, id string) (*Session, error) {
	q := fmt.Sprintf(`SELECT %s
		FROM sessions AS s FINAL
		%s
		WHERE s.org_id = ? AND s.id = ?`, sessionSelectCols, sessionGitLinkJoin)

	var rows []sessionRow
	// orgID for git_links join subquery must come before WHERE args
	if err := s.conn.Select(ctx, &rows, q, orgID, orgID, id); err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("session %s: %w", id, ErrNotFound)
	}
	sess := sessionRowToSession(rows[0])
	return &sess, nil
}

// messageRow is the scan target for message queries.
type messageRow struct {
	OrgID         string     `ch:"org_id"`
	SessionID     string     `ch:"session_id"`
	Ordinal       uint32     `ch:"ordinal"`
	Role          string     `ch:"role"`
	Content       string     `ch:"content"`
	Timestamp     *time.Time `ch:"timestamp"`
	HasThinking   bool       `ch:"has_thinking"`
	HasToolUse    bool       `ch:"has_tool_use"`
	ContentLength uint32     `ch:"content_length"`
}

// GetSessionMessages returns all messages for a session ordered by ordinal ASC.
func (s *ClickHouseStore) GetSessionMessages(ctx context.Context, orgID string, sessionID string) ([]Message, error) {
	var rows []messageRow
	err := s.conn.Select(ctx, &rows,
		`SELECT org_id, session_id, ordinal, role, content,
		timestamp, has_thinking, has_tool_use, content_length
		FROM messages FINAL
		WHERE org_id = ? AND session_id = ?
		ORDER BY ordinal ASC`,
		orgID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session messages: %w", err)
	}

	messages := make([]Message, 0, len(rows))
	for _, r := range rows {
		messages = append(messages, Message{
			OrgID:         r.OrgID,
			SessionID:     r.SessionID,
			Ordinal:       int(r.Ordinal),
			Role:          r.Role,
			Content:       r.Content,
			Timestamp:     r.Timestamp,
			HasThinking:   r.HasThinking,
			HasToolUse:    r.HasToolUse,
			ContentLength: int(r.ContentLength),
		})
	}
	return messages, nil
}

// toolCallRow is the scan target for tool call queries.
type toolCallRow struct {
	OrgID               string  `ch:"org_id"`
	SessionID           string  `ch:"session_id"`
	MessageOrdinal      uint32  `ch:"message_ordinal"`
	ToolUseID           string  `ch:"tool_use_id"`
	ToolName            string  `ch:"tool_name"`
	ToolCategory        string  `ch:"tool_category"`
	InputJSON           string  `ch:"input_json"`
	SkillName           string  `ch:"skill_name"`
	ResultContent       string  `ch:"result_content"`
	ResultContentLength *uint32 `ch:"result_content_length"`
	SubagentSessionID   string  `ch:"subagent_session_id"`
}

// GetSessionToolCalls returns all tool calls for a session ordered by message_ordinal ASC, tool_use_id ASC.
func (s *ClickHouseStore) GetSessionToolCalls(ctx context.Context, orgID string, sessionID string) ([]ToolCall, error) {
	var rows []toolCallRow
	err := s.conn.Select(ctx, &rows,
		`SELECT org_id, session_id, message_ordinal, tool_use_id,
		tool_name, tool_category, input_json, skill_name,
		result_content, result_content_length, subagent_session_id
		FROM tool_calls FINAL
		WHERE org_id = ? AND session_id = ?
		ORDER BY message_ordinal ASC, tool_use_id ASC`,
		orgID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session tool calls: %w", err)
	}

	toolCalls := make([]ToolCall, 0, len(rows))
	for _, r := range rows {
		var rcl *int
		if r.ResultContentLength != nil {
			v := int(*r.ResultContentLength)
			rcl = &v
		}
		toolCalls = append(toolCalls, ToolCall{
			OrgID:               r.OrgID,
			SessionID:           r.SessionID,
			MessageOrdinal:      int(r.MessageOrdinal),
			ToolUseID:           r.ToolUseID,
			ToolName:            r.ToolName,
			Category:            r.ToolCategory,
			InputJSON:           r.InputJSON,
			SkillName:           r.SkillName,
			ResultContent:       r.ResultContent,
			ResultContentLength: rcl,
			SubagentSessionID:   r.SubagentSessionID,
		})
	}
	return toolCalls, nil
}

// userRow is the scan target for user queries.
type userRow struct {
	UserID   string `ch:"user_id"`
	UserName string `ch:"user_name"`
}

// ListUsers returns distinct user_id/user_name pairs from browsable sessions.
func (s *ClickHouseStore) ListUsers(ctx context.Context, orgID string) ([]UserInfo, error) {
	var rows []userRow
	err := s.conn.Select(ctx, &rows,
		`SELECT DISTINCT user_id, user_name
		FROM sessions FINAL
		WHERE org_id = ? AND parent_session_id = '' AND user_message_count > 0
		ORDER BY user_name`,
		orgID)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	users := make([]UserInfo, 0, len(rows))
	for _, r := range rows {
		users = append(users, UserInfo{ID: r.UserID, Name: r.UserName})
	}
	return users, nil
}

// projectRow is the scan target for project queries.
type projectRow struct {
	ProjectID   string `ch:"project_id"`
	ProjectName string `ch:"project_name"`
	ProjectPath string `ch:"project_path"`
}

// ListProjects returns distinct project info from browsable sessions with non-empty project_name.
// Groups by project_id+project_name to deduplicate across users who share a project but have
// different local paths.
func (s *ClickHouseStore) ListProjects(ctx context.Context, orgID string) ([]ProjectInfo, error) {
	var rows []projectRow
	err := s.conn.Select(ctx, &rows,
		`SELECT project_id, project_name, any(project_path) AS project_path
		FROM sessions FINAL
		WHERE org_id = ? AND parent_session_id = '' AND user_message_count > 0 AND project_name != ''
		GROUP BY project_id, project_name
		ORDER BY project_name`,
		orgID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projects := make([]ProjectInfo, 0, len(rows))
	for _, r := range rows {
		projects = append(projects, ProjectInfo{
			ID:   r.ProjectID,
			Name: r.ProjectName,
			Path: r.ProjectPath,
		})
	}
	return projects, nil
}

// ListAgents returns distinct non-empty agent types from browsable sessions.
func (s *ClickHouseStore) ListAgents(ctx context.Context, orgID string) ([]string, error) {
	var rows []struct {
		AgentType string `ch:"agent_type"`
	}
	err := s.conn.Select(ctx, &rows,
		`SELECT DISTINCT agent_type
		FROM sessions FINAL
		WHERE org_id = ? AND parent_session_id = '' AND user_message_count > 0 AND agent_type != ''
		ORDER BY agent_type`,
		orgID)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}

	agents := make([]string, 0, len(rows))
	for _, r := range rows {
		agents = append(agents, r.AgentType)
	}
	return agents, nil
}

// gitLinkResultRow is the scan target for git link lookup queries.
type gitLinkResultRow struct {
	SessionID      string     `ch:"session_id"`
	UserID         string     `ch:"user_id"`
	UserName       string     `ch:"user_name"`
	ProjectID      string     `ch:"project_id"`
	ProjectName    string     `ch:"project_name"`
	AgentType      string     `ch:"agent_type"`
	StartedAt      *time.Time `ch:"started_at"`
	FirstMessage   string     `ch:"first_message"`
	CommitSHA      string     `ch:"commit_sha"`
	PRURL          string     `ch:"pr_url"`
	LinkType       string     `ch:"link_type"`
	Confidence     string     `ch:"confidence"`
	MessageOrdinal uint32     `ch:"message_ordinal"`
}

// LookupGitLinks finds sessions by commit SHA prefix or PR URL.
func (s *ClickHouseStore) LookupGitLinks(ctx context.Context, orgID string, sha string, prURL string) ([]GitLinkResult, error) {
	var condition string
	var condArg interface{}

	if sha != "" {
		condition = "startsWith(gl.commit_sha, ?)"
		condArg = sha
	} else if prURL != "" {
		condition = "gl.pr_url = ?"
		condArg = prURL
	} else {
		return nil, fmt.Errorf("sha or prURL required")
	}

	q := fmt.Sprintf(`SELECT
		gl.session_id AS session_id, s.user_id AS user_id, s.user_name AS user_name,
		s.project_id AS project_id, s.project_name AS project_name, s.agent_type AS agent_type,
		s.started_at AS started_at, s.first_message AS first_message,
		gl.commit_sha AS commit_sha, gl.pr_url AS pr_url, gl.link_type AS link_type,
		gl.confidence AS confidence, gl.message_ordinal AS message_ordinal
		FROM git_links AS gl FINAL
		JOIN sessions AS s FINAL ON s.id = gl.session_id AND s.org_id = gl.org_id
		WHERE gl.org_id = ? AND %s
		ORDER BY gl.detected_at DESC`, condition)

	var rows []gitLinkResultRow
	if err := s.conn.Select(ctx, &rows, q, orgID, condArg); err != nil {
		return nil, fmt.Errorf("lookup git links: %w", err)
	}

	results := make([]GitLinkResult, 0, len(rows))
	for _, r := range rows {
		results = append(results, GitLinkResult{
			SessionID:      r.SessionID,
			UserID:         r.UserID,
			UserName:       r.UserName,
			ProjectID:      r.ProjectID,
			ProjectName:    r.ProjectName,
			AgentType:      r.AgentType,
			StartedAt:      r.StartedAt,
			FirstMessage:   r.FirstMessage,
			CommitSHA:      r.CommitSHA,
			PRURL:          r.PRURL,
			LinkType:       r.LinkType,
			Confidence:     r.Confidence,
			MessageOrdinal: int(r.MessageOrdinal),
		})
	}
	return results, nil
}

// chWhereClause joins conditions with AND and prepends WHERE, or returns empty string.
func chWhereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(conditions, " AND ")
}
