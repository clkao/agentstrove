// ABOUTME: Full-text search for the ClickHouse store.
// ABOUTME: Implements Search across messages and tool calls with snippet extraction and highlighting.
package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// searchResultRow is the scan target for search queries.
type searchResultRow struct {
	SessionID    string     `ch:"session_id"`
	Ordinal      uint32     `ch:"ordinal"`
	Role         string     `ch:"role"`
	Content      string     `ch:"content"`
	UserID       string     `ch:"user_id"`
	UserName     string     `ch:"user_name"`
	ProjectName  string     `ch:"project_name"`
	AgentType    string     `ch:"agent_type"`
	StartedAt    *time.Time `ch:"started_at"`
	FirstMessage string     `ch:"first_message"`
}

// Search performs case-insensitive substring search across messages and tool calls.
func (s *ClickHouseStore) Search(ctx context.Context, orgID string, query SearchQuery) (*SearchPage, error) {
	q := query.Query
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	// Build optional session filters for the JOIN
	var sessionFilters []string
	var sessionArgs []interface{}

	if query.UserID != "" {
		sessionFilters = append(sessionFilters, "s.user_id = ?")
		sessionArgs = append(sessionArgs, query.UserID)
	}
	if query.ProjectID != "" {
		sessionFilters = append(sessionFilters, "s.project_id = ?")
		sessionArgs = append(sessionArgs, query.ProjectID)
	}
	if query.ProjectName != "" {
		sessionFilters = append(sessionFilters, "s.project_name = ?")
		sessionArgs = append(sessionArgs, query.ProjectName)
	}
	if query.AgentType != "" {
		sessionFilters = append(sessionFilters, "s.agent_type = ?")
		sessionArgs = append(sessionArgs, query.AgentType)
	}
	if query.DateFrom != "" {
		sessionFilters = append(sessionFilters, "s.started_at >= ?")
		sessionArgs = append(sessionArgs, query.DateFrom)
	}
	if query.DateTo != "" {
		sessionFilters = append(sessionFilters, "s.started_at < toDate(?) + 1")
		sessionArgs = append(sessionArgs, query.DateTo)
	}

	// Always exclude ghost and subagent sessions from search results
	sessionFilters = append(sessionFilters, "s.parent_session_id = ''")
	sessionFilters = append(sessionFilters, "s.user_message_count > 0")

	sessionJoinCond := " AND " + strings.Join(sessionFilters, " AND ")

	// Search messages
	msgQ := fmt.Sprintf(`SELECT
		m.session_id, m.ordinal, m.role, m.content,
		s.user_id, s.user_name, s.project_name, s.agent_type, s.started_at, s.first_message
		FROM messages AS m FINAL
		JOIN sessions AS s FINAL ON s.id = m.session_id AND s.org_id = m.org_id
		WHERE m.org_id = ? AND positionCaseInsensitive(m.content, ?) > 0%s
		ORDER BY s.started_at DESC
		LIMIT ?`, sessionJoinCond)

	msgArgs := []interface{}{orgID, q}
	msgArgs = append(msgArgs, sessionArgs...)
	msgArgs = append(msgArgs, limit)

	var msgRows []searchResultRow
	if err := s.conn.Select(ctx, &msgRows, msgQ, msgArgs...); err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}

	// Search tool calls (input_json and result_content)
	tcQ := fmt.Sprintf(`SELECT
		tc.session_id, tc.message_ordinal AS ordinal, 'tool' AS role,
		if(positionCaseInsensitive(tc.input_json, ?) > 0, tc.input_json, tc.result_content) AS content,
		s.user_id, s.user_name, s.project_name, s.agent_type, s.started_at, s.first_message
		FROM tool_calls AS tc FINAL
		JOIN sessions AS s FINAL ON s.id = tc.session_id AND s.org_id = tc.org_id
		WHERE tc.org_id = ? AND (positionCaseInsensitive(tc.input_json, ?) > 0 OR positionCaseInsensitive(tc.result_content, ?) > 0)%s
		ORDER BY s.started_at DESC
		LIMIT ?`, sessionJoinCond)

	tcArgs := []interface{}{q, orgID, q, q}
	tcArgs = append(tcArgs, sessionArgs...)
	tcArgs = append(tcArgs, limit)

	var tcRows []searchResultRow
	if err := s.conn.Select(ctx, &tcRows, tcQ, tcArgs...); err != nil {
		return nil, fmt.Errorf("search tool calls: %w", err)
	}

	allRows := append(msgRows, tcRows...)

	// Apply limit after combining both result sets
	if len(allRows) > limit {
		allRows = allRows[:limit]
	}

	results := make([]SearchResult, 0, len(allRows))
	for _, r := range allRows {
		snippet, highlights := extractSnippet(r.Content, q, 200)
		results = append(results, SearchResult{
			SessionID:    r.SessionID,
			Ordinal:      int(r.Ordinal),
			Role:         r.Role,
			UserID:       r.UserID,
			UserName:     r.UserName,
			ProjectName:  r.ProjectName,
			AgentType:    r.AgentType,
			StartedAt:    r.StartedAt,
			FirstMessage: r.FirstMessage,
			Snippet:      snippet,
			Highlights:   highlights,
		})
	}

	return &SearchPage{
		Results: results,
		Total:   len(results),
	}, nil
}

// extractSnippet returns a ~windowSize character window around the first match of term in content,
// with highlight positions within the snippet.
func extractSnippet(content, term string, windowSize int) (string, []Highlight) {
	if content == "" || term == "" {
		return content, make([]Highlight, 0)
	}

	lower := strings.ToLower(content)
	termLower := strings.ToLower(term)

	matchPos := strings.Index(lower, termLower)

	runes := []rune(content)
	runesLower := []rune(lower)

	// Find byte pos to rune pos
	matchRune := -1
	if matchPos >= 0 {
		matchRune = len([]rune(content[:matchPos]))
	}

	var prefix, suffix string
	var snippet []rune

	if matchRune < 0 {
		// No match — return first windowSize runes
		if len(runes) > windowSize {
			snippet = runes[:windowSize]
			suffix = "..."
		} else {
			snippet = runes
		}
		return string(snippet) + suffix, make([]Highlight, 0)
	}

	half := windowSize / 2
	start := matchRune - half
	if start < 0 {
		start = 0
	}
	end := start + windowSize
	if end > len(runes) {
		end = len(runes)
		start = end - windowSize
		if start < 0 {
			start = 0
		}
	}
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}

	snippet = runes[start:end]
	snippetLower := runesLower[start:end]
	termRunes := []rune(termLower)

	highlights := make([]Highlight, 0)
	offset := 0
	tLen := len(termRunes)
	for offset+tLen <= len(snippetLower) {
		pos := runeSliceIndex(snippetLower[offset:], termRunes)
		if pos < 0 {
			break
		}
		byteStart := len(string(snippet[:offset+pos]))
		byteEnd := len(string(snippet[:offset+pos+tLen]))
		highlights = append(highlights, Highlight{
			Start: len(prefix) + byteStart,
			End:   len(prefix) + byteEnd,
		})
		offset += pos + tLen
	}

	return prefix + string(snippet) + suffix, highlights
}

// runeSliceIndex finds the first occurrence of needle in haystack (rune slices).
func runeSliceIndex(haystack, needle []rune) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
