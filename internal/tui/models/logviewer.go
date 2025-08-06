package models

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// LogViewerModel handles log file viewing
type LogViewerModel struct {
	BaseModel
	lines       []string
	scrollPos   int
	filter      string
	loading     bool
	filterMode  bool
	filterInput textinput.Model
}

// Log loading message
type logLinesLoadedMsg struct {
	lines []string
	err   error
}

// NewLogViewerModel creates a new log viewer model
func NewLogViewerModel(shared *SharedData) *LogViewerModel {
	// Initialize text input for filtering
	filterInput := textinput.New()
	filterInput.Placeholder = "Enter filter text..."
	filterInput.CharLimit = 100
	// Width will be set dynamically based on terminal size

	return &LogViewerModel{
		BaseModel:   NewBaseModel(shared),
		lines:       []string{},
		scrollPos:   0,
		filter:      "",
		loading:     false,
		filterMode:  false,
		filterInput: filterInput,
	}
}

// Init initializes the log viewer model
func (m *LogViewerModel) Init() tea.Cmd {
	return m.loadLogLines()
}

// Update handles log viewer updates
func (m *LogViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle filter mode separately
	if m.filterMode {
		return m.updateFilterMode(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m, m.GoBack()
		case "r":
			// Refresh logs
			return m, m.loadLogLines()

		// Scrolling
		case "up", "k":
			if m.scrollPos > 0 {
				m.scrollPos--
			}
		case "down", "j":
			viewableLines := m.shared.GetViewableLines()
			maxScroll := len(m.getFilteredLines()) - viewableLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollPos < maxScroll {
				m.scrollPos++
			}
		case "pageup":
			m.scrollPos -= 10
			if m.scrollPos < 0 {
				m.scrollPos = 0
			}
		case "pagedown":
			viewableLines := m.shared.GetViewableLines()
			maxScroll := len(m.getFilteredLines()) - viewableLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollPos += 10
			if m.scrollPos > maxScroll {
				m.scrollPos = maxScroll
			}
		case "home":
			m.scrollPos = 0
		case "end":
			viewableLines := m.shared.GetViewableLines()
			maxScroll := len(m.getFilteredLines()) - viewableLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollPos = maxScroll

		// Filtering functionality
		case "f":
			// Enter filter mode
			m.filterMode = true
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			return m, textinput.Blink
		case "c":
			// Clear filter
			m.filter = ""
			m.scrollPos = 0
		}

	case logLinesLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.lines = []string{fmt.Sprintf("Error loading logs: %v", msg.err)}
		} else {
			m.lines = msg.lines
		}
		return m, nil
	}

	return m, nil
}

// View renders the log viewer interface
func (m *LogViewerModel) View() string {
	var s strings.Builder

	// Update filter input width dynamically
	m.filterInput.Width = m.shared.GetContentWidth() - 10 // Leave some margin
	if m.filterInput.Width < 30 {
		m.filterInput.Width = 30 // Minimum width
	}

	// Title
	title := "📋 Log Viewer"
	if m.filter != "" {
		title += fmt.Sprintf(" (Filter: %s)", m.filter)
	}
	s.WriteString(titleStyle.Render(title))
	s.WriteString("\n\n")

	// Loading indicator
	if m.loading {
		s.WriteString("Loading logs...\n")
		return s.String()
	}

	// Filter input mode
	if m.filterMode {
		s.WriteString(infoStyle.Render("Filter mode - Enter search terms:"))
		s.WriteString("\n")
		s.WriteString(m.filterInput.View())
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Press Enter to apply filter or Esc to cancel"))
		s.WriteString("\n")
		return s.String()
	}

	// Log content
	filteredLines := m.getFilteredLines()
	if len(filteredLines) == 0 {
		s.WriteString(helpStyle.Render("No log entries found"))
		s.WriteString("\n")
	} else {
		// Show lines with scrolling
		startLine := m.scrollPos
		viewableLines := m.shared.GetViewableLines()
		endLine := startLine + viewableLines
		if endLine > len(filteredLines) {
			endLine = len(filteredLines)
		}

		for i := startLine; i < endLine; i++ {
			line := filteredLines[i]
			// Apply basic syntax highlighting
			if strings.Contains(line, "ERROR") || strings.Contains(line, "FAILED") {
				line = errorStyle.Render(line)
			} else if strings.Contains(line, "WARN") {
				line = warnStyle.Render(line)
			} else if strings.Contains(line, "INFO") || strings.Contains(line, "SUCCESS") {
				line = infoStyle.Render(line)
			}
			s.WriteString(line)
			s.WriteString("\n")
		}

		// Scroll indicator
		if len(filteredLines) > viewableLines {
			s.WriteString("\n")
			scrollInfo := fmt.Sprintf("Showing lines %d-%d of %d",
				startLine+1, endLine, len(filteredLines))
			s.WriteString(helpStyle.Render(scrollInfo))
		}
	}

	// Help text
	s.WriteString("\n")
	help := []string{
		"Navigation: ↑/↓=Scroll, PageUp/PageDown=Fast scroll, Home/End=Jump",
		"Actions: r=Refresh, f=Filter, c=Clear filter",
		"Esc=Back to menu, q=Quit",
	}
	s.WriteString(helpStyle.Render(strings.Join(help, " | ")))

	return s.String()
}

// loadLogLines loads log lines asynchronously
func (m *LogViewerModel) loadLogLines() tea.Cmd {
	m.loading = true
	return func() tea.Msg {
		// Load log lines from the logger
		if m.shared.Logger != nil {
			lines, err := m.shared.Logger.ReadLogLines()
			return logLinesLoadedMsg{lines: lines, err: err}
		}
		return logLinesLoadedMsg{
			lines: []string{"No logger available"},
			err:   nil,
		}
	}
}

// getFilteredLines returns lines filtered by the current filter
func (m *LogViewerModel) getFilteredLines() []string {
	if m.filter == "" {
		return m.lines
	}

	var filtered []string
	for _, line := range m.lines {
		if strings.Contains(strings.ToLower(line), strings.ToLower(m.filter)) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

// updateFilterMode handles input when in filter mode
func (m *LogViewerModel) updateFilterMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Apply filter
			m.filter = m.filterInput.Value()
			m.filterMode = false
			m.filterInput.Blur()
			m.scrollPos = 0 // Reset scroll position
			return m, nil
		case "esc":
			// Cancel filter mode
			m.filterMode = false
			m.filterInput.Blur()
			return m, nil
		}
	}

	// Update the text input
	m.filterInput, cmd = m.filterInput.Update(msg)
	return m, cmd
}

// SetFilter sets the current filter
func (m *LogViewerModel) SetFilter(filter string) {
	m.filter = filter
	m.scrollPos = 0 // Reset scroll when filter changes
}
