package monitor

import (
	"time"
)

// MetricType represents the type of metric being collected
type MetricType string

const (
	MetricTypeCPU     MetricType = "cpu"
	MetricTypeMemory  MetricType = "memory"
	MetricTypeDisk    MetricType = "disk"
	MetricTypeNetwork MetricType = "network"
	MetricTypeLoad    MetricType = "load"
	MetricTypeService MetricType = "service"
	MetricTypeHTTP    MetricType = "http"
	MetricTypeCustom  MetricType = "custom"
)

// Metric represents a single metric data point
type Metric struct {
	Name      string                 `json:"name"`
	Type      MetricType             `json:"type"`
	Value     float64                `json:"value"`
	Unit      string                 `json:"unit"`
	Labels    map[string]string      `json:"labels"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceStatus represents the status of a monitored service
type ServiceStatus struct {
	Name         string            `json:"name"`
	Status       string            `json:"status"` // loaded, failed, etc.
	Active       string            `json:"active"` // active, inactive, failed
	Sub          string            `json:"sub"`    // running, dead, exited, etc.
	Since        time.Time         `json:"since"`  // Time since current state
	RestartCount int               `json:"restart_count"`
	LastRestart  time.Time         `json:"last_restart"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// HTTPCheckResult represents the result of an HTTP health check
type HTTPCheckResult struct {
	Name          string        `json:"name"`
	URL           string        `json:"url"`
	StatusCode    int           `json:"status_code"`
	ResponseTime  time.Duration `json:"response_time"`
	Success       bool          `json:"success"`
	Error         string        `json:"error,omitempty"`
	Timestamp     time.Time     `json:"timestamp"`
	ContentLength int64         `json:"content_length,omitempty"`
	SSLExpiry     *time.Time    `json:"ssl_expiry,omitempty"`
}

// SystemMetrics represents system-wide metrics
type SystemMetrics struct {
	CPU       CPUMetrics       `json:"cpu"`
	Memory    MemoryMetrics    `json:"memory"`
	Disk      []DiskMetrics    `json:"disk"`
	Network   []NetworkMetrics `json:"network"`
	Load      LoadMetrics      `json:"load"`
	Timestamp time.Time        `json:"timestamp"`
}

// CPUMetrics represents CPU usage metrics
type CPUMetrics struct {
	UsagePercent  float64 `json:"usage_percent"`
	UserPercent   float64 `json:"user_percent"`
	SystemPercent float64 `json:"system_percent"`
	IdlePercent   float64 `json:"idle_percent"`
	IOWaitPercent float64 `json:"iowait_percent"`
}

// MemoryMetrics represents memory usage metrics
type MemoryMetrics struct {
	TotalBytes       uint64  `json:"total_bytes"`
	UsedBytes        uint64  `json:"used_bytes"`
	FreeBytes        uint64  `json:"free_bytes"`
	AvailableBytes   uint64  `json:"available_bytes"`
	UsagePercent     float64 `json:"usage_percent"`
	SwapTotalBytes   uint64  `json:"swap_total_bytes"`
	SwapUsedBytes    uint64  `json:"swap_used_bytes"`
	SwapUsagePercent float64 `json:"swap_usage_percent"`
}

// DiskMetrics represents disk usage metrics for a single mount point
type DiskMetrics struct {
	MountPoint   string  `json:"mount_point"`
	Device       string  `json:"device"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	FreeBytes    uint64  `json:"free_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	InodesTotal  uint64  `json:"inodes_total"`
	InodesUsed   uint64  `json:"inodes_used"`
	InodesFree   uint64  `json:"inodes_free"`
}

// NetworkMetrics represents network interface metrics
type NetworkMetrics struct {
	Interface   string `json:"interface"`
	BytesRecv   uint64 `json:"bytes_recv"`
	BytesSent   uint64 `json:"bytes_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	ErrorsRecv  uint64 `json:"errors_recv"`
	ErrorsSent  uint64 `json:"errors_sent"`
	DroppedRecv uint64 `json:"dropped_recv"`
	DroppedSent uint64 `json:"dropped_sent"`
}

// LoadMetrics represents system load average metrics
type LoadMetrics struct {
	Load1  float64 `json:"load_1"`
	Load5  float64 `json:"load_5"`
	Load15 float64 `json:"load_15"`
}

// Alert represents a monitoring alert
type Alert struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Severity    AlertSeverity     `json:"severity"`
	Status      AlertStatus       `json:"status"`
	Source      string            `json:"source"`
	Metrics     []Metric          `json:"metrics"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ResolvedAt  *time.Time        `json:"resolved_at,omitempty"`
}

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertStatus represents the current status of an alert
type AlertStatus string

const (
	AlertStatusActive       AlertStatus = "active"
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	AlertStatusResolved     AlertStatus = "resolved"
)

// Config represents the monitoring configuration
type Config struct {
	Agent         AgentConfig         `yaml:"agent"`
	Collectors    CollectorsConfig    `yaml:"collectors"`
	Storage       StorageConfig       `yaml:"storage"`
	Alerts        AlertsConfig        `yaml:"alerts"`
	Notifications NotificationsConfig `yaml:"notifications"`
	AI            AIConfig            `yaml:"ai"`
}

// AgentConfig represents agent-specific configuration
type AgentConfig struct {
	ListenAddr      string `yaml:"listen_addr"`
	DataRetention   string `yaml:"data_retention"`
	CollectInterval string `yaml:"collect_interval"`
	Debug           bool   `yaml:"debug"`
}

// CollectorsConfig represents collector configuration
type CollectorsConfig struct {
	System     SystemCollectorConfig     `yaml:"system"`
	Services   ServicesCollectorConfig   `yaml:"services"`
	HTTPChecks HTTPChecksCollectorConfig `yaml:"http_checks"`
}

// SystemCollectorConfig represents system metrics collector configuration
type SystemCollectorConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Interval string   `yaml:"interval"`
	Metrics  []string `yaml:"metrics"`
}

// ServicesCollectorConfig represents service monitoring configuration
type ServicesCollectorConfig struct {
	Enabled  bool     `yaml:"enabled"`
	Interval string   `yaml:"interval"`
	Services []string `yaml:"services"`
}

// HTTPChecksCollectorConfig represents HTTP health check configuration
type HTTPChecksCollectorConfig struct {
	Enabled bool        `yaml:"enabled"`
	Checks  []HTTPCheck `yaml:"checks"`
}

// HTTPCheck represents a single HTTP health check configuration
type HTTPCheck struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	Interval       string `yaml:"interval"`
	Timeout        string `yaml:"timeout"`
	ExpectedStatus int    `yaml:"expected_status"`
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	Type        string            `yaml:"type"`
	SQLite      SQLiteConfig      `yaml:"sqlite"`
	Aggregation AggregationConfig `yaml:"aggregation"`
}

// SQLiteConfig represents SQLite storage configuration
type SQLiteConfig struct {
	Path            string          `yaml:"path"`
	BatchSize       int             `yaml:"batch_size"`
	CleanupInterval time.Duration   `yaml:"cleanup_interval"`
	BackupEnabled   bool            `yaml:"backup_enabled"`
	BackupInterval  time.Duration   `yaml:"backup_interval"`
	Retention       RetentionConfig `yaml:"retention"`
}

// RetentionConfig represents data retention configuration
type RetentionConfig struct {
	EventsDays     int `yaml:"events_days"`
	MetricsDays    int `yaml:"metrics_days"`
	AggregatesDays int `yaml:"aggregates_days"`
}

// AggregationConfig represents data aggregation configuration
type AggregationConfig struct {
	RawRetention    string `yaml:"raw_retention"`
	MinuteRetention string `yaml:"minute_retention"`
	HourRetention   string `yaml:"hour_retention"`
}

// AlertsConfig represents alerting configuration
type AlertsConfig struct {
	Enabled       bool            `yaml:"enabled"`
	Thresholds    AlertThresholds `yaml:"thresholds"`
	CheckInterval string          `yaml:"check_interval"`
}

// AlertThresholds represents alert threshold configuration
type AlertThresholds struct {
	CPUPercent     float64 `yaml:"cpu_percent"`
	MemoryPercent  float64 `yaml:"memory_percent"`
	DiskPercent    float64 `yaml:"disk_percent"`
	LoadAverage    float64 `yaml:"load_average"`
	ResponseTimeMs int     `yaml:"response_time_ms"`
}

// NotificationsConfig represents notification configuration
type NotificationsConfig struct {
	Email   EmailConfig   `yaml:"email"`
	Webhook WebhookConfig `yaml:"webhook"`
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	Enabled    bool     `yaml:"enabled"`
	SMTPServer string   `yaml:"smtp_server"`
	SMTPPort   int      `yaml:"smtp_port"`
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
	From       string   `yaml:"from"`
	To         []string `yaml:"to"`
}

// WebhookConfig represents webhook notification configuration
type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}

// AIConfig represents AI/ML configuration
type AIConfig struct {
	Enabled            bool                     `yaml:"enabled"`
	AnomalyDetection   AnomalyDetectionConfig   `yaml:"anomaly_detection"`
	PatternRecognition PatternRecognitionConfig `yaml:"pattern_recognition"`
}

// AnomalyDetectionConfig represents anomaly detection configuration
type AnomalyDetectionConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Sensitivity    string `yaml:"sensitivity"`
	LearningPeriod string `yaml:"learning_period"`
}

// PatternRecognitionConfig represents pattern recognition configuration
type PatternRecognitionConfig struct {
	Enabled             bool    `yaml:"enabled"`
	MinPatternLength    string  `yaml:"min_pattern_length"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
}
