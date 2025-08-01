package logging

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
)

// LoggedExecResult represents the result of a command execution with logging metadata
type LoggedExecResult struct {
	Command   string
	Output    string
	Error     error
	ExitCode  int
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// Logger wraps the charm log.Logger with additional functionality
type Logger struct {
	*log.Logger
	logFilePath string
}

// NewLogger creates a new logger instance with file logging
func NewLogger(logFile string) (*Logger, error) {
	// Ensure log directory exists
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create the base logger
	baseLogger := log.NewWithOptions(os.Stdout, log.Options{
		ReportCaller:    false,
		ReportTimestamp: true,
		Prefix:          "Crucible 🔧",
	})

	return &Logger{
		Logger:      baseLogger,
		logFilePath: logFile,
	}, nil
}

// LogCommand logs the execution of a command with detailed information
func (l *Logger) LogCommand(result LoggedExecResult) error {
	// Log to stdout
	if result.Error != nil {
		l.Error("Command failed", 
			"command", result.Command, 
			"error", result.Error, 
			"exit_code", result.ExitCode,
			"duration", result.Duration)
	} else {
		l.Info("Command executed successfully", 
			"command", result.Command, 
			"duration", result.Duration)
	}

	// Log to file with more detail
	return l.logToFile(result)
}

// logToFile writes detailed command execution information to the log file
func (l *Logger) logToFile(result LoggedExecResult) error {
	file, err := os.OpenFile(l.logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Write detailed log entry
	logEntry := fmt.Sprintf(`
=== COMMAND EXECUTION LOG ===
TIMESTAMP: %s
COMMAND: %s
START_TIME: %s
END_TIME: %s
DURATION: %s
EXIT_CODE: %d
STATUS: %s
OUTPUT:
%s
ERROR: %v
=== END LOG ENTRY ===

`, 
		time.Now().Format(time.RFC3339),
		result.Command,
		result.StartTime.Format(time.RFC3339),
		result.EndTime.Format(time.RFC3339),
		result.Duration.String(),
		result.ExitCode,
		getStatusString(result.Error == nil),
		result.Output,
		result.Error,
	)

	_, err = file.WriteString(logEntry)
	return err
}

// ReadLogLines reads all lines from the log file
func (l *Logger) ReadLogLines() ([]string, error) {
	file, err := os.Open(l.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return lines, nil
}

// ClearLogs clears the log file
func (l *Logger) ClearLogs() error {
	return os.Truncate(l.logFilePath, 0)
}

// GetLogFilePath returns the path to the log file
func (l *Logger) GetLogFilePath() string {
	return l.logFilePath
}

// LogFileExists checks if the log file exists
func (l *Logger) LogFileExists() bool {
	_, err := os.Stat(l.logFilePath)
	return err == nil
}

// GetLogFileSize returns the size of the log file in bytes
func (l *Logger) GetLogFileSize() (int64, error) {
	info, err := os.Stat(l.logFilePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// Helper functions

func getStatusString(success bool) string {
	if success {
		return "SUCCESS"
	}
	return "FAILED"
}

// DefaultLogPath returns the default log file path
func DefaultLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/crucible.log"
	}
	return filepath.Join(homeDir, ".crucible", "logs", "crucible.log")
}