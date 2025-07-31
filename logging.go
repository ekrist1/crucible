package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	logDir  = "/var/log/crucible"
	logFile = "installation.log"
)

// LoggedExecResult contains the result of a logged command execution
type LoggedExecResult struct {
	Command   string
	Output    string
	Error     error
	ExitCode  int
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

// initializeLogging creates the log directory and file if they don't exist
func (m *model) initializeLogging() error {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	logPath := filepath.Join(logDir, logFile)

	// Create log file if it doesn't exist
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		file, err := os.Create(logPath)
		if err != nil {
			return fmt.Errorf("failed to create log file: %v", err)
		}
		defer file.Close()

		// Write initial log header
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		header := fmt.Sprintf("=== Crucible Installation Log - Started %s ===\n", timestamp)
		if _, err := file.WriteString(header); err != nil {
			return fmt.Errorf("failed to write log header: %v", err)
		}
	}

	return nil
}

// logCommand writes a command execution to the log file with timestamp and details
func (m *model) logCommand(result LoggedExecResult) error {
	logPath := filepath.Join(logDir, logFile)

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	// Format log entry
	var logEntry strings.Builder
	logEntry.WriteString(fmt.Sprintf("\n[%s] COMMAND: %s\n",
		result.StartTime.Format("2006-01-02 15:04:05"), result.Command))
	logEntry.WriteString(fmt.Sprintf("[%s] DURATION: %v\n",
		result.StartTime.Format("2006-01-02 15:04:05"), result.Duration))

	if result.Error != nil {
		logEntry.WriteString(fmt.Sprintf("[%s] ERROR: %v\n",
			result.StartTime.Format("2006-01-02 15:04:05"), result.Error))
		logEntry.WriteString(fmt.Sprintf("[%s] EXIT CODE: %d\n",
			result.StartTime.Format("2006-01-02 15:04:05"), result.ExitCode))
	} else {
		logEntry.WriteString(fmt.Sprintf("[%s] STATUS: SUCCESS\n",
			result.StartTime.Format("2006-01-02 15:04:05")))
	}

	// Log output (truncate if too long)
	output := strings.TrimSpace(result.Output)
	if len(output) > 0 {
		if len(output) > 2000 {
			output = output[:2000] + "... [TRUNCATED]"
		}
		logEntry.WriteString(fmt.Sprintf("[%s] OUTPUT:\n%s\n",
			result.StartTime.Format("2006-01-02 15:04:05"), output))
	}

	logEntry.WriteString(fmt.Sprintf("[%s] END\n",
		result.EndTime.Format("2006-01-02 15:04:05")))

	// Write to file
	if _, err := file.WriteString(logEntry.String()); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	return nil
}

// executeAndLogCommand executes a bash command and logs the result
func (m *model) executeAndLogCommand(command string) LoggedExecResult {
	startTime := time.Now()

	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	result := LoggedExecResult{
		Command:   command,
		Output:    string(output),
		Error:     err,
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
	}

	// Log the command execution
	if logErr := m.logCommand(result); logErr != nil {
		m.logger.Error("Failed to log command", "error", logErr)
	}

	return result
}

// readLogFile reads and returns the contents of the installation log file
func (m model) readLogFile() ([]string, error) {
	logPath := filepath.Join(logDir, logFile)

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return []string{"No installation log file found. Run an installation first."}, nil
	}

	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %v", err)
	}

	// If file is empty or has no content
	if len(lines) == 0 {
		return []string{"Log file is empty."}, nil
	}

	return lines, nil
}

// showInstallationLogs displays the installation logs in the TUI
func (m model) showInstallationLogs() (tea.Model, tea.Cmd) {
	// Clear screen before showing logs
	clearScreen()
	m.state = stateProcessing
	m.processingMsg = "Loading installation logs..."
	m.report = []string{}

	logLines, err := m.readLogFile()
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Error reading log file: %v", err)))
		m.processingMsg = ""
		return m, nil
	}

	m.report = append(m.report, infoStyle.Render("=== INSTALLATION LOGS ==="))
	m.report = append(m.report, "")

	// Show last 50 lines or all lines if fewer
	startIdx := 0
	if len(logLines) > 50 {
		startIdx = len(logLines) - 50
		m.report = append(m.report, warnStyle.Render("(Showing last 50 lines)"))
		m.report = append(m.report, "")
	}

	for i := startIdx; i < len(logLines); i++ {
		line := logLines[i]
		// Style different types of log lines
		if strings.Contains(line, "COMMAND:") {
			m.report = append(m.report, infoStyle.Render(line))
		} else if strings.Contains(line, "ERROR:") || strings.Contains(line, "EXIT CODE:") {
			m.report = append(m.report, warnStyle.Render(line))
		} else if strings.Contains(line, "STATUS: SUCCESS") {
			m.report = append(m.report, infoStyle.Render(line))
		} else {
			m.report = append(m.report, line)
		}
	}

	m.report = append(m.report, "")
	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Log file location: %s", filepath.Join(logDir, logFile))))
	m.report = append(m.report, infoStyle.Render("=== END OF LOGS ==="))

	m.processingMsg = ""
	return m, nil
}

// clearLogFile clears the installation log file
func (m *model) clearLogFile() error {
	logPath := filepath.Join(logDir, logFile)

	// Remove the existing log file
	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove log file: %v", err)
	}

	// Reinitialize logging
	return m.initializeLogging()
}

// getLogFileSize returns the size of the log file in bytes
func (m model) getLogFileSize() (int64, error) {
	logPath := filepath.Join(logDir, logFile)

	info, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	return info.Size(), nil
}

// tailLogFile returns the last n lines from the log file
func (m model) tailLogFile(n int) ([]string, error) {
	logLines, err := m.readLogFile()
	if err != nil {
		return nil, err
	}

	if len(logLines) <= n {
		return logLines, nil
	}

	return logLines[len(logLines)-n:], nil
}

// streamCommandOutput executes a command and streams both to log and report
func (m *model) streamCommandOutput(command string, description string) error {
	startTime := time.Now()

	cmd := exec.Command("bash", "-c", command)

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	// Read output in real-time
	var outputBuilder strings.Builder

	// Function to read from a pipe and add to both report and log buffer
	readPipe := func(pipe io.ReadCloser, prefix string) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			outputBuilder.WriteString(line + "\n")
			// Add to report for real-time display
			if strings.TrimSpace(line) != "" {
				m.report = append(m.report, fmt.Sprintf("%s%s", prefix, line))
			}
		}
	}

	// Read stdout and stderr concurrently
	go readPipe(stdout, "")
	go readPipe(stderr, "ERROR: ")

	// Wait for command to complete
	err = cmd.Wait()

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		}
	}

	// Create log result
	result := LoggedExecResult{
		Command:   command,
		Output:    outputBuilder.String(),
		Error:     err,
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  duration,
	}

	// Log the command execution
	if logErr := m.logCommand(result); logErr != nil {
		m.logger.Error("Failed to log command", "error", logErr)
	}

	return err
}
