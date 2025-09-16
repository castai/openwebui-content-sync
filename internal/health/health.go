package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Server provides health check endpoints
type Server struct {
	server *http.Server
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// NewServer creates a new health check server
func NewServer(port int) *Server {
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	healthServer := &Server{
		server: server,
	}

	// Register health check endpoint
	mux.HandleFunc("/health", healthServer.healthHandler)
	mux.HandleFunc("/ready", healthServer.readyHandler)

	return healthServer
}

// Start starts the health check server
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop stops the health check server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// readyHandler handles readiness check requests
func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ready",
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
