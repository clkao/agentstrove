// ABOUTME: API handlers for team analytics endpoints (usage, heatmap, tool distribution).
// ABOUTME: Parses date range query params and delegates to ReadStore analytics methods.
package api

import (
	"net/http"
	"time"
)

func (s *Server) handleUsageOverview(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

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
	projectName := q.Get("project_name")

	result, err := s.store.UsageByUser(r.Context(), "", projectName, dateFrom, dateTo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleActivityHeatmap(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

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
	projectName := q.Get("project_name")

	result, err := s.store.ActivityHeatmap(r.Context(), "", projectName, dateFrom, dateTo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleToolUsage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

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
	projectName := q.Get("project_name")

	result, err := s.store.ToolUsageDistribution(r.Context(), "", projectName, dateFrom, dateTo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleDailyActivity(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

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
	projectName := q.Get("project_name")

	result, err := s.store.DailyActivity(r.Context(), "", projectName, dateFrom, dateTo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}
