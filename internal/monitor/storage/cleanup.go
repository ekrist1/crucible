package storage

import (
	"context"
	"fmt"
	"time"

	"crucible/internal/logging"
)

// CleanupScheduler handles periodic database cleanup operations
type CleanupScheduler struct {
	storage  *SQLiteStorage
	logger   *logging.Logger
	interval time.Duration
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewCleanupScheduler creates a new cleanup scheduler
func NewCleanupScheduler(storage *SQLiteStorage, logger *logging.Logger, interval time.Duration) *CleanupScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	return &CleanupScheduler{
		storage:  storage,
		logger:   logger,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins the cleanup scheduler
func (cs *CleanupScheduler) Start() {
	cs.logger.Info("Starting database cleanup scheduler", "interval", cs.interval)

	go cs.run()
}

// Stop stops the cleanup scheduler
func (cs *CleanupScheduler) Stop() {
	cs.logger.Info("Stopping database cleanup scheduler")
	cs.cancel()
}

// run executes the cleanup loop
func (cs *CleanupScheduler) run() {
	// Run cleanup immediately on start
	cs.performCleanup()

	ticker := time.NewTicker(cs.interval)
	defer ticker.Stop()

	for {
		select {
		case <-cs.ctx.Done():
			return
		case <-ticker.C:
			cs.performCleanup()
		}
	}
}

// performCleanup executes the cleanup operation
func (cs *CleanupScheduler) performCleanup() {
	cs.logger.Debug("Starting database cleanup")

	start := time.Now()

	// Get pre-cleanup stats
	preStats, err := cs.storage.GetDatabaseInfo()
	if err != nil {
		cs.logger.Error("Failed to get pre-cleanup database stats", "error", err)
		return
	}

	// Perform cleanup
	if err := cs.storage.Cleanup(); err != nil {
		cs.logger.Error("Database cleanup failed", "error", err)
		return
	}

	// Get post-cleanup stats
	postStats, err := cs.storage.GetDatabaseInfo()
	if err != nil {
		cs.logger.Error("Failed to get post-cleanup database stats", "error", err)
		return
	}

	duration := time.Since(start)
	spaceSaved := preStats.DatabaseSize - postStats.DatabaseSize
	entitiesRemoved := preStats.EntityCount - postStats.EntityCount
	eventsRemoved := preStats.EventCount - postStats.EventCount
	metricsRemoved := preStats.MetricCount - postStats.MetricCount

	cs.logger.Info("Database cleanup completed",
		"duration", duration,
		"space_saved_bytes", spaceSaved,
		"entities_removed", entitiesRemoved,
		"events_removed", eventsRemoved,
		"metrics_removed", metricsRemoved,
		"final_db_size_bytes", postStats.DatabaseSize,
	)

	// Perform vacuum if significant space was freed
	if spaceSaved > 1024*1024 { // 1MB threshold
		cs.logger.Debug("Running VACUUM to reclaim space")
		if err := cs.storage.Vacuum(); err != nil {
			cs.logger.Error("Database vacuum failed", "error", err)
		} else {
			cs.logger.Debug("Database vacuum completed")
		}
	}
}

// RetentionPolicy defines data retention rules
type RetentionPolicy struct {
	EntityTypes map[string]time.Duration `yaml:"entity_types"`
	EventTypes  map[string]time.Duration `yaml:"event_types"`
	MetricTypes map[string]time.Duration `yaml:"metric_types"`
	GlobalRules GlobalRetentionRules     `yaml:"global"`
}

// GlobalRetentionRules defines global retention policies
type GlobalRetentionRules struct {
	MaxDatabaseSize int64         `yaml:"max_database_size_bytes"`
	MaxEventAge     time.Duration `yaml:"max_event_age"`
	MaxMetricAge    time.Duration `yaml:"max_metric_age"`
	AggregationAge  time.Duration `yaml:"aggregation_age"`
}

// ApplyRetentionPolicy applies custom retention policies beyond the basic cleanup
func (s *SQLiteStorage) ApplyRetentionPolicy(policy *RetentionPolicy) error {
	now := time.Now()

	// Apply entity-specific retention
	for entityType, maxAge := range policy.EntityTypes {
		cutoff := now.Add(-maxAge)
		_, err := s.db.Exec(`
			DELETE FROM entities 
			WHERE type = ? AND updated_at < ? AND status != 'active'
		`, entityType, cutoff.Unix())
		if err != nil {
			return fmt.Errorf("failed to apply retention for entity type %s: %w", entityType, err)
		}
	}

	// Apply event-specific retention
	for eventType, maxAge := range policy.EventTypes {
		cutoff := now.Add(-maxAge)
		_, err := s.db.Exec(`
			DELETE FROM events 
			WHERE event_type = ? AND timestamp < ?
		`, eventType, cutoff.Unix())
		if err != nil {
			return fmt.Errorf("failed to apply retention for event type %s: %w", eventType, err)
		}
	}

	// Apply metric-specific retention
	for metricName, maxAge := range policy.MetricTypes {
		cutoff := now.Add(-maxAge)
		_, err := s.db.Exec(`
			DELETE FROM metrics 
			WHERE metric_name = ? AND timestamp < ? AND aggregation_level = 'raw'
		`, metricName, cutoff.Unix())
		if err != nil {
			return fmt.Errorf("failed to apply retention for metric type %s: %w", metricName, err)
		}
	}

	// Apply size-based cleanup if database is too large
	if policy.GlobalRules.MaxDatabaseSize > 0 {
		dbInfo, err := s.GetDatabaseInfo()
		if err != nil {
			return fmt.Errorf("failed to get database size: %w", err)
		}

		if dbInfo.DatabaseSize > policy.GlobalRules.MaxDatabaseSize {
			// Remove oldest raw metrics first
			_, err := s.db.Exec(`
				DELETE FROM metrics 
				WHERE aggregation_level = 'raw' 
				AND id IN (
					SELECT id FROM metrics 
					WHERE aggregation_level = 'raw' 
					ORDER BY timestamp ASC 
					LIMIT 1000
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to apply size-based cleanup: %w", err)
			}
		}
	}

	return nil
}

// GetRetentionStats returns statistics about data retention
func (s *SQLiteStorage) GetRetentionStats() (*RetentionStats, error) {
	stats := &RetentionStats{
		GeneratedAt: time.Now(),
	}

	// Get data age distribution
	err := s.db.QueryRow(`
		SELECT 
			MIN(timestamp) as oldest_event,
			MAX(timestamp) as newest_event,
			COUNT(*) as total_events
		FROM events
	`).Scan(&stats.OldestEventUnix, &stats.NewestEventUnix, &stats.TotalEvents)
	if err != nil {
		return nil, fmt.Errorf("failed to get event age stats: %w", err)
	}

	err = s.db.QueryRow(`
		SELECT 
			MIN(timestamp) as oldest_metric,
			MAX(timestamp) as newest_metric,
			COUNT(*) as total_metrics
		FROM metrics
	`).Scan(&stats.OldestMetricUnix, &stats.NewestMetricUnix, &stats.TotalMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to get metric age stats: %w", err)
	}

	// Get aggregation level distribution
	rows, err := s.db.Query(`
		SELECT aggregation_level, COUNT(*) 
		FROM metrics 
		GROUP BY aggregation_level
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get aggregation stats: %w", err)
	}
	defer rows.Close()

	stats.AggregationStats = make(map[string]int64)
	for rows.Next() {
		var level string
		var count int64
		if err := rows.Scan(&level, &count); err != nil {
			return nil, fmt.Errorf("failed to scan aggregation stats: %w", err)
		}
		stats.AggregationStats[level] = count
	}

	// Convert timestamps
	if stats.OldestEventUnix > 0 {
		oldest := time.Unix(stats.OldestEventUnix, 0)
		stats.OldestEvent = &oldest
	}
	if stats.NewestEventUnix > 0 {
		newest := time.Unix(stats.NewestEventUnix, 0)
		stats.NewestEvent = &newest
	}
	if stats.OldestMetricUnix > 0 {
		oldest := time.Unix(stats.OldestMetricUnix, 0)
		stats.OldestMetric = &oldest
	}
	if stats.NewestMetricUnix > 0 {
		newest := time.Unix(stats.NewestMetricUnix, 0)
		stats.NewestMetric = &newest
	}

	return stats, nil
}

// RetentionStats provides information about data retention
type RetentionStats struct {
	GeneratedAt      time.Time        `json:"generated_at"`
	TotalEvents      int64            `json:"total_events"`
	TotalMetrics     int64            `json:"total_metrics"`
	OldestEvent      *time.Time       `json:"oldest_event,omitempty"`
	NewestEvent      *time.Time       `json:"newest_event,omitempty"`
	OldestMetric     *time.Time       `json:"oldest_metric,omitempty"`
	NewestMetric     *time.Time       `json:"newest_metric,omitempty"`
	OldestEventUnix  int64            `json:"-"`
	NewestEventUnix  int64            `json:"-"`
	OldestMetricUnix int64            `json:"-"`
	NewestMetricUnix int64            `json:"-"`
	AggregationStats map[string]int64 `json:"aggregation_stats"`
}
