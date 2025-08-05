package models

import (
	"crucible/internal/logging"
)

// SharedData contains data that needs to be accessible across different models
type SharedData struct {
	ServiceStatus    map[string]bool
	ProcessingMsg    string
	CommandQueue     *CommandQueue
	NavigationStack  []AppState
	Logger           *logging.Logger
	TerminalWidth    int
	TerminalHeight   int
}

// CommandQueue manages sequential command execution
type CommandQueue struct {
	Commands        []string
	Descriptions    []string
	Index          int
	ServiceName    string
	Results        []string
	IsProcessing   bool
}

// AppState represents the different states of the application
type AppState int

const (
	StateMenu AppState = iota
	StateSubmenu
	StateInput
	StateProcessing
	StateLogViewer
	StateServiceList
	StateServiceActions
	StateNextJSMenu
	StateNextJSCreate
	StateLaravelCreate
	StateMonitoring
)

// MenuLevel represents different menu levels
type MenuLevel int

const (
	MenuMain MenuLevel = iota
	MenuCoreServices
	MenuLaravelManagement
	MenuServerManagement
	MenuSettings
)

// Message types for async operations
type CmdExecutionMsg struct {
	Command     string
	Description string
	ServiceName string
}

type CmdCompletedMsg struct {
	Result      logging.LoggedExecResult
	ServiceName string
}

type NavigationMsg struct {
	State AppState
	Data  interface{}
}

type BackNavigationMsg struct{}

// NewSharedData creates a new SharedData instance
func NewSharedData(logger *logging.Logger) *SharedData {
	return &SharedData{
		ServiceStatus:   make(map[string]bool),
		ProcessingMsg:   "",
		CommandQueue:    &CommandQueue{},
		NavigationStack: []AppState{},
		Logger:          logger,
		TerminalWidth:   80,  // Default terminal width
		TerminalHeight:  24,  // Default terminal height
	}
}

// AddCommand adds a command to the queue
func (cq *CommandQueue) AddCommand(command, description string) {
	cq.Commands = append(cq.Commands, command)
	cq.Descriptions = append(cq.Descriptions, description)
}

// HasNext returns true if there are more commands to execute
func (cq *CommandQueue) HasNext() bool {
	return cq.Index < len(cq.Commands)
}

// Next returns the next command to execute
func (cq *CommandQueue) Next() (command, description string, ok bool) {
	if !cq.HasNext() {
		return "", "", false
	}
	command = cq.Commands[cq.Index]
	description = cq.Descriptions[cq.Index]
	cq.Index++
	return command, description, true
}

// Reset clears the command queue
func (cq *CommandQueue) Reset() {
	cq.Commands = []string{}
	cq.Descriptions = []string{}
	cq.Index = 0
	cq.ServiceName = ""
	cq.Results = []string{}
	cq.IsProcessing = false
}

// AddResult adds a result to the queue
func (cq *CommandQueue) AddResult(result string) {
	cq.Results = append(cq.Results, result)
}

// SetTerminalSize updates the terminal dimensions
func (sd *SharedData) SetTerminalSize(width, height int) {
	sd.TerminalWidth = width
	sd.TerminalHeight = height
}

// GetViewableLines returns the number of lines available for content display
// Reserves space for title, status lines, and help text
func (sd *SharedData) GetViewableLines() int {
	// Reserve lines for: title (2-3 lines), status/info (2-3 lines), help (2-3 lines)
	reservedLines := 8
	if sd.TerminalHeight <= reservedLines {
		return 5 // Minimum viewable lines
	}
	return sd.TerminalHeight - reservedLines
}

// GetContentWidth returns the usable width for content display
func (sd *SharedData) GetContentWidth() int {
	// Reserve some space for margins and indicators
	reservedWidth := 4
	if sd.TerminalWidth <= reservedWidth {
		return 40 // Minimum content width
	}
	return sd.TerminalWidth - reservedWidth
}