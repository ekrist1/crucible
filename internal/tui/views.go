package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// View rendering functions for different TUI states

// View is the main view function that delegates to specific state views
func (m Model) View() string {
	switch m.State {
	case StateMenu:
		return m.viewMenu()
	case StateSubmenu:
		return m.viewSubmenu()
	case StateInput:
		return m.viewInput()
	case StateProcessing:
		return m.viewProcessing()
	case StateLogViewer:
		return m.viewLogViewer()
	case StateServiceList:
		return m.viewServiceList()
	}
	return ""
}

// viewMenu renders the main menu
func (m Model) viewMenu() string {
	s := TitleStyle.Render("🔧 Crucible - Server Setup made easy for Laravel and Python") + "\n\n"

	for i, choice := range m.Choices {
		cursor := " "
		if m.Cursor == i {
			cursor = ">"
			choice = SelectedStyle.Render(choice)
		} else {
			choice = ChoiceStyle.Render(choice)
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\nPress q to quit, Enter to select.\n"
	return s
}

// viewSubmenu renders submenus with service status icons
func (m Model) viewSubmenu() string {
	var title string
	switch m.CurrentMenu {
	case MenuCoreServices:
		title = "🔧 Core Services"
	case MenuLaravelManagement:
		title = "🚀 Laravel Management"
	case MenuServerManagement:
		title = "⚙️ Server Management"
	}

	s := TitleStyle.Render(title) + "\n\n"

	for i, choice := range m.Choices {
		cursor := " "
		serviceIcon := ""

		// Add service status icons for installation options in Core Services
		if m.CurrentMenu == MenuCoreServices && i < len(m.Choices)-1 {
			switch i {
			case 0: // Install PHP 8.4
				serviceIcon = m.getServiceIcon("php") + " "
			case 1: // Install PHP Composer
				serviceIcon = m.getServiceIcon("composer") + " "
			case 2: // Install Python, pip, and virtualenv
				serviceIcon = m.getServiceIcon("python") + " "
			case 3: // Install Node.js and npm
				serviceIcon = m.getServiceIcon("node") + " "
			case 4: // Install MySQL
				serviceIcon = m.getServiceIcon("mysql") + " "
			case 5: // Install Caddy Server
				serviceIcon = m.getServiceIcon("caddy") + " "
			case 6: // Install Supervisor
				serviceIcon = m.getServiceIcon("supervisor") + " "
			case 7: // Install Git CLI
				serviceIcon = m.getServiceIcon("git") + " "
			}
		}

		if m.Cursor == i {
			cursor = ">"
			choice = SelectedStyle.Render(serviceIcon + choice)
		} else {
			choice = ChoiceStyle.Render(serviceIcon + choice)
		}

		s += fmt.Sprintf("%s %s\n", cursor, choice)
	}

	s += "\nPress Esc or select 'Back to Main Menu' to return, q to quit, r to refresh.\n"
	if m.CurrentMenu == MenuCoreServices {
		s += "\n✅ = Installed  ⬜ = Not installed\n"
	}
	return s
}

// viewInput renders the input form
func (m Model) viewInput() string {
	s := TitleStyle.Render("🔧 Crucible - Laravel Server Setup") + "\n\n"
	s += PromptStyle.Render(m.InputPrompt) + "\n\n"

	// Hide password input
	displayValue := m.InputValue
	if m.InputField == "mysqlRootPassword" || m.InputField == "githubPassphrase" {
		displayValue = strings.Repeat("*", len(m.InputValue))
	}

	// Show text with cursor at correct position
	var inputDisplay string
	if m.InputCursor >= len(displayValue) {
		// Cursor at end
		inputDisplay = displayValue + "│"
	} else {
		// Cursor in middle
		inputDisplay = displayValue[:m.InputCursor] + "│" + displayValue[m.InputCursor:]
	}

	s += InputStyle.Render(inputDisplay) + "\n\n"
	s += "Press Enter to continue, Esc to cancel\n"
	s += "Arrow keys to move cursor, Ctrl+A/E for home/end, Ctrl+U/K to delete line\n"
	return s
}

// viewProcessing renders the processing state with spinner or results
func (m Model) viewProcessing() string {
	s := TitleStyle.Render("🔧 Crucible - Laravel Server Setup") + "\n\n"
	if m.ProcessingMsg != "" {
		s += fmt.Sprintf("%s %s\n\n", m.Spinner.View(), m.ProcessingMsg)
		s += "Please wait...\n"
	} else {
		if len(m.Report) > 0 {
			s += strings.Join(m.Report, "\n") + "\n\n"
		}
		s += "Processing completed!\n"
		s += "Press any key to return to main menu.\n"
	}
	return s
}

// updateLogViewer handles log viewer navigation input
func (m Model) updateLogViewer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			// Return to main menu
			m.State = StateMenu
			m.Cursor = 0
			m.LogLines = []string{}
			m.LogScroll = 0
			return m, tea.ClearScreen
		case "up", "k":
			// Scroll up
			if m.LogScroll > 0 {
				m.LogScroll--
			}
		case "down", "j":
			// Scroll down
			logViewHeight := 18
			maxScroll := len(m.LogLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.LogScroll < maxScroll {
				m.LogScroll++
			}
		case "home", "g":
			// Go to top
			m.LogScroll = 0
		case "end", "G":
			// Go to bottom
			logViewHeight := 18
			maxScroll := len(m.LogLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.LogScroll = maxScroll
		case "pageup":
			// Page up
			logViewHeight := 18
			m.LogScroll -= logViewHeight
			if m.LogScroll < 0 {
				m.LogScroll = 0
			}
		case "pagedown":
			// Page down
			logViewHeight := 18
			maxScroll := len(m.LogLines) - logViewHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.LogScroll += logViewHeight
			if m.LogScroll > maxScroll {
				m.LogScroll = maxScroll
			}
		}
	}
	return m, nil
}

// viewLogViewer renders the log viewer with scrolling support
func (m Model) viewLogViewer() string {
	s := TitleStyle.Render("🔧 Crucible - Installation Logs") + "\n\n"

	// Calculate view height (assuming terminal height of about 24 lines, minus header and footer)
	logViewHeight := 18

	if len(m.LogLines) == 0 {
		s += "No log lines to display.\n\n"
	} else {
		// Calculate visible range
		startIdx := m.LogScroll
		endIdx := startIdx + logViewHeight

		if endIdx > len(m.LogLines) {
			endIdx = len(m.LogLines)
		}

		// Show log lines with line numbers
		for i := startIdx; i < endIdx; i++ {
			line := m.LogLines[i]
			lineNum := fmt.Sprintf("%4d: ", i+1)

			// Style different types of log lines
			if strings.Contains(line, "COMMAND:") {
				s += InfoStyle.Render(lineNum) + InfoStyle.Render(line) + "\n"
			} else if strings.Contains(line, "ERROR:") || strings.Contains(line, "EXIT CODE:") {
				s += WarnStyle.Render(lineNum) + WarnStyle.Render(line) + "\n"
			} else if strings.Contains(line, "STATUS: SUCCESS") {
				s += InfoStyle.Render(lineNum) + InfoStyle.Render(line) + "\n"
			} else {
				s += ChoiceStyle.Render(lineNum) + line + "\n"
			}
		}

		s += "\n"

		// Show scroll position info
		totalLines := len(m.LogLines)
		visibleStart := m.LogScroll + 1
		visibleEnd := m.LogScroll + (endIdx - startIdx)

		s += ChoiceStyle.Render(fmt.Sprintf("Lines %d-%d of %d", visibleStart, visibleEnd, totalLines)) + "\n"
	}

	s += "\nNavigation: ↑/↓ scroll, Home/End jump, PgUp/PgDn page, q/Esc to exit\n"
	return s
}
