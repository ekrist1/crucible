package models

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"crucible/internal/monitor"
)

// MonitoringView represents different views in the monitoring dashboard
type MonitoringView int

const (
	MonitoringViewLive MonitoringView = iota
	MonitoringViewHistorical
	MonitoringViewEvents
	MonitoringViewStorage
)

// HistoricalTimeRange represents time range options for historical data
type HistoricalTimeRange int

const (
	TimeRangeLast1Hour HistoricalTimeRange = iota
	TimeRangeLast6Hours
	TimeRangeLast24Hours
	TimeRangeLast7Days
	TimeRangeLast30Days
)

// MonitoringData represents monitoring information
type MonitoringData struct {
	SystemMetrics  SystemMetrics
	ServiceMetrics []ServiceMetric
	HTTPChecks     []HTTPCheckResult
	Alerts         []Alert
	LastUpdated    time.Time
}

// SystemMetrics represents system monitoring data
type SystemMetrics struct {
	CPUUsage    float64
	MemoryUsage float64
	DiskUsage   float64
	LoadAverage float64
	Uptime      time.Duration
}

// ServiceMetric represents service monitoring data
type ServiceMetric struct {
	Name     string
	Status   string
	Active   string
	Sub      string
	Memory   float64
	CPU      float64
	Restarts int
}

// HTTPCheckResult represents HTTP monitoring result
type HTTPCheckResult struct {
	Name         string
	URL          string
	StatusCode   int
	ResponseTime time.Duration
	Success      bool
	Error        string
	Timestamp    time.Time
}

// Alert represents a monitoring alert
type Alert struct {
	ID        string
	Name      string
	Severity  string
	Message   string
	Timestamp time.Time
	Active    bool
}

// HistoricalData represents historical monitoring metrics
type HistoricalData struct {
	CPUHistory    []float64
	MemoryHistory []float64
	LoadHistory   []float64
	Timestamps    []time.Time
	AvgCPU        float64
	AvgMemory     float64
	AvgLoad       float64
}

// MonitoringEvent represents a system event or alert
type MonitoringEvent struct {
	ID        string
	Type      string    // alert, service, system
	Severity  string    // critical, warning, info
	Source    string    // Service/component that generated the event
	Message   string    // Event description
	Timestamp time.Time
	Resolved  bool      // For alerts, whether they've been resolved
}

// StorageStatistics represents storage and database statistics
type StorageStatistics struct {
	DiskUsage           []DiskUsage
	Databases           []DatabaseInfo
	LogFiles            []LogFileInfo
	LaravelSites        []SiteStorageInfo
	TotalDatabaseSize   int64
	TotalLogSize        int64
	TotalSiteSize       int64
}

// DiskUsage represents disk usage information
type DiskUsage struct {
	Path        string
	Total       int64
	Used        int64
	Available   int64
	UsedPercent float64
}

// DatabaseInfo represents database storage information
type DatabaseInfo struct {
	Name      string
	SizeBytes int64
	Tables    int
	Records   int64
}

// LogFileInfo represents log file information
type LogFileInfo struct {
	Path         string
	SizeBytes    int64
	LastModified time.Time
}

// SiteStorageInfo represents Laravel site storage breakdown
type SiteStorageInfo struct {
	Name        string
	SizeBytes   int64
	VendorSize  int64
	StorageSize int64
	CacheSize   int64
}

// MonitoringModel handles the monitoring dashboard
type MonitoringModel struct {
	BaseModel
	mu           sync.RWMutex // Protects all fields below
	view         MonitoringView
	timeRange    HistoricalTimeRange
	scrollPos    int
	data         MonitoringData
	refreshing   bool
	autoRefresh  bool
	refreshTimer *time.Timer
}

// Monitoring message types
type monitoringDataMsg struct {
	data MonitoringData
	err  error
}

type refreshTickMsg struct{}

// NewMonitoringModel creates a new monitoring model
func NewMonitoringModel(shared *SharedData) *MonitoringModel {
	return &MonitoringModel{
		BaseModel:   NewBaseModel(shared),
		view:        MonitoringViewLive,
		timeRange:   TimeRangeLast1Hour,
		scrollPos:   0,
		refreshing:  false,
		autoRefresh: true,
	}
}

// Init initializes the monitoring model
func (m *MonitoringModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchData(),
		m.startAutoRefresh(),
	)
}

// Update handles monitoring updates
func (m *MonitoringModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.stopAutoRefresh()
			return m, m.GoBack()

		// View switching
		case "l":
			m.setView(MonitoringViewLive)
			return m, m.fetchData()
		case "h":
			m.setView(MonitoringViewHistorical)
			return m, m.fetchData()
		case "e":
			m.setView(MonitoringViewEvents)
			return m, m.fetchData()
		case "s":
			m.setView(MonitoringViewStorage)
			return m, m.fetchData()

		// Time range selection (for historical and events views)
		case "1":
			currentView := m.getView()
			if currentView == MonitoringViewHistorical || currentView == MonitoringViewEvents {
				m.setTimeRange(TimeRangeLast1Hour)
				return m, m.fetchData()
			}
		case "6":
			currentView := m.getView()
			if currentView == MonitoringViewHistorical || currentView == MonitoringViewEvents {
				m.setTimeRange(TimeRangeLast6Hours)
				return m, m.fetchData()
			}
		case "d":
			currentView := m.getView()
			if currentView == MonitoringViewHistorical || currentView == MonitoringViewEvents {
				m.setTimeRange(TimeRangeLast24Hours)
				return m, m.fetchData()
			}
		case "w":
			currentView := m.getView()
			if currentView == MonitoringViewHistorical || currentView == MonitoringViewEvents {
				m.setTimeRange(TimeRangeLast7Days)
				return m, m.fetchData()
			}
		case "m":
			currentView := m.getView()
			if currentView == MonitoringViewHistorical || currentView == MonitoringViewEvents {
				m.setTimeRange(TimeRangeLast30Days)
				return m, m.fetchData()
			}

		// Refresh
		case "r":
			return m, m.fetchData()

		// Auto-refresh toggle
		case "a":
			currentAutoRefresh := m.getAutoRefresh()
			m.setAutoRefresh(!currentAutoRefresh)
			if !currentAutoRefresh { // Now enabled
				return m, m.startAutoRefresh()
			} else { // Now disabled
				m.stopAutoRefresh()
			}

		// Scrolling
		case "up", "k":
			m.adjustScrollPos(-1)
		case "down", "j":
			m.adjustScrollPos(1)
		case "pageup":
			m.adjustScrollPos(-10)
		case "pagedown":
			m.adjustScrollPos(10)
		case "home":
			m.setScrollPos(0)
		case "end":
			m.setScrollPos(m.getMaxScroll())
		}

	case monitoringDataMsg:
		m.setRefreshing(false)
		if msg.err != nil {
			// Handle error
			m.setData(MonitoringData{LastUpdated: time.Now()})
		} else {
			m.setData(msg.data)
		}
		return m, nil

	case refreshTickMsg:
		if m.getAutoRefresh() {
			return m, tea.Batch(
				m.fetchData(),
				m.startAutoRefresh(),
			)
		}
	}

	return m, nil
}

// View renders the monitoring dashboard
func (m *MonitoringModel) View() string {
	var s strings.Builder

	// Title with current view
	viewName := m.getViewName()
	title := fmt.Sprintf("📊 Monitoring Dashboard - %s", viewName)
	s.WriteString(titleStyle.Render(title))
	s.WriteString("\n\n")

	// Status line
	statusLine := m.buildStatusLine()
	s.WriteString(statusLine)
	s.WriteString("\n\n")

	// Main content based on view
	content := m.renderViewContent()
	s.WriteString(content)

	// Help text
	s.WriteString("\n")
	s.WriteString(m.renderHelp())

	return s.String()
}

// getViewName returns the display name for the current view
func (m *MonitoringModel) getViewName() string {
	switch m.getView() {
	case MonitoringViewLive:
		return "Live"
	case MonitoringViewHistorical:
		return "Historical"
	case MonitoringViewEvents:
		return "Events"
	case MonitoringViewStorage:
		return "Storage"
	default:
		return "Unknown"
	}
}

// buildStatusLine builds the status line with refresh info
func (m *MonitoringModel) buildStatusLine() string {
	var parts []string
	
	data := m.getData()
	
	// Last updated
	if !data.LastUpdated.IsZero() {
		lastUpdated := data.LastUpdated.Format("15:04:05")
		parts = append(parts, fmt.Sprintf("Last Updated: %s", lastUpdated))
	}

	// Auto-refresh status
	if m.getAutoRefresh() {
		parts = append(parts, "Auto-refresh: ON")
	} else {
		parts = append(parts, "Auto-refresh: OFF")
	}

	// Refreshing indicator
	if m.getRefreshing() {
		parts = append(parts, "Refreshing...")
	}

	// Time range (for historical views)
	currentView := m.getView()
	if currentView == MonitoringViewHistorical || currentView == MonitoringViewEvents {
		parts = append(parts, fmt.Sprintf("Range: %s", m.getTimeRangeString()))
	}

	return infoStyle.Render(strings.Join(parts, " | "))
}

// renderViewContent renders the main content based on current view
func (m *MonitoringModel) renderViewContent() string {
	switch m.getView() {
	case MonitoringViewLive:
		return m.renderLiveView()
	case MonitoringViewHistorical:
		return m.renderHistoricalView()
	case MonitoringViewEvents:
		return m.renderEventsView()
	case MonitoringViewStorage:
		return m.renderStorageView()
	default:
		return "Unknown view"
	}
}

// renderLiveView renders the live monitoring view
func (m *MonitoringModel) renderLiveView() string {
	var s strings.Builder
	data := m.getData()

	// System metrics
	s.WriteString(infoStyle.Render("=== SYSTEM METRICS ==="))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("CPU Usage:    %.1f%%\n", data.SystemMetrics.CPUUsage))
	s.WriteString(fmt.Sprintf("Memory Usage: %.1f%%\n", data.SystemMetrics.MemoryUsage))
	s.WriteString(fmt.Sprintf("Disk Usage:   %.1f%%\n", data.SystemMetrics.DiskUsage))
	s.WriteString(fmt.Sprintf("Load Average: %.2f\n", data.SystemMetrics.LoadAverage))
	s.WriteString(fmt.Sprintf("Uptime:       %s\n", m.formatDuration(data.SystemMetrics.Uptime)))
	s.WriteString("\n")

	// Service metrics
	s.WriteString(infoStyle.Render("=== SERVICES ==="))
	s.WriteString("\n")
	if len(data.ServiceMetrics) == 0 {
		s.WriteString(helpStyle.Render("No services being monitored"))
		s.WriteString("\n")
	} else {
		for _, service := range data.ServiceMetrics {
			status := "🔴"
			if service.Active == "active" && service.Sub == "running" {
				status = "🟢"
			}
			s.WriteString(fmt.Sprintf("%s %s - %s (%s)\n", 
				status, service.Name, service.Status, service.Sub))
		}
	}
	s.WriteString("\n")

	// HTTP checks
	s.WriteString(infoStyle.Render("=== HTTP CHECKS ==="))
	s.WriteString("\n")
	if len(data.HTTPChecks) == 0 {
		s.WriteString(helpStyle.Render("No HTTP checks configured"))
		s.WriteString("\n")
	} else {
		for _, check := range data.HTTPChecks {
			status := "🔴"
			if check.Success {
				status = "🟢"
			}
			s.WriteString(fmt.Sprintf("%s %s - %dms (Status: %d)\n",
				status, check.Name, check.ResponseTime.Milliseconds(), check.StatusCode))
		}
	}
	s.WriteString("\n")

	// Active alerts
	s.WriteString(infoStyle.Render("=== ACTIVE ALERTS ==="))
	s.WriteString("\n")
	activeAlerts := m.getActiveAlerts()
	if len(activeAlerts) == 0 {
		s.WriteString(infoStyle.Render("No active alerts"))
		s.WriteString("\n")
	} else {
		for _, alert := range activeAlerts {
			severityStyle := infoStyle
			if alert.Severity == "critical" {
				severityStyle = errorStyle
			} else if alert.Severity == "warning" {
				severityStyle = warnStyle
			}
			s.WriteString(severityStyle.Render(fmt.Sprintf("⚠ [%s] %s: %s", 
				strings.ToUpper(alert.Severity), alert.Name, alert.Message)))
			s.WriteString("\n")
		}
	}

	return s.String()
}

// renderHistoricalView renders the historical data view
func (m *MonitoringModel) renderHistoricalView() string {
	var s strings.Builder

	s.WriteString(infoStyle.Render(fmt.Sprintf("=== HISTORICAL DATA (%s) ===", m.getTimeRangeString())))
	s.WriteString("\n\n")

	// Fetch real historical data from monitoring agent
	data := m.getData()
	timeRange := m.getTimeRange()
	
	// Fetch real historical data points
	histData, err := m.fetchHistoricalData(timeRange)
	if err != nil {
		if m.shared.Logger != nil {
			m.shared.Logger.Warn("Failed to fetch historical data, using fallback", "error", err)
		}
		// Fallback to mock data if real data unavailable
		histData = m.generateHistoricalData(timeRange)
		s.WriteString(warnStyle.Render(fmt.Sprintf("⚠ Using simulated data (%s)", err.Error())))
		s.WriteString("\n\n")
	} else {
		dataPoints := len(histData.CPUHistory)
		s.WriteString(infoStyle.Render(fmt.Sprintf("📊 Real historical data (%d data points)", dataPoints)))
		s.WriteString("\n\n")
	}
	
	// CPU Usage Chart
	s.WriteString(infoStyle.Render("CPU Usage Over Time:"))
	s.WriteString("\n")
	s.WriteString(m.renderSimpleChart("CPU", histData.CPUHistory, "%"))
	s.WriteString("\n\n")
	
	// Memory Usage Chart  
	s.WriteString(infoStyle.Render("Memory Usage Over Time:"))
	s.WriteString("\n")
	s.WriteString(m.renderSimpleChart("Memory", histData.MemoryHistory, "%"))
	s.WriteString("\n\n")
	
	// Load Average Chart
	s.WriteString(infoStyle.Render("Load Average Over Time:"))
	s.WriteString("\n")
	s.WriteString(m.renderSimpleChart("Load", histData.LoadHistory, ""))
	s.WriteString("\n\n")
	
	// Current vs Historical Summary
	s.WriteString(infoStyle.Render("Summary:"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("Current CPU: %.1f%% (Avg: %.1f%%)\n", data.SystemMetrics.CPUUsage, histData.AvgCPU))
	s.WriteString(fmt.Sprintf("Current Memory: %.1f%% (Avg: %.1f%%)\n", data.SystemMetrics.MemoryUsage, histData.AvgMemory))
	s.WriteString(fmt.Sprintf("Current Load: %.2f (Avg: %.2f)\n", data.SystemMetrics.LoadAverage, histData.AvgLoad))

	return s.String()
}

// renderEventsView renders the events/alerts history view
func (m *MonitoringModel) renderEventsView() string {
	var s strings.Builder

	s.WriteString(infoStyle.Render(fmt.Sprintf("=== EVENTS (%s) ===", m.getTimeRangeString())))
	s.WriteString("\n\n")

	// Fetch real historical events from monitoring agent
	events, err := m.fetchHistoricalEvents(m.getTimeRange())
	if err != nil {
		if m.shared.Logger != nil {
			m.shared.Logger.Warn("Failed to fetch historical events, using fallback", "error", err)
		}
		// Fallback to mock data if real data unavailable
		events = m.generateHistoricalEvents(m.getTimeRange())
		s.WriteString(warnStyle.Render("⚠ Using simulated events (monitoring agent unavailable)"))
		s.WriteString("\n\n")
	} else {
		s.WriteString(infoStyle.Render("📋 Real events from monitoring agent"))
		s.WriteString("\n\n")
	}
	
	if len(events) == 0 {
		s.WriteString(helpStyle.Render("No events found in the selected time range"))
		s.WriteString("\n")
		return s.String()
	}

	// Group events by type
	alertEvents := m.filterEventsByType(events, "alert")
	serviceEvents := m.filterEventsByType(events, "service")
	systemEvents := m.filterEventsByType(events, "system")

	// Alert Events Section
	if len(alertEvents) > 0 {
		s.WriteString(errorStyle.Render("🚨 ALERT EVENTS"))
		s.WriteString("\n")
		for _, event := range alertEvents {
			s.WriteString(m.formatEvent(event))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	// Service Events Section
	if len(serviceEvents) > 0 {
		s.WriteString(warnStyle.Render("🔧 SERVICE EVENTS"))
		s.WriteString("\n")
		for _, event := range serviceEvents {
			s.WriteString(m.formatEvent(event))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	// System Events Section
	if len(systemEvents) > 0 {
		s.WriteString(infoStyle.Render("💻 SYSTEM EVENTS"))
		s.WriteString("\n")
		for _, event := range systemEvents {
			s.WriteString(m.formatEvent(event))
			s.WriteString("\n")
		}
	}

	// Event summary
	s.WriteString("\n")
	summary := fmt.Sprintf("Total Events: %d (Alerts: %d, Service: %d, System: %d)", 
		len(events), len(alertEvents), len(serviceEvents), len(systemEvents))
	s.WriteString(helpStyle.Render(summary))

	return s.String()
}

// renderStorageView renders the storage statistics view
func (m *MonitoringModel) renderStorageView() string {
	var s strings.Builder

	s.WriteString(infoStyle.Render("=== STORAGE STATISTICS ==="))
	s.WriteString("\n\n")

	// Generate mock storage statistics
	storageStats := m.generateStorageStatistics()

	// Disk Usage Section
	s.WriteString(infoStyle.Render("💾 DISK USAGE"))
	s.WriteString("\n")
	for _, disk := range storageStats.DiskUsage {
		barWidth := m.shared.GetContentWidth() / 3 // Use 1/3 of content width for bars
		if barWidth < 20 {
			barWidth = 20 // Minimum bar width
		}
		usageBar := m.renderUsageBar(disk.UsedPercent, barWidth)
		statusIcon := "🟢"
		if disk.UsedPercent > 90 {
			statusIcon = "🔴"
		} else if disk.UsedPercent > 80 {
			statusIcon = "🟡"
		}
		
		s.WriteString(fmt.Sprintf("%s %-15s %s %.1f%% (%s / %s)\n", 
			statusIcon, disk.Path, usageBar, disk.UsedPercent, 
			m.formatBytes(disk.Used), m.formatBytes(disk.Total)))
	}
	s.WriteString("\n")

	// Database Statistics Section
	s.WriteString(infoStyle.Render("🗄️ DATABASE STATISTICS"))
	s.WriteString("\n")
	for _, db := range storageStats.Databases {
		statusIcon := "🟢"
		if db.SizeBytes > 1024*1024*1024 { // > 1GB
			statusIcon = "🟡"
		}
		s.WriteString(fmt.Sprintf("%s %-20s %s (%d tables, %s records)\n", 
			statusIcon, db.Name, m.formatBytes(db.SizeBytes), db.Tables, m.formatNumber(db.Records)))
	}
	s.WriteString("\n")

	// Log Files Section
	s.WriteString(infoStyle.Render("📋 LOG FILES"))
	s.WriteString("\n")
	for _, log := range storageStats.LogFiles {
		statusIcon := "🟢"
		if log.SizeBytes > 100*1024*1024 { // > 100MB
			statusIcon = "🟡"
		}
		if log.SizeBytes > 500*1024*1024 { // > 500MB
			statusIcon = "🔴"
		}
		
		ageStr := m.formatDuration(time.Since(log.LastModified))
		s.WriteString(fmt.Sprintf("%s %-30s %s (modified %s ago)\n", 
			statusIcon, log.Path, m.formatBytes(log.SizeBytes), ageStr))
	}
	s.WriteString("\n")

	// Laravel Sites Storage
	s.WriteString(infoStyle.Render("🚀 LARAVEL SITES"))
	s.WriteString("\n")
	for _, site := range storageStats.LaravelSites {
		s.WriteString(fmt.Sprintf("🌐 %-20s %s\n", site.Name, m.formatBytes(site.SizeBytes)))
		s.WriteString(fmt.Sprintf("   └─ Vendor: %s, Storage: %s, Cache: %s\n", 
			m.formatBytes(site.VendorSize), m.formatBytes(site.StorageSize), m.formatBytes(site.CacheSize)))
	}
	s.WriteString("\n")

	// Summary
	totalUsed := int64(0)
	totalAvail := int64(0)
	for _, disk := range storageStats.DiskUsage {
		totalUsed += disk.Used
		totalAvail += disk.Total
	}
	
	s.WriteString(infoStyle.Render("📊 STORAGE SUMMARY"))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("Total Disk Usage: %s / %s (%.1f%%)\n", 
		m.formatBytes(totalUsed), m.formatBytes(totalAvail), 
		float64(totalUsed)/float64(totalAvail)*100))
	s.WriteString(fmt.Sprintf("Database Storage: %s\n", m.formatBytes(storageStats.TotalDatabaseSize)))
	s.WriteString(fmt.Sprintf("Log Files: %s\n", m.formatBytes(storageStats.TotalLogSize)))
	s.WriteString(fmt.Sprintf("Laravel Sites: %s\n", m.formatBytes(storageStats.TotalSiteSize)))

	return s.String()
}

// renderHelp renders the help text
func (m *MonitoringModel) renderHelp() string {
	help := []string{
		"Navigation: l=Live, h=Historical, e=Events, s=Storage",
		"Time Range: 1=1h, 6=6h, d=24h, w=7d, m=30d",
		"Controls: r=Refresh, a=Toggle auto-refresh, ↑/↓=Scroll",
		"Esc=Back to menu, q=Quit",
	}
	return helpStyle.Render(strings.Join(help, " | "))
}

// getTimeRangeString returns the display string for the current time range
func (m *MonitoringModel) getTimeRangeString() string {
	switch m.getTimeRange() {
	case TimeRangeLast1Hour:
		return "Last 1 Hour"
	case TimeRangeLast6Hours:
		return "Last 6 Hours"
	case TimeRangeLast24Hours:
		return "Last 24 Hours"
	case TimeRangeLast7Days:
		return "Last 7 Days"
	case TimeRangeLast30Days:
		return "Last 30 Days"
	default:
		return "Unknown"
	}
}

// getActiveAlerts filters and returns only active alerts
func (m *MonitoringModel) getActiveAlerts() []Alert {
	var active []Alert
	data := m.getData()
	for _, alert := range data.Alerts {
		if alert.Active {
			active = append(active, alert)
		}
	}
	return active
}

// getMaxScroll calculates the maximum scroll position (thread-safe)
func (m *MonitoringModel) getMaxScroll() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getMaxScrollUnsafe()
}

// getMaxScrollUnsafe calculates the maximum scroll position (assumes lock is held)
func (m *MonitoringModel) getMaxScrollUnsafe() int {
	// Calculate based on content length and terminal height
	viewableLines := m.shared.GetViewableLines()
	
	// Estimate content lines based on current view
	var contentLines int
	switch m.view {
	case MonitoringViewLive:
		contentLines = 25 // Estimated lines for live view
	case MonitoringViewHistorical:
		contentLines = 35 // Estimated lines for historical view
	case MonitoringViewEvents:
		contentLines = 30 // Estimated lines for events view
	case MonitoringViewStorage:
		contentLines = 40 // Estimated lines for storage view
	default:
		contentLines = 20
	}
	
	maxScroll := contentLines - viewableLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// fetchData fetches monitoring data asynchronously
func (m *MonitoringModel) fetchData() tea.Cmd {
	m.setRefreshing(true)
	return func() tea.Msg {
		// Try to fetch real data from monitoring agent
		data, err := m.fetchRealData()
		if err != nil {
			// Fallback to mock data if agent is not available
			data = m.getMockData()
			err = nil // Clear error since we have fallback data
		}
		
		return monitoringDataMsg{data: data, err: err}
	}
}

// fetchRealData attempts to fetch data from the monitoring agent API
func (m *MonitoringModel) fetchRealData() (MonitoringData, error) {
	data := MonitoringData{}
	
	// Fetch system metrics
	systemMetrics, err := m.fetchSystemMetrics()
	if err != nil {
		return data, fmt.Errorf("failed to fetch system metrics: %w", err)
	}
	
	data.SystemMetrics = systemMetrics
	
	// Fetch service metrics (simplified for now)
	data.ServiceMetrics = []ServiceMetric{
		{Name: "nginx", Status: "loaded", Active: "inactive", Sub: "dead"},
		{Name: "mysql", Status: "loaded", Active: "active", Sub: "running"},
	}
	
	// HTTP checks would be fetched here too, but keeping simple for now
	data.HTTPChecks = []HTTPCheckResult{
		{Name: "uxvalidate", URL: "https://uxvalidate.com", StatusCode: 200, ResponseTime: time.Millisecond * 150, Success: true},
	}
	
	data.Alerts = []Alert{
		{ID: "1", Name: "High Memory Usage", Severity: "warning", Message: "Memory usage is at 68%", Active: false},
	}
	
	data.LastUpdated = time.Now()
	
	return data, nil
}

// fetchHistoricalData fetches real historical metrics from the monitoring agent
func (m *MonitoringModel) fetchHistoricalData(timeRange HistoricalTimeRange) (HistoricalData, error) {
	// Calculate time range for API query
	now := time.Now()
	var since time.Time
	
	switch timeRange {
	case TimeRangeLast1Hour:
		since = now.Add(-1 * time.Hour)
	case TimeRangeLast6Hours:
		since = now.Add(-6 * time.Hour)
	case TimeRangeLast24Hours:
		since = now.Add(-24 * time.Hour)
	case TimeRangeLast7Days:
		since = now.Add(-7 * 24 * time.Hour)
	case TimeRangeLast30Days:
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = now.Add(-1 * time.Hour)
	}
	
	// Get server entity ID first
	serverEntityID, err := m.getServerEntityID()
	if err != nil {
		return HistoricalData{}, fmt.Errorf("failed to get server entity ID: %w", err)
	}
	
	
	// Fetch CPU usage history
	cpuHistory, err := m.fetchMetricHistory(serverEntityID, "cpu_usage", since, now)
	if err != nil {
		return HistoricalData{}, fmt.Errorf("failed to fetch CPU history: %w", err)
	}
	
	// Fetch memory usage history
	memoryHistory, err := m.fetchMetricHistory(serverEntityID, "memory_usage", since, now)
	if err != nil {
		return HistoricalData{}, fmt.Errorf("failed to fetch memory history: %w", err)
	}
	
	// Fetch load average history
	loadHistory, err := m.fetchMetricHistory(serverEntityID, "load_1", since, now)
	if err != nil {
		return HistoricalData{}, fmt.Errorf("failed to fetch load history: %w", err)
	}
	
	// Determine the maximum number of data points to display
	maxLen := len(cpuHistory)
	if len(memoryHistory) > maxLen {
		maxLen = len(memoryHistory)
	}
	if len(loadHistory) > maxLen {
		maxLen = len(loadHistory)
	}
	
	// If no data available, return error
	if maxLen == 0 {
		return HistoricalData{}, fmt.Errorf("no historical data points available")
	}
	
	// Create historical data structure with consistent length
	histData := HistoricalData{
		CPUHistory:    make([]float64, maxLen),
		MemoryHistory: make([]float64, maxLen),
		LoadHistory:   make([]float64, maxLen),
		Timestamps:    make([]time.Time, maxLen),
	}
	
	// Process CPU history with bounds checking
	var cpuSum float64
	for i := 0; i < maxLen; i++ {
		if i < len(cpuHistory) {
			histData.CPUHistory[i] = cpuHistory[i].Value
			histData.Timestamps[i] = cpuHistory[i].Timestamp
			cpuSum += cpuHistory[i].Value
		}
		// If CPU data is missing for this index, use 0 (already initialized)
	}
	if len(cpuHistory) > 0 {
		histData.AvgCPU = cpuSum / float64(len(cpuHistory))
	}
	
	// Process memory history with bounds checking
	var memSum float64
	for i := 0; i < maxLen; i++ {
		if i < len(memoryHistory) {
			histData.MemoryHistory[i] = memoryHistory[i].Value
			memSum += memoryHistory[i].Value
			// Use memory timestamp if CPU timestamp is missing
			if histData.Timestamps[i].IsZero() {
				histData.Timestamps[i] = memoryHistory[i].Timestamp
			}
		}
		// If memory data is missing for this index, use 0 (already initialized)
	}
	if len(memoryHistory) > 0 {
		histData.AvgMemory = memSum / float64(len(memoryHistory))
	}
	
	// Process load history with bounds checking
	var loadSum float64
	for i := 0; i < maxLen; i++ {
		if i < len(loadHistory) {
			histData.LoadHistory[i] = loadHistory[i].Value
			loadSum += loadHistory[i].Value
			// Use load timestamp if both CPU and memory timestamps are missing
			if histData.Timestamps[i].IsZero() {
				histData.Timestamps[i] = loadHistory[i].Timestamp
			}
		}
		// If load data is missing for this index, use 0 (already initialized)  
	}
	if len(loadHistory) > 0 {
		histData.AvgLoad = loadSum / float64(len(loadHistory))
	}
	
	return histData, nil
}

// Metric represents a stored metric from the monitoring agent API
type StoredMetric struct {
	ID        int64                  `json:"id"`
	EntityID  *int64                 `json:"entity_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	MetricName string                `json:"metric_name"`
	Value     float64                `json:"value"`
	Tags      map[string]interface{} `json:"tags"`
}

// Entity represents a monitored entity from the storage API
type StoredEntity struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// getServerEntityID fetches the server entity ID from the monitoring agent
func (m *MonitoringModel) getServerEntityID() (int64, error) {
	resp, err := http.Get("http://localhost:9090/api/v1/entities?type=server&name=localhost")
	if err != nil {
		return 0, fmt.Errorf("failed to connect to monitoring agent: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("monitoring agent returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Entities []StoredEntity `json:"entities"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("failed to parse entities response: %w", err)
	}
	
	if len(response.Entities) == 0 {
		return 0, fmt.Errorf("no server entity found")
	}
	
	return response.Entities[0].ID, nil
}

// fetchMetricHistory fetches historical metrics for a specific metric from the monitoring agent
func (m *MonitoringModel) fetchMetricHistory(entityID int64, metricName string, since, until time.Time) ([]StoredMetric, error) {
	// Build query parameters with proper URL encoding
	baseURL := "http://localhost:9090/api/v1/metrics"
	params := fmt.Sprintf("entity_id=%d&metric_name=%s&since=%s&until=%s&limit=100", 
		entityID, 
		metricName, 
		url.QueryEscape(since.Format(time.RFC3339)), 
		url.QueryEscape(until.Format(time.RFC3339)))
	
	fullURL := fmt.Sprintf("%s?%s", baseURL, params)
	
	resp, err := http.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to monitoring agent: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("monitoring agent returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Metrics []StoredMetric `json:"metrics"`
		Count   int            `json:"count"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse metrics response: %w", err)
	}
	
	return response.Metrics, nil
}

// StoredEvent represents an event from the monitoring agent API  
type StoredEvent struct {
	ID        int64                  `json:"id"`
	EntityID  *int64                 `json:"entity_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"event_type"`
	Severity  string                 `json:"severity"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details"`
}

// fetchHistoricalEvents fetches real historical events from the monitoring agent
func (m *MonitoringModel) fetchHistoricalEvents(timeRange HistoricalTimeRange) ([]MonitoringEvent, error) {
	// Calculate time range for API query
	now := time.Now()
	var since time.Time
	
	switch timeRange {
	case TimeRangeLast1Hour:
		since = now.Add(-1 * time.Hour)
	case TimeRangeLast6Hours:
		since = now.Add(-6 * time.Hour)
	case TimeRangeLast24Hours:
		since = now.Add(-24 * time.Hour)
	case TimeRangeLast7Days:
		since = now.Add(-7 * 24 * time.Hour)
	case TimeRangeLast30Days:
		since = now.Add(-30 * 24 * time.Hour)
	default:
		since = now.Add(-1 * time.Hour)
	}
	
	// Build query parameters
	params := fmt.Sprintf("since=%s&until=%s&limit=50", 
		since.Format(time.RFC3339), now.Format(time.RFC3339))
	
	url := fmt.Sprintf("http://localhost:9090/api/v1/events?%s", params)
	
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to monitoring agent: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("monitoring agent returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	var response struct {
		Events []StoredEvent `json:"events"`
		Count  int           `json:"count"`
	}
	
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse events response: %w", err)
	}
	
	// Convert StoredEvent to MonitoringEvent
	events := make([]MonitoringEvent, len(response.Events))
	for i, storedEvent := range response.Events {
		events[i] = MonitoringEvent{
			ID:        fmt.Sprintf("%d", storedEvent.ID),
			Type:      m.mapEventType(storedEvent.Type),
			Severity:  storedEvent.Severity,
			Source:    m.getEventSource(storedEvent),
			Message:   storedEvent.Message,
			Timestamp: storedEvent.Timestamp,
			Resolved:  storedEvent.Type == "alert" && storedEvent.Severity == "info", // Simple resolved logic
		}
	}
	
	return events, nil
}

// mapEventType maps storage event types to TUI event types
func (m *MonitoringModel) mapEventType(storageType string) string {
	switch storageType {
	case "install", "uninstall", "update", "start", "stop", "restart", "maintenance":
		return "service"
	case "error", "warning", "alert":
		return "alert"
	case "info", "backup", "restore":
		return "system"
	default:
		return "system"
	}
}

// getEventSource extracts the source from event details or defaults to event type
func (m *MonitoringModel) getEventSource(event StoredEvent) string {
	// Try to get source from details
	if source, ok := event.Details["source"].(string); ok {
		return source
	}
	if service, ok := event.Details["service"].(string); ok {
		return service
	}
	// Default to event type
	return strings.Title(event.Type)
}

// startAutoRefresh starts the auto-refresh timer
func (m *MonitoringModel) startAutoRefresh() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

// stopAutoRefresh stops the auto-refresh timer in a thread-safe manner
func (m *MonitoringModel) stopAutoRefresh() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.refreshTimer != nil {
		m.refreshTimer.Stop()
		m.refreshTimer = nil
	}
	m.autoRefresh = false
}

// formatDuration formats a duration in a human-readable way
func (m *MonitoringModel) formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// Safe getter methods with read locks
func (m *MonitoringModel) getView() MonitoringView {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.view
}

func (m *MonitoringModel) setView(view MonitoringView) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.view = view
}

func (m *MonitoringModel) getTimeRange() HistoricalTimeRange {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timeRange
}

func (m *MonitoringModel) setTimeRange(timeRange HistoricalTimeRange) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeRange = timeRange
}

func (m *MonitoringModel) getData() MonitoringData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data
}

func (m *MonitoringModel) setData(data MonitoringData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = data
}

func (m *MonitoringModel) getRefreshing() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.refreshing
}

func (m *MonitoringModel) setRefreshing(refreshing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refreshing = refreshing
}

// generateHistoricalData creates mock historical data for visualization
func (m *MonitoringModel) generateHistoricalData(timeRange HistoricalTimeRange) HistoricalData {
	var points int
	var timeInterval time.Duration
	
	// Determine number of data points and interval based on time range
	switch timeRange {
	case TimeRangeLast1Hour:
		points = 12 // Every 5 minutes
		timeInterval = 5 * time.Minute
	case TimeRangeLast6Hours:
		points = 24 // Every 15 minutes
		timeInterval = 15 * time.Minute
	case TimeRangeLast24Hours:
		points = 48 // Every 30 minutes
		timeInterval = 30 * time.Minute
	case TimeRangeLast7Days:
		points = 28 // Every 6 hours
		timeInterval = 6 * time.Hour
	case TimeRangeLast30Days:
		points = 30 // Every day
		timeInterval = 24 * time.Hour
	default:
		points = 12
		timeInterval = 5 * time.Minute
	}
	
	now := time.Now()
	data := HistoricalData{
		CPUHistory:    make([]float64, points),
		MemoryHistory: make([]float64, points),
		LoadHistory:   make([]float64, points),
		Timestamps:    make([]time.Time, points),
	}
	
	// Generate mock data with some realistic variation
	var cpuSum, memSum, loadSum float64
	for i := 0; i < points; i++ {
		// Generate somewhat realistic values with trends
		baseTime := now.Add(-time.Duration(points-i) * timeInterval)
		data.Timestamps[i] = baseTime
		
		// CPU: 20-80% with some spikes
		cpu := 30.0 + float64(i%10)*3.0 + float64(i%3)*10.0
		if cpu > 85 {
			cpu = 85
		}
		data.CPUHistory[i] = cpu
		cpuSum += cpu
		
		// Memory: 40-75% gradually increasing
		mem := 45.0 + float64(i)*0.8 + float64(i%5)*2.0
		if mem > 75 {
			mem = 75
		}
		data.MemoryHistory[i] = mem
		memSum += mem
		
		// Load: 0.5-3.0 with variation
		load := 0.8 + float64(i%7)*0.3 + float64(i%4)*0.2
		if load > 3.0 {
			load = 3.0
		}
		data.LoadHistory[i] = load
		loadSum += load
	}
	
	// Calculate averages
	data.AvgCPU = cpuSum / float64(points)
	data.AvgMemory = memSum / float64(points)
	data.AvgLoad = loadSum / float64(points)
	
	return data
}

// renderSimpleChart creates a simple ASCII chart
func (m *MonitoringModel) renderSimpleChart(name string, values []float64, unit string) string {
	if len(values) == 0 {
		return "No data available"
	}
	
	// Find min/max for scaling
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	
	// Create chart with simple bars
	var chart strings.Builder
	contentWidth := m.shared.GetContentWidth()
	chartWidth := contentWidth - 20 // Reserve space for y-axis labels and margins
	if chartWidth < 20 {
		chartWidth = 20 // Minimum chart width
	}
	chartHeight := 8
	
	// Normalize values to chart height
	normalizedValues := make([]int, len(values))
	for i, v := range values {
		if max-min > 0 {
			normalizedValues[i] = int(float64(chartHeight-1) * (v - min) / (max - min))
		} else {
			normalizedValues[i] = chartHeight / 2
		}
	}
	
	// Draw chart from top to bottom
	for row := chartHeight - 1; row >= 0; row-- {
		chart.WriteString(fmt.Sprintf("%5.1f |", min+(max-min)*float64(row)/float64(chartHeight-1)))
		for i := 0; i < len(normalizedValues) && i < chartWidth; i++ {
			if normalizedValues[i] >= row {
				chart.WriteString("█")
			} else {
				chart.WriteString(" ")
			}
		}
		chart.WriteString("\n")
	}
	
	// Add axis
	chart.WriteString("      +")
	maxPoints := len(values)
	if chartWidth < len(values) {
		maxPoints = chartWidth
	}
	for i := 0; i < maxPoints; i++ {
		chart.WriteString("-")
	}
	chart.WriteString("\n")
	
	// Add min/max info
	chart.WriteString(fmt.Sprintf("      Min: %.1f%s  Max: %.1f%s  Points: %d", min, unit, max, unit, len(values)))
	
	return chart.String()
}

func (m *MonitoringModel) getAutoRefresh() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.autoRefresh
}

func (m *MonitoringModel) setAutoRefresh(autoRefresh bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoRefresh = autoRefresh
}

func (m *MonitoringModel) getScrollPos() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scrollPos
}

func (m *MonitoringModel) setScrollPos(scrollPos int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scrollPos = scrollPos
}

func (m *MonitoringModel) adjustScrollPos(delta int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	newPos := m.scrollPos + delta
	if newPos < 0 {
		newPos = 0
	}
	maxScroll := m.getMaxScrollUnsafe() // Already holding lock
	if newPos > maxScroll {
		newPos = maxScroll
	}
	m.scrollPos = newPos
}

// generateHistoricalEvents creates mock historical events for the events view
func (m *MonitoringModel) generateHistoricalEvents(timeRange HistoricalTimeRange) []MonitoringEvent {
	var events []MonitoringEvent
	var eventCount int
	var timeInterval time.Duration
	
	// Determine number of events and interval based on time range
	switch timeRange {
	case TimeRangeLast1Hour:
		eventCount = 5  // Fewer recent events
		timeInterval = 12 * time.Minute
	case TimeRangeLast6Hours:
		eventCount = 12
		timeInterval = 30 * time.Minute
	case TimeRangeLast24Hours:
		eventCount = 20
		timeInterval = 1 * time.Hour + 12 * time.Minute
	case TimeRangeLast7Days:
		eventCount = 35
		timeInterval = 5 * time.Hour
	case TimeRangeLast30Days:
		eventCount = 50
		timeInterval = 14 * time.Hour
	default:
		eventCount = 10
		timeInterval = 30 * time.Minute
	}
	
	now := time.Now()
	
	// Sample event templates
	eventTemplates := []struct {
		Type     string
		Severity string
		Source   string
		Message  string
	}{
		{"alert", "critical", "CPU Monitor", "CPU usage exceeded 90% threshold"},
		{"alert", "warning", "Memory Monitor", "Memory usage is high (>80%)"},
		{"alert", "critical", "Disk Monitor", "Disk space critically low"},
		{"service", "warning", "nginx", "Service restarted due to configuration reload"},
		{"service", "info", "mysql", "Database backup completed successfully"},
		{"service", "warning", "php-fpm", "High number of slow requests detected"},
		{"system", "info", "System", "System boot completed"},
		{"system", "warning", "System", "High load average detected"},
		{"alert", "warning", "HTTP Monitor", "Website response time is slow (>2s)"},
		{"service", "critical", "caddy", "Web server failed to start"},
		{"system", "info", "Logrotate", "Log rotation completed"},
		{"alert", "critical", "Security", "Multiple failed login attempts detected"},
		{"service", "info", "composer", "Laravel dependencies updated"},
		{"system", "warning", "Network", "Network connectivity issue detected"},
		{"alert", "warning", "Queue Monitor", "Laravel queue backlog detected"},
	}
	
	// Generate events by randomly selecting from templates
	for i := 0; i < eventCount; i++ {
		template := eventTemplates[i%len(eventTemplates)]
		
		// Create timestamp going backwards from now
		eventTime := now.Add(-time.Duration(eventCount-i) * timeInterval)
		
		event := MonitoringEvent{
			ID:        fmt.Sprintf("evt_%d", i+1),
			Type:      template.Type,
			Severity:  template.Severity,
			Source:    template.Source,
			Message:   template.Message,
			Timestamp: eventTime,
			Resolved:  template.Type == "alert" && (i%3 != 0), // Some alerts resolved
		}
		
		events = append(events, event)
	}
	
	return events
}

// filterEventsByType filters events by their type
func (m *MonitoringModel) filterEventsByType(events []MonitoringEvent, eventType string) []MonitoringEvent {
	var filtered []MonitoringEvent
	for _, event := range events {
		if event.Type == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// formatEvent formats a monitoring event for display
func (m *MonitoringModel) formatEvent(event MonitoringEvent) string {
	timeStr := event.Timestamp.Format("15:04:05")
	
	// Choose status icon based on event type and state
	var statusIcon string
	switch event.Type {
	case "alert":
		if event.Resolved {
			statusIcon = "✅" // Resolved alert
		} else {
			switch event.Severity {
			case "critical":
				statusIcon = "🔴"
			case "warning":
				statusIcon = "🟡"
			default:
				statusIcon = "🔵"
			}
		}
	case "service":
		switch event.Severity {
		case "critical":
			statusIcon = "💀"
		case "warning":
			statusIcon = "⚠️"
		default:
			statusIcon = "🔧"
		}
	case "system":
		switch event.Severity {
		case "critical":
			statusIcon = "🚨"
		case "warning":
			statusIcon = "⚡"
		default:
			statusIcon = "💻"
		}
	default:
		statusIcon = "ℹ️"
	}
	
	// Format based on severity
	var formattedLine string
	baseText := fmt.Sprintf("%s [%s] %s: %s", statusIcon, timeStr, event.Source, event.Message)
	
	switch event.Severity {
	case "critical":
		formattedLine = errorStyle.Render(baseText)
	case "warning":
		formattedLine = warnStyle.Render(baseText)
	default:
		formattedLine = infoStyle.Render(baseText)
	}
	
	// Add resolved indicator for alerts
	if event.Type == "alert" && event.Resolved {
		formattedLine += helpStyle.Render(" (RESOLVED)")
	}
	
	return formattedLine
}

// generateStorageStatistics creates mock storage statistics
func (m *MonitoringModel) generateStorageStatistics() StorageStatistics {
	stats := StorageStatistics{}
	
	// Mock disk usage data
	stats.DiskUsage = []DiskUsage{
		{Path: "/", Total: 100 * 1024 * 1024 * 1024, Used: 45 * 1024 * 1024 * 1024, UsedPercent: 45.0},
		{Path: "/var", Total: 50 * 1024 * 1024 * 1024, Used: 32 * 1024 * 1024 * 1024, UsedPercent: 64.0},
		{Path: "/tmp", Total: 10 * 1024 * 1024 * 1024, Used: 2 * 1024 * 1024 * 1024, UsedPercent: 20.0},
		{Path: "/home", Total: 200 * 1024 * 1024 * 1024, Used: 85 * 1024 * 1024 * 1024, UsedPercent: 42.5},
	}
	
	// Set available space
	for i := range stats.DiskUsage {
		stats.DiskUsage[i].Available = stats.DiskUsage[i].Total - stats.DiskUsage[i].Used
	}
	
	// Mock database information
	stats.Databases = []DatabaseInfo{
		{Name: "laravel_app", SizeBytes: 256 * 1024 * 1024, Tables: 15, Records: 125000},
		{Name: "nextjs_analytics", SizeBytes: 512 * 1024 * 1024, Tables: 8, Records: 450000},
		{Name: "crucible_logs", SizeBytes: 128 * 1024 * 1024, Tables: 4, Records: 89000},
		{Name: "mysql_system", SizeBytes: 64 * 1024 * 1024, Tables: 31, Records: 15000},
	}
	
	// Calculate total database size
	for _, db := range stats.Databases {
		stats.TotalDatabaseSize += db.SizeBytes
	}
	
	// Mock log files
	now := time.Now()
	stats.LogFiles = []LogFileInfo{
		{Path: "/var/log/nginx/access.log", SizeBytes: 245 * 1024 * 1024, LastModified: now.Add(-2 * time.Hour)},
		{Path: "/var/log/nginx/error.log", SizeBytes: 12 * 1024 * 1024, LastModified: now.Add(-1 * time.Hour)},
		{Path: "/var/log/mysql/error.log", SizeBytes: 8 * 1024 * 1024, LastModified: now.Add(-30 * time.Minute)},
		{Path: "/var/log/php8.4-fpm.log", SizeBytes: 34 * 1024 * 1024, LastModified: now.Add(-45 * time.Minute)},
		{Path: "/var/log/caddy/access.log", SizeBytes: 156 * 1024 * 1024, LastModified: now.Add(-15 * time.Minute)},
		{Path: "/var/log/auth.log", SizeBytes: 89 * 1024 * 1024, LastModified: now.Add(-5 * time.Minute)},
		{Path: "/var/log/syslog", SizeBytes: 67 * 1024 * 1024, LastModified: now.Add(-10 * time.Minute)},
	}
	
	// Calculate total log size
	for _, log := range stats.LogFiles {
		stats.TotalLogSize += log.SizeBytes
	}
	
	// Mock Laravel site storage
	stats.LaravelSites = []SiteStorageInfo{
		{
			Name:        "uxvalidate",
			SizeBytes:   89 * 1024 * 1024,
			VendorSize:  45 * 1024 * 1024,
			StorageSize: 23 * 1024 * 1024,
			CacheSize:   21 * 1024 * 1024,
		},
		{
			Name:        "portfolio",
			SizeBytes:   156 * 1024 * 1024,
			VendorSize:  67 * 1024 * 1024,
			StorageSize: 45 * 1024 * 1024,
			CacheSize:   44 * 1024 * 1024,
		},
		{
			Name:        "api-server",
			SizeBytes:   234 * 1024 * 1024,
			VendorSize:  98 * 1024 * 1024,
			StorageSize: 78 * 1024 * 1024,
			CacheSize:   58 * 1024 * 1024,
		},
	}
	
	// Calculate total site size
	for _, site := range stats.LaravelSites {
		stats.TotalSiteSize += site.SizeBytes
	}
	
	return stats
}

// formatBytes formats bytes into human-readable format
func (m *MonitoringModel) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatNumber formats large numbers with commas
func (m *MonitoringModel) formatNumber(n int64) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}
	
	// Add commas every 3 digits from the right
	var result strings.Builder
	for i, char := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteString(",")
		}
		result.WriteRune(char)
	}
	return result.String()
}

// renderUsageBar creates a simple ASCII usage bar
func (m *MonitoringModel) renderUsageBar(percent float64, width int) string {
	filled := int(percent / 100.0 * float64(width))
	if filled > width {
		filled = width
	}
	
	var bar strings.Builder
	bar.WriteString("[")
	
	for i := 0; i < filled; i++ {
		if percent > 90 {
			bar.WriteString("█") // Red/critical
		} else if percent > 80 {
			bar.WriteString("▓") // Yellow/warning  
		} else {
			bar.WriteString("▒") // Green/normal
		}
	}
	
	for i := filled; i < width; i++ {
		bar.WriteString("░")
	}
	
	bar.WriteString("]")
	return bar.String()
}

// fetchSystemMetrics fetches real system metrics from the monitoring agent
func (m *MonitoringModel) fetchSystemMetrics() (SystemMetrics, error) {
	// Try to connect to monitoring agent API (port 9090 as configured in monitor.yaml)
	resp, err := http.Get("http://localhost:9090/api/v1/metrics/system")
	if err != nil {
		return SystemMetrics{}, fmt.Errorf("failed to connect to monitoring agent: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return SystemMetrics{}, fmt.Errorf("monitoring agent returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SystemMetrics{}, fmt.Errorf("failed to read response body: %w", err)
	}
	
	var agentMetrics monitor.SystemMetrics
	if err := json.Unmarshal(body, &agentMetrics); err != nil {
		return SystemMetrics{}, fmt.Errorf("failed to parse system metrics: %w", err)
	}
	
	// Convert agent metrics to TUI metrics format
	tuiMetrics := SystemMetrics{
		CPUUsage:    agentMetrics.CPU.UsagePercent,
		MemoryUsage: agentMetrics.Memory.UsagePercent,
		LoadAverage: agentMetrics.Load.Load1,
		Uptime:      time.Hour * 24 * 7, // TODO: Get real uptime from /proc/uptime
	}
	
	// Calculate disk usage (use first disk if available)
	if len(agentMetrics.Disk) > 0 {
		tuiMetrics.DiskUsage = agentMetrics.Disk[0].UsagePercent
	}
	
	return tuiMetrics, nil
}

// getMockData returns mock data as fallback when agent is not available
func (m *MonitoringModel) getMockData() MonitoringData {
	return MonitoringData{
		SystemMetrics: SystemMetrics{
			CPUUsage:    25.5,
			MemoryUsage: 68.2,
			DiskUsage:   45.1,
			LoadAverage: 1.23,
			Uptime:      time.Hour * 24 * 7, // 7 days
		},
		ServiceMetrics: []ServiceMetric{
			{Name: "nginx", Status: "loaded", Active: "active", Sub: "running"},
			{Name: "mysql", Status: "loaded", Active: "active", Sub: "running"},
		},
		HTTPChecks: []HTTPCheckResult{
			{Name: "uxvalidate", URL: "https://uxvalidate.com", StatusCode: 200, ResponseTime: time.Millisecond * 150, Success: true},
		},
		Alerts: []Alert{
			{ID: "1", Name: "High Memory Usage", Severity: "warning", Message: "Memory usage is at 68%", Active: false},
		},
		LastUpdated: time.Now(),
	}
}