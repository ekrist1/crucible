package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"crucible/internal/monitor"
)

// System monitoring and dashboard functions

// showMonitoringDashboard displays the monitoring dashboard with system metrics
func (m Model) showMonitoringDashboard() (tea.Model, tea.Cmd) {
	m.State = StateProcessing
	m.Report = []string{}
	m.ProcessingMsg = "Loading monitoring data..."
	m.MonitoringScroll = 0 // Reset scroll position when refreshing

	// Initialize monitoring view if not set
	if m.MonitoringView == 0 && m.MonitoringTimeRange == 0 {
		m.MonitoringView = MonitoringViewLive
		m.MonitoringTimeRange = TimeRangeLast1Hour
	}

	// Build monitoring dashboard report based on current view
	switch m.MonitoringView {
	case MonitoringViewLive:
		return m.showLiveMonitoringData()
	case MonitoringViewHistorical:
		return m.showHistoricalMonitoringData()
	case MonitoringViewEvents:
		return m.showEventsMonitoringData()
	case MonitoringViewStorage:
		return m.showStorageMonitoringData()
	default:
		return m.showLiveMonitoringData()
	}
}

// showLiveMonitoringData displays live monitoring data (original functionality)
func (m Model) showLiveMonitoringData() (tea.Model, tea.Cmd) {
	// Build monitoring dashboard report
	m.Report = append(m.Report, TitleStyle.Render("=== LIVE MONITORING DASHBOARD ==="))
	m.Report = append(m.Report, "")

	// Add navigation help
	m.Report = append(m.Report, ChoiceStyle.Render("Navigation: [h] Historical | [e] Events | [s] Storage | [r] Refresh | [q] Back"))
	m.Report = append(m.Report, ChoiceStyle.Render("Scroll: ↑↓ or k/j | Page: PgUp/PgDn | Home/End"))
	m.Report = append(m.Report, "")

	// Check if monitoring agent is running
	agentStatus := m.checkMonitoringAgent()
	m.Report = append(m.Report, InfoStyle.Render("🔧 Monitoring Agent:"))
	m.Report = append(m.Report, agentStatus)
	m.Report = append(m.Report, "")

	// If agent is running, fetch metrics
	if strings.Contains(agentStatus, "✅") {
		// Fetch system metrics
		systemMetrics := m.fetchSystemMetrics()
		m.Report = append(m.Report, InfoStyle.Render("📊 System Metrics:"))
		m.Report = append(m.Report, systemMetrics...)
		m.Report = append(m.Report, "")

		// Fetch service metrics
		serviceMetrics := m.fetchServiceMetrics()
		m.Report = append(m.Report, InfoStyle.Render("⚙️ Service Status:"))
		m.Report = append(m.Report, serviceMetrics...)
		m.Report = append(m.Report, "")

		// Fetch HTTP check results
		httpMetrics := m.fetchHTTPMetrics()
		m.Report = append(m.Report, InfoStyle.Render("🌐 HTTP Health Checks:"))
		m.Report = append(m.Report, httpMetrics...)
		m.Report = append(m.Report, "")

		// Fetch active alerts
		alertMetrics := m.fetchActiveAlerts()
		m.Report = append(m.Report, InfoStyle.Render("🚨 Active Alerts:"))
		m.Report = append(m.Report, alertMetrics...)
	} else {
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Start monitoring agent with: ./crucible-monitor"))
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Or use: make run-monitor"))
	}

	m.ProcessingMsg = ""
	return m, tea.ClearScreen
}

// showHistoricalMonitoringData displays historical monitoring data
func (m Model) showHistoricalMonitoringData() (tea.Model, tea.Cmd) {
	// Build historical monitoring dashboard report
	m.Report = append(m.Report, TitleStyle.Render("=== HISTORICAL MONITORING DATA ==="))
	m.Report = append(m.Report, "")

	// Add navigation help and time range selector
	m.Report = append(m.Report, ChoiceStyle.Render("Navigation: [l] Live | [e] Events | [s] Storage | [r] Refresh | [q] Back"))
	m.Report = append(m.Report, ChoiceStyle.Render("Time Range: [1] 1h | [6] 6h | [d] 24h | [w] 7d | [m] 30d"))
	m.Report = append(m.Report, ChoiceStyle.Render("Scroll: ↑↓ or k/j | Page: PgUp/PgDn | Home/End"))
	m.Report = append(m.Report, InfoStyle.Render(fmt.Sprintf("Current Range: %s", m.MonitoringTimeRange.String())))
	m.Report = append(m.Report, "")

	// Check if monitoring agent is running
	agentStatus := m.checkMonitoringAgent()
	m.Report = append(m.Report, InfoStyle.Render("🔧 Monitoring Agent:"))
	m.Report = append(m.Report, agentStatus)
	m.Report = append(m.Report, "")

	// If agent is running, fetch historical data
	if strings.Contains(agentStatus, "✅") {
		// Fetch entities
		entities := m.fetchEntities()
		m.Report = append(m.Report, InfoStyle.Render("📊 Monitored Entities:"))
		m.Report = append(m.Report, entities...)
		m.Report = append(m.Report, "")

		// Fetch recent historical metrics for key entities
		historicalMetrics := m.fetchHistoricalMetrics()
		m.Report = append(m.Report, InfoStyle.Render("📈 Historical Metrics Summary:"))
		m.Report = append(m.Report, historicalMetrics...)
		m.Report = append(m.Report, "")

		// Storage statistics
		storageStats := m.fetchStorageStats()
		m.Report = append(m.Report, InfoStyle.Render("💾 Storage Statistics:"))
		m.Report = append(m.Report, storageStats...)
	} else {
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Start monitoring agent with: ./crucible-monitor"))
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Or use: make run-monitor"))
	}

	m.ProcessingMsg = ""
	return m, tea.ClearScreen
}

// showEventsMonitoringData displays recent events
func (m Model) showEventsMonitoringData() (tea.Model, tea.Cmd) {
	// Build events monitoring dashboard report
	m.Report = append(m.Report, TitleStyle.Render("=== MONITORING EVENTS ==="))
	m.Report = append(m.Report, "")

	// Add navigation help and time range selector
	m.Report = append(m.Report, ChoiceStyle.Render("Navigation: [l] Live | [h] Historical | [s] Storage | [r] Refresh | [q] Back"))
	m.Report = append(m.Report, ChoiceStyle.Render("Time Range: [1] 1h | [6] 6h | [d] 24h | [w] 7d | [m] 30d"))
	m.Report = append(m.Report, ChoiceStyle.Render("Scroll: ↑↓ or k/j | Page: PgUp/PgDn | Home/End"))
	m.Report = append(m.Report, InfoStyle.Render(fmt.Sprintf("Current Range: %s", m.MonitoringTimeRange.String())))
	m.Report = append(m.Report, "")

	// Check if monitoring agent is running
	agentStatus := m.checkMonitoringAgent()
	m.Report = append(m.Report, InfoStyle.Render("🔧 Monitoring Agent:"))
	m.Report = append(m.Report, agentStatus)
	m.Report = append(m.Report, "")

	// If agent is running, fetch events
	if strings.Contains(agentStatus, "✅") {
		// Fetch recent events
		events := m.fetchEvents()
		m.Report = append(m.Report, InfoStyle.Render("📋 Recent Events:"))
		m.Report = append(m.Report, events...)
	} else {
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Start monitoring agent with: ./crucible-monitor"))
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Or use: make run-monitor"))
	}

	m.ProcessingMsg = ""
	return m, tea.ClearScreen
}

// showStorageMonitoringData displays storage health and statistics
func (m Model) showStorageMonitoringData() (tea.Model, tea.Cmd) {
	// Build storage monitoring dashboard report
	m.Report = append(m.Report, TitleStyle.Render("=== STORAGE MONITORING ==="))
	m.Report = append(m.Report, "")

	// Add navigation help
	m.Report = append(m.Report, ChoiceStyle.Render("Navigation: [l] Live | [h] Historical | [e] Events | [r] Refresh | [q] Back"))
	m.Report = append(m.Report, ChoiceStyle.Render("Scroll: ↑↓ or k/j | Page: PgUp/PgDn | Home/End"))
	m.Report = append(m.Report, "")

	// Check if monitoring agent is running
	agentStatus := m.checkMonitoringAgent()
	m.Report = append(m.Report, InfoStyle.Render("🔧 Monitoring Agent:"))
	m.Report = append(m.Report, agentStatus)
	m.Report = append(m.Report, "")

	// If agent is running, fetch storage information
	if strings.Contains(agentStatus, "✅") {
		// Fetch storage health
		storageHealth := m.fetchStorageHealth()
		m.Report = append(m.Report, InfoStyle.Render("🏥 Storage Health:"))
		m.Report = append(m.Report, storageHealth...)
		m.Report = append(m.Report, "")

		// Fetch detailed storage statistics
		storageStats := m.fetchStorageStats()
		m.Report = append(m.Report, InfoStyle.Render("📊 Database Statistics:"))
		m.Report = append(m.Report, storageStats...)
		m.Report = append(m.Report, "")

		// Entity summary
		entities := m.fetchEntities()
		if len(entities) > 0 {
			m.Report = append(m.Report, InfoStyle.Render("📦 Entity Summary:"))
			m.Report = append(m.Report, entities...)
		}
	} else {
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Start monitoring agent with: ./crucible-monitor"))
		m.Report = append(m.Report, WarnStyle.Render("⚠️ Or use: make run-monitor"))
	}

	m.ProcessingMsg = ""
	return m, tea.ClearScreen
}

// checkMonitoringAgent checks if the monitoring agent is running
func (m Model) checkMonitoringAgent() string {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/health")
	if err != nil {
		return WarnStyle.Render("❌ Agent not running")
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return InfoStyle.Render("✅ Agent running on port 9090")
	}
	return WarnStyle.Render("❌ Agent unhealthy")
}

// fetchSystemMetrics fetches system metrics from the monitoring agent
func (m Model) fetchSystemMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/system")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch system metrics")}
	}
	defer resp.Body.Close()

	var metrics monitor.SystemMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse system metrics")}
	}

	var result []string

	// CPU metrics
	result = append(result, fmt.Sprintf("  CPU Usage: %.1f%% (User: %.1f%%, System: %.1f%%, I/O Wait: %.1f%%)",
		metrics.CPU.UsagePercent, metrics.CPU.UserPercent, metrics.CPU.SystemPercent, metrics.CPU.IOWaitPercent))

	// Memory metrics
	memUsedGB := float64(metrics.Memory.UsedBytes) / (1024 * 1024 * 1024)
	memTotalGB := float64(metrics.Memory.TotalBytes) / (1024 * 1024 * 1024)
	result = append(result, fmt.Sprintf("  Memory: %.1fGB/%.1fGB (%.1f%%) | Swap: %.1f%%",
		memUsedGB, memTotalGB, metrics.Memory.UsagePercent, metrics.Memory.SwapUsagePercent))

	// Load average
	result = append(result, fmt.Sprintf("  Load Average: %.2f, %.2f, %.2f",
		metrics.Load.Load1, metrics.Load.Load5, metrics.Load.Load15))

	// Disk usage for main partitions
	for _, disk := range metrics.Disk {
		if disk.MountPoint == "/" || disk.MountPoint == "/home" {
			usedGB := float64(disk.UsedBytes) / (1024 * 1024 * 1024)
			totalGB := float64(disk.TotalBytes) / (1024 * 1024 * 1024)
			result = append(result, fmt.Sprintf("  Disk %s: %.1fGB/%.1fGB (%.1f%%)",
				disk.MountPoint, usedGB, totalGB, disk.UsagePercent))
		}
	}

	// Network stats (top interfaces)
	for i, net := range metrics.Network {
		if i >= 2 { // Limit to top 2 interfaces
			break
		}
		recvMB := float64(net.BytesRecv) / (1024 * 1024)
		sentMB := float64(net.BytesSent) / (1024 * 1024)
		result = append(result, fmt.Sprintf("  Network %s: ↓%.1fMB ↑%.1fMB (Errors: %d)",
			net.Interface, recvMB, sentMB, net.ErrorsRecv+net.ErrorsSent))
	}

	return result
}

// fetchServiceMetrics fetches service status from the monitoring agent
func (m Model) fetchServiceMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/services")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch service metrics")}
	}
	defer resp.Body.Close()

	var services []monitor.ServiceStatus
	if err := json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse service metrics")}
	}

	var result []string

	// Group services by category
	categories := map[string][]monitor.ServiceStatus{
		"database":  {},
		"webserver": {},
		"runtime":   {},
		"security":  {},
		"system":    {},
	}

	for _, service := range services {
		category := "system"
		if cat, exists := service.Metadata["category"]; exists {
			category = cat
		}
		categories[category] = append(categories[category], service)
	}

	// Display important categories first
	for _, category := range []string{"database", "webserver", "runtime", "security"} {
		if len(categories[category]) > 0 {
			for _, service := range categories[category] {
				status := "❌"
				if service.Active == "active" && service.Sub == "running" {
					status = "✅"
				} else if service.Active == "active" {
					status = "⚠️"
				}

				uptime := time.Since(service.Since)
				result = append(result, fmt.Sprintf("  %s %s (%s) - Up: %s",
					status, service.Name, service.Sub, formatDuration(uptime)))
			}
		}
	}

	// Show count of other services
	otherCount := len(categories["system"])
	if otherCount > 0 {
		result = append(result, fmt.Sprintf("  + %d other system services", otherCount))
	}

	return result
}

// fetchHTTPMetrics fetches HTTP check results from the monitoring agent
func (m Model) fetchHTTPMetrics() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/metrics/http")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch HTTP metrics")}
	}
	defer resp.Body.Close()

	var checks []monitor.HTTPCheckResult
	if err := json.NewDecoder(resp.Body).Decode(&checks); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse HTTP metrics")}
	}

	if len(checks) == 0 {
		return []string{
			ChoiceStyle.Render("  No HTTP checks configured"),
			ChoiceStyle.Render("  Enable in configs/monitor.yaml to monitor web endpoints"),
		}
	}

	var result []string
	for _, check := range checks {
		status := "❌"
		if check.Success {
			status = "✅"
		}

		result = append(result, fmt.Sprintf("  %s %s - %dms (Status: %d)",
			status, check.Name, check.ResponseTime.Milliseconds(), check.StatusCode))

		if check.Error != "" {
			// Simplify connection refused errors
			errorMsg := check.Error
			if strings.Contains(errorMsg, "connection refused") {
				errorMsg = "Connection refused - service not running"
			} else if strings.Contains(errorMsg, "no such host") {
				errorMsg = "Host not found"
			} else if strings.Contains(errorMsg, "timeout") {
				errorMsg = "Request timeout"
			}
			result = append(result, fmt.Sprintf("    %s", errorMsg))
		}
	}

	return result
}

// fetchActiveAlerts fetches active alerts from the monitoring agent
func (m Model) fetchActiveAlerts() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/alerts")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch alerts")}
	}
	defer resp.Body.Close()

	// Define Alert struct for JSON unmarshaling (matching alerts package)
	type Alert struct {
		ID          string                 `json:"id"`
		Name        string                 `json:"name"`
		Type        string                 `json:"type"`
		Severity    string                 `json:"severity"`
		Status      string                 `json:"status"`
		Message     string                 `json:"message"`
		Details     map[string]interface{} `json:"details"`
		Labels      map[string]string      `json:"labels"`
		Annotations map[string]string      `json:"annotations"`
		StartsAt    time.Time              `json:"starts_at"`
		EndsAt      *time.Time             `json:"ends_at,omitempty"`
		LastSent    *time.Time             `json:"last_sent,omitempty"`
		RuleID      string                 `json:"rule_id"`
	}

	var alerts []Alert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse alerts")}
	}

	if len(alerts) == 0 {
		return []string{
			InfoStyle.Render("  ✅ No active alerts"),
			ChoiceStyle.Render("  System is healthy"),
		}
	}

	var result []string
	for _, alert := range alerts {
		// Get severity icon and color
		var severityIcon string
		var style = ChoiceStyle
		switch alert.Severity {
		case "critical":
			severityIcon = "🚨"
			style = WarnStyle
		case "warning":
			severityIcon = "⚠️"
			style = WarnStyle
		case "info":
			severityIcon = "ℹ️"
			style = InfoStyle
		default:
			severityIcon = "📋"
		}

		// Format alert duration
		duration := time.Since(alert.StartsAt)
		durationStr := formatDuration(duration)

		// Status indicator
		statusIcon := "🔥"
		if alert.Status == "acknowledged" {
			statusIcon = "✋"
		} else if alert.Status == "resolved" {
			statusIcon = "✅"
		}

		// Main alert line
		result = append(result, style.Render(fmt.Sprintf("  %s %s [%s] %s (%s ago)",
			statusIcon, severityIcon, strings.ToUpper(alert.Severity), alert.Name, durationStr)))

		// Alert message
		if alert.Message != "" {
			result = append(result, ChoiceStyle.Render(fmt.Sprintf("    💬 %s", alert.Message)))
		}

		// Show alert details if available
		if len(alert.Details) > 0 {
			for key, value := range alert.Details {
				result = append(result, ChoiceStyle.Render(fmt.Sprintf("    📊 %s: %v", key, value)))
			}
		}

		// Add spacing between alerts
		if len(alerts) > 1 {
			result = append(result, "")
		}
	}

	return result
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	} else {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// HISTORICAL DATA CLIENT FUNCTIONS

// fetchEntities fetches entities from the monitoring agent
func (m Model) fetchEntities() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/entities?limit=20")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch entities")}
	}
	defer resp.Body.Close()

	// Define response structure
	type EntityResponse struct {
		Entities []struct {
			ID       int64  `json:"id"`
			Type     string `json:"type"`
			Name     string `json:"name"`
			Status   string `json:"status"`
			LastSeen string `json:"last_seen"`
		} `json:"entities"`
		Count int `json:"count"`
	}

	var response EntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse entities")}
	}

	if len(response.Entities) == 0 {
		return []string{ChoiceStyle.Render("  No entities found in database")}
	}

	var result []string
	entityTypes := map[string]int{}

	for _, entity := range response.Entities {
		status := "❌"
		if entity.Status == "active" {
			status = "✅"
		} else if entity.Status == "inactive" {
			status = "⚠️"
		}

		// Parse last seen time
		lastSeen, err := time.Parse(time.RFC3339, entity.LastSeen)
		if err != nil {
			lastSeen = time.Now()
		}
		timeSince := time.Since(lastSeen)

		result = append(result, fmt.Sprintf("  %s %s [%s] %s (Last seen: %s ago)",
			status, entity.Type, entity.Name, entity.Status, formatDuration(timeSince)))

		entityTypes[entity.Type]++
	}

	// Add summary
	result = append(result, "")
	result = append(result, ChoiceStyle.Render("Entity Summary:"))
	for entityType, count := range entityTypes {
		result = append(result, fmt.Sprintf("  %s: %d", entityType, count))
	}

	return result
}

// fetchEvents fetches recent events from the monitoring agent
func (m Model) fetchEvents() []string {
	since := time.Now().Add(-m.MonitoringTimeRange.Duration())
	url := fmt.Sprintf("http://127.0.0.1:9090/api/v1/events?since=%s&limit=20", since.Format(time.RFC3339))

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch events")}
	}
	defer resp.Body.Close()

	// Define response structure
	type EventResponse struct {
		Events []struct {
			ID        int64     `json:"id"`
			EntityID  int64     `json:"entity_id"`
			Timestamp time.Time `json:"timestamp"`
			Type      string    `json:"event_type"`
			Severity  string    `json:"severity"`
			Message   string    `json:"message"`
		} `json:"events"`
		Count int `json:"count"`
	}

	var response EventResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse events")}
	}

	if len(response.Events) == 0 {
		return []string{ChoiceStyle.Render(fmt.Sprintf("  No events found in the last %s", m.MonitoringTimeRange.String()))}
	}

	var result []string
	for i, event := range response.Events {
		if i >= 15 { // Limit display to prevent overwhelming
			break
		}

		// Get severity icon
		var severityIcon string
		switch event.Severity {
		case "error":
			severityIcon = "🚨"
		case "warning":
			severityIcon = "⚠️"
		case "info":
			severityIcon = "ℹ️"
		default:
			severityIcon = "📋"
		}

		// Format time
		timeSince := time.Since(event.Timestamp)
		timeStr := formatDuration(timeSince)

		result = append(result, fmt.Sprintf("  %s [%s] %s (%s ago)",
			severityIcon, strings.ToUpper(event.Severity), event.Message, timeStr))
	}

	// Add summary
	if len(response.Events) > 15 {
		result = append(result, "")
		result = append(result, ChoiceStyle.Render(fmt.Sprintf("... and %d more events", len(response.Events)-15)))
	}

	return result
}

// fetchHistoricalMetrics fetches recent metrics for key entities
func (m Model) fetchHistoricalMetrics() []string {
	// Get entities first to find key ones
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/entities?type=server&limit=1")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch server entity")}
	}
	defer resp.Body.Close()

	type EntityResponse struct {
		Entities []struct {
			ID int64 `json:"id"`
		} `json:"entities"`
	}

	var entities EntityResponse
	if err := json.NewDecoder(resp.Body).Decode(&entities); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse entities")}
	}

	if len(entities.Entities) == 0 {
		return []string{ChoiceStyle.Render("  No server entity found")}
	}

	serverEntityID := entities.Entities[0].ID
	since := time.Now().Add(-m.MonitoringTimeRange.Duration())
	url := fmt.Sprintf("http://127.0.0.1:9090/api/v1/entities/%d/metrics?since=%s&limit=50",
		serverEntityID, since.Format(time.RFC3339))

	resp2, err := client.Get(url)
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch historical metrics")}
	}
	defer resp2.Body.Close()

	type MetricResponse struct {
		Metrics []struct {
			MetricName string    `json:"metric_name"`
			Value      float64   `json:"value"`
			Timestamp  time.Time `json:"timestamp"`
		} `json:"metrics"`
		Count int `json:"count"`
	}

	var metrics MetricResponse
	if err := json.NewDecoder(resp2.Body).Decode(&metrics); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse metrics")}
	}

	if len(metrics.Metrics) == 0 {
		return []string{ChoiceStyle.Render(fmt.Sprintf("  No metrics found in the last %s", m.MonitoringTimeRange.String()))}
	}

	// Group metrics by name and calculate averages
	metricGroups := make(map[string][]float64)
	latestTimes := make(map[string]time.Time)

	for _, metric := range metrics.Metrics {
		metricGroups[metric.MetricName] = append(metricGroups[metric.MetricName], metric.Value)
		if metric.Timestamp.After(latestTimes[metric.MetricName]) {
			latestTimes[metric.MetricName] = metric.Timestamp
		}
	}

	var result []string
	for metricName, values := range metricGroups {
		if len(values) == 0 {
			continue
		}

		// Calculate average
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		avg := sum / float64(len(values))

		// Get latest value
		latest := values[len(values)-1]

		// Format based on metric type
		var display string
		switch metricName {
		case "cpu_usage", "memory_usage", "disk_usage", "disk_usage_root":
			display = fmt.Sprintf("%.1f%% (avg: %.1f%%)", latest, avg)
		case "load_1", "load_5", "load_15":
			display = fmt.Sprintf("%.2f (avg: %.2f)", latest, avg)
		default:
			display = fmt.Sprintf("%.2f (avg: %.2f)", latest, avg)
		}

		timeSince := time.Since(latestTimes[metricName])
		result = append(result, fmt.Sprintf("  📊 %s: %s (%d samples, %s ago)",
			metricName, display, len(values), formatDuration(timeSince)))
	}

	if len(result) == 0 {
		return []string{ChoiceStyle.Render("  No metrics data available")}
	}

	return result
}

// fetchStorageHealth fetches storage health information
func (m Model) fetchStorageHealth() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/storage/health")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch storage health")}
	}
	defer resp.Body.Close()

	type HealthResponse struct {
		Status  string `json:"status"`
		Type    string `json:"type"`
		Message string `json:"message"`
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse storage health")}
	}

	var result []string
	status := "❌"
	if health.Status == "operational" {
		status = "✅"
	} else if health.Status == "degraded" {
		status = "⚠️"
	}

	result = append(result, fmt.Sprintf("  %s Storage Type: %s", status, health.Type))
	result = append(result, fmt.Sprintf("  %s Status: %s", status, health.Status))
	if health.Message != "" {
		result = append(result, fmt.Sprintf("  💬 %s", health.Message))
	}

	return result
}

// fetchStorageStats fetches detailed storage statistics
func (m Model) fetchStorageStats() []string {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/api/v1/storage/stats")
	if err != nil {
		return []string{WarnStyle.Render("❌ Failed to fetch storage stats")}
	}
	defer resp.Body.Close()

	type StatsResponse struct {
		DBVersion         string     `json:"db_version"`
		CrucibleVersion   string     `json:"crucible_version"`
		DatabaseSizeBytes int64      `json:"database_size_bytes"`
		EntitiesCount     int        `json:"entities_count"`
		EventsCount       int        `json:"events_count"`
		MetricsCount      int        `json:"metrics_count"`
		CreatedAt         time.Time  `json:"created_at"`
		UpdatedAt         time.Time  `json:"updated_at"`
		LastCleanup       *time.Time `json:"last_cleanup_timestamp"`
	}

	var stats StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return []string{WarnStyle.Render("❌ Failed to parse storage stats")}
	}

	var result []string

	// Database info
	result = append(result, fmt.Sprintf("  📁 Database Version: %s", stats.DBVersion))
	result = append(result, fmt.Sprintf("  🔧 Crucible Version: %s", stats.CrucibleVersion))

	// Size info
	sizeMB := float64(stats.DatabaseSizeBytes) / (1024 * 1024)
	result = append(result, fmt.Sprintf("  💾 Database Size: %.2f MB", sizeMB))

	// Record counts
	result = append(result, fmt.Sprintf("  📦 Entities: %d", stats.EntitiesCount))
	result = append(result, fmt.Sprintf("  📋 Events: %d", stats.EventsCount))
	result = append(result, fmt.Sprintf("  📊 Metrics: %d", stats.MetricsCount))

	// Timestamps
	createdAgo := time.Since(stats.CreatedAt)
	result = append(result, fmt.Sprintf("  🕐 Created: %s ago", formatDuration(createdAgo)))

	if stats.LastCleanup != nil {
		cleanupAgo := time.Since(*stats.LastCleanup)
		result = append(result, fmt.Sprintf("  🧹 Last Cleanup: %s ago", formatDuration(cleanupAgo)))
	} else {
		result = append(result, "  🧹 Last Cleanup: Never")
	}

	return result
}
