package actions

import (
	"fmt"
	"path/filepath"
	"time"
)

// MySQLBackupConfig contains configuration for MySQL backup
type MySQLBackupConfig struct {
	DBName          string
	DBUser          string
	DBPassword      string
	DestinationType string // "local", "scp", "s3"
	LocalPath       string
	RemoteHost      string
	RemotePath      string
	SSHKeyPath      string
	RemoteUser      string
	S3Bucket        string
	S3Region        string
	S3AccessKey     string
	S3SecretKey     string
	RetentionDays   int
	CompressBackup  bool
}

// BackupMySQL returns the commands and descriptions for backing up MySQL database
func BackupMySQL(config MySQLBackupConfig) ([]string, []string) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFileName := fmt.Sprintf("%s_backup_%s.sql", config.DBName, timestamp)

	var commands []string
	var descriptions []string

	// Set default values
	if config.DestinationType == "" {
		config.DestinationType = "local"
	}
	if config.LocalPath == "" {
		config.LocalPath = "/var/backups/mysql"
	}
	if config.CompressBackup == false {
		config.CompressBackup = true
	}

	// Determine backup paths
	var workingPath, finalPath string

	switch config.DestinationType {
	case "local":
		// Create backup directory if it doesn't exist
		commands = append(commands, fmt.Sprintf("sudo mkdir -p %s", config.LocalPath))
		descriptions = append(descriptions, "Creating backup directory...")

		workingPath = filepath.Join(config.LocalPath, backupFileName)
		if config.CompressBackup {
			finalPath = workingPath + ".gz"
		} else {
			finalPath = workingPath
		}

	case "scp", "s3":
		// Use temporary directory for remote transfers
		workingPath = filepath.Join("/tmp", backupFileName)
		if config.CompressBackup {
			finalPath = workingPath + ".gz"
		} else {
			finalPath = workingPath
		}
	}

	// 1. Create MySQL dump with secure credential handling
	var dumpCmd string
	if config.DBPassword != "" {
		// Use password (less secure but more common)
		dumpCmd = fmt.Sprintf("mysqldump -u %s -p'%s' --single-transaction --routines --triggers --opt %s > %s",
			config.DBUser, config.DBPassword, config.DBName, workingPath)
	} else {
		// Use socket authentication or .my.cnf (more secure)
		dumpCmd = fmt.Sprintf("mysqldump -u %s --single-transaction --routines --triggers --opt %s > %s",
			config.DBUser, config.DBName, workingPath)
	}
	commands = append(commands, dumpCmd)
	descriptions = append(descriptions, fmt.Sprintf("Creating MySQL dump of database: %s", config.DBName))

	// 2. Compress backup if requested
	if config.CompressBackup {
		commands = append(commands, fmt.Sprintf("gzip %s", workingPath))
		descriptions = append(descriptions, "Compressing backup file...")
	}

	// 3. Handle destination-specific actions
	switch config.DestinationType {
	case "local":
		// Set proper permissions for local backups
		commands = append(commands, fmt.Sprintf("sudo chown mysql:mysql %s", finalPath))
		commands = append(commands, fmt.Sprintf("sudo chmod 600 %s", finalPath))
		descriptions = append(descriptions, "Setting secure backup file permissions...")

		// Optional: Clean up old backups based on retention policy
		if config.RetentionDays > 0 {
			cleanupCmd := fmt.Sprintf("find %s -name '%s_backup_*.sql*' -type f -mtime +%d -delete",
				config.LocalPath, config.DBName, config.RetentionDays)
			commands = append(commands, cleanupCmd)
			descriptions = append(descriptions, fmt.Sprintf("Cleaning up backups older than %d days...", config.RetentionDays))
		}

	case "scp":
		// Transfer via SCP
		var scpCmd string
		if config.SSHKeyPath != "" {
			// Use SSH key authentication (more secure)
			scpCmd = fmt.Sprintf("scp -i %s %s %s@%s:%s",
				config.SSHKeyPath, finalPath, config.RemoteUser, config.RemoteHost, config.RemotePath)
		} else {
			// Use password authentication (less secure)
			scpCmd = fmt.Sprintf("scp %s %s@%s:%s",
				finalPath, config.RemoteUser, config.RemoteHost, config.RemotePath)
		}
		commands = append(commands, scpCmd)
		descriptions = append(descriptions, fmt.Sprintf("Transferring backup to %s@%s:%s", config.RemoteUser, config.RemoteHost, config.RemotePath))

		// Clean up temporary backup
		commands = append(commands, fmt.Sprintf("rm -f %s", finalPath))
		descriptions = append(descriptions, "Cleaning up temporary backup file...")

	case "s3":
		// AWS S3 upload (requires aws-cli)
		s3Path := fmt.Sprintf("s3://%s/%s", config.S3Bucket, filepath.Base(finalPath))
		var awsCmd string
		if config.S3AccessKey != "" && config.S3SecretKey != "" {
			// Use provided credentials
			awsCmd = fmt.Sprintf("AWS_ACCESS_KEY_ID=%s AWS_SECRET_ACCESS_KEY=%s aws s3 cp %s %s --region %s",
				config.S3AccessKey, config.S3SecretKey, finalPath, s3Path, config.S3Region)
		} else {
			// Use default AWS credentials
			awsCmd = fmt.Sprintf("aws s3 cp %s %s --region %s", finalPath, s3Path, config.S3Region)
		}
		commands = append(commands, awsCmd)
		descriptions = append(descriptions, fmt.Sprintf("Uploading backup to S3: %s", s3Path))

		// Clean up temporary backup
		commands = append(commands, fmt.Sprintf("rm -f %s", finalPath))
		descriptions = append(descriptions, "Cleaning up temporary backup file...")
	}

	return commands, descriptions
}

// MySQLRestoreConfig contains configuration for MySQL restore
type MySQLRestoreConfig struct {
	DBName     string
	DBUser     string
	DBPassword string
	BackupFile string
}

// RestoreMySQL returns the commands and descriptions for restoring MySQL database
func RestoreMySQL(config MySQLRestoreConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	// 1. Create database if it doesn't exist
	createDBCmd := fmt.Sprintf("mysql -u %s -p%s -e 'CREATE DATABASE IF NOT EXISTS %s;'",
		config.DBUser, config.DBPassword, config.DBName)
	commands = append(commands, createDBCmd)
	descriptions = append(descriptions, fmt.Sprintf("Creating database %s if it doesn't exist...", config.DBName))

	// 2. Check if backup file is compressed and decompress if needed
	if filepath.Ext(config.BackupFile) == ".gz" {
		decompressCmd := fmt.Sprintf("gunzip -c %s | mysql -u %s -p%s %s",
			config.BackupFile, config.DBUser, config.DBPassword, config.DBName)
		commands = append(commands, decompressCmd)
		descriptions = append(descriptions, "Decompressing and restoring database...")
	} else {
		restoreCmd := fmt.Sprintf("mysql -u %s -p%s %s < %s",
			config.DBUser, config.DBPassword, config.DBName, config.BackupFile)
		commands = append(commands, restoreCmd)
		descriptions = append(descriptions, "Restoring database...")
	}

	return commands, descriptions
}

// MySQLUserConfig contains configuration for MySQL user management
type MySQLUserConfig struct {
	AdminUser     string
	AdminPassword string
	Username      string
	Password      string
	Host          string
	Database      string
	Privileges    []string
}

// CreateMySQLUser returns the commands and descriptions for creating MySQL user
func CreateMySQLUser(config MySQLUserConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	// 1. Create user
	createUserCmd := fmt.Sprintf("mysql -u %s -p%s -e \"CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s';\"",
		config.AdminUser, config.AdminPassword, config.Username, config.Host, config.Password)
	commands = append(commands, createUserCmd)
	descriptions = append(descriptions, fmt.Sprintf("Creating MySQL user %s@%s...", config.Username, config.Host))

	// 2. Grant privileges
	privileges := "ALL PRIVILEGES"
	if len(config.Privileges) > 0 {
		privileges = fmt.Sprintf("%s", config.Privileges[0])
		for _, priv := range config.Privileges[1:] {
			privileges += ", " + priv
		}
	}

	grantCmd := fmt.Sprintf("mysql -u %s -p%s -e \"GRANT %s ON %s.* TO '%s'@'%s';\"",
		config.AdminUser, config.AdminPassword, privileges, config.Database, config.Username, config.Host)
	commands = append(commands, grantCmd)
	descriptions = append(descriptions, fmt.Sprintf("Granting %s privileges on %s to %s@%s...", privileges, config.Database, config.Username, config.Host))

	// 3. Flush privileges
	flushCmd := fmt.Sprintf("mysql -u %s -p%s -e \"FLUSH PRIVILEGES;\"",
		config.AdminUser, config.AdminPassword)
	commands = append(commands, flushCmd)
	descriptions = append(descriptions, "Flushing MySQL privileges...")

	return commands, descriptions
}

// MySQLDatabaseConfig contains configuration for database operations
type MySQLDatabaseConfig struct {
	AdminUser     string
	AdminPassword string
	DatabaseName  string
	CharacterSet  string
	Collation     string
}

// CreateMySQLDatabase returns the commands and descriptions for creating MySQL database
func CreateMySQLDatabase(config MySQLDatabaseConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	charset := config.CharacterSet
	if charset == "" {
		charset = "utf8mb4"
	}

	collation := config.Collation
	if collation == "" {
		collation = "utf8mb4_unicode_ci"
	}

	// Create database with specified character set and collation
	createDBCmd := fmt.Sprintf("mysql -u %s -p%s -e \"CREATE DATABASE IF NOT EXISTS %s CHARACTER SET %s COLLATE %s;\"",
		config.AdminUser, config.AdminPassword, config.DatabaseName, charset, collation)
	commands = append(commands, createDBCmd)
	descriptions = append(descriptions, fmt.Sprintf("Creating database %s with charset %s...", config.DatabaseName, charset))

	return commands, descriptions
}

// GetMySQLStatus returns commands to check MySQL status
func GetMySQLStatus() ([]string, []string) {
	var commands []string
	var descriptions []string

	// Check MySQL service status
	commands = append(commands, "systemctl is-active mysql || systemctl is-active mysqld")
	descriptions = append(descriptions, "Checking MySQL service status...")

	// Check MySQL version
	commands = append(commands, "mysql --version")
	descriptions = append(descriptions, "Getting MySQL version...")

	// Check MySQL processes
	commands = append(commands, "pgrep -l mysql")
	descriptions = append(descriptions, "Checking MySQL processes...")

	return commands, descriptions
}

// OptimizeMySQLDatabase returns commands for database optimization
func OptimizeMySQLDatabase(config MySQLDatabaseConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	// Analyze tables
	analyzeCmd := fmt.Sprintf("mysql -u %s -p%s -e \"USE %s; ANALYZE TABLE %s;\"",
		config.AdminUser, config.AdminPassword, config.DatabaseName, "*")
	commands = append(commands, analyzeCmd)
	descriptions = append(descriptions, fmt.Sprintf("Analyzing tables in database %s...", config.DatabaseName))

	// Optimize tables
	optimizeCmd := fmt.Sprintf("mysql -u %s -p%s -e \"USE %s; OPTIMIZE TABLE %s;\"",
		config.AdminUser, config.AdminPassword, config.DatabaseName, "*")
	commands = append(commands, optimizeCmd)
	descriptions = append(descriptions, fmt.Sprintf("Optimizing tables in database %s...", config.DatabaseName))

	// Check tables
	checkCmd := fmt.Sprintf("mysql -u %s -p%s -e \"USE %s; CHECK TABLE %s;\"",
		config.AdminUser, config.AdminPassword, config.DatabaseName, "*")
	commands = append(commands, checkCmd)
	descriptions = append(descriptions, fmt.Sprintf("Checking tables in database %s...", config.DatabaseName))

	return commands, descriptions
}
