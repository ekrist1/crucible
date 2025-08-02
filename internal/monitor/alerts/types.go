package alerts

import (
	"time"
)

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// AlertStatus represents the current status of an alert
type AlertStatus string

const (
	StatusFiring       AlertStatus = "firing"
	StatusResolved     AlertStatus = "resolved"
	StatusAcknowledged AlertStatus = "acknowledged"
	StatusSuppressed   AlertStatus = "suppressed"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertTypeSystem  AlertType = "system"
	AlertTypeService AlertType = "service"
	AlertTypeHTTP    AlertType = "http"
	AlertTypeCustom  AlertType = "custom"
)

// Alert represents an active or historical alert
type Alert struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        AlertType              `json:"type"`
	Severity    AlertSeverity          `json:"severity"`
	Status      AlertStatus            `json:"status"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`

	// Timing information
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
	LastSent *time.Time `json:"last_sent,omitempty"`

	// Alert rule information
	RuleID string `json:"rule_id"`

	// Notification tracking
	NotificationsSent int      `json:"notifications_sent"`
	SentTo            []string `json:"sent_to"`
}

// AlertRule defines the conditions for triggering an alert
type AlertRule struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Type     AlertType     `json:"type"`
	Severity AlertSeverity `json:"severity"`
	Enabled  bool          `json:"enabled"`

	// Condition configuration
	Conditions AlertConditions `json:"conditions"`

	// Notification configuration
	NotifyEmails   []string `json:"notify_emails"`
	NotifyWebhooks []string `json:"notify_webhooks"`

	// Rate limiting
	MinInterval      time.Duration `json:"min_interval"`
	MaxNotifications int           `json:"max_notifications"`

	// Metadata
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// AlertConditions defines the conditions that trigger an alert
type AlertConditions struct {
	// System metric conditions
	CPUThreshold    *float64 `json:"cpu_threshold,omitempty"`
	MemoryThreshold *float64 `json:"memory_threshold,omitempty"`
	DiskThreshold   *float64 `json:"disk_threshold,omitempty"`
	LoadThreshold   *float64 `json:"load_threshold,omitempty"`

	// Service conditions
	ServiceName   string `json:"service_name,omitempty"`
	ServiceStatus string `json:"service_status,omitempty"`

	// HTTP endpoint conditions
	HTTPEndpoint    string        `json:"http_endpoint,omitempty"`
	ResponseTimeout time.Duration `json:"response_timeout,omitempty"`
	ExpectedStatus  int           `json:"expected_status,omitempty"`

	// Duration requirements
	Duration time.Duration `json:"duration,omitempty"` // How long condition must be true
}

// AlertManager manages the alert system
type AlertManager struct {
	rules        map[string]*AlertRule
	activeAlerts map[string]*Alert
	alertHistory []*Alert
	notifiers    []Notifier

	// Configuration
	config *Config

	// State tracking
	lastEvaluation time.Time
}

// Notifier interface for different notification channels
type Notifier interface {
	Name() string
	Send(alert *Alert) error
	IsEnabled() bool
}

// Config represents the alert system configuration
type Config struct {
	// Global settings
	EvaluationInterval time.Duration `yaml:"evaluation_interval"`
	DefaultSeverity    AlertSeverity `yaml:"default_severity"`
	MaxAlertHistory    int           `yaml:"max_alert_history"`

	// Email configuration
	Email EmailConfig `yaml:"email"`

	// Webhook configuration
	Webhooks []WebhookConfig `yaml:"webhooks"`

	// Rate limiting
	GlobalRateLimit struct {
		MaxPerHour   int           `yaml:"max_per_hour"`
		CooldownTime time.Duration `yaml:"cooldown_time"`
	} `yaml:"global_rate_limit"`
}

// EmailConfig represents email notification configuration
type EmailConfig struct {
	Enabled      bool   `yaml:"enabled"`
	ResendAPIKey string `yaml:"resend_api_key"`
	FromEmail    string `yaml:"from_email"`
	FromName     string `yaml:"from_name"`

	// Default recipients
	DefaultTo []string `yaml:"default_to"`

	// Templates
	SubjectTemplate string `yaml:"subject_template"`
	BodyTemplate    string `yaml:"body_template"`
}

// WebhookConfig represents webhook notification configuration
type WebhookConfig struct {
	Name    string            `yaml:"name"`
	Enabled bool              `yaml:"enabled"`
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Timeout time.Duration     `yaml:"timeout"`
}

// MetricData represents a data point for alert evaluation
type MetricData struct {
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

// EvaluationContext provides context for rule evaluation
type EvaluationContext struct {
	SystemMetrics map[string]MetricData
	ServiceStates map[string]string
	HTTPResults   map[string]HTTPCheckResult
	CurrentTime   time.Time
}

// HTTPCheckResult represents the result of an HTTP health check
type HTTPCheckResult struct {
	URL          string
	StatusCode   int
	ResponseTime time.Duration
	Success      bool
	Error        string
	Timestamp    time.Time
}
