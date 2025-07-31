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
	m.state = stateProcessing
	m.processingMsg = "Creating new Laravel site..."
	m.report = []string{infoStyle.Render("Creating new Laravel site")}

	siteName := m.formData["siteName"]
	domain := m.formData["domain"]
	gitRepo := m.formData["gitRepo"]

	// Create /var/www if it doesn't exist
	if err := os.MkdirAll("/var/www", 0755); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to create /var/www directory: %v", err)))
		return m, nil
	}

	sitePath := filepath.Join("/var/www", siteName)

	// Create Laravel site
	if gitRepo != "" {
		// Clone from Git
		m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Cloning Laravel app from Git: %s to %s", gitRepo, sitePath)))
		cmd := exec.Command("git", "clone", gitRepo, sitePath)
		if output, err := cmd.CombinedOutput(); err != nil {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to clone repository: %v\nOutput: %s", err, string(output))))
			return m, nil
		}
	} else {
		// Create fresh Laravel installation
		m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Creating fresh Laravel installation at %s", sitePath)))
		cmd := exec.Command("composer", "create-project", "laravel/laravel", sitePath)
		if output, err := cmd.CombinedOutput(); err != nil {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to create Laravel project: %v\nOutput: %s", err, string(output))))
			return m, nil
		}
	}

	// Set proper permissions
	m.setLaravelPermissions(sitePath)

	// Install dependencies if composer.json exists
	if _, err := os.Stat(filepath.Join(sitePath, "composer.json")); err == nil {
		m.report = append(m.report, infoStyle.Render("Installing Composer dependencies"))
		cmd := exec.Command("composer", "install", "--no-dev", "--optimize-autoloader")
		cmd.Dir = sitePath
		if output, err := cmd.CombinedOutput(); err != nil {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to install Composer dependencies: %v\nOutput: %s", err, string(output))))
		}
	}

	// Generate app key if .env.example exists
	if _, err := os.Stat(filepath.Join(sitePath, ".env.example")); err == nil {
		m.report = append(m.report, infoStyle.Render("Setting up Laravel environment"))

		// Copy .env.example to .env
		cmd := exec.Command("cp", ".env.example", ".env")
		cmd.Dir = sitePath
		cmd.Run()

		// Generate app key
		cmd = exec.Command("php", "artisan", "key:generate")
		cmd.Dir = sitePath
		if output, err := cmd.CombinedOutput(); err != nil {
			m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to generate app key: %v\nOutput: %s", err, string(output))))
		}
	}

	// Create Caddy site configuration
	m.createCaddySiteConfig(domain, sitePath)

	// Reload Caddy
	cmd := exec.Command("sudo", "systemctl", "reload", "caddy")
	if output, err := cmd.CombinedOutput(); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("⚠ Failed to reload Caddy: %v\nOutput: %s", err, string(output))))
	}

	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Laravel site created successfully: %s (domain: %s, path: %s)", siteName, domain, sitePath)))
	m.processingMsg = ""
	return m, nil
}

func (m model) updateLaravelSiteWithData() (tea.Model, tea.Cmd) {
	m.state = stateProcessing
	m.report = []string{infoStyle.Render("Updating Laravel site")}

	// List available sites
	sites, err := m.listLaravelSites()
	if err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to list sites: %v", err)))
		return m, nil
	}

	if len(sites) == 0 {
		m.report = append(m.report, infoStyle.Render("📋 No Laravel sites found in /var/www"))
		return m, nil
	}

	// Parse site selection
	siteIndex := m.formData["siteIndex"]
	var selectedSite string
	if idx := parseInt(siteIndex); idx > 0 && idx <= len(sites) {
		selectedSite = sites[idx-1]
	} else {
		m.report = append(m.report, warnStyle.Render("❌ Invalid site selection"))
		return m, nil
	}

	sitePath := filepath.Join("/var/www", selectedSite)

	// Check if it's a Git repository
	if _, err := os.Stat(filepath.Join(sitePath, ".git")); os.IsNotExist(err) {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Site is not a Git repository: %s", selectedSite)))
		return m, nil
	}

	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Updating Laravel site: %s (path: %s)", selectedSite, sitePath)))

	// Put site in maintenance mode
	cmd := exec.Command("php", "artisan", "down")
	cmd.Dir = sitePath
	cmd.Run()

	// Git pull
	cmd = exec.Command("git", "pull", "origin", "main")
	cmd.Dir = sitePath
	if output, err := cmd.CombinedOutput(); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to pull from Git: %v\nOutput: %s", err, string(output))))
		// Try to bring site back up
		cmd = exec.Command("php", "artisan", "up")
		cmd.Dir = sitePath
		cmd.Run()
		return m, nil
	}

	// Install/update dependencies
	cmd = exec.Command("composer", "install", "--no-dev", "--optimize-autoloader")
	cmd.Dir = sitePath
	if output, err := cmd.CombinedOutput(); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("❌ Failed to update Composer dependencies: %v\nOutput: %s", err, string(output))))
	}

	// Run migrations
	cmd = exec.Command("php", "artisan", "migrate", "--force")
	cmd.Dir = sitePath
	if output, err := cmd.CombinedOutput(); err != nil {
		m.report = append(m.report, warnStyle.Render(fmt.Sprintf("⚠ Migration failed: %v\nOutput: %s", err, string(output))))
	}

	// Clear cache
	cmd = exec.Command("php", "artisan", "cache:clear")
	cmd.Dir = sitePath
	cmd.Run()

	cmd = exec.Command("php", "artisan", "config:clear")
	cmd.Dir = sitePath
	cmd.Run()

	cmd = exec.Command("php", "artisan", "view:clear")
	cmd.Dir = sitePath
	cmd.Run()

	// Set permissions
	m.setLaravelPermissions(sitePath)

	// Bring site back up
	cmd = exec.Command("php", "artisan", "up")
	cmd.Dir = sitePath
	cmd.Run()

	m.report = append(m.report, infoStyle.Render(fmt.Sprintf("Laravel site updated successfully: %s", selectedSite)))
	return m, nil
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
