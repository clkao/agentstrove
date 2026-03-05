// ABOUTME: API handlers for session list, detail, messages, and metadata endpoints.
// ABOUTME: Maps HTTP query params to store filters and returns JSON responses.
package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/clkao/agentstrove/internal/store"
)

// MessageWithToolCalls combines a message with its associated tool calls for API responses.
type MessageWithToolCalls struct {
	SessionID     string           `json:"session_id"`
	Ordinal       int              `json:"ordinal"`
	Role          string           `json:"role"`
	Content       string           `json:"content"`
	Timestamp     *time.Time       `json:"timestamp"`
	HasThinking   bool             `json:"has_thinking"`
	HasToolUse    bool             `json:"has_tool_use"`
	ContentLength int              `json:"content_length"`
	ToolCalls     []store.ToolCall `json:"tool_calls"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 50
	if l := q.Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}

	dateFrom := q.Get("date_from")
	if dateFrom != "" {
		if _, err := time.Parse("2006-01-02", dateFrom); err != nil {
			writeError(w, http.StatusBadRequest, "invalid date_from: "+dateFrom)
			return
		}
	}
	dateTo := q.Get("date_to")
	if dateTo != "" {
		if _, err := time.Parse("2006-01-02", dateTo); err != nil {
			writeError(w, http.StatusBadRequest, "invalid date_to: "+dateTo)
			return
		}
	}

	filter := store.SessionFilter{
		UserID:      q.Get("user_id"),
		ProjectID:   q.Get("project_id"),
		ProjectName: q.Get("project_name"),
		AgentType:   q.Get("agent_type"),
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		Cursor:    q.Get("cursor"),
		Limit:     limit,
	}

	page, err := s.store.ListSessions(r.Context(), "", filter)
	if err != nil {
		if strings.Contains(err.Error(), "invalid cursor") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	session, err := s.store.GetSession(r.Context(), "", id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	msgs, err := s.store.GetSessionMessages(r.Context(), "", sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tcs, err := s.store.GetSessionToolCalls(r.Context(), "", sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Group tool calls by message ordinal
	tcByOrdinal := make(map[int][]store.ToolCall)
	for _, tc := range tcs {
		tcByOrdinal[tc.MessageOrdinal] = append(tcByOrdinal[tc.MessageOrdinal], tc)
	}

	result := make([]MessageWithToolCalls, len(msgs))
	for i, m := range msgs {
		result[i] = MessageWithToolCalls{
			SessionID:     m.SessionID,
			Ordinal:       m.Ordinal,
			Role:          m.Role,
			Content:       m.Content,
			Timestamp:     m.Timestamp,
			HasThinking:   m.HasThinking,
			HasToolUse:    m.HasToolUse,
			ContentLength: m.ContentLength,
			ToolCalls:     tcByOrdinal[m.Ordinal],
		}
		if result[i].ToolCalls == nil {
			result[i].ToolCalls = []store.ToolCall{}
		}
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.store.ListProjects(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.ListAgents(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agents)
}
