package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/benfb/oaamonitor/config"
	"github.com/benfb/oaamonitor/database"
	"github.com/benfb/oaamonitor/middleware"
	"github.com/benfb/oaamonitor/renderer"
)

// Server represents the HTTP server
type Server struct {
	db       *database.DB
	router   *http.ServeMux
	renderer *renderer.Renderer
	config   *config.Config
}

// New creates a new Server instance
func New(db *database.DB, cfg *config.Config) (*Server, error) {
	r, err := renderer.New()
	if err != nil {
		return nil, err
	}

	s := &Server{
		db:       db,
		router:   http.NewServeMux(),
		renderer: r,
		config:   cfg,
	}

	s.registerRoutes()
	return s, nil
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	// Apply middleware to all requests
	var handler http.Handler = s.router
	handler = middleware.Logger(handler)
	handler = middleware.Recover(handler)

	// Register routes
	s.router.HandleFunc("/", s.handleIndexPage)
	s.router.HandleFunc("GET /player/{id}", s.handlePlayerPage)
	s.router.HandleFunc("GET /team/{id}", s.handleTeamPage)
	s.router.HandleFunc("GET /search", s.handleSearch)
	s.router.HandleFunc("GET /download", s.handleDatabaseDownload)
	s.router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe(addr string) error {
	server := &http.Server{
		Addr:         addr,
		Handler:      s,
		ReadTimeout:  time.Duration(s.config.RequestTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.RequestTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return server.ListenAndServe()
}

// ServeHTTP implements the http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Close the database connection
	return s.db.Close()
}

// normalizeTeamName normalizes team names to a standard format
func normalizeTeamName(teamName string) string {
	switch teamName {
	case "diamondbacks", "dbacks":
		return "d-backs"
	case "blue-jays", "red-sox", "white-sox":
		return strings.ReplaceAll(teamName, "-", " ")
	case "blue+jays", "red+sox", "white+sox":
		return strings.ReplaceAll(teamName, "+", " ")
	case "bluejays":
		return "blue jays"
	case "redsox":
		return "red sox"
	case "whitesox":
		return "white sox"
	default:
		return teamName
	}
}
