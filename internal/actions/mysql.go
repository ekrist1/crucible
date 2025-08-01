package actions

import (
	"fmt"
	"path/filepath"
	"time"
)

// MySQLBackupConfig contains configuration for MySQL backup
type MySQLBackupConfig struct {
	DBName     string
	DBUser     string
	DBPassword string
	RemoteHost string
	RemotePath string
}

// BackupMySQL returns the commands and descriptions for backing up MySQL database
func BackupMySQL(config MySQLBackupConfig) ([]string, []string) {
	// Create timestamp for backup filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFileName := fmt.Sprintf("%s_backup_%s.sql", config.DBName, timestamp)
	localBackupPath := filepath.Join("/tmp", backupFileName)
	compressedPath := localBackupPath + ".gz"

	var commands []string
	var descriptions []string

	// 1. Create MySQL dump
	dumpCmd := fmt.Sprintf("mysqldump -u %s -p%s --single-transaction --routines --triggers %s > %s",
		config.DBUser, config.DBPassword, config.DBName, localBackupPath)
	commands = append(commands, dumpCmd)
	descriptions = append(descriptions, fmt.Sprintf("Creating MySQL dump: %s", config.DBName))

	// 2. Compress the backup
	commands = append(commands, fmt.Sprintf("gzip %s", localBackupPath))
	descriptions = append(descriptions, "Compressing backup file...")

	// 3. Transfer to remote host via SCP
	scpCmd := fmt.Sprintf("scp %s %s:%s", compressedPath, config.RemoteHost, config.RemotePath)
	commands = append(commands, scpCmd)
	descriptions = append(descriptions, fmt.Sprintf("Transferring backup to %s:%s", config.RemoteHost, config.RemotePath))

	// 4. Clean up local backup
	commands = append(commands, fmt.Sprintf("rm -f %s", compressedPath))
	descriptions = append(descriptions, "Cleaning up local backup file...")

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