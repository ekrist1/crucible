package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	"crucible/internal/actions"
	"crucible/internal/logging"
)

type AppState int

const (
	StateMenu AppState = iota
	StateSubmenu
	StateInput
	StateProcessing
	StateLogViewer
	StateServiceList
	StateServiceActions
)

type MenuLevel int

const (
	MenuMain MenuLevel = iota
	MenuCoreServices
	MenuLaravelManagement
	MenuServerManagement
	MenuSettings
)

// MonitoringView represents different views in the monitoring dashboard
type MonitoringView int

const (
	MonitoringViewLive MonitoringView = iota
	MonitoringViewHistorical
	MonitoringViewEvents
	MonitoringViewStorage
)

// HistoricalTimeRange represents time range options for historical data
type HistoricalTimeRange int

const (
	TimeRangeLast1Hour HistoricalTimeRange = iota
	TimeRangeLast6Hours
	TimeRangeLast24Hours
	TimeRangeLast7Days
	TimeRangeLast30Days
)

// getTimeRangeString returns the display string for a time range
func (h HistoricalTimeRange) String() string {
	switch h {
	case TimeRangeLast1Hour:
		return "Last 1 Hour"
	case TimeRangeLast6Hours:
		return "Last 6 Hours"
	case TimeRangeLast24Hours:
		return "Last 24 Hours"
	case TimeRangeLast7Days:
		return "Last 7 Days"
	case TimeRangeLast30Days:
		return "Last 30 Days"
	default:
		return "Last 1 Hour"
	}
}

// getTimeRangeDuration returns the duration for a time range
func (h HistoricalTimeRange) Duration() time.Duration {
	switch h {
	case TimeRangeLast1Hour:
		return time.Hour
	case TimeRangeLast6Hours:
		return 6 * time.Hour
	case TimeRangeLast24Hours:
		return 24 * time.Hour
	case TimeRangeLast7Days:
		return 7 * 24 * time.Hour
	case TimeRangeLast30Days:
		return 30 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// isInMonitoringDashboard checks if we're currently viewing the monitoring dashboard
func (m Model) isInMonitoringDashboard() bool {
	return m.State == StateProcessing &&
		len(m.Report) > 0 &&
		strings.Contains(m.Report[0], "MONITORING")
}

// Message types for async command execution
type CmdExecutionMsg struct {
	Command     string
	Description string
	ServiceName string
}

type CmdCompletedMsg struct {
	Result      logging.LoggedExecResult
	ServiceName string
}

type CmdQueueMsg struct {
	Commands     []string
	Descriptions []string
	ServiceName  string
	CurrentIndex int
}

// ServiceItem represents a service in the list
type ServiceItem struct {
	ServiceInfo actions.ServiceInfo
}

// Implement list.Item interface
func (i ServiceItem) FilterValue() string { return i.ServiceInfo.Name }
func (i ServiceItem) Title() string       { return i.ServiceInfo.Name }
func (i ServiceItem) Description() string {
	status := "●"
	if i.ServiceInfo.Active == "active" {
		status = "🟢"
	} else {
		status = "🔴"
	}
	return fmt.Sprintf("%s %s - %s", status, i.ServiceInfo.Status, i.ServiceInfo.Sub)
}

type Model struct {
	Choices       []string
	Cursor        int
	Selected      map[int]struct{}
	Logger        *logging.Logger
	State         AppState
	CurrentMenu   MenuLevel // Track which menu we're in
	InputPrompt   string
	InputValue    string
	InputField    string
	InputCursor   int // Cursor position within the input field
	FormData      map[string]string
	CurrentAction int
	ServiceStatus map[string]bool // Track installation status of services
	Spinner       spinner.Model   // Spinner for long-running tasks
	ProcessingMsg string          // Message to display during processing
	Report        []string        // System status report lines
	// Log viewer state
	LogLines  []string // All log lines
	LogScroll int      // Current scroll position
	// Command queue state
	CommandQueue     []string // Commands to execute in sequence
	DescriptionQueue []string // Descriptions for each command
	QueueIndex       int      // Current command index
	QueueServiceName string   // Service name for queue
	// Service list state
	ServiceList         list.Model            // Service management list
	Services            []actions.ServiceInfo // Parsed services
	ReturnToServiceList bool                  // Flag to return to service list instead of main menu
	// Monitoring dashboard state
	MonitoringView      MonitoringView      // Current monitoring view
	MonitoringTimeRange HistoricalTimeRange // Selected time range for historical data
	MonitoringScroll    int                 // Current scroll position for monitoring dashboard
	// Service management state
	CurrentService actions.ServiceInfo // Currently selected service for actions
}

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")). // White
			Background(lipgloss.Color("5")).  // Magenta
			Padding(0, 1)

	SelectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // Bright Green
			Bold(true)

	ChoiceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // Gray

	InputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14")). // Cyan
			Background(lipgloss.Color("0")).  // Black
			Padding(0, 1)

	PromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // Yellow
			Bold(true)

	InfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green for info/success
	WarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow for warnings
)

func NewModel() Model {
	logger, err := logging.NewLogger(logging.DefaultLogPath())
	if err != nil {
		// Fallback to basic stdout logger if file logger fails
		baseLogger := log.NewWithOptions(os.Stdout, log.Options{
			ReportCaller:    false,
			ReportTimestamp: true,
			Prefix:          "Crucible 🔧",
		})
		logger = &logging.Logger{Logger: baseLogger}
	}

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Bright Green

	m := Model{
		Choices: []string{
			"Core Services",
			"Laravel Management",
			"Server Management",
			"Monitoring Dashboard",
			"Settings",
			"Exit",
		},
		Selected:      make(map[int]struct{}),
		Logger:        logger,
		State:         StateMenu,
		CurrentMenu:   MenuMain,
		FormData:      make(map[string]string),
		ServiceStatus: make(map[string]bool),
		Spinner:       s,
		ProcessingMsg: "",
		Report:        []string{},
	}

	// Check initial service installation status
	m.checkServiceInstallations()

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.Spinner.Tick,
		tea.EnableBracketedPaste,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.State {
	case StateMenu:
		return m.updateMenu(msg)
	case StateSubmenu:
		return m.updateSubmenu(msg)
	case StateInput:
		return m.updateInput(msg)
	case StateProcessing:
		return m.updateProcessing(msg)
	case StateLogViewer:
		return m.updateLogViewer(msg)
	case StateServiceList:
		return m.updateServiceList(msg)
	case StateServiceActions:
		return m.updateServiceActions(msg)
	}
	return m, nil
}

// updateServiceActions handles input in service actions state
func (m Model) updateServiceActions(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			// Return to service list
			m.State = StateServiceList
			m.Report = []string{} // Clear report
			return m, tea.ClearScreen
		case "s":
			// Show service status
			commands, descriptions := actions.GetServiceStatus(m.CurrentService.Name)
			return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-status-%s", m.CurrentService.Name))
		case "r":
			// Restart service
			config := actions.ServiceActionConfig{
				ServiceName: m.CurrentService.Name,
				Action:      "restart",
			}
			commands, descriptions, err := actions.ControlService(config)
			if err != nil {
				m.State = StateProcessing
				m.ProcessingMsg = ""
				m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
				return m, tea.ClearScreen
			}
			return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-%s-restart", m.CurrentService.Name))
		case "t":
			// Stop service
			config := actions.ServiceActionConfig{
				ServiceName: m.CurrentService.Name,
				Action:      "stop",
			}
			commands, descriptions, err := actions.ControlService(config)
			if err != nil {
				m.State = StateProcessing
				m.ProcessingMsg = ""
				m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
				return m, tea.ClearScreen
			}
			return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-%s-stop", m.CurrentService.Name))
		case "a":
			// Start service
			config := actions.ServiceActionConfig{
				ServiceName: m.CurrentService.Name,
				Action:      "start",
			}
			commands, descriptions, err := actions.ControlService(config)
			if err != nil {
				m.State = StateProcessing
				m.ProcessingMsg = ""
				m.Report = []string{WarnStyle.Render(fmt.Sprintf("❌ Error: %v", err))}
				return m, tea.ClearScreen
			}
			return m.startCommandQueue(commands, descriptions, fmt.Sprintf("service-%s-start", m.CurrentService.Name))
		}
	}
	return m, nil
}

func (m Model) updateProcessing(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "enter", " ":
			// Only allow exit if processing is complete (no processingMsg)
			if m.ProcessingMsg == "" {
				// Special case: if we're showing service list, start interactive control
				if m.QueueServiceName == "list-services" {
					m.QueueServiceName = "" // Clear the service name
					return m.controlServiceInteractive()
				}

				// Special case: if we need to return to service list
				if m.ReturnToServiceList {
					m.ReturnToServiceList = false // Clear the flag
					m.State = StateServiceList
					m.Report = []string{} // Clear report
					return m, tea.ClearScreen
				}

				// Return to main menu after processing and refresh service status
				m.State = StateMenu
				m.CurrentMenu = MenuMain
				m.Choices = []string{
					"Core Services",
					"Laravel Management",
					"Server Management",
					"Monitoring Dashboard",
					"Exit",
				}
				m.FormData = make(map[string]string)
				m.Report = []string{} // Clear report
				m.Cursor = 0          // Reset cursor to top of menu
				modelPtr := &m
				modelPtr.checkServiceInstallations() // Refresh all service statuses
				return *modelPtr, tea.ClearScreen
			}
		// Monitoring dashboard navigation (only when processing is complete)
		case "l":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				m.MonitoringView = MonitoringViewLive
				return m.showMonitoringDashboard()
			}
		case "h":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				m.MonitoringView = MonitoringViewHistorical
				return m.showMonitoringDashboard()
			}
		case "e":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				m.MonitoringView = MonitoringViewEvents
				return m.showMonitoringDashboard()
			}
		case "s":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				m.MonitoringView = MonitoringViewStorage
				return m.showMonitoringDashboard()
			}
		// Time range selection (only for historical and events views)
		case "1":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() &&
				(m.MonitoringView == MonitoringViewHistorical || m.MonitoringView == MonitoringViewEvents) {
				m.MonitoringTimeRange = TimeRangeLast1Hour
				return m.showMonitoringDashboard()
			}
		case "6":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() &&
				(m.MonitoringView == MonitoringViewHistorical || m.MonitoringView == MonitoringViewEvents) {
				m.MonitoringTimeRange = TimeRangeLast6Hours
				return m.showMonitoringDashboard()
			}
		case "d":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() &&
				(m.MonitoringView == MonitoringViewHistorical || m.MonitoringView == MonitoringViewEvents) {
				m.MonitoringTimeRange = TimeRangeLast24Hours
				return m.showMonitoringDashboard()
			}
		case "w":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() &&
				(m.MonitoringView == MonitoringViewHistorical || m.MonitoringView == MonitoringViewEvents) {
				m.MonitoringTimeRange = TimeRangeLast7Days
				return m.showMonitoringDashboard()
			}
		case "m":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() &&
				(m.MonitoringView == MonitoringViewHistorical || m.MonitoringView == MonitoringViewEvents) {
				m.MonitoringTimeRange = TimeRangeLast30Days
				return m.showMonitoringDashboard()
			}
		case "r":
			// Refresh current monitoring view
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				return m.showMonitoringDashboard()
			}
		// Scrolling controls for monitoring dashboard
		case "up", "k":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				if m.MonitoringScroll > 0 {
					m.MonitoringScroll--
				}
				return m, nil
			}
		case "down", "j":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				// Calculate maximum scroll based on content length and screen height
				maxScroll := len(m.Report) - 20 // Assume ~20 lines visible on screen
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.MonitoringScroll < maxScroll {
					m.MonitoringScroll++
				}
				return m, nil
			}
		case "pageup":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				m.MonitoringScroll -= 10
				if m.MonitoringScroll < 0 {
					m.MonitoringScroll = 0
				}
				return m, nil
			}
		case "pagedown":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				maxScroll := len(m.Report) - 20
				if maxScroll < 0 {
					maxScroll = 0
				}
				m.MonitoringScroll += 10
				if m.MonitoringScroll > maxScroll {
					m.MonitoringScroll = maxScroll
				}
				return m, nil
			}
		case "home":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				m.MonitoringScroll = 0
				return m, nil
			}
		case "end":
			if m.ProcessingMsg == "" && m.isInMonitoringDashboard() {
				maxScroll := len(m.Report) - 20
				if maxScroll < 0 {
					maxScroll = 0
				}
				m.MonitoringScroll = maxScroll
				return m, nil
			}
		}
	case CmdCompletedMsg:
		// Command execution completed
		modelPtr := &m

		// Log the command execution
		if logErr := modelPtr.logCommand(msg.Result); logErr != nil {
			m.Logger.Error("Failed to log command", "error", logErr)
		}

		if msg.Result.Error != nil {
			// Command failed - stop queue and show error
			m.ProcessingMsg = ""
			m.CommandQueue = []string{}
			m.DescriptionQueue = []string{}
			m.QueueIndex = 0
			m.Report = append(m.Report, WarnStyle.Render(fmt.Sprintf("❌ Failed: %v", msg.Result.Error)))
			if strings.TrimSpace(msg.Result.Output) != "" {
				m.Report = append(m.Report, WarnStyle.Render(fmt.Sprintf("Output: %s", msg.Result.Output)))
			}
			return *modelPtr, nil
		}

		// Command succeeded - check if there are more commands in queue
		if len(m.CommandQueue) > 0 && m.QueueIndex < len(m.CommandQueue)-1 {
			// Execute next command in queue
			m.QueueIndex++
			m.ProcessingMsg = m.DescriptionQueue[m.QueueIndex]
			m.Report = append(m.Report, InfoStyle.Render(fmt.Sprintf("✅ %s", m.DescriptionQueue[m.QueueIndex-1])))
			return *modelPtr, tea.Batch(
				m.Spinner.Tick,
				ExecuteCommandAsync(m.CommandQueue[m.QueueIndex], m.DescriptionQueue[m.QueueIndex], m.QueueServiceName),
			)
		}

		// All commands completed successfully
		m.ProcessingMsg = ""
		m.Report = append(m.Report, InfoStyle.Render("✅ All operations completed successfully"))
		if msg.ServiceName != "" || m.QueueServiceName != "" {
			serviceName := msg.ServiceName
			if serviceName == "" {
				serviceName = m.QueueServiceName
			}

			// Handle service-specific post-installation setup
			if serviceName == "caddy" {
				modelPtr.setupCaddyLaravelConfig()
			} else if serviceName == "github-ssh" {
				// Show the generated SSH key and instructions
				modelPtr.showGeneratedSSHKey()
			} else if serviceName == "github-test" {
				// Show GitHub connection test results
				modelPtr.showGitHubTestResults()
			} else if serviceName == "list-services" {
				// Parse services and create list
				services := actions.ParseServiceList(msg.Result.Output)
				modelPtr.createServiceList(services)
				modelPtr.State = StateServiceList
				return *modelPtr, tea.ClearScreen
			} else if strings.HasPrefix(serviceName, "service-status-") {
				// Service status command completed - show results and return to service list
				serviceNameOnly := strings.TrimPrefix(serviceName, "service-status-")
				modelPtr.showServiceStatusResults(serviceNameOnly, msg.Result.Output)
				return *modelPtr, tea.ClearScreen
			} else if strings.HasPrefix(serviceName, "service-") && (strings.Contains(serviceName, "-restart") || strings.Contains(serviceName, "-stop") || strings.Contains(serviceName, "-start")) {
				// Service action command completed - show results and return to service list
				parts := strings.Split(serviceName, "-")
				if len(parts) >= 3 {
					serviceNameOnly := strings.Join(parts[1:len(parts)-1], "-") // Remove "service-" prefix and "-action" suffix
					action := parts[len(parts)-1]
					modelPtr.showServiceActionResults(serviceNameOnly, action, msg.Result.Output)
				}
				return *modelPtr, tea.ClearScreen
			}

			modelPtr.refreshServiceStatus(serviceName)
		}

		// Clear queue
		m.CommandQueue = []string{}
		m.DescriptionQueue = []string{}
		m.QueueIndex = 0
		m.QueueServiceName = ""

		return *modelPtr, nil
	default:
		// Update spinner
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

// startCommandQueue starts executing a queue of commands sequentially
func (m Model) startCommandQueue(commands, descriptions []string, serviceName string) (Model, tea.Cmd) {
	if len(commands) == 0 || len(descriptions) == 0 {
		return m, nil
	}

	m.State = StateProcessing
	m.CommandQueue = commands
	m.DescriptionQueue = descriptions
	m.QueueIndex = 0
	m.QueueServiceName = serviceName
	m.ProcessingMsg = descriptions[0]
	m.Report = []string{InfoStyle.Render("Starting multi-step operation...")}

	return m, tea.Batch(
		m.Spinner.Tick,
		ExecuteCommandAsync(commands[0], descriptions[0], serviceName),
	)
}

// ExecuteCommandAsync creates a command that executes a shell command asynchronously
func ExecuteCommandAsync(command, description, serviceName string) tea.Cmd {
	return func() tea.Msg {
		// Execute the command using the existing logging infrastructure
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

		result := logging.LoggedExecResult{
			Command:   command,
			Output:    string(output),
			Error:     err,
			ExitCode:  exitCode,
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  duration,
		}

		return CmdCompletedMsg{
			Result:      result,
			ServiceName: serviceName,
		}
	}
}

// logCommand logs command execution results
func (m Model) logCommand(result logging.LoggedExecResult) error {
	if m.Logger != nil {
		return m.Logger.LogCommand(result)
	}
	return nil
}
