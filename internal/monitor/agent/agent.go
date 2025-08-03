package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"crucible/internal/logging"
	"crucible/internal/monitor"
	"crucible/internal/monitor/alerts"
	"crucible/internal/monitor/collectors"
	"crucible/internal/monitor/storage"
)

// Agent represents the monitoring agent
type Agent struct {
	config    *monitor.Config
	logger    *logging.Logger
	server    *Server
	startTime time.Time
	mu        sync.RWMutex

	// Data storage
	systemMetrics     *monitor.SystemMetrics
	serviceMetrics    []monitor.ServiceStatus
	httpCheckResults  []monitor.HTTPCheckResult
	metricsCount      int64
	activeAlertsCount int

	// Storage adapter
	storageAdapter *storage.StorageAdapter

	// Collectors
	systemCollector   *collectors.SystemCollector
	servicesCollector *collectors.ServicesCollector
	httpCollector     *collectors.HTTPCollector

	// Alert manager
	alertManager *alerts.AlertManager

	// Collection timestamps
	lastSystemCollect     *time.Time
	lastServicesCollect   *time.Time
	lastHTTPChecksCollect *time.Time

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewAgent creates a new monitoring agent
func NewAgent(config *monitor.Config, logger *logging.Logger) (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		config:    config,
		logger:    logger,
		startTime: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Initialize storage adapter if configured
	if config.Storage.Type == "sqlite" {
		storageAdapter, err := storage.NewStorageAdapter(config, logger)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create storage adapter: %w", err)
		}
		agent.storageAdapter = storageAdapter
	}

	// Initialize collectors
	agent.systemCollector = collectors.NewSystemCollector()
	agent.servicesCollector = collectors.NewServicesCollector(config.Collectors.Services.Services)
	agent.httpCollector = collectors.NewHTTPCollector()

	// Initialize alert manager if alerts are enabled
	if config.Alerts.Enabled {
		alertConfig, err := alerts.LoadConfig("configs/alerts.yaml")
		if err != nil {
			logger.Warn("Failed to load alert config, using defaults", "error", err)
			alertConfig = alerts.CreateDefaultConfig()
		}

		alertRules, err := alerts.LoadRules("configs/alerts.yaml")
		if err != nil {
			logger.Warn("Failed to load alert rules", "error", err)
			alertRules = []*alerts.AlertRule{}
		}

		agent.alertManager = alerts.NewAlertManager(alertConfig)
		for _, rule := range alertRules {
			agent.alertManager.AddRule(rule)
		}
	}

	// Create HTTP server
	agent.server = NewServer(config, logger, agent)

	return agent, nil
}

// Start starts the monitoring agent
func (a *Agent) Start() error {
	a.logger.Info("Starting monitoring agent")

	// Start background collectors
	a.startCollectors()

	// Start HTTP API server
	if err := a.server.Start(); err != nil {
		return err
	}

	return nil
}

// Stop gracefully stops the monitoring agent
func (a *Agent) Stop() error {
	a.logger.Info("Stopping monitoring agent")

	// Cancel context to stop collectors
	a.cancel()

	// Close storage adapter
	if a.storageAdapter != nil {
		if err := a.storageAdapter.Close(); err != nil {
			a.logger.Error("Failed to close storage adapter", "error", err)
		}
	}

	// Stop HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Stop(ctx); err != nil {
		return err
	}

	return nil
}

// startCollectors starts all enabled data collectors
func (a *Agent) startCollectors() {
	// Start system metrics collector
	if a.config.Collectors.System.Enabled {
		go a.systemCollectorLoop()
	}

	// Start service metrics collector
	if a.config.Collectors.Services.Enabled {
		go a.servicesCollectorLoop()
	}

	// Start HTTP checks collector
	if a.config.Collectors.HTTPChecks.Enabled && len(a.config.Collectors.HTTPChecks.Checks) > 0 {
		go a.httpChecksCollectorLoop()
	}

	// Start alert evaluation loop
	if a.alertManager != nil {
		go a.alertEvaluationLoop()
	}
}

// systemCollectorLoop runs the system metrics collection loop
func (a *Agent) systemCollectorLoop() {
	ticker := time.NewTicker(a.config.GetSystemCollectorInterval())
	defer ticker.Stop()

	// Collect immediately on start
	a.collectSystemMetrics()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.collectSystemMetrics()
		}
	}
}

// servicesCollectorLoop runs the service metrics collection loop
func (a *Agent) servicesCollectorLoop() {
	ticker := time.NewTicker(a.config.GetServicesCollectorInterval())
	defer ticker.Stop()

	// Collect immediately on start
	a.collectServiceMetrics()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.collectServiceMetrics()
		}
	}
}

// httpChecksCollectorLoop runs the HTTP checks collection loop
func (a *Agent) httpChecksCollectorLoop() {
	// Start each HTTP check in its own goroutine
	for _, check := range a.config.Collectors.HTTPChecks.Checks {
		go a.httpCheckLoop(check)
	}
}

// httpCheckLoop runs a single HTTP check loop
func (a *Agent) httpCheckLoop(check monitor.HTTPCheck) {
	ticker := time.NewTicker(check.GetInterval())
	defer ticker.Stop()

	// Check immediately on start
	a.performHTTPCheck(check)

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.performHTTPCheck(check)
		}
	}
}

// collectSystemMetrics collects current system metrics
func (a *Agent) collectSystemMetrics() {
	a.logger.Debug("Collecting system metrics")

	metrics, err := a.systemCollector.Collect()
	if err != nil {
		a.logger.Error("Failed to collect system metrics", "error", err)
		return
	}

	a.mu.Lock()
	a.systemMetrics = metrics
	now := time.Now()
	a.lastSystemCollect = &now
	a.metricsCount++
	a.mu.Unlock()

	// Store in persistent storage if available
	if a.storageAdapter != nil {
		if err := a.storageAdapter.StoreSystemMetrics(metrics); err != nil {
			a.logger.Error("Failed to store system metrics", "error", err)
		}
	}
}

// collectServiceMetrics collects current service metrics
func (a *Agent) collectServiceMetrics() {
	a.logger.Debug("Collecting service metrics")

	services, err := a.servicesCollector.Collect()
	if err != nil {
		a.logger.Error("Failed to collect service metrics", "error", err)
		return
	}

	a.mu.Lock()
	a.serviceMetrics = services
	now := time.Now()
	a.lastServicesCollect = &now
	a.mu.Unlock()

	// Store in persistent storage if available
	if a.storageAdapter != nil {
		if err := a.storageAdapter.StoreServiceMetrics(services); err != nil {
			a.logger.Error("Failed to store service metrics", "error", err)
		}
	}
}

// performHTTPCheck performs a single HTTP health check
func (a *Agent) performHTTPCheck(check monitor.HTTPCheck) {
	a.logger.Debug("Performing HTTP check", "name", check.Name, "url", check.URL)

	result := a.httpCollector.PerformCheck(check)

	a.mu.Lock()
	// Update or append result
	found := false
	for i, existing := range a.httpCheckResults {
		if existing.Name == check.Name {
			a.httpCheckResults[i] = result
			found = true
			break
		}
	}
	if !found {
		a.httpCheckResults = append(a.httpCheckResults, result)
	}
	now := time.Now()
	a.lastHTTPChecksCollect = &now
	a.mu.Unlock()

	// Store in persistent storage if available
	if a.storageAdapter != nil {
		if err := a.storageAdapter.StoreHTTPCheckResults([]monitor.HTTPCheckResult{result}); err != nil {
			a.logger.Error("Failed to store HTTP check results", "error", err)
		}
	}
}

// Getter methods for server endpoints

// GetUptime returns the agent uptime
func (a *Agent) GetUptime() time.Duration {
	return time.Since(a.startTime)
}

// GetStartTime returns the agent start time
func (a *Agent) GetStartTime() time.Time {
	return a.startTime
}

// GetSystemMetrics returns the latest system metrics
func (a *Agent) GetSystemMetrics() (*monitor.SystemMetrics, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.systemMetrics == nil {
		return nil, nil // No data yet
	}

	// Return a copy to avoid data races
	metrics := *a.systemMetrics
	return &metrics, nil
}

// GetServiceMetrics returns the latest service metrics
func (a *Agent) GetServiceMetrics() ([]monitor.ServiceStatus, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Return a copy to avoid data races
	services := make([]monitor.ServiceStatus, len(a.serviceMetrics))
	copy(services, a.serviceMetrics)
	return services, nil
}

// GetHTTPCheckResults returns the latest HTTP check results
func (a *Agent) GetHTTPCheckResults() ([]monitor.HTTPCheckResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Return a copy to avoid data races
	results := make([]monitor.HTTPCheckResult, len(a.httpCheckResults))
	copy(results, a.httpCheckResults)
	return results, nil
}

// GetMetricsCount returns the total number of metrics collected
func (a *Agent) GetMetricsCount() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.metricsCount
}

// GetActiveAlertsCount returns the number of active alerts
func (a *Agent) GetActiveAlertsCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeAlertsCount
}

// GetLastSystemCollect returns the timestamp of the last system metrics collection
func (a *Agent) GetLastSystemCollect() *time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastSystemCollect
}

// GetLastServicesCollect returns the timestamp of the last services collection
func (a *Agent) GetLastServicesCollect() *time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastServicesCollect
}

// GetLastHTTPChecksCollect returns the timestamp of the last HTTP checks collection
func (a *Agent) GetLastHTTPChecksCollect() *time.Time {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastHTTPChecksCollect
}

// alertEvaluationLoop runs the alert evaluation loop
func (a *Agent) alertEvaluationLoop() {
	ticker := time.NewTicker(30 * time.Second) // Evaluate every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.evaluateAlerts()
		}
	}
}

// evaluateAlerts evaluates all alert rules against current metrics
func (a *Agent) evaluateAlerts() {
	if a.alertManager == nil {
		return
	}

	a.logger.Debug("Evaluating alert rules")

	// Get current metrics
	a.mu.RLock()
	systemMetrics := a.systemMetrics
	serviceMetrics := a.serviceMetrics
	httpCheckResults := a.httpCheckResults
	a.mu.RUnlock()

	// Skip evaluation if we don't have enough data yet
	if systemMetrics == nil {
		return
	}

	// Create evaluation context
	ctx := &alerts.EvaluationContext{
		SystemMetrics: map[string]alerts.MetricData{
			"cpu_usage": {
				Timestamp: time.Now(),
				Value:     systemMetrics.CPU.UsagePercent,
				Labels:    map[string]string{"type": "cpu"},
			},
			"memory_usage": {
				Timestamp: time.Now(),
				Value:     systemMetrics.Memory.UsagePercent,
				Labels:    map[string]string{"type": "memory"},
			},
			"load_1": {
				Timestamp: time.Now(),
				Value:     systemMetrics.Load.Load1,
				Labels:    map[string]string{"type": "load"},
			},
		},
		ServiceStates: make(map[string]string),
		HTTPResults:   make(map[string]alerts.HTTPCheckResult),
		CurrentTime:   time.Now(),
	}

	// Add disk usage metrics
	for _, disk := range systemMetrics.Disk {
		if disk.MountPoint == "/" {
			ctx.SystemMetrics["disk_usage_root"] = alerts.MetricData{
				Timestamp: time.Now(),
				Value:     disk.UsagePercent,
				Labels:    map[string]string{"type": "disk", "mount": "/"},
			}
		}
	}

	// Add service states
	for _, service := range serviceMetrics {
		status := "inactive"
		if service.Active == "active" && service.Sub == "running" {
			status = "active"
		}
		ctx.ServiceStates[service.Name] = status
	}

	// Add HTTP check results
	for _, check := range httpCheckResults {
		ctx.HTTPResults[check.Name] = alerts.HTTPCheckResult{
			URL:          check.URL,
			StatusCode:   check.StatusCode,
			ResponseTime: check.ResponseTime,
			Success:      check.Success,
			Error:        check.Error,
			Timestamp:    check.Timestamp,
		}
	}

	// Evaluate rules
	err := a.alertManager.EvaluateRules(ctx)
	if err != nil {
		a.logger.Error("Failed to evaluate alert rules", "error", err)
	}

	// Update active alerts count
	a.mu.Lock()
	activeAlerts := a.alertManager.GetActiveAlerts()
	a.activeAlertsCount = len(activeAlerts)
	a.mu.Unlock()
}

// Alert management methods for API endpoints

// GetActiveAlerts returns all active alerts
func (a *Agent) GetActiveAlerts() ([]*alerts.Alert, error) {
	if a.alertManager == nil {
		return []*alerts.Alert{}, nil
	}
	return a.alertManager.GetActiveAlerts(), nil
}

// GetAlert returns a specific alert by ID
func (a *Agent) GetAlert(alertID string) (*alerts.Alert, error) {
	if a.alertManager == nil {
		return nil, fmt.Errorf("alert manager not initialized")
	}
	return a.alertManager.GetAlert(alertID)
}

// AcknowledgeAlert acknowledges an alert
func (a *Agent) AcknowledgeAlert(alertID string) error {
	if a.alertManager == nil {
		return fmt.Errorf("alert manager not initialized")
	}
	return a.alertManager.AcknowledgeAlert(alertID)
}

// ResolveAlert manually resolves an alert
func (a *Agent) ResolveAlert(alertID string) error {
	if a.alertManager == nil {
		return fmt.Errorf("alert manager not initialized")
	}
	return a.alertManager.ResolveAlert(alertID)
}

// GetStorageAdapter returns the storage adapter for accessing historical data
func (a *Agent) GetStorageAdapter() *storage.StorageAdapter {
	return a.storageAdapter
}
