package tui

import (
	"fmt"
	"os/exec"
	"unicode"

	"crucible/internal/actions"
	tea "github.com/charmbracelet/bubbletea"
)

// Laravel site creation message types
type laravelSiteCreatedMsg struct {
	siteName string
}

type laravelErrorMsg struct {
	err error
}

// enterLaravelCreate enters the Laravel site creation form
func (m Model) enterLaravelCreate() (tea.Model, tea.Cmd) {
	m.State = StateLaravelCreate
	// Reset the create form
	m.LaravelCreateForm = LaravelSiteForm{
		step:         0,
		branch:       "main",
		currentField: "siteName",
		inputCursor:  0,
	}
	return m, nil
}

// updateLaravelCreate handles the Laravel site creation form
func (m Model) updateLaravelCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			return m.returnToMainMenu()
		case "tab":
			return m.nextLaravelField()
		case "shift+tab":
			return m.prevLaravelField()
		case "enter":
			return m.nextLaravelFormStep()
		default:
			// Handle text input
			return m.handleLaravelFormInput(msg.String())
		}
	case laravelSiteCreatedMsg:
		m.LaravelMessage = fmt.Sprintf("✅ Site '%s' created successfully!", msg.siteName)
		return m.returnToMainMenu()
	case laravelErrorMsg:
		m.LaravelMessage = fmt.Sprintf("❌ Error: %s", msg.err.Error())
		return m, nil
	}
	return m, nil
}

// Laravel form field navigation
func (m Model) nextLaravelField() (tea.Model, tea.Cmd) {
	switch m.LaravelCreateForm.step {
	case 0: // Basic info
		switch m.LaravelCreateForm.currentField {
		case "siteName":
			m.LaravelCreateForm.currentField = "domain"
			m.LaravelCreateForm.inputCursor = len(m.LaravelCreateForm.domain)
		case "domain":
			m.LaravelCreateForm.currentField = "siteName"
			m.LaravelCreateForm.inputCursor = len(m.LaravelCreateForm.siteName)
		}
	case 1: // Git repo
		switch m.LaravelCreateForm.currentField {
		case "gitRepo":
			m.LaravelCreateForm.currentField = "branch"
			m.LaravelCreateForm.inputCursor = len(m.LaravelCreateForm.branch)
		case "branch":
			m.LaravelCreateForm.currentField = "gitRepo"
			m.LaravelCreateForm.inputCursor = len(m.LaravelCreateForm.gitRepo)
		}
	}
	return m, nil
}

func (m Model) prevLaravelField() (tea.Model, tea.Cmd) {
	// For simplicity, just call nextField since we only have 2 fields per step
	return m.nextLaravelField()
}

// Laravel form step navigation
func (m Model) nextLaravelFormStep() (tea.Model, tea.Cmd) {
	switch m.LaravelCreateForm.step {
	case 0: // Basic info
		if m.LaravelCreateForm.siteName == "" {
			m.LaravelMessage = "❌ Site name is required"
			return m, nil
		}
		if m.LaravelCreateForm.domain == "" {
			m.LaravelMessage = "❌ Domain is required"
			return m, nil
		}
		m.LaravelCreateForm.step = 1
		m.LaravelCreateForm.currentField = "gitRepo"
		m.LaravelCreateForm.inputCursor = len(m.LaravelCreateForm.gitRepo)
	case 1: // Git repository
		// Validate GitHub URL if provided
		if m.LaravelCreateForm.gitRepo != "" {
			if !isValidGitURL(m.LaravelCreateForm.gitRepo) {
				m.LaravelMessage = "❌ Invalid Git repository URL. Please use format: https://github.com/user/repo.git or git@github.com:user/repo.git"
				return m, nil
			}

			// Test if repository is accessible before proceeding
			if !isGitRepoAccessible(m.LaravelCreateForm.gitRepo) {
				m.LaravelMessage = "❌ Repository not accessible or does not exist. Please check the URL and try again."
				return m, nil
			}
		}
		m.LaravelCreateForm.step = 2
		m.LaravelCreateForm.currentField = ""
	case 2: // Final confirmation
		return m, m.createLaravelSite()
	}
	return m, nil
}

func (m Model) prevLaravelFormStep() (tea.Model, tea.Cmd) {
	if m.LaravelCreateForm.step > 0 {
		m.LaravelCreateForm.step--
		switch m.LaravelCreateForm.step {
		case 0:
			m.LaravelCreateForm.currentField = "siteName"
			m.LaravelCreateForm.inputCursor = len(m.LaravelCreateForm.siteName)
		case 1:
			m.LaravelCreateForm.currentField = "gitRepo"
			m.LaravelCreateForm.inputCursor = len(m.LaravelCreateForm.gitRepo)
		}
	}
	return m, nil
}

// Laravel form input handling
func (m Model) handleLaravelFormInput(key string) (tea.Model, tea.Cmd) {
	// Handle special keys first
	switch key {
	case "backspace":
		return m.handleLaravelBackspace(), nil
	case "delete":
		return m.handleLaravelDelete(), nil
	case "left":
		return m.handleLaravelCursorLeft(), nil
	case "right":
		return m.handleLaravelCursorRight(), nil
	case "home", "ctrl+a":
		m.LaravelCreateForm.inputCursor = 0
		return m, nil
	case "end", "ctrl+e":
		m.LaravelCreateForm.inputCursor = len(m.getCurrentLaravelFieldValue())
		return m, nil
	case "ctrl+u":
		return m.handleLaravelClearToStart(), nil
	case "ctrl+k":
		return m.handleLaravelClearToEnd(), nil
	default:
		// Handle printable characters
		if len(key) == 1 && unicode.IsPrint(rune(key[0])) {
			return m.handleLaravelCharacterInput(key), nil
		}
	}
	return m, nil
}

// Helper functions for Laravel form input handling
func (m Model) getCurrentLaravelFieldValue() string {
	switch m.LaravelCreateForm.currentField {
	case "siteName":
		return m.LaravelCreateForm.siteName
	case "domain":
		return m.LaravelCreateForm.domain
	case "gitRepo":
		return m.LaravelCreateForm.gitRepo
	case "branch":
		return m.LaravelCreateForm.branch
	default:
		return ""
	}
}

func (m Model) setCurrentLaravelFieldValue(value string) Model {
	switch m.LaravelCreateForm.currentField {
	case "siteName":
		m.LaravelCreateForm.siteName = value
	case "domain":
		m.LaravelCreateForm.domain = value
	case "gitRepo":
		m.LaravelCreateForm.gitRepo = value
	case "branch":
		m.LaravelCreateForm.branch = value
	}
	return m
}

func (m Model) handleLaravelBackspace() Model {
	if m.LaravelCreateForm.inputCursor > 0 {
		value := m.getCurrentLaravelFieldValue()
		newValue := value[:m.LaravelCreateForm.inputCursor-1] + value[m.LaravelCreateForm.inputCursor:]
		m = m.setCurrentLaravelFieldValue(newValue)
		m.LaravelCreateForm.inputCursor--
	}
	return m
}

func (m Model) handleLaravelDelete() Model {
	value := m.getCurrentLaravelFieldValue()
	if m.LaravelCreateForm.inputCursor < len(value) {
		newValue := value[:m.LaravelCreateForm.inputCursor] + value[m.LaravelCreateForm.inputCursor+1:]
		m = m.setCurrentLaravelFieldValue(newValue)
	}
	return m
}

func (m Model) handleLaravelCursorLeft() Model {
	if m.LaravelCreateForm.inputCursor > 0 {
		m.LaravelCreateForm.inputCursor--
	}
	return m
}

func (m Model) handleLaravelCursorRight() Model {
	if m.LaravelCreateForm.inputCursor < len(m.getCurrentLaravelFieldValue()) {
		m.LaravelCreateForm.inputCursor++
	}
	return m
}

func (m Model) handleLaravelClearToStart() Model {
	value := m.getCurrentLaravelFieldValue()
	newValue := value[m.LaravelCreateForm.inputCursor:]
	m = m.setCurrentLaravelFieldValue(newValue)
	m.LaravelCreateForm.inputCursor = 0
	return m
}

func (m Model) handleLaravelClearToEnd() Model {
	value := m.getCurrentLaravelFieldValue()
	newValue := value[:m.LaravelCreateForm.inputCursor]
	m = m.setCurrentLaravelFieldValue(newValue)
	return m
}

func (m Model) handleLaravelCharacterInput(key string) Model {
	value := m.getCurrentLaravelFieldValue()
	newValue := value[:m.LaravelCreateForm.inputCursor] + key + value[m.LaravelCreateForm.inputCursor:]
	m = m.setCurrentLaravelFieldValue(newValue)
	m.LaravelCreateForm.inputCursor++
	return m
}

// createLaravelSite creates a new Laravel site using the actions package
func (m Model) createLaravelSite() tea.Cmd {
	return func() tea.Msg {
		config := actions.LaravelSiteConfig{
			SiteName: m.LaravelCreateForm.siteName,
			Domain:   m.LaravelCreateForm.domain,
			GitRepo:  m.LaravelCreateForm.gitRepo,
		}

		commands, descriptions := actions.CreateLaravelSite(config)

		// Execute commands synchronously for now (can be made async later)
		for i, command := range commands {
			cmd := exec.Command("bash", "-c", command)
			if err := cmd.Run(); err != nil {
				return laravelErrorMsg{fmt.Errorf("failed at step '%s': %v", descriptions[i], err)}
			}
		}

		return laravelSiteCreatedMsg{m.LaravelCreateForm.siteName}
	}
}

// Helper functions are reused from forms.go (isValidGitURL and isGitRepoAccessible)
