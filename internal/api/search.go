// ABOUTME: HTTP handler for the search API endpoint.
// ABOUTME: Maps query parameters to search filters and returns BM25-ranked results with snippets.
package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/clkao/agentlore/internal/store"
)

var (
	shaPattern = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	prPattern  = regexp.MustCompile(`^https://github\.com/[\w.\-]+/[\w.\-]+/pull/\d+$`)
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	// Auto-recognize commit SHA or PR URL patterns and check git_links first.
	if shaPattern.MatchString(query) {
		if page := s.lookupGitLinksAsSearch(r, query, ""); page != nil {
			writeJSON(w, http.StatusOK, page)
			return
		}
	} else if prPattern.MatchString(query) {
		if page := s.lookupGitLinksAsSearch(r, "", query); page != nil {
			writeJSON(w, http.StatusOK, page)
			return
		}
	}

	searchQuery := store.SearchQuery{
		Query:       query,
		UserID:      q.Get("user_id"),
		ProjectID:   q.Get("project_id"),
		ProjectName: q.Get("project_name"),
		AgentType:   q.Get("agent_type"),
		DateFrom:    q.Get("date_from"),
		DateTo:      q.Get("date_to"),
	}

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
	searchQuery.Limit = limit

	page, err := s.store.Search(r.Context(), "", searchQuery)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, page)
}

// lookupGitLinksAsSearch queries git_links and converts results to a SearchPage.
// Returns nil if no git link matches found (caller should fall through to FTS).
func (s *Server) lookupGitLinksAsSearch(r *http.Request, sha, prURL string) *store.SearchPage {
	results, err := s.store.LookupGitLinks(r.Context(), "", sha, prURL)
	if err != nil || len(results) == 0 {
		return nil
	}

	searchResults := make([]store.SearchResult, len(results))
	for i, gl := range results {
		var snippet string
		if gl.CommitSHA != "" {
			short := gl.CommitSHA
			if len(short) > 7 {
				short = short[:7]
			}
			snippet = fmt.Sprintf("Commit %s (%s confidence)", short, gl.Confidence)
		} else if gl.PRURL != "" {
			snippet = fmt.Sprintf("PR %s (%s confidence)", gl.PRURL, gl.Confidence)
		}

		searchResults[i] = store.SearchResult{
			SessionID:    gl.SessionID,
			Ordinal:      gl.MessageOrdinal,
			Role:         "assistant",
			UserID:       gl.UserID,
			UserName:     gl.UserName,
			ProjectName:  gl.ProjectName,
			AgentType:    gl.AgentType,
			StartedAt:    gl.StartedAt,
			FirstMessage: gl.FirstMessage,
			Snippet:      snippet,
			Highlights:   []store.Highlight{},
		}
	}

	return &store.SearchPage{
		Results: searchResults,
		Total:   len(searchResults),
	}
}
