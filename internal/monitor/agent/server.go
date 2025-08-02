package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"crucible/internal/logging"
	"crucible/internal/monitor"
)

// Server represents the monitoring agent HTTP API server
type Server struct {
	config *monitor.Config
	logger *logging.Logger
	server *http.Server
	agent  *Agent
}

// NewServer creates a new monitoring API server
func NewServer(config *monitor.Config, logger *logging.Logger, agent *Agent) *Server {
	return &Server{
		config: config,
		logger: logger,
		agent:  agent,
	}
}

// Start starts the HTTP API server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	
	// Health and status endpoints
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	
	// Metrics endpoints
	mux.HandleFunc("/api/v1/metrics/system", s.handleSystemMetrics)
	mux.HandleFunc("/api/v1/metrics/services", s.handleServiceMetrics)
	mux.HandleFunc("/api/v1/metrics/http", s.handleHTTPMetrics)
	
	// Configuration endpoints
	mux.HandleFunc("/api/v1/config", s.handleConfig)
	
	// CORS middleware for development
	handler := s.corsMiddleware(mux)
	
	s.server = &http.Server{
		Addr:         s.config.Agent.ListenAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	s.logger.Info("Starting monitoring API server", "addr", s.config.Agent.ListenAddr)
	
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	
	return nil
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping monitoring API server")
	return s.server.Shutdown(ctx)
}

// corsMiddleware adds CORS headers for development
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// handleHealth returns the health status of the monitoring agent
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0", // TODO: Get from build info
		"uptime":    s.agent.GetUptime(),
		"config": map[string]interface{}{
			"listen_addr":      s.config.Agent.ListenAddr,
			"data_retention":   s.config.Agent.DataRetention,
			"collect_interval": s.config.Agent.CollectInterval,
		},
		"collectors": s.getCollectorStatus(),
	}

	s.writeJSONResponse(w, health)
}

// handleStatus returns detailed status information
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := map[string]interface{}{
		"agent": map[string]interface{}{
			"status":     "running",
			"started_at": s.agent.GetStartTime(),
			"uptime":     s.agent.GetUptime(),
			"version":    "1.0.0",
		},
		"collectors": s.getCollectorStatus(),
		"storage": map[string]interface{}{
			"type":          s.config.Storage.Type,
			"metrics_count": s.agent.GetMetricsCount(),
		},
		"alerts": map[string]interface{}{
			"enabled":      s.config.Alerts.Enabled,
			"active_count": s.agent.GetActiveAlertsCount(),
		},
	}

	s.writeJSONResponse(w, status)
}

// handleSystemMetrics returns current system metrics
func (s *Server) handleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics, err := s.agent.GetSystemMetrics()
	if err != nil {
		s.logger.Error("Failed to get system metrics", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, metrics)
}

// handleServiceMetrics returns current service metrics
func (s *Server) handleServiceMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	services, err := s.agent.GetServiceMetrics()
	if err != nil {
		s.logger.Error("Failed to get service metrics", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, services)
}

// handleHTTPMetrics returns HTTP check results
func (s *Server) handleHTTPMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	httpChecks, err := s.agent.GetHTTPCheckResults()
	if err != nil {
		s.logger.Error("Failed to get HTTP check results", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, httpChecks)
}

// handleConfig returns or updates the configuration
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSONResponse(w, s.config)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getCollectorStatus returns the status of all collectors
func (s *Server) getCollectorStatus() map[string]interface{} {
	return map[string]interface{}{
		"system": map[string]interface{}{
			"enabled":      s.config.Collectors.System.Enabled,
			"interval":     s.config.Collectors.System.Interval,
			"last_collect": s.agent.GetLastSystemCollect(),
		},
		"services": map[string]interface{}{
			"enabled":       s.config.Collectors.Services.Enabled,
			"interval":      s.config.Collectors.Services.Interval,
			"services_count": len(s.config.Collectors.Services.Services),
			"last_collect":  s.agent.GetLastServicesCollect(),
		},
		"http_checks": map[string]interface{}{
			"enabled":     s.config.Collectors.HTTPChecks.Enabled,
			"checks_count": len(s.config.Collectors.HTTPChecks.Checks),
			"last_collect": s.agent.GetLastHTTPChecksCollect(),
		},
	}
}

// writeJSONResponse writes a JSON response
func (s *Server) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("Failed to encode JSON response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}