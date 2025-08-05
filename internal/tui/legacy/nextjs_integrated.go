package tui

import (
	"fmt"
	"unicode"

	"crucible/internal/nextjs"
	tea "github.com/charmbracelet/bubbletea"
)

// NextJS site loading message types
type nextjsSitesLoadedMsg struct {
	sites []*nextjs.Site
}

type nextjsSiteCreatedMsg struct {
	name string
}

type nextjsErrorMsg struct {
	err error
}

// loadNextJSSites loads the NextJS sites
func (m Model) loadNextJSSites() tea.Cmd {
	return func() tea.Msg {
		sites, err := m.NextJSManager.ListSites()
		if err != nil {
			return nextjsErrorMsg{err}
		}
		return nextjsSitesLoadedMsg{sites}
	}
}

// updateNextJSMenu handles the NextJS menu screen
func (m Model) updateNextJSMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m.returnToMainMenu()
		case "c":
			m.State = StateNextJSCreate
			// Reset the create form
			m.NextJSCreateForm = NextJSSiteForm{
				step:         0,
				branch:       "main",
				environment:  "production",
				nodeVersion:  "18",
				pkgManager:   "auto",
				buildCmd:     "npm run build",
				startCmd:     "npm start",
				instances:    1,
				envVars:      make(map[string]string),
				currentField: "repository",
				inputCursor:  0,
			}
			return m, nil
		case "r":
			return m, m.loadNextJSSites()
		case "up", "k":
			if m.NextJSSelected > 0 {
				m.NextJSSelected--
			}
		case "down", "j":
			if m.NextJSSelected < len(m.NextJSSites)-1 {
				m.NextJSSelected++
			}
		case "s":
			if len(m.NextJSSites) > 0 {
				site := m.NextJSSites[m.NextJSSelected]
				return m, m.toggleNextJSSite(site.Name)
			}
		case "u":
			if len(m.NextJSSites) > 0 {
				site := m.NextJSSites[m.NextJSSelected]
				return m, m.updateNextJSSite(site.Name)
			}
		case "d":
			if len(m.NextJSSites) > 0 {
				site := m.NextJSSites[m.NextJSSelected]
				return m, m.deleteNextJSSite(site.Name)
			}
		case "l":
			if len(m.NextJSSites) > 0 {
				site := m.NextJSSites[m.NextJSSelected]
				return m, m.viewNextJSLogs(site.Name)
			}
		}
	case nextjsSitesLoadedMsg:
		m.NextJSSites = msg.sites
		m.NextJSMessage = fmt.Sprintf("Loaded %d Next.js sites", len(m.NextJSSites))
		return m, nil
	case nextjsSiteCreatedMsg:
		m.NextJSMessage = fmt.Sprintf("✅ Site '%s' created successfully!", msg.name)
		return m, m.loadNextJSSites()
	case nextjsErrorMsg:
		m.NextJSMessage = fmt.Sprintf("❌ Error: %s", msg.err.Error())
		return m, nil
	}
	return m, nil
}

// NextJS site action commands
func (m Model) toggleNextJSSite(name string) tea.Cmd {
	return func() tea.Msg {
		// Implementation for start/stop site
		return nextjsErrorMsg{fmt.Errorf("Site toggle not implemented yet")}
	}
}

func (m Model) updateNextJSSite(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.NextJSManager.UpdateSite(name)
		if err != nil {
			return nextjsErrorMsg{err}
		}
		return nextjsSitesLoadedMsg{} // Trigger reload
	}
}

func (m Model) deleteNextJSSite(name string) tea.Cmd {
	return func() tea.Msg {
		err := m.NextJSManager.DeleteSite(name)
		if err != nil {
			return nextjsErrorMsg{err}
		}
		return nextjsSitesLoadedMsg{} // Trigger reload
	}
}

func (m Model) viewNextJSLogs(name string) tea.Cmd {
	return func() tea.Msg {
		// Implementation for viewing logs
		return nextjsErrorMsg{fmt.Errorf("Log viewing not implemented yet")}
	}
}

// updateNextJSCreate handles the NextJS site creation form
func (m Model) updateNextJSCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.State = StateNextJSMenu
			return m, nil
		case "tab":
			return m.nextNextJSField()
		case "shift+tab":
			return m.prevNextJSField()
		case "enter":
			return m.nextNextJSFormStep()
		default:
			// Handle text input
			return m.handleNextJSFormInput(msg.String())
		}
	}
	return m, nil
}

// NextJS form field navigation
func (m Model) nextNextJSField() (tea.Model, tea.Cmd) {
	switch m.NextJSCreateForm.step {
	case 0: // Repository info
		switch m.NextJSCreateForm.currentField {
		case "repository":
			m.NextJSCreateForm.currentField = "branch"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.branch)
		case "branch":
			m.NextJSCreateForm.currentField = "repository"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.repository)
		}
	case 1: // Site config
		switch m.NextJSCreateForm.currentField {
		case "name":
			m.NextJSCreateForm.currentField = "domain"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.domain)
		case "domain":
			m.NextJSCreateForm.currentField = "name"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.name)
		}
	case 2: // Build settings
		switch m.NextJSCreateForm.currentField {
		case "buildCmd":
			m.NextJSCreateForm.currentField = "startCmd"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.startCmd)
		case "startCmd":
			m.NextJSCreateForm.currentField = "buildCmd"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.buildCmd)
		}
	}
	return m, nil
}

func (m Model) prevNextJSField() (tea.Model, tea.Cmd) {
	// For simplicity, just call nextField since we only have 2 fields per step
	return m.nextNextJSField()
}

// NextJS form step navigation
func (m Model) nextNextJSFormStep() (tea.Model, tea.Cmd) {
	switch m.NextJSCreateForm.step {
	case 0: // Repository info
		if m.NextJSCreateForm.repository == "" {
			m.NextJSMessage = "❌ Repository URL is required"
			return m, nil
		}
		m.NextJSCreateForm.step = 1
		m.NextJSCreateForm.currentField = "name"
		m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.name)
	case 1: // Site config
		if m.NextJSCreateForm.name == "" || m.NextJSCreateForm.domain == "" {
			m.NextJSMessage = "❌ Site name and domain are required"
			return m, nil
		}
		m.NextJSCreateForm.step = 2
		m.NextJSCreateForm.currentField = "buildCmd"
		m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.buildCmd)
	case 2: // Build settings
		m.NextJSCreateForm.step = 3
		m.NextJSCreateForm.currentField = ""
	case 3: // Final confirmation
		return m, m.createNextJSSite()
	}
	return m, nil
}

func (m Model) prevNextJSFormStep() (tea.Model, tea.Cmd) {
	if m.NextJSCreateForm.step > 0 {
		m.NextJSCreateForm.step--
		switch m.NextJSCreateForm.step {
		case 0:
			m.NextJSCreateForm.currentField = "repository"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.repository)
		case 1:
			m.NextJSCreateForm.currentField = "name"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.name)
		case 2:
			m.NextJSCreateForm.currentField = "buildCmd"
			m.NextJSCreateForm.inputCursor = len(m.NextJSCreateForm.buildCmd)
		}
	}
	return m, nil
}

// NextJS form input handling
func (m Model) handleNextJSFormInput(key string) (tea.Model, tea.Cmd) {
	// Handle special keys first
	switch key {
	case "backspace":
		return m.handleNextJSBackspace(), nil
	case "delete":
		return m.handleNextJSDelete(), nil
	case "left":
		return m.handleNextJSCursorLeft(), nil
	case "right":
		return m.handleNextJSCursorRight(), nil
	case "home", "ctrl+a":
		m.NextJSCreateForm.inputCursor = 0
		return m, nil
	case "end", "ctrl+e":
		m.NextJSCreateForm.inputCursor = len(m.getCurrentNextJSFieldValue())
		return m, nil
	case "ctrl+u":
		return m.handleNextJSClearToStart(), nil
	case "ctrl+k":
		return m.handleNextJSClearToEnd(), nil
	default:
		// Handle printable characters
		if len(key) == 1 && unicode.IsPrint(rune(key[0])) {
			return m.handleNextJSCharacterInput(key), nil
		}
	}
	return m, nil
}

// Helper functions for NextJS form input handling
func (m Model) getCurrentNextJSFieldValue() string {
	switch m.NextJSCreateForm.currentField {
	case "repository":
		return m.NextJSCreateForm.repository
	case "branch":
		return m.NextJSCreateForm.branch
	case "name":
		return m.NextJSCreateForm.name
	case "domain":
		return m.NextJSCreateForm.domain
	case "environment":
		return m.NextJSCreateForm.environment
	case "nodeVersion":
		return m.NextJSCreateForm.nodeVersion
	case "pkgManager":
		return m.NextJSCreateForm.pkgManager
	case "buildCmd":
		return m.NextJSCreateForm.buildCmd
	case "startCmd":
		return m.NextJSCreateForm.startCmd
	default:
		return ""
	}
}

func (m Model) setCurrentNextJSFieldValue(value string) Model {
	switch m.NextJSCreateForm.currentField {
	case "repository":
		m.NextJSCreateForm.repository = value
	case "branch":
		m.NextJSCreateForm.branch = value
	case "name":
		m.NextJSCreateForm.name = value
	case "domain":
		m.NextJSCreateForm.domain = value
	case "environment":
		m.NextJSCreateForm.environment = value
	case "nodeVersion":
		m.NextJSCreateForm.nodeVersion = value
	case "pkgManager":
		m.NextJSCreateForm.pkgManager = value
	case "buildCmd":
		m.NextJSCreateForm.buildCmd = value
	case "startCmd":
		m.NextJSCreateForm.startCmd = value
	}
	return m
}

func (m Model) handleNextJSBackspace() Model {
	if m.NextJSCreateForm.inputCursor > 0 {
		value := m.getCurrentNextJSFieldValue()
		newValue := value[:m.NextJSCreateForm.inputCursor-1] + value[m.NextJSCreateForm.inputCursor:]
		m = m.setCurrentNextJSFieldValue(newValue)
		m.NextJSCreateForm.inputCursor--
	}
	return m
}

func (m Model) handleNextJSDelete() Model {
	value := m.getCurrentNextJSFieldValue()
	if m.NextJSCreateForm.inputCursor < len(value) {
		newValue := value[:m.NextJSCreateForm.inputCursor] + value[m.NextJSCreateForm.inputCursor+1:]
		m = m.setCurrentNextJSFieldValue(newValue)
	}
	return m
}

func (m Model) handleNextJSCursorLeft() Model {
	if m.NextJSCreateForm.inputCursor > 0 {
		m.NextJSCreateForm.inputCursor--
	}
	return m
}

func (m Model) handleNextJSCursorRight() Model {
	if m.NextJSCreateForm.inputCursor < len(m.getCurrentNextJSFieldValue()) {
		m.NextJSCreateForm.inputCursor++
	}
	return m
}

func (m Model) handleNextJSClearToStart() Model {
	value := m.getCurrentNextJSFieldValue()
	newValue := value[m.NextJSCreateForm.inputCursor:]
	m = m.setCurrentNextJSFieldValue(newValue)
	m.NextJSCreateForm.inputCursor = 0
	return m
}

func (m Model) handleNextJSClearToEnd() Model {
	value := m.getCurrentNextJSFieldValue()
	newValue := value[:m.NextJSCreateForm.inputCursor]
	m = m.setCurrentNextJSFieldValue(newValue)
	return m
}

func (m Model) handleNextJSCharacterInput(key string) Model {
	value := m.getCurrentNextJSFieldValue()
	newValue := value[:m.NextJSCreateForm.inputCursor] + key + value[m.NextJSCreateForm.inputCursor:]
	m = m.setCurrentNextJSFieldValue(newValue)
	m.NextJSCreateForm.inputCursor++
	return m
}

// createNextJSSite creates a new NextJS site
func (m Model) createNextJSSite() tea.Cmd {
	return func() tea.Msg {
		site := &nextjs.Site{
			Name:           m.NextJSCreateForm.name,
			Repository:     m.NextJSCreateForm.repository,
			Branch:         m.NextJSCreateForm.branch,
			Domain:         m.NextJSCreateForm.domain,
			BuildCommand:   m.NextJSCreateForm.buildCmd,
			StartCommand:   m.NextJSCreateForm.startCmd,
			Environment:    m.NextJSCreateForm.environment,
			PM2Instances:   m.NextJSCreateForm.instances,
			NodeVersion:    m.NextJSCreateForm.nodeVersion,
			PackageManager: m.NextJSCreateForm.pkgManager,
			EnvVars:        m.NextJSCreateForm.envVars,
		}

		err := m.NextJSManager.CreateSite(site)
		if err != nil {
			return nextjsErrorMsg{err}
		}
		return nextjsSiteCreatedMsg{m.NextJSCreateForm.name}
	}
}
