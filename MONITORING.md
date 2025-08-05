
# Crucible Monitoring System

## Overview

The Crucible Monitoring System provides comprehensive monitoring for Laravel server environments with real-time metrics collection, alerting, and persistent data storage. The system tracks system metrics, service status, HTTP endpoint health, and provides a unified dashboard for monitoring your infrastructure.

## Architecture

### Core Components

- **Monitoring Agent**: Collects metrics and manages alerts
- **TUI Dashboard**: Real-time terminal-based monitoring interface
- **Storage Layer**: SQLite-based persistent data storage
- **Alert System**: Rule-based alerting with multiple severity levels
- **HTTP API**: RESTful endpoints for programmatic access

### Data Collection

The system collects three main types of data:

1. **System Metrics**: CPU, memory, disk, network, and load average
2. **Service Status**: systemd service monitoring with state tracking
3. **HTTP Health Checks**: Website/API endpoint availability monitoring

## Storage System

### Database Location

The monitoring system uses SQLite for persistent data storage:

- **Default Location**: `/var/lib/crucible/monitor.db`
- **Configurable**: Can be changed via `configs/monitor.yaml`
- **Permissions**: Requires write access to the parent directory

### Database Schema

The system uses an **entity-centric design** with the following core tables:

- **`entities`**: Monitored resources (services, sites, servers, disks, network interfaces)
- **`events`**: Historical events and state changes
- **`metrics`**: Time-series data points with aggregation support
- **`metadata`**: Database versioning and configuration
- **`schema_migrations`**: Migration tracking for schema evolution

### Data Retention Policy

Default retention periods (configurable in `configs/monitor.yaml`):

- **Events**: 90 days
- **Raw Metrics**: 30 days  
- **Aggregated Metrics**: 365 days

### Automatic Cleanup

- **Cleanup Interval**: Every hour (configurable)
- **Expired Data Removal**: Automatic cleanup based on TTL and retention policies
- **Database Optimization**: Periodic VACUUM operations to reclaim space
- **Storage Statistics**: Real-time database size and record count monitoring

## Configuration

### Main Configuration File

**Location**: `configs/monitor.yaml`

Key configuration sections:

```yaml
# Storage configuration
storage:
  type: "sqlite"
  sqlite:
    path: "/var/lib/crucible/monitor.db"
    batch_size: 100
    cleanup_interval: "1h"
    backup_enabled: true
    backup_interval: "24h"
    retention:
      events_days: 90
      metrics_days: 30
      aggregates_days: 365

# Data collectors
collectors:
  system:
    enabled: true
    interval: "30s"
    metrics: ["cpu", "memory", "disk", "network", "load"]
  
  services:
    enabled: true
    interval: "60s"
    services: ["mysqld", "docker", "sshd", "firewalld", "NetworkManager"]
  
  http_checks:
    enabled: true
    checks:
      - name: "uxvalidate"
        url: "https://uxvalidate.com"
        interval: "60s"
        timeout: "10s"
        expected_status: 200

# Alert thresholds
alerts:
  enabled: true
  thresholds:
    cpu_percent: 80.0
    memory_percent: 90.0
    disk_percent: 85.0
    load_average: 5.0
    response_time_ms: 5000
```

### Alert Configuration

**Location**: `configs/alerts.yaml`

Defines alert rules with conditions and thresholds. Supports:
- System metric alerts (CPU, memory, disk, load)
- Service status alerts (service down/failed)
- HTTP endpoint alerts (response time, status codes)

## API Endpoints

The monitoring agent exposes an HTTP API on `127.0.0.1:9090` (configurable):

### Metrics Endpoints
- `GET /api/v1/metrics/system` - Current system metrics
- `GET /api/v1/metrics/services` - Service status information
- `GET /api/v1/metrics/http` - HTTP check results

### Alert Endpoints
- `GET /api/v1/alerts` - Active alerts
- `POST /api/v1/alerts/{id}/acknowledge` - Acknowledge alert
- `POST /api/v1/alerts/{id}/resolve` - Resolve alert

### Historical Data Endpoints

**Entity Management:**
- `GET /api/v1/entities` - List all monitored entities with filtering
- `GET /api/v1/entities/{id}` - Get specific entity details
- `GET /api/v1/entities/{id}/metrics` - Get historical metrics for an entity
- `GET /api/v1/entities/{id}/events` - Get event history for an entity

**Event History:**
- `GET /api/v1/events` - List all events with filtering
  - Query params: `entity_id`, `type`, `severity`, `since`, `until`, `limit`, `offset`

**Historical Metrics:**
- `GET /api/v1/metrics` - List historical metrics with filtering
  - Query params: `entity_id`, `metric_name`, `aggregation_level`, `since`, `until`, `limit`, `offset`
- `GET /api/v1/metrics/summary` - Get aggregated metric summaries
  - Query params: `entity_id`, `metric_name`, `since`, `until`

**Storage Management:**
- `GET /api/v1/storage/health` - Storage system health status
- `GET /api/v1/storage/stats` - Database statistics and record counts

### Query Parameters

**Time Filtering:**
- `since` - Start time (RFC3339 format, e.g., `2025-08-03T08:00:00Z`)
- `until` - End time (RFC3339 format)

**Pagination:**
- `limit` - Maximum number of records (default varies by endpoint)
- `offset` - Number of records to skip

**Entity Filtering:**
- `type` - Entity type (`server`, `service`, `site`, `disk`, `network_interface`)
- `status` - Entity status (`active`, `inactive`, `error`, `maintenance`)
- `name` - Entity name (partial match supported)

**Example Queries:**
```bash
# Get all service entities
curl "http://127.0.0.1:9090/api/v1/entities?type=service"

# Get CPU metrics from last hour
curl "http://127.0.0.1:9090/api/v1/metrics?metric_name=cpu_usage&since=2025-08-03T08:00:00Z"

# Get recent error events
curl "http://127.0.0.1:9090/api/v1/events?severity=error&limit=10"

# Get metrics for a specific entity
curl "http://127.0.0.1:9090/api/v1/entities/1/metrics?limit=50"
```

## TUI Dashboard

### Starting the Dashboard
```bash
# Start main Crucible TUI
sudo ./crucible

# Navigate to monitoring section
# Press 'm' or select "Monitoring" from menu
```

### Dashboard Features

**Currently Implemented:**
- **Real-time Metrics**: Live system performance data
- **Service Status**: Visual indicators for monitored services  
- **Active Alerts**: Current alert status with severity indicators
- **HTTP Checks**: Website/API endpoint status

**Future Enhancements (Not Yet Implemented):**
- **Historical Trends**: Access to stored historical data from SQLite
- **Storage Stats**: Database size and cleanup status
- **Historical Charts**: Trend visualization over time
- **Event History**: Browse past events and state changes

### Dashboard Controls

- **`r`**: Refresh data manually
- **`q`**: Return to main menu
- **`Esc`**: Exit monitoring mode
- **Arrow keys**: Navigate between sections

## Advanced Features

### Migration System

The storage system includes a robust migration framework:
- **Automatic Migrations**: Applied on startup
- **Version Tracking**: Schema versioning with checksums
- **Rollback Support**: Safe rollback of migrations
- **Validation**: Schema integrity verification

### Batch Processing

For high-throughput scenarios:
- **Batch Size**: Configurable batch size (default: 100)
- **Transaction Support**: Atomic batch operations
- **Performance Optimization**: Reduced database overhead

### Health Monitoring

System self-monitoring capabilities:
- **Database Health**: Connection and query performance
- **Storage Utilization**: Disk space and growth tracking
- **Collection Status**: Collector health and last update times
- **Alert Engine**: Alert evaluation performance

## Troubleshooting

### Common Issues

1. **Database Permission Errors**:
   - Ensure `/var/lib/crucible/` directory exists
   - Check write permissions for the crucible user
   - Verify SQLite is installed

2. **High Storage Usage**:
   - Check retention policy configuration
   - Review cleanup interval settings
   - Monitor database size via storage stats

3. **Missing Metrics**:
   - Verify collector configuration
   - Check service permissions (requires sudo)
   - Review system logs for collection errors

4. **Alert Issues**:
   - Validate alert rule configuration
   - Check threshold settings
   - Verify alert engine is enabled

### Log Files

- **Application Logs**: Configurable via `agent.log_file` in `monitor.yaml` (default: `/var/log/crucible-monitor.log`)
- **Error Details**: Check terminal output for real-time errors
- **Debug Mode**: Enable via `debug: true` in configuration

### Performance Tuning

For optimal performance:
- Adjust collection intervals based on monitoring needs
- Configure appropriate retention periods
- Monitor database size and cleanup frequency
- Use batch processing for high-frequency metrics

## Environment Variables

### Optional Configuration

```bash
# Override configuration file location
export CRUCIBLE_CONFIG="/path/to/monitor.yaml"

# Enable debug logging
export CRUCIBLE_DEBUG="true"

# Custom database location
export CRUCIBLE_DB_PATH="/custom/path/monitor.db"
```

### Legacy Email Configuration

**Location**: `/etc/crucible/.env` (or `~/.config/crucible/.env`)

```bash
# For email notifications (if enabled)
export RESEND_API_KEY="re_your_api_key_here"
```

**Security**: File permissions should be set to 600 (owner read/write only)

### Starting Crucial monitor

✅ Option 1: Use the start script
   ./start-monitor-with-env.sh

✅ Option 2: Load environment manually before starting
   set -a && source .env && set +a
   sudo -E ./crucible-monitor

✅ Option 3: Pass environment explicitly
   sudo env RESEND_API_KEY="$RESEND_API_KEY" ALERT_FROM_EMAIL="$ALERT_FROM_EMAIL" ./crucible-monitor

❌ What NOT to do:
   sudo ./crucible-monitor  # This loses environment variables


🏆 Recommended Production Setup

  For most production servers, use Systemd:

  # 1. Install as systemd service
  sudo ./install-systemd-service.sh

  # 2. Manage the service
  sudo systemctl status crucible-monitor
  sudo journalctl -u crucible-monitor -f

  # 3. Update configuration
  sudo nano /opt/crucible/.env
  sudo systemctl restart crucible-monitor

  🔐 Security Best Practices

  1. Run as non-root user: The systemd service creates a dedicated crucible user
  2. Environment file permissions: Secure the .env file:
  sudo chmod 600 /opt/crucible/.env
  sudo chown crucible:crucible /opt/crucible/.env
  3. Database permissions: Ensure proper SQLite database permissions in /var/lib/crucible/
  4. Network access: The monitor only needs outbound HTTPS access to Resend API

  The systemd approach is recommended because it:
  - Handles environment variables properly via EnvironmentFile
  - Provides automatic startup/restart
  - Integrates with system logging
  - Follows Linux security best practices
  - Doesn't require manual sudo commands
