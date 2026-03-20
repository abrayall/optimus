package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"optimus/core/lib/engine"
	"optimus/core/lib/ui"
)

// Version is set via ldflags during build
var Version = "dev"

// ServerConfig holds all server configuration
type ServerConfig struct {
	Host string
	Port string

	// Engine API keys (forwarded to every job)
	EngineKeys engine.Config

	// Publishing
	Publish    string // "local" or "s3"
	S3Bucket   string
	S3Region   string
	S3Endpoint string
}

// Server is the HTTP server that manages async jobs
type Server struct {
	config ServerConfig
	jobs   map[string]*Job
	mu     sync.RWMutex
	mux    *http.ServeMux
}

// NewServer creates a new Server with routes registered
func NewServer(cfg ServerConfig) *Server {
	s := &Server{
		config: cfg,
		jobs:   make(map[string]*Job),
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// registerRoutes sets up all API endpoints
func (s *Server) registerRoutes() {
	// Static assets
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))

	// Pages
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.HandleFunc("GET /blog", s.handleBlog)

	// API
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/skills", s.handleListSkills)
	s.mux.HandleFunc("POST /api/jobs", s.handleCreateJob)
	s.mux.HandleFunc("GET /api/jobs", s.handleListJobs)
	s.mux.HandleFunc("GET /api/jobs/{id}", s.handleGetJob)
	s.mux.HandleFunc("DELETE /api/jobs/{id}", s.handleDeleteJob)
}

// Start prints the banner, config summary, and begins listening
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)

	// Print header
	ui.PrintHeader(Version)
	ui.PrintKeyValue("Mode", "server")
	ui.PrintKeyValue("Listen", addr)
	ui.PrintKeyValue("Publish", s.config.Publish)
	if s.config.Publish == "s3" {
		ui.PrintKeyValue("S3 Bucket", s.config.S3Bucket)
		ui.PrintKeyValue("S3 Region", s.config.S3Region)
		if s.config.S3Endpoint != "" {
			ui.PrintKeyValue("S3 Endpoint", s.config.S3Endpoint)
		}
	}

	// Show which API keys are configured
	var keys []string
	if s.config.EngineKeys.SerpAPIKey != "" {
		keys = append(keys, "serpapi")
	}
	if s.config.EngineKeys.GoogleAPIKey != "" {
		keys = append(keys, "google")
	}
	if s.config.EngineKeys.GoogleCSEID != "" {
		keys = append(keys, "google-cse")
	}
	if s.config.EngineKeys.GSCCredentials != "" {
		keys = append(keys, "gsc")
	}
	if s.config.EngineKeys.PerplexityKey != "" {
		keys = append(keys, "perplexity")
	}
	if s.config.EngineKeys.MozAPIKey != "" {
		keys = append(keys, "moz")
	}
	if s.config.EngineKeys.AhrefsAPIKey != "" {
		keys = append(keys, "ahrefs")
	}
	if s.config.EngineKeys.BingAPIKey != "" {
		keys = append(keys, "bing")
	}
	if s.config.EngineKeys.RedditClientID != "" {
		keys = append(keys, "reddit")
	}
	if s.config.EngineKeys.TwitterBearerToken != "" {
		keys = append(keys, "twitter")
	}
	if len(keys) > 0 {
		ui.PrintKeyValue("API Keys", strings.Join(keys, ", "))
	} else {
		ui.PrintKeyValue("API Keys", "none")
	}

	fmt.Println()
	ui.PrintSuccess("Server started on %s", addr)
	fmt.Println()

	return http.ListenAndServe(addr, s.cors(s.mux))
}

// cors wraps a handler with permissive CORS headers
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
