package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"crucible/internal/logging"
	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStorage implements persistent storage using SQLite
type SQLiteStorage struct {
	db         *sql.DB
	config     *Config
	logger     *logging.Logger
	batchSize  int
	batchItems []BatchItem
}

// Config represents storage configuration
type Config struct {
	DatabasePath    string        `yaml:"database_path"`
	RetentionDays   RetentionDays `yaml:"retention"`
	BatchSize       int           `yaml:"batch_size"`
	CleanupInterval time.Duration `yaml:"cleanup_interval"`
	BackupEnabled   bool          `yaml:"backup_enabled"`
	BackupInterval  time.Duration `yaml:"backup_interval"`
}

// RetentionDays defines data retention periods
type RetentionDays struct {
	EventsDays     int `yaml:"events_days"`
	MetricsDays    int `yaml:"metrics_days"`
	AggregatesDays int `yaml:"aggregates_days"`
}

// BatchItem represents an item to be batched for insertion
type BatchItem struct {
	Type string
	Data interface{}
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(config *Config, logger *logging.Logger) (*SQLiteStorage, error) {
	if config == nil {
		config = &Config{
			DatabasePath: "/var/lib/crucible/monitor.db",
			RetentionDays: RetentionDays{
				EventsDays:     90,
				MetricsDays:    30,
				AggregatesDays: 365,
			},
			BatchSize:       100,
			CleanupInterval: time.Hour,
			BackupEnabled:   true,
			BackupInterval:  24 * time.Hour,
		}
	}

	db, err := sql.Open("sqlite3", config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &SQLiteStorage{
		db:         db,
		config:     config,
		logger:     logger,
		batchSize:  config.BatchSize,
		batchItems: make([]BatchItem, 0, config.BatchSize),
	}

	if err := storage.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return storage, nil
}

// initialize sets up the database schema and optimizations
func (s *SQLiteStorage) initialize() error {
	// Enable WAL mode for better concurrency
	if _, err := s.db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Optimize for time-series workload
	pragmas := []string{
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA temp_store=memory",
		"PRAGMA mmap_size=268435456", // 256MB
	}

	for _, pragma := range pragmas {
		if _, err := s.db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}

	// Run migrations instead of creating schema directly
	migrationManager := NewMigrationManager(s.db, s.logger)
	if err := migrationManager.ApplyMigrations(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	// Initialize metadata
	if err := s.initializeMetadata(); err != nil {
		return fmt.Errorf("failed to initialize metadata: %w", err)
	}

	return nil
}

// createSchema creates the database tables and indexes
func (s *SQLiteStorage) createSchema() error {
	schema := `
	-- Metadata table for DB versioning and config
	CREATE TABLE IF NOT EXISTS metadata (
		id INTEGER PRIMARY KEY DEFAULT 1,
		db_version TEXT NOT NULL DEFAULT '1.0',
		schema_hash TEXT,
		crucible_version TEXT,
		last_cleanup_timestamp INTEGER,
		retention_config JSON DEFAULT '{"events_days": 90, "metrics_days": 30, "aggregates_days": 365}',
		created_at INTEGER NOT NULL DEFAULT (unixepoch()),
		updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
		CONSTRAINT single_row CHECK (id = 1)
	);

	-- Entities table for sites, services, backups, etc.
	CREATE TABLE IF NOT EXISTS entities (
		id INTEGER PRIMARY KEY,
		type TEXT NOT NULL,
		name TEXT NOT NULL,
		status TEXT DEFAULT 'active',
		details JSON,
		created_at INTEGER NOT NULL DEFAULT (unixepoch()),
		updated_at INTEGER NOT NULL DEFAULT (unixepoch()),
		last_seen INTEGER,
		UNIQUE(type, name)
	);

	-- Events table for historic events and actions
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY,
		entity_id INTEGER,
		timestamp INTEGER NOT NULL DEFAULT (unixepoch()),
		event_type TEXT NOT NULL,
		severity TEXT DEFAULT 'info',
		message TEXT,
		details JSON,
		expires_at INTEGER,
		FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE SET NULL
	);

	-- Metrics table for time-series data
	CREATE TABLE IF NOT EXISTS metrics (
		id INTEGER PRIMARY KEY,
		entity_id INTEGER,
		timestamp INTEGER NOT NULL,
		metric_name TEXT NOT NULL,
		value REAL NOT NULL,
		aggregation_level TEXT DEFAULT 'raw',
		sample_count INTEGER DEFAULT 1,
		tags JSON,
		expires_at INTEGER,
		FOREIGN KEY (entity_id) REFERENCES entities(id) ON DELETE CASCADE
	);

	-- Create indexes
	CREATE INDEX IF NOT EXISTS idx_entities_type ON entities(type);
	CREATE INDEX IF NOT EXISTS idx_entities_name ON entities(name);
	CREATE INDEX IF NOT EXISTS idx_entities_status ON entities(status);
	CREATE INDEX IF NOT EXISTS idx_entities_last_seen ON entities(last_seen);

	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
	CREATE INDEX IF NOT EXISTS idx_events_entity_id ON events(entity_id);
	CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity);
	CREATE INDEX IF NOT EXISTS idx_events_expires_at ON events(expires_at) WHERE expires_at IS NOT NULL;

	CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp);
	CREATE INDEX IF NOT EXISTS idx_metrics_metric_name ON metrics(metric_name);
	CREATE INDEX IF NOT EXISTS idx_metrics_entity_id ON metrics(entity_id);
	CREATE INDEX IF NOT EXISTS idx_metrics_entity_metric_time ON metrics(entity_id, metric_name, timestamp);
	CREATE INDEX IF NOT EXISTS idx_metrics_aggregation_level ON metrics(aggregation_level);
	CREATE INDEX IF NOT EXISTS idx_metrics_expires_at ON metrics(expires_at) WHERE expires_at IS NOT NULL;
	`

	_, err := s.db.Exec(schema)
	return err
}

// initializeMetadata sets up initial metadata if not exists
func (s *SQLiteStorage) initializeMetadata() error {
	retentionJSON, _ := json.Marshal(s.config.RetentionDays)

	query := `
	INSERT OR IGNORE INTO metadata (id, db_version, crucible_version, retention_config, created_at, updated_at)
	VALUES (1, '1.0', '1.0.0', ?, unixepoch(), unixepoch())`

	_, err := s.db.Exec(query, string(retentionJSON))
	return err
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	if err := s.FlushBatch(); err != nil {
		// Log error but don't fail close
		fmt.Printf("Warning: failed to flush batch during close: %v\n", err)
	}
	return s.db.Close()
}

// Health checks database connectivity and returns status
func (s *SQLiteStorage) Health() error {
	var version string
	err := s.db.QueryRow("SELECT db_version FROM metadata WHERE id = 1").Scan(&version)
	if err != nil {
		return fmt.Errorf("database health check failed: %w", err)
	}
	return nil
}

// GetDatabaseInfo returns information about the database
func (s *SQLiteStorage) GetDatabaseInfo() (*DatabaseInfo, error) {
	info := &DatabaseInfo{}

	// Get metadata - scan timestamps as int64 then convert
	var createdAtUnix, updatedAtUnix int64
	var lastCleanupUnix *int64
	err := s.db.QueryRow(`
		SELECT db_version, crucible_version, created_at, updated_at, last_cleanup_timestamp
		FROM metadata WHERE id = 1
	`).Scan(&info.DBVersion, &info.CrucibleVersion, &createdAtUnix, &updatedAtUnix, &lastCleanupUnix)
	if err != nil {
		return nil, err
	}

	// Convert Unix timestamps to time.Time
	info.CreatedAt = time.Unix(createdAtUnix, 0)
	info.UpdatedAt = time.Unix(updatedAtUnix, 0)
	if lastCleanupUnix != nil {
		lastCleanup := time.Unix(*lastCleanupUnix, 0)
		info.LastCleanup = &lastCleanup
	}

	// Get counts
	if err := s.db.QueryRow("SELECT COUNT(*) FROM entities").Scan(&info.EntityCount); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&info.EventCount); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow("SELECT COUNT(*) FROM metrics").Scan(&info.MetricCount); err != nil {
		return nil, err
	}

	// Get database size
	var pageCount, pageSize int64
	if err := s.db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return nil, err
	}
	info.DatabaseSize = pageCount * pageSize

	return info, nil
}

// DatabaseInfo contains information about the database
type DatabaseInfo struct {
	DBVersion       string     `json:"db_version"`
	CrucibleVersion string     `json:"crucible_version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastCleanup     *time.Time `json:"last_cleanup,omitempty"`
	EntityCount     int64      `json:"entity_count"`
	EventCount      int64      `json:"event_count"`
	MetricCount     int64      `json:"metric_count"`
	DatabaseSize    int64      `json:"database_size_bytes"`
}

// Storage interface for the SQLite storage layer
type Storage interface {
	// Entity operations
	CreateEntity(entity *Entity) error
	GetEntity(id int64) (*Entity, error)
	GetEntityByName(entityType, name string) (*Entity, error)
	UpdateEntity(entity *Entity) error
	DeleteEntity(id int64) error
	ListEntities(filter *EntityFilter) ([]*Entity, error)

	// Event operations
	CreateEvent(event *Event) error
	GetEvent(id int64) (*Event, error)
	ListEvents(filter *EventFilter) ([]*Event, error)
	DeleteEvent(id int64) error

	// Metric operations
	CreateMetric(metric *Metric) error
	GetMetric(id int64) (*Metric, error)
	ListMetrics(filter *MetricFilter) ([]*Metric, error)
	GetMetricSummary(filter *MetricFilter) (*MetricSummary, error)
	DeleteMetric(id int64) error

	// Batch operations
	BatchWrite(items []BatchItem) error
	FlushBatch() error

	// Maintenance operations
	Cleanup() error
	Vacuum() error
	GetSystemHealth() (*SystemHealth, error)

	// Database info
	Health() error
	Close() error
}

// ENTITY OPERATIONS

// CreateEntity creates a new entity in the database
func (s *SQLiteStorage) CreateEntity(entity *Entity) error {
	query := `
		INSERT INTO entities (type, name, status, details, created_at, updated_at, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	detailsJSON, err := entity.Details.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal entity details: %w", err)
	}

	var lastSeenUnix *int64
	if entity.LastSeen != nil {
		unix := entity.LastSeen.Unix()
		lastSeenUnix = &unix
	}

	result, err := s.db.Exec(query,
		entity.Type, entity.Name, entity.Status, detailsJSON,
		entity.CreatedAt.Unix(), entity.UpdatedAt.Unix(), lastSeenUnix)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get entity ID: %w", err)
	}
	entity.ID = id

	return nil
}

// GetEntity retrieves an entity by ID
func (s *SQLiteStorage) GetEntity(id int64) (*Entity, error) {
	query := `
		SELECT id, type, name, status, details, created_at, updated_at, last_seen
		FROM entities WHERE id = ?`

	entity := &Entity{}
	var detailsJSON []byte
	var createdAtUnix, updatedAtUnix int64
	var lastSeenUnix *int64

	err := s.db.QueryRow(query, id).Scan(
		&entity.ID, &entity.Type, &entity.Name, &entity.Status,
		&detailsJSON, &createdAtUnix, &updatedAtUnix, &lastSeenUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entity not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	// Parse JSON details
	if len(detailsJSON) > 0 {
		entity.Details = make(JSON)
		if err := entity.Details.Scan(detailsJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entity details: %w", err)
		}
	}

	// Convert timestamps
	entity.CreatedAt = time.Unix(createdAtUnix, 0)
	entity.UpdatedAt = time.Unix(updatedAtUnix, 0)
	if lastSeenUnix != nil {
		lastSeen := time.Unix(*lastSeenUnix, 0)
		entity.LastSeen = &lastSeen
	}

	return entity, nil
}

// GetEntityByName retrieves an entity by type and name
func (s *SQLiteStorage) GetEntityByName(entityType, name string) (*Entity, error) {
	query := `
		SELECT id, type, name, status, details, created_at, updated_at, last_seen
		FROM entities WHERE type = ? AND name = ?`

	entity := &Entity{}
	var detailsJSON []byte
	var createdAtUnix, updatedAtUnix int64
	var lastSeenUnix *int64

	err := s.db.QueryRow(query, entityType, name).Scan(
		&entity.ID, &entity.Type, &entity.Name, &entity.Status,
		&detailsJSON, &createdAtUnix, &updatedAtUnix, &lastSeenUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entity not found: %s/%s", entityType, name)
		}
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	// Parse JSON details
	if len(detailsJSON) > 0 {
		entity.Details = make(JSON)
		if err := entity.Details.Scan(detailsJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entity details: %w", err)
		}
	}

	// Convert timestamps
	entity.CreatedAt = time.Unix(createdAtUnix, 0)
	entity.UpdatedAt = time.Unix(updatedAtUnix, 0)
	if lastSeenUnix != nil {
		lastSeen := time.Unix(*lastSeenUnix, 0)
		entity.LastSeen = &lastSeen
	}

	return entity, nil
}

// UpdateEntity updates an existing entity
func (s *SQLiteStorage) UpdateEntity(entity *Entity) error {
	query := `
		UPDATE entities 
		SET status = ?, details = ?, updated_at = ?, last_seen = ?
		WHERE id = ?`

	detailsJSON, err := entity.Details.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal entity details: %w", err)
	}

	var lastSeenUnix *int64
	if entity.LastSeen != nil {
		unix := entity.LastSeen.Unix()
		lastSeenUnix = &unix
	}

	entity.UpdatedAt = time.Now()
	result, err := s.db.Exec(query,
		entity.Status, detailsJSON, entity.UpdatedAt.Unix(), lastSeenUnix, entity.ID)
	if err != nil {
		return fmt.Errorf("failed to update entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("entity not found: %d", entity.ID)
	}

	return nil
}

// DeleteEntity removes an entity from the database
func (s *SQLiteStorage) DeleteEntity(id int64) error {
	query := `DELETE FROM entities WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete entity: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("entity not found: %d", id)
	}

	return nil
}

// ListEntities returns entities based on filter criteria
func (s *SQLiteStorage) ListEntities(filter *EntityFilter) ([]*Entity, error) {
	query := `SELECT id, type, name, status, details, created_at, updated_at, last_seen FROM entities WHERE 1=1`
	args := []interface{}{}

	if filter != nil {
		if filter.Type != nil {
			query += ` AND type = ?`
			args = append(args, *filter.Type)
		}
		if filter.Status != nil {
			query += ` AND status = ?`
			args = append(args, *filter.Status)
		}
		if filter.Name != nil {
			query += ` AND name LIKE ?`
			args = append(args, "%"+*filter.Name+"%")
		}
		if filter.Since != nil {
			query += ` AND updated_at >= ?`
			args = append(args, filter.Since.Unix())
		}
	}

	query += ` ORDER BY updated_at DESC`

	if filter != nil {
		if filter.Limit != nil {
			query += ` LIMIT ?`
			args = append(args, *filter.Limit)
		}
		if filter.Offset != nil {
			query += ` OFFSET ?`
			args = append(args, *filter.Offset)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list entities: %w", err)
	}
	defer rows.Close()

	var entities []*Entity
	for rows.Next() {
		entity := &Entity{}
		var detailsJSON []byte
		var createdAtUnix, updatedAtUnix int64
		var lastSeenUnix *int64

		err := rows.Scan(&entity.ID, &entity.Type, &entity.Name, &entity.Status,
			&detailsJSON, &createdAtUnix, &updatedAtUnix, &lastSeenUnix)
		if err != nil {
			return nil, fmt.Errorf("failed to scan entity: %w", err)
		}

		// Parse JSON details
		if len(detailsJSON) > 0 {
			entity.Details = make(JSON)
			if err := entity.Details.Scan(detailsJSON); err != nil {
				return nil, fmt.Errorf("failed to unmarshal entity details: %w", err)
			}
		}

		// Convert timestamps
		entity.CreatedAt = time.Unix(createdAtUnix, 0)
		entity.UpdatedAt = time.Unix(updatedAtUnix, 0)
		if lastSeenUnix != nil {
			lastSeen := time.Unix(*lastSeenUnix, 0)
			entity.LastSeen = &lastSeen
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

// EVENT OPERATIONS

// CreateEvent creates a new event in the database
func (s *SQLiteStorage) CreateEvent(event *Event) error {
	query := `
		INSERT INTO events (entity_id, timestamp, event_type, severity, message, details, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	detailsJSON, err := event.Details.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal event details: %w", err)
	}

	var expiresAtUnix *int64
	if event.ExpiresAt != nil {
		unix := event.ExpiresAt.Unix()
		expiresAtUnix = &unix
	}

	result, err := s.db.Exec(query,
		event.EntityID, event.Timestamp.Unix(), event.Type, event.Severity,
		event.Message, detailsJSON, expiresAtUnix)
	if err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get event ID: %w", err)
	}
	event.ID = id

	return nil
}

// GetEvent retrieves an event by ID
func (s *SQLiteStorage) GetEvent(id int64) (*Event, error) {
	query := `
		SELECT id, entity_id, timestamp, event_type, severity, message, details, expires_at
		FROM events WHERE id = ?`

	event := &Event{}
	var detailsJSON []byte
	var timestampUnix int64
	var expiresAtUnix *int64

	err := s.db.QueryRow(query, id).Scan(
		&event.ID, &event.EntityID, &timestampUnix, &event.Type,
		&event.Severity, &event.Message, &detailsJSON, &expiresAtUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	// Parse JSON details
	if len(detailsJSON) > 0 {
		event.Details = make(JSON)
		if err := event.Details.Scan(detailsJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event details: %w", err)
		}
	}

	// Convert timestamps
	event.Timestamp = time.Unix(timestampUnix, 0)
	if expiresAtUnix != nil {
		expiresAt := time.Unix(*expiresAtUnix, 0)
		event.ExpiresAt = &expiresAt
	}

	return event, nil
}

// ListEvents returns events based on filter criteria
func (s *SQLiteStorage) ListEvents(filter *EventFilter) ([]*Event, error) {
	query := `SELECT id, entity_id, timestamp, event_type, severity, message, details, expires_at FROM events WHERE 1=1`
	args := []interface{}{}

	if filter != nil {
		if filter.EntityID != nil {
			query += ` AND entity_id = ?`
			args = append(args, *filter.EntityID)
		}
		if filter.Type != nil {
			query += ` AND event_type = ?`
			args = append(args, *filter.Type)
		}
		if filter.Severity != nil {
			query += ` AND severity = ?`
			args = append(args, *filter.Severity)
		}
		if filter.Since != nil {
			query += ` AND timestamp >= ?`
			args = append(args, filter.Since.Unix())
		}
		if filter.Until != nil {
			query += ` AND timestamp <= ?`
			args = append(args, filter.Until.Unix())
		}
	}

	query += ` ORDER BY timestamp DESC`

	if filter != nil {
		if filter.Limit != nil {
			query += ` LIMIT ?`
			args = append(args, *filter.Limit)
		}
		if filter.Offset != nil {
			query += ` OFFSET ?`
			args = append(args, *filter.Offset)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		var detailsJSON []byte
		var timestampUnix int64
		var expiresAtUnix *int64

		err := rows.Scan(&event.ID, &event.EntityID, &timestampUnix, &event.Type,
			&event.Severity, &event.Message, &detailsJSON, &expiresAtUnix)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		// Parse JSON details
		if len(detailsJSON) > 0 {
			event.Details = make(JSON)
			if err := event.Details.Scan(detailsJSON); err != nil {
				return nil, fmt.Errorf("failed to unmarshal event details: %w", err)
			}
		}

		// Convert timestamps
		event.Timestamp = time.Unix(timestampUnix, 0)
		if expiresAtUnix != nil {
			expiresAt := time.Unix(*expiresAtUnix, 0)
			event.ExpiresAt = &expiresAt
		}

		events = append(events, event)
	}

	return events, nil
}

// DeleteEvent removes an event from the database
func (s *SQLiteStorage) DeleteEvent(id int64) error {
	query := `DELETE FROM events WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("event not found: %d", id)
	}

	return nil
}

// METRIC OPERATIONS

// CreateMetric creates a new metric in the database
func (s *SQLiteStorage) CreateMetric(metric *Metric) error {
	query := `
		INSERT INTO metrics (entity_id, timestamp, metric_name, value, aggregation_level, sample_count, tags, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	tagsJSON, err := metric.Tags.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal metric tags: %w", err)
	}

	var expiresAtUnix *int64
	if metric.ExpiresAt != nil {
		unix := metric.ExpiresAt.Unix()
		expiresAtUnix = &unix
	}

	result, err := s.db.Exec(query,
		metric.EntityID, metric.Timestamp.Unix(), metric.MetricName, metric.Value,
		metric.AggregationLevel, metric.SampleCount, tagsJSON, expiresAtUnix)
	if err != nil {
		return fmt.Errorf("failed to create metric: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get metric ID: %w", err)
	}
	metric.ID = id

	return nil
}

// GetMetric retrieves a metric by ID
func (s *SQLiteStorage) GetMetric(id int64) (*Metric, error) {
	query := `
		SELECT id, entity_id, timestamp, metric_name, value, aggregation_level, sample_count, tags, expires_at
		FROM metrics WHERE id = ?`

	metric := &Metric{}
	var tagsJSON []byte
	var timestampUnix int64
	var expiresAtUnix *int64

	err := s.db.QueryRow(query, id).Scan(
		&metric.ID, &metric.EntityID, &timestampUnix, &metric.MetricName, &metric.Value,
		&metric.AggregationLevel, &metric.SampleCount, &tagsJSON, &expiresAtUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("metric not found: %d", id)
		}
		return nil, fmt.Errorf("failed to get metric: %w", err)
	}

	// Parse JSON tags
	if len(tagsJSON) > 0 {
		metric.Tags = make(JSON)
		if err := metric.Tags.Scan(tagsJSON); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metric tags: %w", err)
		}
	}

	// Convert timestamps
	metric.Timestamp = time.Unix(timestampUnix, 0)
	if expiresAtUnix != nil {
		expiresAt := time.Unix(*expiresAtUnix, 0)
		metric.ExpiresAt = &expiresAt
	}

	return metric, nil
}

// ListMetrics returns metrics based on filter criteria
func (s *SQLiteStorage) ListMetrics(filter *MetricFilter) ([]*Metric, error) {
	query := `SELECT id, entity_id, timestamp, metric_name, value, aggregation_level, sample_count, tags, expires_at FROM metrics WHERE 1=1`
	args := []interface{}{}

	if filter != nil {
		if filter.EntityID != nil {
			query += ` AND entity_id = ?`
			args = append(args, *filter.EntityID)
		}
		if filter.MetricName != nil {
			query += ` AND metric_name = ?`
			args = append(args, *filter.MetricName)
		}
		if filter.AggregationLevel != nil {
			query += ` AND aggregation_level = ?`
			args = append(args, *filter.AggregationLevel)
		}
		if filter.Since != nil {
			query += ` AND timestamp >= ?`
			args = append(args, filter.Since.Unix())
		}
		if filter.Until != nil {
			query += ` AND timestamp <= ?`
			args = append(args, filter.Until.Unix())
		}
	}

	query += ` ORDER BY timestamp DESC`

	if filter != nil {
		if filter.Limit != nil {
			query += ` LIMIT ?`
			args = append(args, *filter.Limit)
		}
		if filter.Offset != nil {
			query += ` OFFSET ?`
			args = append(args, *filter.Offset)
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list metrics: %w", err)
	}
	defer rows.Close()

	var metrics []*Metric
	for rows.Next() {
		metric := &Metric{}
		var tagsJSON []byte
		var timestampUnix int64
		var expiresAtUnix *int64

		err := rows.Scan(&metric.ID, &metric.EntityID, &timestampUnix, &metric.MetricName, &metric.Value,
			&metric.AggregationLevel, &metric.SampleCount, &tagsJSON, &expiresAtUnix)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}

		// Parse JSON tags
		if len(tagsJSON) > 0 {
			metric.Tags = make(JSON)
			if err := metric.Tags.Scan(tagsJSON); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metric tags: %w", err)
			}
		}

		// Convert timestamps
		metric.Timestamp = time.Unix(timestampUnix, 0)
		if expiresAtUnix != nil {
			expiresAt := time.Unix(*expiresAtUnix, 0)
			metric.ExpiresAt = &expiresAt
		}

		metrics = append(metrics, metric)
	}

	return metrics, nil
}

// GetMetricSummary returns aggregated metric data
func (s *SQLiteStorage) GetMetricSummary(filter *MetricFilter) (*MetricSummary, error) {
	query := `
		SELECT 
			entity_id,
			metric_name,
			COUNT(*) as count,
			AVG(value) as average,
			MIN(value) as min,
			MAX(value) as max,
			value as latest,
			timestamp
		FROM metrics 
		WHERE 1=1`
	args := []interface{}{}

	if filter != nil {
		if filter.EntityID != nil {
			query += ` AND entity_id = ?`
			args = append(args, *filter.EntityID)
		}
		if filter.MetricName != nil {
			query += ` AND metric_name = ?`
			args = append(args, *filter.MetricName)
		}
		if filter.AggregationLevel != nil {
			query += ` AND aggregation_level = ?`
			args = append(args, *filter.AggregationLevel)
		}
		if filter.Since != nil {
			query += ` AND timestamp >= ?`
			args = append(args, filter.Since.Unix())
		}
		if filter.Until != nil {
			query += ` AND timestamp <= ?`
			args = append(args, filter.Until.Unix())
		}
	}

	query += ` GROUP BY entity_id, metric_name ORDER BY timestamp DESC LIMIT 1`

	summary := &MetricSummary{}
	var timestampUnix int64

	err := s.db.QueryRow(query, args...).Scan(
		&summary.EntityID, &summary.MetricName, &summary.Count,
		&summary.Average, &summary.Min, &summary.Max, &summary.Latest, &timestampUnix)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no metrics found")
		}
		return nil, fmt.Errorf("failed to get metric summary: %w", err)
	}

	summary.Timestamp = time.Unix(timestampUnix, 0)
	return summary, nil
}

// DeleteMetric removes a metric from the database
func (s *SQLiteStorage) DeleteMetric(id int64) error {
	query := `DELETE FROM metrics WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete metric: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("metric not found: %d", id)
	}

	return nil
}

// BATCH OPERATIONS

// BatchWrite writes multiple items in a single transaction
func (s *SQLiteStorage) BatchWrite(items []BatchItem) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, item := range items {
		switch item.Type {
		case "entity":
			entity, ok := item.Data.(*Entity)
			if !ok {
				return fmt.Errorf("invalid entity data type")
			}
			if err := s.createEntityInTx(tx, entity); err != nil {
				return err
			}
		case "event":
			event, ok := item.Data.(*Event)
			if !ok {
				return fmt.Errorf("invalid event data type")
			}
			if err := s.createEventInTx(tx, event); err != nil {
				return err
			}
		case "metric":
			metric, ok := item.Data.(*Metric)
			if !ok {
				return fmt.Errorf("invalid metric data type")
			}
			if err := s.createMetricInTx(tx, metric); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown batch item type: %s", item.Type)
		}
	}

	return tx.Commit()
}

// FlushBatch flushes any pending batch items
func (s *SQLiteStorage) FlushBatch() error {
	if len(s.batchItems) == 0 {
		return nil
	}

	items := make([]BatchItem, len(s.batchItems))
	copy(items, s.batchItems)
	s.batchItems = s.batchItems[:0] // Clear the slice

	return s.BatchWrite(items)
}

// Helper functions for batch operations in transactions
func (s *SQLiteStorage) createEntityInTx(tx *sql.Tx, entity *Entity) error {
	query := `
		INSERT INTO entities (type, name, status, details, created_at, updated_at, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	detailsJSON, err := entity.Details.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal entity details: %w", err)
	}

	var lastSeenUnix *int64
	if entity.LastSeen != nil {
		unix := entity.LastSeen.Unix()
		lastSeenUnix = &unix
	}

	result, err := tx.Exec(query,
		entity.Type, entity.Name, entity.Status, detailsJSON,
		entity.CreatedAt.Unix(), entity.UpdatedAt.Unix(), lastSeenUnix)
	if err != nil {
		return fmt.Errorf("failed to create entity in transaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get entity ID: %w", err)
	}
	entity.ID = id

	return nil
}

func (s *SQLiteStorage) createEventInTx(tx *sql.Tx, event *Event) error {
	query := `
		INSERT INTO events (entity_id, timestamp, event_type, severity, message, details, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	detailsJSON, err := event.Details.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal event details: %w", err)
	}

	var expiresAtUnix *int64
	if event.ExpiresAt != nil {
		unix := event.ExpiresAt.Unix()
		expiresAtUnix = &unix
	}

	result, err := tx.Exec(query,
		event.EntityID, event.Timestamp.Unix(), event.Type, event.Severity,
		event.Message, detailsJSON, expiresAtUnix)
	if err != nil {
		return fmt.Errorf("failed to create event in transaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get event ID: %w", err)
	}
	event.ID = id

	return nil
}

func (s *SQLiteStorage) createMetricInTx(tx *sql.Tx, metric *Metric) error {
	query := `
		INSERT INTO metrics (entity_id, timestamp, metric_name, value, aggregation_level, sample_count, tags, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	tagsJSON, err := metric.Tags.Value()
	if err != nil {
		return fmt.Errorf("failed to marshal metric tags: %w", err)
	}

	var expiresAtUnix *int64
	if metric.ExpiresAt != nil {
		unix := metric.ExpiresAt.Unix()
		expiresAtUnix = &unix
	}

	result, err := tx.Exec(query,
		metric.EntityID, metric.Timestamp.Unix(), metric.MetricName, metric.Value,
		metric.AggregationLevel, metric.SampleCount, tagsJSON, expiresAtUnix)
	if err != nil {
		return fmt.Errorf("failed to create metric in transaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get metric ID: %w", err)
	}
	metric.ID = id

	return nil
}

// MAINTENANCE OPERATIONS

// Cleanup removes expired data based on retention policies
func (s *SQLiteStorage) Cleanup() error {
	now := time.Now()

	// Clean up expired events
	_, err := s.db.Exec(`DELETE FROM events WHERE expires_at IS NOT NULL AND expires_at < ?`, now.Unix())
	if err != nil {
		return fmt.Errorf("failed to clean up expired events: %w", err)
	}

	// Clean up expired metrics
	_, err = s.db.Exec(`DELETE FROM metrics WHERE expires_at IS NOT NULL AND expires_at < ?`, now.Unix())
	if err != nil {
		return fmt.Errorf("failed to clean up expired metrics: %w", err)
	}

	// Clean up based on retention policy
	retentionDays := s.config.RetentionDays

	// Clean old events
	eventCutoff := now.AddDate(0, 0, -retentionDays.EventsDays)
	_, err = s.db.Exec(`DELETE FROM events WHERE expires_at IS NULL AND timestamp < ?`, eventCutoff.Unix())
	if err != nil {
		return fmt.Errorf("failed to clean up old events: %w", err)
	}

	// Clean old raw metrics
	metricCutoff := now.AddDate(0, 0, -retentionDays.MetricsDays)
	_, err = s.db.Exec(`DELETE FROM metrics WHERE expires_at IS NULL AND aggregation_level = 'raw' AND timestamp < ?`, metricCutoff.Unix())
	if err != nil {
		return fmt.Errorf("failed to clean up old raw metrics: %w", err)
	}

	// Clean old aggregated metrics
	aggregateCutoff := now.AddDate(0, 0, -retentionDays.AggregatesDays)
	_, err = s.db.Exec(`DELETE FROM metrics WHERE expires_at IS NULL AND aggregation_level != 'raw' AND timestamp < ?`, aggregateCutoff.Unix())
	if err != nil {
		return fmt.Errorf("failed to clean up old aggregated metrics: %w", err)
	}

	// Update cleanup timestamp
	_, err = s.db.Exec(`UPDATE metadata SET last_cleanup_timestamp = ?, updated_at = ? WHERE id = 1`,
		now.Unix(), now.Unix())
	if err != nil {
		return fmt.Errorf("failed to update cleanup timestamp: %w", err)
	}

	return nil
}

// Vacuum optimizes the database by reclaiming space
func (s *SQLiteStorage) Vacuum() error {
	_, err := s.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	return nil
}

// GetSystemHealth returns overall system health metrics
func (s *SQLiteStorage) GetSystemHealth() (*SystemHealth, error) {
	health := &SystemHealth{
		GeneratedAt: time.Now(),
	}

	// Get entity counts
	err := s.db.QueryRow(`SELECT COUNT(*) FROM entities`).Scan(&health.TotalEntities)
	if err != nil {
		return nil, fmt.Errorf("failed to get total entities: %w", err)
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM entities WHERE status = 'active'`).Scan(&health.ActiveEntities)
	if err != nil {
		return nil, fmt.Errorf("failed to get active entities: %w", err)
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM entities WHERE status = 'error'`).Scan(&health.ErrorEntities)
	if err != nil {
		return nil, fmt.Errorf("failed to get error entities: %w", err)
	}

	// Get recent events (last 24 hours)
	dayAgo := time.Now().AddDate(0, 0, -1).Unix()
	err = s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE timestamp >= ?`, dayAgo).Scan(&health.RecentEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent events: %w", err)
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE timestamp >= ? AND severity IN ('error', 'critical')`, dayAgo).Scan(&health.RecentErrors)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent errors: %w", err)
	}

	// Get service counts
	err = s.db.QueryRow(`SELECT COUNT(*) FROM entities WHERE type = 'service' AND status = 'active'`).Scan(&health.ServicesUp)
	if err != nil {
		return nil, fmt.Errorf("failed to get services up: %w", err)
	}

	err = s.db.QueryRow(`SELECT COUNT(*) FROM entities WHERE type = 'service' AND status != 'active'`).Scan(&health.ServicesDown)
	if err != nil {
		return nil, fmt.Errorf("failed to get services down: %w", err)
	}

	// Get site counts
	err = s.db.QueryRow(`SELECT COUNT(*) FROM entities WHERE type = 'site' AND status = 'active'`).Scan(&health.SitesActive)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sites: %w", err)
	}

	// Get last backup time
	var lastBackupUnix *int64
	err = s.db.QueryRow(`
		SELECT MAX(timestamp) FROM events 
		WHERE event_type = 'backup' AND severity = 'info'
	`).Scan(&lastBackupUnix)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get last backup time: %w", err)
	}
	if lastBackupUnix != nil {
		lastBackup := time.Unix(*lastBackupUnix, 0)
		health.LastBackup = &lastBackup
	}

	// Get metrics summary for key system metrics
	metricNames := []string{"cpu_usage", "memory_usage", "disk_usage_root", "load_1"}
	for _, metricName := range metricNames {
		summary := &MetricSummary{MetricName: metricName}
		err := s.db.QueryRow(`
			SELECT COUNT(*), AVG(value), MIN(value), MAX(value), 
				   (SELECT value FROM metrics WHERE metric_name = ? ORDER BY timestamp DESC LIMIT 1) as latest,
				   (SELECT timestamp FROM metrics WHERE metric_name = ? ORDER BY timestamp DESC LIMIT 1) as latest_ts
			FROM metrics WHERE metric_name = ? AND timestamp >= ?
		`, metricName, metricName, metricName, dayAgo).Scan(
			&summary.Count, &summary.Average, &summary.Min, &summary.Max, &summary.Latest, &summary.Timestamp)

		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get metric summary for %s: %w", metricName, err)
		}

		if err != sql.ErrNoRows {
			health.MetricsSummary = append(health.MetricsSummary, *summary)
		}
	}

	return health, nil
}
