// ABOUTME: HTTP handlers for git link lookup and per-session git link queries.
// ABOUTME: Queries the store for git links and returns matching results.
package api

import "net/http"

func (s *Server) handleGetSessionGitLinks(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	links, err := s.store.GetSessionGitLinks(r.Context(), "", sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, links)
}

func (s *Server) handleLookupGitLinks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	sha := q.Get("sha")
	pr := q.Get("pr")

	if sha == "" && pr == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'sha' or 'pr' is required")
		return
	}

	results, err := s.store.LookupGitLinks(r.Context(), "", sha, pr)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, results)
}
