package models

import (
	"database/sql"
	"fmt"

	"crucible/internal/actions"
	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// MySQLBackupModel handles MySQL backup configuration
type MySQLBackupModel struct {
	BaseModel
	form *HybridFormModel
}

// NewMySQLBackupModel creates a new MySQL backup form model
func NewMySQLBackupModel(shared *SharedData) *MySQLBackupModel {
	model := &MySQLBackupModel{
		BaseModel: NewBaseModel(shared),
	}
	model.setupForm()
	return model
}

// setupForm configures the MySQL backup form
func (m *MySQLBackupModel) setupForm() {
	m.form = NewHybridFormModel(
		m.shared,
		"💾 MySQL Database Backup",
		"Configure backup settings for your MySQL database",
	)

	// Database selection field
	databases := m.getAvailableDatabases()
	if len(databases) == 0 {
		databases = []SelectionOption{
			{Value: "custom", Description: "Enter database name manually"},
		}
	}

	m.form.AddField(HybridFormField{
		Label:     "Database",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options:   databases,
	})

	// Custom database name field (shown when "custom" is selected)
	m.form.AddField(HybridFormField{
		Label:       "Database Name",
		FieldType:   HybridFieldTypeText,
		Placeholder: "Enter database name",
		Required:    false, // Only required if custom is selected
		MaxLength:   64,
	})

	// MySQL username field
	m.form.AddField(HybridFormField{
		Label:       "MySQL Username",
		FieldType:   HybridFieldTypeText,
		Placeholder: "root",
		Required:    true,
		MaxLength:   32,
	})

	// MySQL password field
	m.form.AddField(HybridFormField{
		Label:       "MySQL Password",
		FieldType:   HybridFieldTypePassword,
		Placeholder: "Leave empty for socket auth",
		Required:    false,
		MaxLength:   64,
	})

	// Backup destination type
	m.form.AddField(HybridFormField{
		Label:     "Backup Destination",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{Value: "local", Description: "Local server storage (recommended)"},
			{Value: "scp", Description: "Remote server via SSH/SCP"},
			{Value: "s3", Description: "Amazon S3 (requires aws-cli)"},
		},
		SelectedIndex: 0, // Default to local
	})

	// Local backup path (shown for local backups)
	m.form.AddField(HybridFormField{
		Label:       "Local Backup Path",
		FieldType:   HybridFieldTypeText,
		Placeholder: "/var/backups/mysql",
		Required:    false,
		MaxLength:   255,
	})

	// Remote host (shown for SCP)
	m.form.AddField(HybridFormField{
		Label:       "Remote Host",
		FieldType:   HybridFieldTypeText,
		Placeholder: "backup.example.com",
		Required:    false,
		MaxLength:   255,
	})

	// Remote user (shown for SCP)
	m.form.AddField(HybridFormField{
		Label:       "Remote Username",
		FieldType:   HybridFieldTypeText,
		Placeholder: "backup-user",
		Required:    false,
		MaxLength:   32,
	})

	// Remote path (shown for SCP)
	m.form.AddField(HybridFormField{
		Label:       "Remote Path",
		FieldType:   HybridFieldTypeText,
		Placeholder: "/home/backup/mysql/",
		Required:    false,
		MaxLength:   255,
	})

	// SSH key path (shown for SCP)
	m.form.AddField(HybridFormField{
		Label:       "SSH Key Path",
		FieldType:   HybridFieldTypeText,
		Placeholder: "/root/.ssh/backup_key (optional)",
		Required:    false,
		MaxLength:   255,
	})

	// S3 bucket (shown for S3)
	m.form.AddField(HybridFormField{
		Label:       "S3 Bucket",
		FieldType:   HybridFieldTypeText,
		Placeholder: "my-backup-bucket",
		Required:    false,
		MaxLength:   63,
	})

	// S3 region (shown for S3)
	m.form.AddField(HybridFormField{
		Label:       "S3 Region",
		FieldType:   HybridFieldTypeText,
		Placeholder: "us-east-1",
		Required:    false,
		MaxLength:   32,
	})

	// Compression option
	m.form.AddField(HybridFormField{
		Label:     "Compress Backup",
		FieldType: HybridFieldTypeSelection,
		Required:  true,
		Options: []SelectionOption{
			{Value: "yes", Description: "Yes - Compress with gzip (recommended)"},
			{Value: "no", Description: "No - Keep uncompressed"},
		},
		SelectedIndex: 0, // Default to compressed
	})

	// Retention policy (for local backups)
	m.form.AddField(HybridFormField{
		Label:       "Retention Days",
		FieldType:   HybridFieldTypeText,
		Placeholder: "7 (keep backups for 7 days, 0 = keep all)",
		Required:    false,
		MaxLength:   3,
	})

	// Set submit label
	m.form.SetSubmitLabel("Create Backup")

	// Set handlers
	m.form.SetSubmitHandler(m.handleSubmit)
	m.form.SetCancelHandler(m.handleCancel)
}

// Init initializes the form
func (m *MySQLBackupModel) Init() tea.Cmd {
	return m.form.Init()
}

// Update handles form updates
func (m *MySQLBackupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := m.form.Update(msg)
	if hybridModel, ok := newModel.(*HybridFormModel); ok {
		m.form = hybridModel
	}
	return m, cmd
}

// View renders the form
func (m *MySQLBackupModel) View() string {
	return m.form.View()
}

// getAvailableDatabases gets a list of available databases from MySQL
func (m *MySQLBackupModel) getAvailableDatabases() []SelectionOption {
	var databases []SelectionOption

	// Try to connect to MySQL to get database list
	connectionStrings := []string{
		"root:@tcp(localhost:3306)/",
		"root:root@tcp(localhost:3306)/",
	}

	for _, connStr := range connectionStrings {
		if db, err := sql.Open("mysql", connStr); err == nil {
			if err := db.Ping(); err == nil {
				if rows, err := db.Query("SHOW DATABASES"); err == nil {
					for rows.Next() {
						var dbName string
						if err := rows.Scan(&dbName); err == nil {
							// Skip system databases
							if dbName != "information_schema" && dbName != "performance_schema" &&
								dbName != "mysql" && dbName != "sys" {
								databases = append(databases, SelectionOption{
									Value:       dbName,
									Description: fmt.Sprintf("Database: %s", dbName),
								})
							}
						}
					}
					rows.Close()
				}
				db.Close()
				break // Successfully connected, use this connection
			}
			db.Close()
		}
	}

	// Add custom option
	databases = append(databases, SelectionOption{
		Value:       "custom",
		Description: "Enter custom database name",
	})

	return databases
}

// handleSubmit handles form submission
func (m *MySQLBackupModel) handleSubmit(values []string) tea.Cmd {
	return func() tea.Msg {
		// Extract form values
		selectedDB := values[0]
		customDBName := values[1]
		username := values[2]
		password := values[3]
		destination := values[4]
		localPath := values[5]
		remoteHost := values[6]
		remoteUser := values[7]
		remotePath := values[8]
		sshKeyPath := values[9]
		s3Bucket := values[10]
		s3Region := values[11]
		compress := values[12]
		retentionStr := values[13]

		// Determine database name
		dbName := selectedDB
		if selectedDB == "custom" {
			dbName = customDBName
		}

		// Validate required fields based on destination
		if dbName == "" || dbName == "custom" {
			return backupErrorMsg{err: fmt.Errorf("database name is required")}
		}
		if username == "" {
			return backupErrorMsg{err: fmt.Errorf("MySQL username is required")}
		}

		// Validate destination-specific fields
		switch destination {
		case "scp":
			if remoteHost == "" || remoteUser == "" || remotePath == "" {
				return backupErrorMsg{err: fmt.Errorf("remote host, user, and path are required for SCP backup")}
			}
		case "s3":
			if s3Bucket == "" || s3Region == "" {
				return backupErrorMsg{err: fmt.Errorf("S3 bucket and region are required for S3 backup")}
			}
		}

		// Parse retention days
		retentionDays := 0
		if retentionStr != "" {
			if _, err := fmt.Sscanf(retentionStr, "%d", &retentionDays); err != nil {
				retentionDays = 0
			}
		}

		// Create backup configuration
		config := actions.MySQLBackupConfig{
			DBName:          dbName,
			DBUser:          username,
			DBPassword:      password,
			DestinationType: destination,
			LocalPath:       localPath,
			RemoteHost:      remoteHost,
			RemoteUser:      remoteUser,
			RemotePath:      remotePath,
			SSHKeyPath:      sshKeyPath,
			S3Bucket:        s3Bucket,
			S3Region:        s3Region,
			RetentionDays:   retentionDays,
			CompressBackup:  compress == "yes",
		}

		// Generate backup commands
		commands, descriptions := actions.BackupMySQL(config)

		return backupCreatedMsg{
			config:       config,
			commands:     commands,
			descriptions: descriptions,
		}
	}
}

// handleCancel handles form cancellation
func (m *MySQLBackupModel) handleCancel() tea.Cmd {
	return m.GoBack()
}

// Message types for backup operations
type backupCreatedMsg struct {
	config       actions.MySQLBackupConfig
	commands     []string
	descriptions []string
}

type backupErrorMsg struct {
	err error
}
