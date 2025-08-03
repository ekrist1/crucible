package storage

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"crucible/internal/logging"
)

// Migration represents a database schema migration
type Migration struct {
	Version     string
	Description string
	UpSQL       string
	DownSQL     string
}

// MigrationManager handles database schema migrations
type MigrationManager struct {
	db         *sql.DB
	logger     *logging.Logger
	migrations []Migration
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB, logger *logging.Logger) *MigrationManager {
	return &MigrationManager{
		db:         db,
		logger:     logger,
		migrations: GetAllMigrations(),
	}
}

// GetAllMigrations returns all available migrations in order
func GetAllMigrations() []Migration {
	return []Migration{
		{
			Version:     "1.0.0",
			Description: "Initial schema creation",
			UpSQL: `
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
			`,
			DownSQL: `
				DROP INDEX IF EXISTS idx_metrics_expires_at;
				DROP INDEX IF EXISTS idx_metrics_aggregation_level;
				DROP INDEX IF EXISTS idx_metrics_entity_metric_time;
				DROP INDEX IF EXISTS idx_metrics_entity_id;
				DROP INDEX IF EXISTS idx_metrics_metric_name;
				DROP INDEX IF EXISTS idx_metrics_timestamp;
				DROP INDEX IF EXISTS idx_events_expires_at;
				DROP INDEX IF EXISTS idx_events_severity;
				DROP INDEX IF EXISTS idx_events_entity_id;
				DROP INDEX IF EXISTS idx_events_event_type;
				DROP INDEX IF EXISTS idx_events_timestamp;
				DROP INDEX IF EXISTS idx_entities_last_seen;
				DROP INDEX IF EXISTS idx_entities_status;
				DROP INDEX IF EXISTS idx_entities_name;
				DROP INDEX IF EXISTS idx_entities_type;
				DROP TABLE IF EXISTS metrics;
				DROP TABLE IF EXISTS events;
				DROP TABLE IF EXISTS entities;
				DROP TABLE IF EXISTS metadata;
			`,
		},
		{
			Version:     "1.1.0",
			Description: "Add migration tracking table",
			UpSQL: `
				-- Migration tracking table
				CREATE TABLE IF NOT EXISTS schema_migrations (
					version TEXT PRIMARY KEY,
					description TEXT,
					applied_at INTEGER NOT NULL DEFAULT (unixepoch()),
					checksum TEXT
				);
			`,
			DownSQL: `
				DROP TABLE IF EXISTS schema_migrations;
			`,
		},
	}
}

// ApplyMigrations applies all pending migrations
func (mm *MigrationManager) ApplyMigrations() error {
	mm.logger.Info("Checking for pending migrations")

	// Ensure migration tracking table exists
	if err := mm.ensureMigrationTable(); err != nil {
		return fmt.Errorf("failed to ensure migration table: %w", err)
	}

	// Get applied migrations
	applied, err := mm.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Sort migrations by version
	sort.Slice(mm.migrations, func(i, j int) bool {
		return mm.migrations[i].Version < mm.migrations[j].Version
	})

	// Apply pending migrations
	pending := mm.getPendingMigrations(applied)
	if len(pending) == 0 {
		mm.logger.Info("No pending migrations")
		return nil
	}

	mm.logger.Info("Applying migrations", "count", len(pending))

	for _, migration := range pending {
		if err := mm.applyMigration(migration); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
		}
	}

	// Update schema hash in metadata
	if err := mm.updateSchemaHash(); err != nil {
		mm.logger.Warn("Failed to update schema hash", "error", err)
	}

	mm.logger.Info("All migrations applied successfully")
	return nil
}

// RollbackMigration rolls back the last applied migration
func (mm *MigrationManager) RollbackMigration() error {
	// Get the last applied migration
	var version, description string
	err := mm.db.QueryRow(`
		SELECT version, description 
		FROM schema_migrations 
		ORDER BY applied_at DESC 
		LIMIT 1
	`).Scan(&version, &description)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no migrations to rollback")
		}
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	// Find the migration
	var migration *Migration
	for _, m := range mm.migrations {
		if m.Version == version {
			migration = &m
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %s not found", version)
	}

	mm.logger.Info("Rolling back migration", "version", version, "description", description)

	// Execute rollback in transaction
	tx, err := mm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute down SQL
	if migration.DownSQL != "" {
		if _, err := tx.Exec(migration.DownSQL); err != nil {
			return fmt.Errorf("failed to execute rollback SQL: %w", err)
		}
	}

	// Remove from migrations table
	if _, err := tx.Exec(`DELETE FROM schema_migrations WHERE version = ?`, version); err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	mm.logger.Info("Migration rolled back successfully", "version", version)
	return nil
}

// GetMigrationStatus returns the status of all migrations
func (mm *MigrationManager) GetMigrationStatus() ([]MigrationStatus, error) {
	applied, err := mm.getAppliedMigrations()
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	var status []MigrationStatus
	for _, migration := range mm.migrations {
		ms := MigrationStatus{
			Version:     migration.Version,
			Description: migration.Description,
			Applied:     false,
		}

		if info, exists := applied[migration.Version]; exists {
			ms.Applied = true
			ms.AppliedAt = &info.AppliedAt
		}

		status = append(status, ms)
	}

	return status, nil
}

// ensureMigrationTable creates the migration tracking table if it doesn't exist
func (mm *MigrationManager) ensureMigrationTable() error {
	_, err := mm.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			description TEXT,
			applied_at INTEGER NOT NULL DEFAULT (unixepoch()),
			checksum TEXT
		)
	`)
	return err
}

// getAppliedMigrations returns a map of applied migrations
func (mm *MigrationManager) getAppliedMigrations() (map[string]AppliedMigration, error) {
	rows, err := mm.db.Query(`
		SELECT version, description, applied_at, checksum 
		FROM schema_migrations 
		ORDER BY applied_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]AppliedMigration)
	for rows.Next() {
		var am AppliedMigration
		var appliedAtUnix int64

		err := rows.Scan(&am.Version, &am.Description, &appliedAtUnix, &am.Checksum)
		if err != nil {
			return nil, err
		}

		am.AppliedAt = time.Unix(appliedAtUnix, 0)
		applied[am.Version] = am
	}

	return applied, nil
}

// getPendingMigrations returns migrations that haven't been applied
func (mm *MigrationManager) getPendingMigrations(applied map[string]AppliedMigration) []Migration {
	var pending []Migration
	for _, migration := range mm.migrations {
		if _, exists := applied[migration.Version]; !exists {
			pending = append(pending, migration)
		}
	}
	return pending
}

// applyMigration applies a single migration
func (mm *MigrationManager) applyMigration(migration Migration) error {
	mm.logger.Info("Applying migration", "version", migration.Version, "description", migration.Description)

	// Calculate checksum
	checksum := mm.calculateChecksum(migration.UpSQL)

	// Execute migration in transaction
	tx, err := mm.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute up SQL
	if _, err := tx.Exec(migration.UpSQL); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Record migration
	if _, err := tx.Exec(`
		INSERT INTO schema_migrations (version, description, applied_at, checksum)
		VALUES (?, ?, ?, ?)
	`, migration.Version, migration.Description, time.Now().Unix(), checksum); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	mm.logger.Info("Migration applied successfully", "version", migration.Version)
	return nil
}

// calculateChecksum calculates a checksum for migration SQL
func (mm *MigrationManager) calculateChecksum(sql string) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(sql)))
	return fmt.Sprintf("%x", hash)
}

// updateSchemaHash updates the schema hash in metadata table
func (mm *MigrationManager) updateSchemaHash() error {
	// Calculate hash of current schema
	var allSQL strings.Builder
	for _, migration := range mm.migrations {
		allSQL.WriteString(migration.UpSQL)
	}

	hash := mm.calculateChecksum(allSQL.String())

	_, err := mm.db.Exec(`
		UPDATE metadata 
		SET schema_hash = ?, updated_at = ? 
		WHERE id = 1
	`, hash, time.Now().Unix())

	return err
}

// ValidateSchema validates the current database schema against migrations
func (mm *MigrationManager) ValidateSchema() error {
	// Get current schema hash
	var currentHash string
	err := mm.db.QueryRow(`SELECT schema_hash FROM metadata WHERE id = 1`).Scan(&currentHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("metadata not found")
		}
		return fmt.Errorf("failed to get schema hash: %w", err)
	}

	// Calculate expected hash
	var allSQL strings.Builder
	for _, migration := range mm.migrations {
		allSQL.WriteString(migration.UpSQL)
	}
	expectedHash := mm.calculateChecksum(allSQL.String())

	if currentHash != expectedHash {
		return fmt.Errorf("schema validation failed: hash mismatch (expected: %s, got: %s)", expectedHash, currentHash)
	}

	mm.logger.Debug("Schema validation passed", "hash", currentHash)
	return nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     string     `json:"version"`
	Description string     `json:"description"`
	Applied     bool       `json:"applied"`
	AppliedAt   *time.Time `json:"applied_at,omitempty"`
}

// AppliedMigration represents a migration that has been applied
type AppliedMigration struct {
	Version     string    `json:"version"`
	Description string    `json:"description"`
	AppliedAt   time.Time `json:"applied_at"`
	Checksum    string    `json:"checksum"`
}
