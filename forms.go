package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleLaravelSiteForm() (tea.Model, tea.Cmd) {
	switch m.inputField {
	case "siteName":
		if m.inputValue == "" {
			m.report = []string{warnStyle.Render("❌ Site name cannot be empty")}
			m.state = stateMenu
			return m, nil
		}
		m.formData["siteName"] = m.inputValue
		return m.startInput("Enter domain (e.g., myapp.local):", "domain", 7)

	case "domain":
		if m.inputValue == "" {
			m.report = []string{warnStyle.Render("❌ Domain cannot be empty")}
			m.state = stateMenu
			return m, nil
		}
		m.formData["domain"] = m.inputValue
		return m.startInput("Enter Git repository URL (https://github.com/user/repo.git or git@github.com:user/repo.git, optional):", "gitRepo", 7)

	case "gitRepo":
		// Validate GitHub URL if provided
		if m.inputValue != "" && !isValidGitURL(m.inputValue) {
			m.report = []string{warnStyle.Render("❌ Invalid Git repository URL. Please use format: https://github.com/user/repo.git or git@github.com:user/repo.git")}
			m.state = stateMenu
			return m, nil
		}
		m.formData["gitRepo"] = m.inputValue
		newModel, cmd := m.createLaravelSiteWithData()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	m.state = stateMenu
	return m, nil
}

func (m model) handleUpdateSiteForm() (tea.Model, tea.Cmd) {
	switch m.inputField {
	case "siteIndex":
		if m.inputValue == "" {
			m.report = []string{warnStyle.Render("❌ Site index cannot be empty")}
			m.state = stateMenu
			return m, nil
		}
		m.formData["siteIndex"] = m.inputValue
		newModel, cmd := m.updateLaravelSiteWithData()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	m.state = stateMenu
	return m, nil
}

func (m model) handleBackupForm() (tea.Model, tea.Cmd) {
	switch m.inputField {
	case "dbName":
		if m.inputValue == "" {
			m.report = []string{warnStyle.Render("❌ Database name cannot be empty")}
			m.state = stateMenu
			return m, nil
		}
		m.formData["dbName"] = m.inputValue
		return m.startInput("Enter MySQL username (default: root):", "dbUser", 8)

	case "dbUser":
		if m.inputValue == "" {
			m.inputValue = "root"
		}
		m.formData["dbUser"] = m.inputValue
		return m.startInput("Enter MySQL password:", "dbPassword", 8)

	case "dbPassword":
		if m.inputValue == "" {
			m.report = []string{warnStyle.Render("❌ Password cannot be empty")}
			m.state = stateMenu
			return m, nil
		}
		m.formData["dbPassword"] = m.inputValue
		return m.startInput("Enter remote host (e.g., user@server.com):", "remoteHost", 8)

	case "remoteHost":
		if m.inputValue == "" {
			m.report = []string{warnStyle.Render("❌ Remote host cannot be empty")}
			m.state = stateMenu
			return m, nil
		}
		m.formData["remoteHost"] = m.inputValue
		return m.startInput("Enter remote backup path (default: ~/backups/):", "remotePath", 8)

	case "remotePath":
		if m.inputValue == "" {
			m.inputValue = "~/backups/"
		}
		m.formData["remotePath"] = m.inputValue
		newModel, cmd := m.backupMySQLWithData()
		return newModel, tea.Batch(tea.ClearScreen, cmd)
	}

	m.state = stateMenu
	return m, nil
}

func (m model) createLaravelSiteWithData() (tea.Model, tea.Cmd) {
	siteName := m.formData["siteName"]
	domain := m.formData["domain"]
	gitRepo := m.formData["gitRepo"]
	sitePath := filepath.Join("/var/www", siteName)

	var commands []string
	var descriptions []string

	// 1. Create /var/www directory
	commands = append(commands, fmt.Sprintf("sudo mkdir -p %s", "/var/www"))
	descriptions = append(descriptions, "Creating base directory...")

	// 2. Create Laravel site (clone or new)
	if gitRepo != "" {
		commands = append(commands, fmt.Sprintf("git clone %s %s", gitRepo, sitePath))
		descriptions = append(descriptions, fmt.Sprintf("Cloning Laravel app from %s...", gitRepo))
	} else {
		commands = append(commands, fmt.Sprintf("composer create-project laravel/laravel %s", sitePath))
		descriptions = append(descriptions, fmt.Sprintf("Creating fresh Laravel installation at %s...", sitePath))
	}

	// 3. Set ownership and permissions
	commands = append(commands, fmt.Sprintf("sudo chown -R www-data:www-data %s", sitePath))
	descriptions = append(descriptions, "Setting ownership...")
	commands = append(commands, fmt.Sprintf("find %s -type d -exec chmod 755 {} + && find %s -type f -exec chmod 644 {} +", sitePath, sitePath))
	descriptions = append(descriptions, "Setting file permissions...")
	commands = append(commands, fmt.Sprintf("chmod -R 775 %s/storage %s/bootstrap/cache", sitePath, sitePath))
	descriptions = append(descriptions, "Setting writable permissions for storage and cache...")

	// 4. Install Composer dependencies
	commands = append(commands, fmt.Sprintf("cd %s && composer install --no-dev --optimize-autoloader", sitePath))
	descriptions = append(descriptions, "Installing Composer dependencies...")

	// 5. Set up .env and generate app key
	commands = append(commands, fmt.Sprintf("cd %s && cp .env.example .env && php artisan key:generate", sitePath))
	descriptions = append(descriptions, "Generating app key...")

	// 6. Create Caddy site configuration
	caddyConfig := fmt.Sprintf(`%s {
    import laravel-app %s
}`, domain, sitePath)
	caddyConfigPath := fmt.Sprintf("/etc/caddy/sites/%s.caddy", domain)
	// Use a 'heredoc' to safely write the multi-line config to a file
	writeConfigCmd := fmt.Sprintf("sudo mkdir -p /etc/caddy/sites && echo '%s' | sudo tee %s > /dev/null", caddyConfig, caddyConfigPath)
	commands = append(commands, writeConfigCmd)
	descriptions = append(descriptions, "Creating Caddy site configuration...")

	// 7. Reload Caddy
	commands = append(commands, "sudo systemctl reload caddy")
	descriptions = append(descriptions, "Reloading Caddy server...")

	// Start the command queue to execute all commands sequentially
	return m.startCommandQueue(commands, descriptions, "")
}

func (m model) updateLaravelSiteWithData() (tea.Model, tea.Cmd) {
	// List available sites
	sites, err := m.listLaravelSites()
	if err != nil {
		m.state = stateProcessing
		m.processingMsg = ""
		m.report = []string{warnStyle.Render(fmt.Sprintf("❌ Failed to list sites: %v", err))}
		return m, nil
	}

	if len(sites) == 0 {
		m.state = stateProcessing
		m.processingMsg = ""
		m.report = []string{infoStyle.Render("📋 No Laravel sites found in /var/www")}
		return m, nil
	}

	// Parse site selection
	siteIndex := m.formData["siteIndex"]
	var selectedSite string
	if idx := parseInt(siteIndex); idx > 0 && idx <= len(sites) {
		selectedSite = sites[idx-1]
	} else {
		m.state = stateProcessing
		m.processingMsg = ""
		m.report = []string{warnStyle.Render("❌ Invalid site selection")}
		return m, nil
	}

	sitePath := filepath.Join("/var/www", selectedSite)

	// Check if it's a Git repository
	if _, err := os.Stat(filepath.Join(sitePath, ".git")); os.IsNotExist(err) {
		m.state = stateProcessing
		m.processingMsg = ""
		m.report = []string{warnStyle.Render(fmt.Sprintf("❌ Site is not a Git repository: %s", selectedSite))}
		return m, nil
	}

	var commands []string
	var descriptions []string

	// 1. Put site in maintenance mode
	commands = append(commands, fmt.Sprintf("cd %s && php artisan down", sitePath))
	descriptions = append(descriptions, "Putting site in maintenance mode...")

	// 2. Git pull
	commands = append(commands, fmt.Sprintf("cd %s && git pull origin main", sitePath))
	descriptions = append(descriptions, "Pulling latest changes from Git...")

	// 3. Install/update dependencies
	commands = append(commands, fmt.Sprintf("cd %s && composer install --no-dev --optimize-autoloader", sitePath))
	descriptions = append(descriptions, "Updating Composer dependencies...")

	// 4. Run migrations
	commands = append(commands, fmt.Sprintf("cd %s && php artisan migrate --force", sitePath))
	descriptions = append(descriptions, "Running database migrations...")

	// 5. Clear cache
	commands = append(commands, fmt.Sprintf("cd %s && php artisan cache:clear", sitePath))
	descriptions = append(descriptions, "Clearing application cache...")

	commands = append(commands, fmt.Sprintf("cd %s && php artisan config:clear", sitePath))
	descriptions = append(descriptions, "Clearing configuration cache...")

	commands = append(commands, fmt.Sprintf("cd %s && php artisan view:clear", sitePath))
	descriptions = append(descriptions, "Clearing view cache...")

	// 6. Set permissions
	commands = append(commands, fmt.Sprintf("sudo chown -R www-data:www-data %s", sitePath))
	descriptions = append(descriptions, "Setting ownership...")
	commands = append(commands, fmt.Sprintf("find %s -type d -exec chmod 755 {} + && find %s -type f -exec chmod 644 {} +", sitePath, sitePath))
	descriptions = append(descriptions, "Setting file permissions...")
	commands = append(commands, fmt.Sprintf("chmod -R 775 %s/storage %s/bootstrap/cache", sitePath, sitePath))
	descriptions = append(descriptions, "Setting writable permissions...")

	// 7. Bring site back up
	commands = append(commands, fmt.Sprintf("cd %s && php artisan up", sitePath))
	descriptions = append(descriptions, "Bringing site back online...")

	// Start the command queue to execute all commands sequentially
	return m.startCommandQueue(commands, descriptions, "")
}

func (m model) backupMySQLWithData() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.report = []string{infoStyle.Render("Starting MySQL backup")}

	dbName := m.formData["dbName"]
	dbUser := m.formData["dbUser"]
	dbPassword := m.formData["dbPassword"]
	remoteHost := m.formData["remoteHost"]
	remotePath := m.formData["remotePath"]

	// Create timestamp for backup filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFileName := fmt.Sprintf("%s_backup_%s.sql", dbName, timestamp)
	localBackupPath := filepath.Join("/tmp", backupFileName)

	// Create MySQL dump
	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Creating MySQL dump: %s to %s", dbName, localBackupPath)))

	cmd := exec.Command("mysqldump",
		"-u", dbUser,
		fmt.Sprintf("-p%s", dbPassword),
		"--single-transaction",
		"--routines",
		"--triggers",
		dbName,
	)

	output, err := cmd.Output()
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to create MySQL dump: %v", err)))
		return m, nil
	}

	// Write dump to file
	err = os.WriteFile(localBackupPath, output, 0600)
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to write backup file: %v", err)))
		return m, nil
	}

	// Compress the backup
	compressedPath := localBackupPath + ".gz"
	cmd = exec.Command("gzip", localBackupPath)
	if err := cmd.Run(); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to compress backup: %v", err)))
		return m, nil
	}

	// Transfer to remote host via SCP
	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Transferring backup to %s:%s", remoteHost, remotePath)))

	cmd = exec.Command("scp", compressedPath, fmt.Sprintf("%s:%s", remoteHost, remotePath))
	if output, err := cmd.CombinedOutput(); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to transfer backup: %v\nOutput: %s", err, string(output))))
		return m, nil
	}

	// Clean up local backup
	os.Remove(compressedPath)

	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("MySQL backup completed successfully: %s (file: %s.gz, host: %s, path: %s)", dbName, backupFileName, remoteHost, remotePath)))
	return m, nil
}

func (m model) setLaravelPermissions(sitePath string) {
	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Setting Laravel permissions for %s", sitePath)))

	// Set ownership
	cmd := exec.Command("sudo", "chown", "-R", "www-data:www-data", sitePath)
	cmd.Run()

	// Set directory permissions
	cmd = exec.Command("find", sitePath, "-type", "d", "-exec", "chmod", "755", "{}", "+")
	cmd.Run()

	// Set file permissions
	cmd = exec.Command("find", sitePath, "-type", "f", "-exec", "chmod", "644", "{}", "+")
	cmd.Run()

	// Set writable permissions for storage and cache
	storagePath := filepath.Join(sitePath, "storage")
	if _, err := os.Stat(storagePath); err == nil {
		cmd = exec.Command("chmod", "-R", "775", storagePath)
		cmd.Run()
	}

	cachePath := filepath.Join(sitePath, "bootstrap", "cache")
	if _, err := os.Stat(cachePath); err == nil {
		cmd = exec.Command("chmod", "-R", "775", cachePath)
		cmd.Run()
	}
}

func (m model) createCaddySiteConfig(domain, sitePath string) {
	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Creating Caddy site configuration for %s at %s", domain, sitePath)))

	configPath := fmt.Sprintf("/etc/caddy/sites/%s.caddy", domain)

	// Create sites directory
	os.MkdirAll("/etc/caddy/sites", 0755)

	config := fmt.Sprintf(`%s {
	import laravel-app %s
}`, domain, sitePath)

	err := os.WriteFile(configPath, []byte(config), 0644)
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to create Caddy site configuration: %v", err)))
		return
	}

	// Update main Caddyfile to import sites
	caddyfilePath := "/etc/caddy/Caddyfile"
	content, err := os.ReadFile(caddyfilePath)
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to read Caddyfile: %v", err)))
		return
	}

	if !strings.Contains(string(content), "import sites/*") {
		newContent := "import sites/*\n" + string(content)
		err = os.WriteFile(caddyfilePath, []byte(newContent), 0644)
		if err != nil {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to update Caddyfile: %v", err)))
		}
	}

	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Caddy site configuration created at %s", configPath)))
}

func (m model) listLaravelSites() ([]string, error) {
	entries, err := os.ReadDir("/var/www")
	if err != nil {
		return nil, err
	}

	var sites []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it looks like a Laravel site
			sitePath := filepath.Join("/var/www", entry.Name())
			if m.isLaravelSite(sitePath) {
				sites = append(sites, entry.Name())
			}
		}
	}

	return sites, nil
}

func (m model) isLaravelSite(path string) bool {
	// Check for Laravel indicators
	indicators := []string{"artisan", "composer.json", "app/Http/Kernel.php"}

	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			return true
		}
	}

	return false
}

func parseInt(s string) int {
	if s == "1" {
		return 1
	}
	if s == "2" {
		return 2
	}
	if s == "3" {
		return 3
	}
	if s == "4" {
		return 4
	}
	if s == "5" {
		return 5
	}
	if s == "6" {
		return 6
	}
	if s == "7" {
		return 7
	}
	if s == "8" {
		return 8
	}
	if s == "9" {
		return 9
	}
	return 0
}

// isValidGitURL validates common Git repository URL formats
func isValidGitURL(url string) bool {
	// HTTPS format: https://github.com/user/repo.git
	httpsPattern := `^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+\.git$`

	// SSH format: git@github.com:user/repo.git
	sshPattern := `^git@github\.com:[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+\.git$`

	// HTTPS without .git suffix: https://github.com/user/repo
	httpsNoGitPattern := `^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+$`

	httpsRegex := regexp.MustCompile(httpsPattern)
	sshRegex := regexp.MustCompile(sshPattern)
	httpsNoGitRegex := regexp.MustCompile(httpsNoGitPattern)

	return httpsRegex.MatchString(url) || sshRegex.MatchString(url) || httpsNoGitRegex.MatchString(url)
}
