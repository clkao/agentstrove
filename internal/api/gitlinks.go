// ABOUTME: HTTP handler for git link lookup by commit SHA or PR URL.
// ABOUTME: Queries the store for git links and returns matching results with session metadata.
package api

import "net/http"

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
