package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"crucible/internal/logging"
	"crucible/internal/monitor"
	"crucible/internal/monitor/storage"
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

	// Alert endpoints
	mux.HandleFunc("/api/v1/alerts", s.handleAlerts)
	mux.HandleFunc("/api/v1/alerts/", s.handleAlertActions)

	// Storage endpoints (for historical data)
	mux.HandleFunc("/api/v1/entities", s.handleEntities)
	mux.HandleFunc("/api/v1/entities/", s.handleEntityDetails)
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/metrics", s.handleMetrics)
	mux.HandleFunc("/api/v1/metrics/summary", s.handleMetricSummary)
	mux.HandleFunc("/api/v1/storage/health", s.handleStorageHealth)
	mux.HandleFunc("/api/v1/storage/stats", s.handleStorageStats)

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

// handleAlerts returns active alerts
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	alerts, err := s.agent.GetActiveAlerts()
	if err != nil {
		s.logger.Error("Failed to get active alerts", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, alerts)
}

// handleAlertActions handles alert management actions (acknowledge, etc.)
func (s *Server) handleAlertActions(w http.ResponseWriter, r *http.Request) {
	// Extract alert ID from URL path
	path := r.URL.Path
	if len(path) <= len("/api/v1/alerts/") {
		http.Error(w, "Alert ID required", http.StatusBadRequest)
		return
	}

	alertID := path[len("/api/v1/alerts/"):]

	// Remove trailing action if present (e.g., /acknowledge)
	if idx := strings.Index(alertID, "/"); idx > 0 {
		action := alertID[idx+1:]
		alertID = alertID[:idx]

		switch r.Method {
		case http.MethodPost:
			switch action {
			case "acknowledge":
				err := s.agent.AcknowledgeAlert(alertID)
				if err != nil {
					s.logger.Error("Failed to acknowledge alert", "alert_id", alertID, "error", err)
					http.Error(w, "Failed to acknowledge alert", http.StatusInternalServerError)
					return
				}
				s.writeJSONResponse(w, map[string]string{"status": "acknowledged"})
			case "resolve":
				err := s.agent.ResolveAlert(alertID)
				if err != nil {
					s.logger.Error("Failed to resolve alert", "alert_id", alertID, "error", err)
					http.Error(w, "Failed to resolve alert", http.StatusInternalServerError)
					return
				}
				s.writeJSONResponse(w, map[string]string{"status": "resolved"})
			default:
				http.Error(w, "Unknown action", http.StatusBadRequest)
			}
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	} else {
		// Get specific alert details
		if r.Method == http.MethodGet {
			alert, err := s.agent.GetAlert(alertID)
			if err != nil {
				s.logger.Error("Failed to get alert", "alert_id", alertID, "error", err)
				http.Error(w, "Alert not found", http.StatusNotFound)
				return
			}
			s.writeJSONResponse(w, alert)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
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
			"enabled":        s.config.Collectors.Services.Enabled,
			"interval":       s.config.Collectors.Services.Interval,
			"services_count": len(s.config.Collectors.Services.Services),
			"last_collect":   s.agent.GetLastServicesCollect(),
		},
		"http_checks": map[string]interface{}{
			"enabled":      s.config.Collectors.HTTPChecks.Enabled,
			"checks_count": len(s.config.Collectors.HTTPChecks.Checks),
			"last_collect": s.agent.GetLastHTTPChecksCollect(),
		},
	}
}

// handleEntities returns a list of entities from storage
func (s *Server) handleEntities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if storage adapter is available
	storageAdapter := s.agent.GetStorageAdapter()
	if storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for filtering
	filter := &storage.EntityFilter{}
	query := r.URL.Query()

	if entityType := query.Get("type"); entityType != "" {
		filter.Type = &entityType
	}
	if status := query.Get("status"); status != "" {
		filter.Status = &status
	}
	if name := query.Get("name"); name != "" {
		filter.Name = &name
	}
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = &l
		}
	}
	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = &o
		}
	}
	if since := query.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}

	// Get entities from storage
	entities, err := storageAdapter.ListEntities(filter)
	if err != nil {
		s.logger.Error("Failed to list entities", "error", err)
		http.Error(w, "Failed to retrieve entities", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"entities": entities,
		"count":    len(entities),
		"filter":   filter,
	}

	s.writeJSONResponse(w, response)
}

// handleEntityDetails returns details for a specific entity
func (s *Server) handleEntityDetails(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract entity ID from URL path
	path := r.URL.Path
	if len(path) <= len("/api/v1/entities/") {
		http.Error(w, "Entity ID required", http.StatusBadRequest)
		return
	}

	entityIDStr := path[len("/api/v1/entities/"):]
	// Remove any sub-paths (e.g., /metrics, /events)
	if idx := strings.Index(entityIDStr, "/"); idx > 0 {
		subPath := entityIDStr[idx+1:]
		entityIDStr = entityIDStr[:idx]

		// Handle sub-paths like /api/v1/entities/{id}/metrics
		entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid entity ID", http.StatusBadRequest)
			return
		}

		switch subPath {
		case "metrics":
			s.handleEntityMetrics(w, r, entityID)
		case "events":
			s.handleEntityEvents(w, r, entityID)
		default:
			http.Error(w, "Unknown entity sub-path", http.StatusNotFound)
		}
		return
	}

	// Get entity details
	entityID, err := strconv.ParseInt(entityIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid entity ID", http.StatusBadRequest)
		return
	}

	storageAdapter := s.agent.GetStorageAdapter()
	if storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	entity, err := storageAdapter.GetEntity(entityID)
	if err != nil {
		s.logger.Error("Failed to get entity", "entity_id", entityID, "error", err)
		http.Error(w, "Entity not found", http.StatusNotFound)
		return
	}

	s.writeJSONResponse(w, entity)
}

// handleStorageHealth returns storage system health
func (s *Server) handleStorageHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.agent.storageAdapter == nil {
		response := map[string]interface{}{
			"status":  "not_configured",
			"type":    s.config.Storage.Type,
			"message": "Storage adapter not initialized",
		}
		s.writeJSONResponse(w, response)
		return
	}

	// Try to get storage health (we need to add this method)
	response := map[string]interface{}{
		"status":  "operational",
		"type":    s.config.Storage.Type,
		"message": "SQLite storage is configured and operational",
		"note":    "Detailed health metrics not yet implemented",
	}

	s.writeJSONResponse(w, response)
}

// handleStorageStats returns storage statistics
func (s *Server) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.agent.storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Get storage stats using the adapter
	stats, err := s.agent.storageAdapter.GetStorageStats()
	if err != nil {
		s.logger.Error("Failed to get storage stats", "error", err)
		http.Error(w, "Failed to get storage statistics", http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, stats)
}

// handleEvents returns events from storage
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storageAdapter := s.agent.GetStorageAdapter()
	if storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for filtering
	filter := &storage.EventFilter{}
	query := r.URL.Query()

	if entityID := query.Get("entity_id"); entityID != "" {
		if id, err := strconv.ParseInt(entityID, 10, 64); err == nil {
			filter.EntityID = &id
		}
	}
	if eventType := query.Get("type"); eventType != "" {
		filter.Type = &eventType
	}
	if severity := query.Get("severity"); severity != "" {
		filter.Severity = &severity
	}
	if since := query.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := query.Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = &l
		}
	}
	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = &o
		}
	}

	events, err := storageAdapter.ListEvents(filter)
	if err != nil {
		s.logger.Error("Failed to list events", "error", err)
		http.Error(w, "Failed to retrieve events", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"events": events,
		"count":  len(events),
		"filter": filter,
	}

	s.writeJSONResponse(w, response)
}

// handleMetrics returns metrics from storage
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storageAdapter := s.agent.GetStorageAdapter()
	if storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for filtering
	filter := &storage.MetricFilter{}
	query := r.URL.Query()

	if entityID := query.Get("entity_id"); entityID != "" {
		if id, err := strconv.ParseInt(entityID, 10, 64); err == nil {
			filter.EntityID = &id
		}
	}
	if metricName := query.Get("metric_name"); metricName != "" {
		filter.MetricName = &metricName
	}
	if aggregationLevel := query.Get("aggregation_level"); aggregationLevel != "" {
		filter.AggregationLevel = &aggregationLevel
	}
	if since := query.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := query.Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = &l
		}
	}
	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = &o
		}
	}

	metrics, err := storageAdapter.ListMetrics(filter)
	if err != nil {
		s.logger.Error("Failed to list metrics", "error", err)
		http.Error(w, "Failed to retrieve metrics", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"metrics": metrics,
		"count":   len(metrics),
		"filter":  filter,
	}

	s.writeJSONResponse(w, response)
}

// handleMetricSummary returns aggregated metric summaries
func (s *Server) handleMetricSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	storageAdapter := s.agent.GetStorageAdapter()
	if storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for filtering
	filter := &storage.MetricFilter{}
	query := r.URL.Query()

	if entityID := query.Get("entity_id"); entityID != "" {
		if id, err := strconv.ParseInt(entityID, 10, 64); err == nil {
			filter.EntityID = &id
		}
	}
	if metricName := query.Get("metric_name"); metricName != "" {
		filter.MetricName = &metricName
	}
	if aggregationLevel := query.Get("aggregation_level"); aggregationLevel != "" {
		filter.AggregationLevel = &aggregationLevel
	}
	if since := query.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := query.Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}

	summary, err := storageAdapter.GetMetricSummary(filter)
	if err != nil {
		s.logger.Error("Failed to get metric summary", "error", err)
		http.Error(w, "Failed to retrieve metric summary", http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, summary)
}

// handleEntityMetrics returns metrics for a specific entity
func (s *Server) handleEntityMetrics(w http.ResponseWriter, r *http.Request, entityID int64) {
	storageAdapter := s.agent.GetStorageAdapter()
	if storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for additional filtering
	filter := &storage.MetricFilter{EntityID: &entityID}
	query := r.URL.Query()

	if metricName := query.Get("metric_name"); metricName != "" {
		filter.MetricName = &metricName
	}
	if aggregationLevel := query.Get("aggregation_level"); aggregationLevel != "" {
		filter.AggregationLevel = &aggregationLevel
	}
	if since := query.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := query.Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = &l
		}
	}
	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = &o
		}
	}

	metrics, err := storageAdapter.ListMetrics(filter)
	if err != nil {
		s.logger.Error("Failed to list entity metrics", "entity_id", entityID, "error", err)
		http.Error(w, "Failed to retrieve entity metrics", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"entity_id": entityID,
		"metrics":   metrics,
		"count":     len(metrics),
		"filter":    filter,
	}

	s.writeJSONResponse(w, response)
}

// handleEntityEvents returns events for a specific entity
func (s *Server) handleEntityEvents(w http.ResponseWriter, r *http.Request, entityID int64) {
	storageAdapter := s.agent.GetStorageAdapter()
	if storageAdapter == nil {
		http.Error(w, "Storage not configured", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for additional filtering
	filter := &storage.EventFilter{EntityID: &entityID}
	query := r.URL.Query()

	if eventType := query.Get("type"); eventType != "" {
		filter.Type = &eventType
	}
	if severity := query.Get("severity"); severity != "" {
		filter.Severity = &severity
	}
	if since := query.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = &t
		}
	}
	if until := query.Get("until"); until != "" {
		if t, err := time.Parse(time.RFC3339, until); err == nil {
			filter.Until = &t
		}
	}
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = &l
		}
	}
	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = &o
		}
	}

	events, err := storageAdapter.ListEvents(filter)
	if err != nil {
		s.logger.Error("Failed to list entity events", "entity_id", entityID, "error", err)
		http.Error(w, "Failed to retrieve entity events", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"entity_id": entityID,
		"events":    events,
		"count":     len(events),
		"filter":    filter,
	}

	s.writeJSONResponse(w, response)
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
