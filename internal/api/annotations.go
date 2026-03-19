// ABOUTME: HTTP handlers for session stars, message pins, and session deletes endpoints.
// ABOUTME: Returns JSON arrays of annotation records for the given session or org.
package api

import "net/http"

func (s *Server) handleGetSessionStars(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	stars, err := s.store.ListSessionStars(r.Context(), "", sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stars)
}

func (s *Server) handleGetSessionPins(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	pins, err := s.store.ListMessagePins(r.Context(), "", sessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pins)
}

func (s *Server) handleListSessionDeletes(w http.ResponseWriter, r *http.Request) {
	deletes, err := s.store.ListSessionDeletes(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deletes)
}
