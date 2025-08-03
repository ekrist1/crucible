package storage

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// JSON represents a JSON field that can be stored in SQLite
type JSON map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan %T into JSON", value)
	}

	return json.Unmarshal(bytes, j)
}

// Entity represents a monitored entity (site, service, backup, etc.)
type Entity struct {
	ID        int64      `json:"id" db:"id"`
	Type      string     `json:"type" db:"type"`
	Name      string     `json:"name" db:"name"`
	Status    string     `json:"status" db:"status"`
	Details   JSON       `json:"details" db:"details"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
	LastSeen  *time.Time `json:"last_seen,omitempty" db:"last_seen"`
}

// Event represents a discrete action or incident
type Event struct {
	ID        int64      `json:"id" db:"id"`
	EntityID  *int64     `json:"entity_id,omitempty" db:"entity_id"`
	Timestamp time.Time  `json:"timestamp" db:"timestamp"`
	Type      string     `json:"event_type" db:"event_type"`
	Severity  string     `json:"severity" db:"severity"`
	Message   string     `json:"message" db:"message"`
	Details   JSON       `json:"details" db:"details"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// Metric represents a time-series data point
type Metric struct {
	ID               int64      `json:"id" db:"id"`
	EntityID         *int64     `json:"entity_id,omitempty" db:"entity_id"`
	Timestamp        time.Time  `json:"timestamp" db:"timestamp"`
	MetricName       string     `json:"metric_name" db:"metric_name"`
	Value            float64    `json:"value" db:"value"`
	AggregationLevel string     `json:"aggregation_level" db:"aggregation_level"`
	SampleCount      int        `json:"sample_count" db:"sample_count"`
	Tags             JSON       `json:"tags" db:"tags"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty" db:"expires_at"`
}

// EntityType constants
const (
	EntityTypeSite    = "site"
	EntityTypeService = "service"
	EntityTypeBackup  = "backup"
	EntityTypeServer  = "server"
	EntityTypeUser    = "user"
)

// EntityStatus constants
const (
	EntityStatusActive      = "active"
	EntityStatusInactive    = "inactive"
	EntityStatusError       = "error"
	EntityStatusMaintenance = "maintenance"
	EntityStatusUnknown     = "unknown"
)

// EventType constants
const (
	EventTypeInstall     = "install"
	EventTypeUninstall   = "uninstall"
	EventTypeUpdate      = "update"
	EventTypeBackup      = "backup"
	EventTypeRestore     = "restore"
	EventTypeStart       = "start"
	EventTypeStop        = "stop"
	EventTypeRestart     = "restart"
	EventTypeError       = "error"
	EventTypeWarning     = "warning"
	EventTypeInfo        = "info"
	EventTypeAlert       = "alert"
	EventTypeMaintenance = "maintenance"
)

// EventSeverity constants
const (
	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityError    = "error"
	SeverityCritical = "critical"
)

// MetricAggregationLevel constants
const (
	AggregationLevelRaw    = "raw"
	AggregationLevelHourly = "hourly"
	AggregationLevelDaily  = "daily"
)

// EntityFilter represents filters for querying entities
type EntityFilter struct {
	Type   *string    `json:"type,omitempty"`
	Status *string    `json:"status,omitempty"`
	Name   *string    `json:"name,omitempty"`
	Since  *time.Time `json:"since,omitempty"`
	Limit  *int       `json:"limit,omitempty"`
	Offset *int       `json:"offset,omitempty"`
}

// EventFilter represents filters for querying events
type EventFilter struct {
	EntityID *int64     `json:"entity_id,omitempty"`
	Type     *string    `json:"event_type,omitempty"`
	Severity *string    `json:"severity,omitempty"`
	Since    *time.Time `json:"since,omitempty"`
	Until    *time.Time `json:"until,omitempty"`
	Limit    *int       `json:"limit,omitempty"`
	Offset   *int       `json:"offset,omitempty"`
}

// MetricFilter represents filters for querying metrics
type MetricFilter struct {
	EntityID         *int64     `json:"entity_id,omitempty"`
	MetricName       *string    `json:"metric_name,omitempty"`
	AggregationLevel *string    `json:"aggregation_level,omitempty"`
	Since            *time.Time `json:"since,omitempty"`
	Until            *time.Time `json:"until,omitempty"`
	Limit            *int       `json:"limit,omitempty"`
	Offset           *int       `json:"offset,omitempty"`
}

// MetricSummary represents aggregated metric data
type MetricSummary struct {
	EntityID   *int64    `json:"entity_id,omitempty"`
	MetricName string    `json:"metric_name"`
	Count      int64     `json:"count"`
	Average    float64   `json:"average"`
	Min        float64   `json:"min"`
	Max        float64   `json:"max"`
	Latest     float64   `json:"latest"`
	Timestamp  time.Time `json:"timestamp"`
}

// SystemHealth represents overall system health metrics
type SystemHealth struct {
	TotalEntities  int64           `json:"total_entities"`
	ActiveEntities int64           `json:"active_entities"`
	ErrorEntities  int64           `json:"error_entities"`
	RecentEvents   int64           `json:"recent_events"`
	RecentErrors   int64           `json:"recent_errors"`
	ServicesUp     int64           `json:"services_up"`
	ServicesDown   int64           `json:"services_down"`
	SitesActive    int64           `json:"sites_active"`
	LastBackup     *time.Time      `json:"last_backup,omitempty"`
	MetricsSummary []MetricSummary `json:"metrics_summary"`
	GeneratedAt    time.Time       `json:"generated_at"`
}

// NewEntity creates a new entity with defaults
func NewEntity(entityType, name string) *Entity {
	now := time.Now()
	return &Entity{
		Type:      entityType,
		Name:      name,
		Status:    EntityStatusActive,
		Details:   make(JSON),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewEvent creates a new event with defaults
func NewEvent(entityID *int64, eventType, message string) *Event {
	now := time.Now()
	return &Event{
		EntityID:  entityID,
		Timestamp: now,
		Type:      eventType,
		Severity:  SeverityInfo,
		Message:   message,
		Details:   make(JSON),
	}
}

// NewMetric creates a new metric with defaults
func NewMetric(entityID *int64, metricName string, value float64) *Metric {
	now := time.Now()
	return &Metric{
		EntityID:         entityID,
		Timestamp:        now,
		MetricName:       metricName,
		Value:            value,
		AggregationLevel: AggregationLevelRaw,
		SampleCount:      1,
		Tags:             make(JSON),
	}
}

// SetTTL sets the time-to-live for the entity based on retention config
func (e *Event) SetTTL(days int) {
	if days > 0 {
		expiry := e.Timestamp.Add(time.Duration(days) * 24 * time.Hour)
		e.ExpiresAt = &expiry
	}
}

// SetTTL sets the time-to-live for the metric based on retention config
func (m *Metric) SetTTL(days int) {
	if days > 0 {
		expiry := m.Timestamp.Add(time.Duration(days) * 24 * time.Hour)
		m.ExpiresAt = &expiry
	}
}

// IsExpired checks if the event has expired
func (e *Event) IsExpired() bool {
	return e.ExpiresAt != nil && time.Now().After(*e.ExpiresAt)
}

// IsExpired checks if the metric has expired
func (m *Metric) IsExpired() bool {
	return m.ExpiresAt != nil && time.Now().After(*m.ExpiresAt)
}

// Touch updates the LastSeen timestamp for the entity
func (e *Entity) Touch() {
	now := time.Now()
	e.LastSeen = &now
	e.UpdatedAt = now
}

// IsStale checks if the entity hasn't been seen recently
func (e *Entity) IsStale(threshold time.Duration) bool {
	if e.LastSeen == nil {
		return true
	}
	return time.Since(*e.LastSeen) > threshold
}
