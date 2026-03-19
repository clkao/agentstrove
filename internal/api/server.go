// ABOUTME: HTTP API server for the team conversation browser.
// ABOUTME: Routes API requests to handlers and serves the SPA fallback for non-API paths.
package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/clkao/agentlore/internal/store"
	"github.com/clkao/agentlore/internal/web"
)

// Server serves the REST API and SPA frontend.
type Server struct {
	store   store.ReadStore
	mux     *http.ServeMux
	handler http.Handler
}

// New creates an API server with all routes registered.
func New(s store.ReadStore) *Server {
	srv := &Server{store: s, mux: http.NewServeMux()}
	srv.routes()
	srv.handler = corsMiddleware(logMiddleware(srv.mux))
	return srv
}

// ServeHTTP delegates to the middleware-wrapped handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/gitlinks", s.handleGetSessionGitLinks)
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/stars", s.handleGetSessionStars)
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/pins", s.handleGetSessionPins)
	s.mux.HandleFunc("GET /api/v1/sessions/{id}/messages", s.handleGetMessages)
	s.mux.HandleFunc("GET /api/v1/sessions/{id}", s.handleGetSession)
	s.mux.HandleFunc("GET /api/v1/sessions", s.handleListSessions)
	s.mux.HandleFunc("GET /api/v1/users", s.handleListUsers)
	s.mux.HandleFunc("GET /api/v1/projects", s.handleListProjects)
	s.mux.HandleFunc("GET /api/v1/agents", s.handleListAgents)
	s.mux.HandleFunc("GET /api/v1/search", s.handleSearch)
	s.mux.HandleFunc("GET /api/v1/gitlinks", s.handleLookupGitLinks)
	s.mux.HandleFunc("GET /api/v1/analytics/usage", s.handleUsageOverview)
	s.mux.HandleFunc("GET /api/v1/analytics/heatmap", s.handleActivityHeatmap)
	s.mux.HandleFunc("GET /api/v1/analytics/tools", s.handleToolUsage)
	s.mux.HandleFunc("GET /api/v1/analytics/daily", s.handleDailyActivity)
	s.mux.HandleFunc("GET /api/v1/analytics/tokens-by-model", s.handleTokensByModel)
	s.mux.HandleFunc("GET /api/v1/session-deletes", s.handleListSessionDeletes)

	distSub, _ := fs.Sub(web.DistFS, "dist")
	fileServer := http.FileServerFS(distSub)
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// For non-API routes, try serving a static file first.
		// If the path has no extension (SPA route), serve index.html.
		path := r.URL.Path
		if path != "/" && !strings.Contains(path, ".") {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
