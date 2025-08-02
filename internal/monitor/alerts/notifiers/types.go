package notifiers

import "time"

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

// AlertSeverity represents the severity level of an alert
type AlertSeverity string

const (
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
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
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details"`
	Labels      map[string]string      `json:"labels"`
	Annotations map[string]string      `json:"annotations"`

	// Timing information
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at,omitempty"`
}
