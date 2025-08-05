package models

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"crucible/internal/actions"
	"crucible/internal/logging"
	"crucible/internal/services"
)

// ProcessingModel handles command execution and result display
type ProcessingModel struct {
	BaseModel
	message      string
	report       []string
	scrollPos    int
	isComplete   bool
	canNavigate  bool
}

// NewProcessingModel creates a new processing model
func NewProcessingModel(shared *SharedData) *ProcessingModel {
	return &ProcessingModel{
		BaseModel:   NewBaseModel(shared),
		message:     "",
		report:      []string{},
		scrollPos:   0,
		isComplete:  false,
		canNavigate: true,
	}
}

// Init initializes the processing model
func (m *ProcessingModel) Init() tea.Cmd {
	// Start processing if there are commands in the queue
	if m.shared.CommandQueue != nil && m.shared.CommandQueue.HasNext() {
		return m.startProcessing()
	}
	return nil
}

// Update handles processing updates
func (m *ProcessingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.isComplete {
				return m, tea.Quit
			}
			// Don't quit while processing
			return m, nil

		case "esc", "enter", " ":
			if m.isComplete && m.canNavigate {
				return m, m.GoBack()
			}
			return m, nil

		// Scrolling controls
		case "up", "k":
			if m.scrollPos > 0 {
				m.scrollPos--
			}
		case "down", "j":
			viewableLines := m.shared.GetViewableLines()
			maxScroll := len(m.report) - viewableLines
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
			maxScroll := len(m.report) - viewableLines
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
			maxScroll := len(m.report) - viewableLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollPos = maxScroll
		}

	case CmdCompletedMsg:
		// Handle command completion
		m.handleCommandCompleted(msg)
		return m, nil
	}

	return m, nil
}

// View renders the processing interface
func (m *ProcessingModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Render("🔄 Processing"))
	s.WriteString("\n\n")

	// Current processing message
	if m.message != "" && !m.isComplete {
		s.WriteString(infoStyle.Render(fmt.Sprintf("⏳ %s", m.message)))
		s.WriteString("\n\n")
	}

	// Report content (with scrolling)
	if len(m.report) > 0 {
		startLine := m.scrollPos
		viewableLines := m.shared.GetViewableLines()
		endLine := startLine + viewableLines
		if endLine > len(m.report) {
			endLine = len(m.report)
		}

		for i := startLine; i < endLine; i++ {
			s.WriteString(m.report[i])
			s.WriteString("\n")
		}

		// Scroll indicator
		if len(m.report) > viewableLines {
			totalLines := len(m.report)
			s.WriteString("\n")
			scrollInfo := fmt.Sprintf("Showing lines %d-%d of %d", 
				startLine+1, endLine, totalLines)
			s.WriteString(helpStyle.Render(scrollInfo))
		}
	}

	// Help text
	s.WriteString("\n")
	if m.isComplete {
		if m.canNavigate {
			s.WriteString(helpStyle.Render("Press Enter, Space, or Esc to continue"))
		} else {
			s.WriteString(helpStyle.Render("Process completed"))
		}
	} else {
		s.WriteString(helpStyle.Render("Processing... Press Ctrl+C to quit"))
	}

	// Scrolling help
	if len(m.report) > m.shared.GetViewableLines() {
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("↑/↓=Scroll, PageUp/PageDown=Fast scroll, Home/End=Jump"))
	}

	return s.String()
}

// handleCommandCompleted processes command completion
func (m *ProcessingModel) handleCommandCompleted(msg CmdCompletedMsg) {
	queue := m.shared.CommandQueue

	if msg.Result.Error != nil {
		// Command failed
		queue.Reset()
		m.report = append(m.report, errorStyle.Render(fmt.Sprintf("❌ Failed: %v", msg.Result.Error)))
		if strings.TrimSpace(msg.Result.Output) != "" {
			m.report = append(m.report, fmt.Sprintf("Output: %s", msg.Result.Output))
		}
		m.message = ""
		m.isComplete = true
		return
	}

	// Command succeeded
	m.report = append(m.report, infoStyle.Render("✅ Command completed successfully"))
	if strings.TrimSpace(msg.Result.Output) != "" {
		m.report = append(m.report, msg.Result.Output)
	}

	// Check if there are more commands in queue
	if queue.HasNext() {
		// Execute next command
		command, description, ok := queue.Next()
		if ok {
			m.message = description
			// Execute the command asynchronously
			go m.executeCommand(command, msg.ServiceName)
		}
	} else {
		// All commands completed
		queue.Reset()
		m.message = ""
		m.isComplete = true
		m.report = append(m.report, infoStyle.Render("✅ All operations completed successfully"))
	}
}

// SetMessage sets the current processing message
func (m *ProcessingModel) SetMessage(message string) {
	m.message = message
	m.isComplete = false
}

// AddReportLine adds a line to the report
func (m *ProcessingModel) AddReportLine(line string) {
	m.report = append(m.report, line)
}

// SetReport sets the entire report
func (m *ProcessingModel) SetReport(report []string) {
	m.report = report
}

// SetComplete marks processing as complete
func (m *ProcessingModel) SetComplete(canNavigate bool) {
	m.isComplete = true
	m.canNavigate = canNavigate
	m.message = ""
}

// Initialize implements the ModelInitializer interface
func (m *ProcessingModel) Initialize(data interface{}) {
	if initData, ok := data.(map[string]interface{}); ok {
		// Handle specific actions
		if action, exists := initData["action"].(string); exists {
			m.handleAction(action, initData)
			return
		}
		
		// Handle legacy initialization data
		if message, exists := initData["message"].(string); exists {
			m.SetMessage(message)
		}
		if report, exists := initData["report"].([]string); exists {
			m.SetReport(report)
		}
		if complete, exists := initData["complete"].(bool); exists {
			m.SetComplete(complete)
		}
	}
}

// handleAction handles different processing actions
func (m *ProcessingModel) handleAction(action string, data map[string]interface{}) {
	switch action {
	case "install":
		if serviceKey, ok := data["service"].(string); ok {
			m.handleServiceInstallation(serviceKey, data)
		} else {
			m.SetMessage("No service specified for installation")
			m.SetComplete(true)
		}
	case "system-status":
		m.handleSystemStatus()
	case "laravel-list":
		m.handleLaravelList()
	case "laravel-queue":
		m.handleLaravelQueue()
	case "github-auth":
		m.handleGitHubAuth()
	case "service-status":
		if serviceName, ok := data["service"].(string); ok {
			m.handleServiceStatus(serviceName)
		}
	case "service-control":
		if serviceName, ok := data["service"].(string); ok {
			if serviceAction, ok := data["serviceAction"].(string); ok {
				m.handleServiceControl(serviceName, serviceAction)
			}
		}
	default:
		m.SetMessage(fmt.Sprintf("Unknown action: %s", action))
		m.SetComplete(true)
	}
}

// handleServiceInstallation handles the installation of a specific service
func (m *ProcessingModel) handleServiceInstallation(serviceKey string, data map[string]interface{}) {
	labelInterface := data["label"]
	label := fmt.Sprintf("Installing %s", serviceKey)
	if labelInterface != nil {
		if labelStr, ok := labelInterface.(string); ok {
			label = labelStr
		}
	}
	
	m.message = fmt.Sprintf("Starting %s installation...", serviceKey)
	m.isComplete = false
	
	// Get installation commands based on service key
	var commands []string
	var descriptions []string
	var err error
	
	switch serviceKey {
	case "php":
		commands, descriptions, err = services.InstallPHP()
	case "composer":
		commands, descriptions, err = services.InstallComposer()
	case "python":
		commands, descriptions, err = services.InstallPython()
	case "nodejs":
		// Install Node.js with PM2 option - ask the user during processing
		m.SetReport([]string{
			infoStyle.Render("📦 Installing Node.js and npm..."),
			"",
			helpStyle.Render("Note: PM2 process manager will also be installed for production deployments"),
		})
		commands, descriptions, err = services.InstallNodeWithPM2(true)
	case "mysql":
		// For now, install MySQL without password (interactive mode)
		commands, descriptions, err = services.InstallMySQL("")
	case "caddy":
		commands, descriptions, err = services.InstallCaddy()
	case "supervisor":
		commands, descriptions, err = services.InstallSupervisor()
	case "git":
		commands, descriptions, err = services.InstallGit()
	default:
		m.SetReport([]string{
			errorStyle.Render(fmt.Sprintf("❌ Unknown service: %s", serviceKey)),
			"",
			helpStyle.Render("Supported services: php, composer, python, nodejs, mysql, caddy, supervisor, git"),
		})
		m.SetComplete(true)
		return
	}
	
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render(fmt.Sprintf("❌ Error getting installation commands for %s:", serviceKey)),
			err.Error(),
		})
		m.SetComplete(true)
		return
	}
	
	if len(commands) == 0 {
		m.SetReport([]string{
			errorStyle.Render(fmt.Sprintf("❌ No installation commands available for %s", serviceKey)),
		})
		m.SetComplete(true)
		return
	}
	
	// Initialize command queue
	if m.shared.CommandQueue == nil {
		m.shared.CommandQueue = NewCommandQueue()
	}
	
	// Reset and populate command queue
	m.shared.CommandQueue.Reset()
	m.shared.CommandQueue.ServiceName = serviceKey
	
	// Add commands to queue
	for i, command := range commands {
		description := fmt.Sprintf("Step %d/%d: Installing %s", i+1, len(commands), serviceKey)
		if i < len(descriptions) {
			description = descriptions[i]
		}
		m.shared.CommandQueue.AddCommand(command, description)
	}
	
	// Set initial report
	report := []string{
		titleStyle.Render(fmt.Sprintf("🔧 %s", label)),
		"",
		infoStyle.Render(fmt.Sprintf("Starting installation of %s...", serviceKey)),
		"",
	}
	
	// Show installation steps
	for i, desc := range descriptions {
		report = append(report, fmt.Sprintf("%d. %s", i+1, desc))
	}
	
	report = append(report, "", infoStyle.Render("Starting installation..."))
	m.SetReport(report)
	
	// Start processing the queue
	if m.shared.CommandQueue.HasNext() {
		command, description, ok := m.shared.CommandQueue.Next()
		if ok {
			m.message = description
			// Execute the command asynchronously
			go m.executeCommand(command, serviceKey)
		}
	}
}

// executeCommand executes a shell command asynchronously
func (m *ProcessingModel) executeCommand(command, serviceName string) {
	startTime := time.Now()
	
	// Parse the command to separate command and arguments
	parts := strings.Fields(command)
	if len(parts) == 0 {
		// Empty command, send completion message with error
		result := logging.LoggedExecResult{
			Command:   command,
			Output:    "",
			Error:     fmt.Errorf("empty command"),
			ExitCode:  1,
			StartTime: startTime,
		}
		
		// Handle the error result
		completedMsg := CmdCompletedMsg{
			Result:      result,
			ServiceName: serviceName,
		}
		m.handleCommandCompleted(completedMsg)
		return
	}
	
	var cmd *exec.Cmd
	if len(parts) == 1 {
		cmd = exec.Command(parts[0])
	} else {
		cmd = exec.Command(parts[0], parts[1:]...)
	}
	
	// Execute the command
	output, err := cmd.CombinedOutput()
	
	// Get exit code safely
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1 // Default error code
		}
	} else if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	
	// Create the result
	result := logging.LoggedExecResult{
		Command:   command,
		Output:    string(output),
		Error:     err,
		ExitCode:  exitCode,
		StartTime: startTime,
	}
	
	// Log the command execution
	if m.shared.Logger != nil {
		m.shared.Logger.LogCommand(result)
	}
	
	// Send completion message - this is the tricky part
	// We need to send this back to the Bubble Tea program
	// In a real implementation, we'd use a channel or other mechanism
	// For now, we'll simulate this by directly calling the handler
	// This is a limitation of the current architecture
	completedMsg := CmdCompletedMsg{
		Result:      result,
		ServiceName: serviceName,
	}
	
	// In a proper implementation, we'd send this through a channel
	// that the main Update loop monitors, but for now we'll handle it directly
	// This is a limitation of the current architecture
	m.handleCommandCompleted(completedMsg)
}

// startProcessing begins processing the command queue
func (m *ProcessingModel) startProcessing() tea.Cmd {
	queue := m.shared.CommandQueue
	if queue == nil || !queue.HasNext() {
		return nil
	}
	
	// Mark as processing
	queue.IsProcessing = true
	m.isComplete = false
	
	// Get the first command
	command, description, ok := queue.Next()
	if !ok {
		return nil
	}
	
	// Set the current message
	m.message = description
	
	// Start executing the command asynchronously
	go m.executeCommand(command, queue.ServiceName)
	
	return nil
}

// handleSystemStatus displays system status information
func (m *ProcessingModel) handleSystemStatus() {
	m.message = "Generating system status report..."
	m.isComplete = false
	
	// Create a comprehensive system status report
	report := []string{
		titleStyle.Render("=== SYSTEM STATUS REPORT ==="),
		"",
		infoStyle.Render("📊 System Information:"),
	}
	
	// Get system information
	if output, err := m.executeCommandSync("uname", "-a"); err == nil {
		report = append(report, fmt.Sprintf("System: %s", strings.TrimSpace(output)))
	}
	
	if output, err := m.executeCommandSync("uptime"); err == nil {
		report = append(report, fmt.Sprintf("Uptime: %s", strings.TrimSpace(output)))
	}
	
	if output, err := m.executeCommandSync("df", "-h", "/"); err == nil {
		lines := strings.Split(output, "\n")
		if len(lines) > 1 {
			report = append(report, fmt.Sprintf("Disk Usage: %s", strings.TrimSpace(lines[1])))
		}
	}
	
	if output, err := m.executeCommandSync("free", "-h"); err == nil {
		lines := strings.Split(output, "\n")
		if len(lines) > 1 {
			report = append(report, fmt.Sprintf("Memory: %s", strings.TrimSpace(lines[1])))
		}
	}
	
	report = append(report, "", infoStyle.Render("🔧 Service Status:"))
	
	// Check common services
	services := []string{"mysql", "mysqld", "caddy", "nginx", "php-fpm", "supervisor"}
	for _, service := range services {
		if output, err := m.executeCommandSync("systemctl", "is-active", service); err == nil {
			status := strings.TrimSpace(output)
			icon := "🔴"
			if status == "active" {
				icon = "🟢"
			}
			report = append(report, fmt.Sprintf("%s %s: %s", icon, service, status))
		}
	}
	
	report = append(report, "", infoStyle.Render("💻 Software Versions:"))
	
	// Check software versions
	softwareChecks := map[string][]string{
		"PHP":     {"php", "--version"},
		"Node.js": {"node", "--version"},
		"Git":     {"git", "--version"},
		"Composer": {"composer", "--version"},
		"Python":  {"python3", "--version"},
	}
	
	for name, cmd := range softwareChecks {
		if output, err := m.executeCommandSync(cmd[0], cmd[1:]...); err == nil {
			lines := strings.Split(output, "\n")
			if len(lines) > 0 {
				version := strings.TrimSpace(lines[0])
				if len(version) > 60 {
					version = version[:60] + "..."
				}
				report = append(report, fmt.Sprintf("✅ %s: %s", name, version))
			}
		} else {
			report = append(report, fmt.Sprintf("❌ %s: Not installed", name))
		}
	}
	
	m.SetReport(report)
	m.SetComplete(true)
}

// handleLaravelList shows Laravel sites for update selection
func (m *ProcessingModel) handleLaravelList() {
	m.message = "Loading Laravel sites..."
	m.isComplete = false
	
	// Import the actions package functionality
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render("❌ Error loading Laravel sites:"),
			err.Error(),
		})
		m.SetComplete(true)
		return
	}
	
	if len(sites) == 0 {
		m.SetReport([]string{
			warnStyle.Render("⚠️ No Laravel sites found"),
			"",
			helpStyle.Render("Create a Laravel site first from the Laravel Management menu."),
		})
		m.SetComplete(true)
		return
	}
	
	report := []string{
		titleStyle.Render("🚀 Laravel Sites Available for Update"),
		"",
		infoStyle.Render("Available Laravel installations:"),
		"",
	}
	
	for i, site := range sites {
		statusIcon := "🔴"
		if isRunning, err := actions.GetLaravelSiteStatus(site); err == nil && isRunning {
			statusIcon = "🟢"
		}
		
		// Get Git repository status if available
		gitStatus := ""
		if repoStatus, err := actions.GetLaravelRepositoryStatus(site); err == nil {
			gitStatus = fmt.Sprintf(" (Branch: %s)", repoStatus.CurrentBranch)
			if repoStatus.HasUncommittedChanges {
				gitStatus += " [Modified]"
			}
		}
		
		report = append(report, fmt.Sprintf("%s [%d] %s%s (/var/www/%s)", statusIcon, i+1, site, gitStatus, site))
	}
	
	report = append(report,
		"",
		infoStyle.Render("Starting update process for all sites..."),
		"",
		helpStyle.Render("Note: Only Git repositories will be updated."),
		helpStyle.Render("Sites will be put in maintenance mode during updates."),
		"",
	)
	
	// Initialize command queue for updating all sites
	if m.shared.CommandQueue == nil {
		m.shared.CommandQueue = NewCommandQueue()
	}
	
	// Reset and populate command queue
	m.shared.CommandQueue.Reset()
	m.shared.CommandQueue.ServiceName = "Laravel Site Updates"
	
	// Process each site
	updatedSites := 0
	for i, site := range sites {
		// Try to update this site
		updateConfig := actions.UpdateSiteConfig{
			SiteIndex: fmt.Sprintf("%d", i+1),
			Sites:     sites,
		}
		
		commands, descriptions, err := actions.UpdateLaravelSite(updateConfig)
		if err != nil {
			// Add error to report but continue with other sites
			report = append(report, errorStyle.Render(fmt.Sprintf("❌ Cannot update %s: %v", site, err)))
			continue
		}
		
		// Add separator between sites
		if updatedSites > 0 {
			m.shared.CommandQueue.AddCommand("echo \"\"", fmt.Sprintf("--- Updating %s ---", site))
		}
		
		// Add commands for this site
		for j, cmd := range commands {
			desc := fmt.Sprintf("Step %d: Updating %s", j+1, site)
			if j < len(descriptions) {
				desc = descriptions[j]
			}
			m.shared.CommandQueue.AddCommand(cmd, desc)
		}
		
		updatedSites++
	}
	
	if updatedSites == 0 {
		report = append(report,
			warnStyle.Render("⚠️ No sites can be updated"),
			"",
			helpStyle.Render("Make sure your Laravel sites are Git repositories."),
		)
		m.SetReport(report)
		m.SetComplete(true)
		return
	}
	
	report = append(report, 
		infoStyle.Render(fmt.Sprintf("Updating %d Laravel sites...", updatedSites)),
		"",
		helpStyle.Render("Update process:"),
		"• Put sites in maintenance mode",
		"• Pull latest changes from Git",
		"• Update Composer dependencies",
		"• Run database migrations",
		"• Clear caches",
		"• Set proper permissions",
		"• Bring sites back online",
		"",
		infoStyle.Render("Starting updates..."),
	)
	
	m.SetReport(report)
	
	// Start processing the queue
	if m.shared.CommandQueue.HasNext() {
		command, description, ok := m.shared.CommandQueue.Next()
		if ok {
			m.message = description
			go m.executeCommand(command, "Laravel Site Updates")
		}
	}
}

// handleLaravelQueue shows Laravel queue worker setup
func (m *ProcessingModel) handleLaravelQueue() {
	m.message = "Setting up Laravel Queue Worker..."
	m.isComplete = false
	
	// Get available Laravel sites
	sites, err := actions.ListLaravelSites()
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render("❌ Error loading Laravel sites:"),
			err.Error(),
		})
		m.SetComplete(true)
		return
	}
	
	if len(sites) == 0 {
		m.SetReport([]string{
			warnStyle.Render("⚠️ No Laravel sites found"),
			"",
			helpStyle.Render("Create a Laravel site first from the Laravel Management menu."),
		})
		m.SetComplete(true)
		return
	}
	
	// For now, we'll set up queue workers for all Laravel sites
	report := []string{
		titleStyle.Render("🔄 Laravel Queue Worker Setup"),
		"",
		infoStyle.Render("Available Laravel sites:"),
		"",
	}
	
	for i, site := range sites {
		report = append(report, fmt.Sprintf("%d. %s (/var/www/%s)", i+1, site, site))
	}
	
	report = append(report, 
		"",
		infoStyle.Render("Setting up Supervisor configuration for Laravel queue workers..."),
		"",
	)
	
	// Initialize command queue for queue worker setup
	if m.shared.CommandQueue == nil {
		m.shared.CommandQueue = NewCommandQueue()
	}
	
	// Reset and populate command queue
	m.shared.CommandQueue.Reset()
	m.shared.CommandQueue.ServiceName = "Laravel Queue Workers"
	
	// Add commands for each site
	for _, site := range sites {
		// Create supervisor config for this site
		supervisorConfigPath := fmt.Sprintf("/etc/supervisor/conf.d/laravel-queue-%s.conf", site)
		
		// Commands to set up the queue worker
		commands := []string{
			// Use a heredoc to properly handle multiline config
			fmt.Sprintf(`sudo bash -c 'cat > %s << "EOF"
[program:laravel-queue-%s]
process_name=%%(program_name)s_%%(process_num)02d
command=php /var/www/%s/artisan queue:work --sleep=3 --tries=3 --max-time=3600
autostart=true
autorestart=true
user=www-data
numprocs=2
redirect_stderr=true
stdout_logfile=/var/log/supervisor/laravel-queue-%s.log
stopwaitsecs=3600
EOF'`, supervisorConfigPath, site, site, site),
			"sudo supervisorctl reread",
			"sudo supervisorctl update",
			fmt.Sprintf("sudo supervisorctl start laravel-queue-%s:*", site),
		}
		
		descriptions := []string{
			fmt.Sprintf("Creating Supervisor config for %s queue worker...", site),
			"Reloading Supervisor configuration...",
			"Updating Supervisor programs...",
			fmt.Sprintf("Starting %s queue workers...", site),
		}
		
		for i, cmd := range commands {
			desc := fmt.Sprintf("Step %d: Setting up queue worker for %s", i+1, site)
			if i < len(descriptions) {
				desc = descriptions[i]
			}
			m.shared.CommandQueue.AddCommand(cmd, desc)
		}
	}
	
	// Add final status check
	m.shared.CommandQueue.AddCommand("sudo supervisorctl status", "Checking queue worker status...")
	
	report = append(report, 
		fmt.Sprintf("Setting up queue workers for %d Laravel sites...", len(sites)),
		"",
		infoStyle.Render("Steps:"),
		"1. Create Supervisor configuration files",
		"2. Reload Supervisor configuration", 
		"3. Start queue worker processes",
		"4. Verify status",
		"",
		infoStyle.Render("Starting setup..."),
	)
	
	m.SetReport(report)
	
	// Start processing the queue
	if m.shared.CommandQueue.HasNext() {
		command, description, ok := m.shared.CommandQueue.Next()
		if ok {
			m.message = description
			go m.executeCommand(command, "Laravel Queue Workers")
		}
	}
}

// handleGitHubAuth shows GitHub authentication setup
func (m *ProcessingModel) handleGitHubAuth() {
	m.message = "Setting up GitHub Authentication..."
	m.isComplete = false
	
	// Check if SSH key already exists
	homeDir, err := os.UserHomeDir()
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render("❌ Error getting home directory:"),
			err.Error(),
		})
		m.SetComplete(true)
		return
	}
	
	pubKeyPath := fmt.Sprintf("%s/.ssh/id_ed25519.pub", homeDir)
	if _, err := os.Stat(pubKeyPath); err == nil {
		// SSH key exists, show it
		m.showExistingSSHKey(pubKeyPath)
		return
	}
	
	// SSH key doesn't exist, generate one
	m.generateDefaultSSHKey(homeDir)
}

// showExistingSSHKey displays the existing SSH public key
func (m *ProcessingModel) showExistingSSHKey(pubKeyPath string) {
	content, err := os.ReadFile(pubKeyPath)
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render("❌ Error reading SSH key:"),
			err.Error(),
		})
		m.SetComplete(true)
		return
	}
	
	report := []string{
		titleStyle.Render("🔑 GitHub SSH Key Found"),
		"",
		infoStyle.Render("Your existing SSH public key:"),
		"",
		choiceStyle.Render(string(content)),
		"",
		infoStyle.Render("📋 Instructions to add this key to GitHub:"),
		"1. Copy the key above (select and Ctrl+C)",
		"2. Go to GitHub.com → Settings → SSH and GPG keys",
		"3. Click 'New SSH key'",
		"4. Paste your key and give it a title",
		"5. Click 'Add SSH key'",
		"",
		infoStyle.Render("🧪 Test your connection with:"),
		choiceStyle.Render("ssh -T git@github.com"),
		"",
		warnStyle.Render("Note: You may see a warning about authenticity - type 'yes' to continue"),
		"",
		infoStyle.Render("💡 Want to test the connection automatically? Choose 'GitHub Authentication' again"),
	}
	
	// Test the connection automatically
	testCommands := []string{"timeout 10 ssh -o ConnectTimeout=5 -o BatchMode=yes -T git@github.com"}
	testDescriptions := []string{"Testing GitHub SSH connection..."}
	
	// Initialize command queue
	if m.shared.CommandQueue == nil {
		m.shared.CommandQueue = NewCommandQueue()
	}
	
	// Reset and populate command queue
	m.shared.CommandQueue.Reset()
	m.shared.CommandQueue.ServiceName = "GitHub SSH Test"
	
	for i, cmd := range testCommands {
		desc := "Testing connection..."
		if i < len(testDescriptions) {
			desc = testDescriptions[i]
		}
		m.shared.CommandQueue.AddCommand(cmd, desc)
	}
	
	report = append(report, 
		"",
		infoStyle.Render("Testing SSH connection..."),
	)
	
	m.SetReport(report)
	
	// Start processing the queue
	if m.shared.CommandQueue.HasNext() {
		command, description, ok := m.shared.CommandQueue.Next()
		if ok {
			m.message = description
			go m.executeCommand(command, "GitHub SSH Test")
		}
	}
}

// generateDefaultSSHKey creates a new SSH key for GitHub with default email
func (m *ProcessingModel) generateDefaultSSHKey(homeDir string) {
	// Use a default email or system user
	defaultEmail := "user@example.com" // Could be improved to get system user
	if gitEmail, err := m.executeCommandSync("git", "config", "--global", "user.email"); err == nil && strings.TrimSpace(gitEmail) != "" {
		defaultEmail = strings.TrimSpace(gitEmail)
	}
	
	sshDir := fmt.Sprintf("%s/.ssh", homeDir)
	
	var commands []string
	var descriptions []string
	
	// Create .ssh directory if it doesn't exist
	commands = append(commands, fmt.Sprintf("mkdir -p %s", sshDir))
	descriptions = append(descriptions, "Creating SSH directory...")
	
	// Remove existing key files first to avoid prompts
	commands = append(commands, fmt.Sprintf("rm -f %s/id_ed25519 %s/id_ed25519.pub", sshDir, sshDir))
	descriptions = append(descriptions, "Removing existing SSH keys...")
	
	// Generate SSH key without passphrase for automation
	keygenCmd := fmt.Sprintf("ssh-keygen -t ed25519 -C \"%s\" -f %s/id_ed25519 -N \"\"", defaultEmail, sshDir)
	commands = append(commands, keygenCmd)
	descriptions = append(descriptions, "Generating SSH key...")
	
	// Set proper permissions
	commands = append(commands, fmt.Sprintf("chmod 600 %s/id_ed25519", sshDir))
	descriptions = append(descriptions, "Setting private key permissions...")
	commands = append(commands, fmt.Sprintf("chmod 644 %s/id_ed25519.pub", sshDir))
	descriptions = append(descriptions, "Setting public key permissions...")
	
	// Initialize command queue
	if m.shared.CommandQueue == nil {
		m.shared.CommandQueue = NewCommandQueue()
	}
	
	// Reset and populate command queue
	m.shared.CommandQueue.Reset()
	m.shared.CommandQueue.ServiceName = "GitHub SSH Key Generation"
	
	for i, cmd := range commands {
		desc := fmt.Sprintf("Step %d: Setting up SSH key", i+1)
		if i < len(descriptions) {
			desc = descriptions[i]
		}
		m.shared.CommandQueue.AddCommand(cmd, desc)
	}
	
	report := []string{
		titleStyle.Render("🔐 GitHub Authentication Setup"),
		"",
		infoStyle.Render("No SSH key found. Generating a new one..."),
		"",
		fmt.Sprintf("Using email: %s", defaultEmail),
		"",
		infoStyle.Render("Steps:"),
		"1. Create SSH directory",
		"2. Generate ED25519 SSH key",
		"3. Set proper file permissions",
		"",
		infoStyle.Render("Starting SSH key generation..."),
	}
	
	m.SetReport(report)
	
	// Start processing the queue
	if m.shared.CommandQueue.HasNext() {
		command, description, ok := m.shared.CommandQueue.Next()
		if ok {
			m.message = description
			go m.executeCommand(command, "GitHub SSH Key Generation")
		}
	}
}

// executeCommandSync executes a command synchronously and returns output
func (m *ProcessingModel) executeCommandSync(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.Output()
	return string(output), err
}

// handleServiceStatus displays detailed status for a service
func (m *ProcessingModel) handleServiceStatus(serviceName string) {
	m.message = fmt.Sprintf("Getting detailed status for %s...", serviceName)
	m.isComplete = false
	
	commands, _ := actions.GetServiceStatus(serviceName)
	if len(commands) == 0 {
		m.SetReport([]string{
			errorStyle.Render("❌ No status command available"),
		})
		m.SetComplete(true)
		return
	}
	
	report := []string{
		titleStyle.Render(fmt.Sprintf("🔧 Service Status: %s", serviceName)),
		"",
	}
	
	// Execute the status command
	if output, err := m.executeCommandSync("systemctl", "status", serviceName, "--no-pager", "--lines=15"); err == nil {
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				report = append(report, line)
			}
		}
	} else {
		report = append(report, errorStyle.Render(fmt.Sprintf("❌ Error getting status: %v", err)))
	}
	
	m.SetReport(report)
	m.SetComplete(true)
}

// handleServiceControl executes a control action on a service
func (m *ProcessingModel) handleServiceControl(serviceName, serviceAction string) {
	m.message = fmt.Sprintf("Performing %s on %s...", serviceAction, serviceName)
	m.isComplete = false
	
	config := actions.ServiceActionConfig{
		ServiceName: serviceName,
		Action:      serviceAction,
	}
	
	commands, descriptions, err := actions.ControlService(config)
	if err != nil {
		m.SetReport([]string{
			errorStyle.Render(fmt.Sprintf("❌ Invalid action: %v", err)),
		})
		m.SetComplete(true)
		return
	}
	
	report := []string{
		titleStyle.Render(fmt.Sprintf("🔧 Service Control: %s %s", serviceAction, serviceName)),
		"",
	}
	
	// Execute each command
	for i, command := range commands {
		if i < len(descriptions) {
			report = append(report, infoStyle.Render(descriptions[i]))
		}
		
		// Parse command into parts
		parts := strings.Fields(command)
		if len(parts) == 0 {
			continue
		}
		
		var cmd *exec.Cmd
		if len(parts) == 1 {
			cmd = exec.Command(parts[0])
		} else {
			cmd = exec.Command(parts[0], parts[1:]...)
		}
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			report = append(report, errorStyle.Render(fmt.Sprintf("❌ Command failed: %v", err)))
			if len(output) > 0 {
				report = append(report, fmt.Sprintf("Output: %s", string(output)))
			}
		} else {
			report = append(report, infoStyle.Render("✅ Command completed successfully"))
			if len(output) > 0 && strings.TrimSpace(string(output)) != "" {
				report = append(report, fmt.Sprintf("Output: %s", string(output)))
			}
		}
		report = append(report, "")
	}
	
	m.SetReport(report)
	m.SetComplete(true)
}