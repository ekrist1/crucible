package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"crucible/internal/logging"
	"crucible/internal/monitor"
)

// StorageAdapter bridges the monitoring system with persistent storage
type StorageAdapter struct {
	storage      Storage
	logger       *logging.Logger
	cleanupSched *CleanupScheduler
	entityCache  map[string]*Entity // Cache for entity lookups by type/name
}

// NewStorageAdapter creates a new storage adapter
func NewStorageAdapter(config *monitor.Config, logger *logging.Logger) (*StorageAdapter, error) {
	var storage Storage
	var err error

	switch config.Storage.Type {
	case "sqlite":
		storageConfig := &Config{
			DatabasePath:    config.Storage.SQLite.Path,
			BatchSize:       config.Storage.SQLite.BatchSize,
			CleanupInterval: config.Storage.SQLite.CleanupInterval,
			BackupEnabled:   config.Storage.SQLite.BackupEnabled,
			BackupInterval:  config.Storage.SQLite.BackupInterval,
			RetentionDays: RetentionDays{
				EventsDays:     config.Storage.SQLite.Retention.EventsDays,
				MetricsDays:    config.Storage.SQLite.Retention.MetricsDays,
				AggregatesDays: config.Storage.SQLite.Retention.AggregatesDays,
			},
		}

		// Ensure directory exists
		if err := ensureDir(filepath.Dir(storageConfig.DatabasePath)); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}

		storage, err = NewSQLiteStorage(storageConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create SQLite storage: %w", err)
		}
	case "memory":
		// For backwards compatibility - could implement in-memory storage
		return nil, fmt.Errorf("memory storage not implemented with persistent layer")
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Storage.Type)
	}

	adapter := &StorageAdapter{
		storage:     storage,
		logger:      logger,
		entityCache: make(map[string]*Entity),
	}

	// Start cleanup scheduler for SQLite storage
	if sqliteStorage, ok := storage.(*SQLiteStorage); ok {
		adapter.cleanupSched = NewCleanupScheduler(sqliteStorage, logger, config.Storage.SQLite.CleanupInterval)
		adapter.cleanupSched.Start()
	}

	return adapter, nil
}

// Close shuts down the storage adapter
func (sa *StorageAdapter) Close() error {
	if sa.cleanupSched != nil {
		sa.cleanupSched.Stop()
	}
	return sa.storage.Close()
}

// SYSTEM METRICS INTEGRATION

// StoreSystemMetrics stores system metrics as entities and metrics
func (sa *StorageAdapter) StoreSystemMetrics(metrics *monitor.SystemMetrics) error {
	now := time.Now()

	// Get or create server entity
	serverEntity, err := sa.getOrCreateEntity(EntityTypeServer, "localhost")
	if err != nil {
		return fmt.Errorf("failed to get server entity: %w", err)
	}

	// Update server entity status and last seen
	serverEntity.Status = EntityStatusActive
	serverEntity.Touch()
	if err := sa.storage.UpdateEntity(serverEntity); err != nil {
		return fmt.Errorf("failed to update server entity: %w", err)
	}

	// Store CPU metrics
	if err := sa.storeSystemMetric(serverEntity.ID, "cpu_usage", metrics.CPU.UsagePercent, now, map[string]interface{}{
		"user":   metrics.CPU.UserPercent,
		"system": metrics.CPU.SystemPercent,
		"idle":   metrics.CPU.IdlePercent,
		"iowait": metrics.CPU.IOWaitPercent,
	}); err != nil {
		return fmt.Errorf("failed to store CPU metrics: %w", err)
	}

	// Store memory metrics
	if err := sa.storeSystemMetric(serverEntity.ID, "memory_usage", metrics.Memory.UsagePercent, now, map[string]interface{}{
		"total":     metrics.Memory.TotalBytes,
		"available": metrics.Memory.AvailableBytes,
		"used":      metrics.Memory.UsedBytes,
		"free":      metrics.Memory.FreeBytes,
	}); err != nil {
		return fmt.Errorf("failed to store memory metrics: %w", err)
	}

	// Store load metrics
	if err := sa.storeSystemMetric(serverEntity.ID, "load_1", metrics.Load.Load1, now, nil); err != nil {
		return fmt.Errorf("failed to store load metrics: %w", err)
	}
	if err := sa.storeSystemMetric(serverEntity.ID, "load_5", metrics.Load.Load5, now, nil); err != nil {
		return fmt.Errorf("failed to store load metrics: %w", err)
	}
	if err := sa.storeSystemMetric(serverEntity.ID, "load_15", metrics.Load.Load15, now, nil); err != nil {
		return fmt.Errorf("failed to store load metrics: %w", err)
	}

	// Store disk metrics
	for _, disk := range metrics.Disk {
		diskEntity, err := sa.getOrCreateEntity("disk", disk.MountPoint)
		if err != nil {
			return fmt.Errorf("failed to get disk entity: %w", err)
		}

		diskEntity.Status = EntityStatusActive
		diskEntity.Touch()
		diskEntity.Details["device"] = disk.Device
		if err := sa.storage.UpdateEntity(diskEntity); err != nil {
			return fmt.Errorf("failed to update disk entity: %w", err)
		}

		metricName := "disk_usage"
		if disk.MountPoint == "/" {
			metricName = "disk_usage_root"
		}

		if err := sa.storeSystemMetric(diskEntity.ID, metricName, disk.UsagePercent, now, map[string]interface{}{
			"total": disk.TotalBytes,
			"used":  disk.UsedBytes,
			"free":  disk.FreeBytes,
		}); err != nil {
			return fmt.Errorf("failed to store disk metrics: %w", err)
		}
	}

	// Store network metrics
	for _, iface := range metrics.Network {
		netEntity, err := sa.getOrCreateEntity("network_interface", iface.Interface)
		if err != nil {
			return fmt.Errorf("failed to get network entity: %w", err)
		}

		netEntity.Status = EntityStatusActive
		netEntity.Touch()
		if err := sa.storage.UpdateEntity(netEntity); err != nil {
			return fmt.Errorf("failed to update network entity: %w", err)
		}

		if err := sa.storeSystemMetric(netEntity.ID, "network_bytes_sent", float64(iface.BytesSent), now, nil); err != nil {
			return fmt.Errorf("failed to store network sent metrics: %w", err)
		}
		if err := sa.storeSystemMetric(netEntity.ID, "network_bytes_recv", float64(iface.BytesRecv), now, nil); err != nil {
			return fmt.Errorf("failed to store network recv metrics: %w", err)
		}
	}

	return nil
}

// SERVICE METRICS INTEGRATION

// StoreServiceMetrics stores service status as entities and events
func (sa *StorageAdapter) StoreServiceMetrics(services []monitor.ServiceStatus) error {
	for _, service := range services {
		// Get or create service entity
		serviceEntity, err := sa.getOrCreateEntity(EntityTypeService, service.Name)
		if err != nil {
			return fmt.Errorf("failed to get service entity: %w", err)
		}

		// Determine entity status
		var entityStatus string
		if service.Active == "active" && service.Sub == "running" {
			entityStatus = EntityStatusActive
		} else if service.Active == "failed" {
			entityStatus = EntityStatusError
		} else {
			entityStatus = EntityStatusInactive
		}

		// Check if status changed
		statusChanged := serviceEntity.Status != entityStatus

		// Update service entity
		serviceEntity.Status = entityStatus
		serviceEntity.Touch()
		serviceEntity.Details["active_state"] = service.Active
		serviceEntity.Details["sub_state"] = service.Sub
		serviceEntity.Details["restart_count"] = service.RestartCount
		if !service.Since.IsZero() {
			serviceEntity.Details["since"] = service.Since.Unix()
		}
		if !service.LastRestart.IsZero() {
			serviceEntity.Details["last_restart"] = service.LastRestart.Unix()
		}

		// Copy metadata
		for key, value := range service.Metadata {
			serviceEntity.Details[key] = value
		}

		if err := sa.storage.UpdateEntity(serviceEntity); err != nil {
			return fmt.Errorf("failed to update service entity: %w", err)
		}

		// Create event if status changed
		if statusChanged {
			event := NewEvent(&serviceEntity.ID, EventTypeInfo, fmt.Sprintf("Service %s status changed to %s", service.Name, entityStatus))
			event.Severity = SeverityInfo
			if entityStatus == EntityStatusError {
				event.Type = EventTypeError
				event.Severity = SeverityError
			}

			event.Details["previous_status"] = serviceEntity.Status
			event.Details["new_status"] = entityStatus
			event.Details["active_state"] = service.Active
			event.Details["sub_state"] = service.Sub

			if err := sa.storage.CreateEvent(event); err != nil {
				return fmt.Errorf("failed to create service status event: %w", err)
			}
		}
	}

	return nil
}

// HTTP CHECK INTEGRATION

// StoreHTTPCheckResults stores HTTP check results as entities and metrics
func (sa *StorageAdapter) StoreHTTPCheckResults(results []monitor.HTTPCheckResult) error {
	now := time.Now()

	for _, result := range results {
		// Get or create site entity
		siteEntity, err := sa.getOrCreateEntity(EntityTypeSite, result.Name)
		if err != nil {
			return fmt.Errorf("failed to get site entity: %w", err)
		}

		// Determine entity status
		var entityStatus string
		if result.Success {
			entityStatus = EntityStatusActive
		} else {
			entityStatus = EntityStatusError
		}

		// Check if status changed
		statusChanged := siteEntity.Status != entityStatus

		// Update site entity
		siteEntity.Status = entityStatus
		siteEntity.Touch()
		siteEntity.Details["url"] = result.URL
		siteEntity.Details["last_status_code"] = result.StatusCode
		siteEntity.Details["last_response_time_ms"] = result.ResponseTime.Milliseconds()
		if result.Error != "" {
			siteEntity.Details["last_error"] = result.Error
		}

		if err := sa.storage.UpdateEntity(siteEntity); err != nil {
			return fmt.Errorf("failed to update site entity: %w", err)
		}

		// Store response time metric
		if err := sa.storeSystemMetric(siteEntity.ID, "response_time_ms", float64(result.ResponseTime.Milliseconds()), now, map[string]interface{}{
			"status_code": result.StatusCode,
			"success":     result.Success,
		}); err != nil {
			return fmt.Errorf("failed to store response time metric: %w", err)
		}

		// Create event if status changed or there's an error
		if statusChanged || !result.Success {
			var eventType, severity string
			var message string

			if result.Success {
				eventType = EventTypeInfo
				severity = SeverityInfo
				message = fmt.Sprintf("Site %s is now responding correctly", result.Name)
			} else {
				eventType = EventTypeError
				severity = SeverityError
				message = fmt.Sprintf("Site %s check failed: %s", result.Name, result.Error)
			}

			event := NewEvent(&siteEntity.ID, eventType, message)
			event.Severity = severity
			event.Details["url"] = result.URL
			event.Details["status_code"] = result.StatusCode
			event.Details["response_time_ms"] = result.ResponseTime.Milliseconds()
			if result.Error != "" {
				event.Details["error"] = result.Error
			}

			if err := sa.storage.CreateEvent(event); err != nil {
				return fmt.Errorf("failed to create HTTP check event: %w", err)
			}
		}
	}

	return nil
}

// HELPER METHODS

// getOrCreateEntity gets an existing entity or creates a new one
func (sa *StorageAdapter) getOrCreateEntity(entityType, name string) (*Entity, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", entityType, name)
	if entity, exists := sa.entityCache[cacheKey]; exists {
		return entity, nil
	}

	// Try to get from storage
	entity, err := sa.storage.GetEntityByName(entityType, name)
	if err == nil {
		sa.entityCache[cacheKey] = entity
		return entity, nil
	}

	// Create new entity
	entity = NewEntity(entityType, name)
	if err := sa.storage.CreateEntity(entity); err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	sa.entityCache[cacheKey] = entity
	return entity, nil
}

// storeSystemMetric stores a system metric with TTL
func (sa *StorageAdapter) storeSystemMetric(entityID int64, metricName string, value float64, timestamp time.Time, tags map[string]interface{}) error {
	metric := NewMetric(&entityID, metricName, value)
	metric.Timestamp = timestamp

	// Set TTL based on metric type (30 days for raw metrics)
	metric.SetTTL(30)

	// Add tags if provided
	if tags != nil {
		for key, val := range tags {
			metric.Tags[key] = val
		}
	}

	return sa.storage.CreateMetric(metric)
}

// GetStorageStats returns storage statistics
func (sa *StorageAdapter) GetStorageStats() (interface{}, error) {
	if sqliteStorage, ok := sa.storage.(*SQLiteStorage); ok {
		return sqliteStorage.GetDatabaseInfo()
	}
	return nil, fmt.Errorf("storage stats not supported for this storage type")
}

// GetSystemHealth returns system health from storage
func (sa *StorageAdapter) GetSystemHealth() (*SystemHealth, error) {
	return sa.storage.GetSystemHealth()
}

// GetStorage returns the underlying storage interface for direct access
func (sa *StorageAdapter) GetStorage() Storage {
	return sa.storage
}

// ListEntities returns entities based on filter criteria
func (sa *StorageAdapter) ListEntities(filter *EntityFilter) ([]*Entity, error) {
	return sa.storage.ListEntities(filter)
}

// GetEntity returns a specific entity by ID
func (sa *StorageAdapter) GetEntity(id int64) (*Entity, error) {
	return sa.storage.GetEntity(id)
}

// ListEvents returns events based on filter criteria
func (sa *StorageAdapter) ListEvents(filter *EventFilter) ([]*Event, error) {
	return sa.storage.ListEvents(filter)
}

// ListMetrics returns metrics based on filter criteria
func (sa *StorageAdapter) ListMetrics(filter *MetricFilter) ([]*Metric, error) {
	return sa.storage.ListMetrics(filter)
}

// GetMetricSummary returns aggregated metric data
func (sa *StorageAdapter) GetMetricSummary(filter *MetricFilter) (*MetricSummary, error) {
	return sa.storage.GetMetricSummary(filter)
}

// ensureDir creates a directory if it doesn't exist
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
