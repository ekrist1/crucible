package actions

import (
	"fmt"
	"os"
	"path/filepath"

	"crucible/internal/system"
)

// LaravelSiteConfig contains configuration for creating a Laravel site
type LaravelSiteConfig struct {
	SiteName string
	Domain   string
	GitRepo  string
}

// UpdateSiteConfig contains configuration for updating a Laravel site
type UpdateSiteConfig struct {
	SiteIndex string
	Sites     []string
}

// QueueWorkerConfig contains configuration for setting up Laravel queue worker
type QueueWorkerConfig struct {
	SiteName   string
	Connection string
	Processes  string
	QueueName  string
}

// CreateLaravelSite returns the commands and descriptions for creating a Laravel site
func CreateLaravelSite(config LaravelSiteConfig) ([]string, []string) {
	sitePath := filepath.Join("/var/www", config.SiteName)

	var commands []string
	var descriptions []string

	// 1. Create /var/www directory
	commands = append(commands, fmt.Sprintf("sudo mkdir -p %s", "/var/www"))
	descriptions = append(descriptions, "Creating base directory...")

	// 2. Create Laravel site (clone or new)
	if config.GitRepo != "" {
		commands = append(commands, fmt.Sprintf("git clone %s %s", config.GitRepo, sitePath))
		descriptions = append(descriptions, fmt.Sprintf("Cloning Laravel app from %s...", config.GitRepo))
	} else {
		commands = append(commands, fmt.Sprintf("composer create-project laravel/laravel %s", sitePath))
		descriptions = append(descriptions, fmt.Sprintf("Creating fresh Laravel installation at %s...", sitePath))
	}

	// 3. Set ownership and permissions
	webUser := system.GetWebServerUser()
	commands = append(commands, fmt.Sprintf("sudo chown -R %s:%s %s", webUser, webUser, sitePath))
	descriptions = append(descriptions, fmt.Sprintf("Setting ownership to %s...", webUser))
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
    root * %s/public
    php_fastcgi 127.0.0.1:9000
    encode gzip
    
    # Laravel specific rewrite rules
    try_files {path} {path}/ /index.php?{query}
    
    # Security headers
    header {
        Strict-Transport-Security max-age=31536000;
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        X-XSS-Protection "1; mode=block"
    }
}`, config.Domain, sitePath)
	caddyConfigPath := fmt.Sprintf("/etc/caddy/sites/%s.caddy", config.Domain)
	// Use a 'heredoc' to safely write the multi-line config to a file
	writeConfigCmd := fmt.Sprintf("sudo mkdir -p /etc/caddy/sites && echo '%s' | sudo tee %s > /dev/null", caddyConfig, caddyConfigPath)
	commands = append(commands, writeConfigCmd)
	descriptions = append(descriptions, "Creating Caddy site configuration...")

	// 6b. Ensure main Caddyfile imports site configs
	commands = append(commands, "sudo bash -c 'grep -q \"import sites/\\*\" /etc/caddy/Caddyfile 2>/dev/null || echo \"import sites/*\" >> /etc/caddy/Caddyfile'")
	descriptions = append(descriptions, "Updating main Caddyfile...")

	// 7. Ensure PHP-FPM is running
	commands = append(commands, "sudo systemctl start php*-fpm || sudo systemctl start php-fpm || true")
	descriptions = append(descriptions, "Starting PHP-FPM service...")
	commands = append(commands, "sudo systemctl enable php*-fpm || sudo systemctl enable php-fpm || true")
	descriptions = append(descriptions, "Enabling PHP-FPM service...")

	// 8. Start and reload Caddy (ensure it's running first)
	commands = append(commands, "sudo systemctl start caddy || true")
	descriptions = append(descriptions, "Starting Caddy server...")
	commands = append(commands, "sudo systemctl enable caddy || true")
	descriptions = append(descriptions, "Enabling Caddy service...")
	commands = append(commands, "sudo systemctl reload caddy || sudo systemctl restart caddy")
	descriptions = append(descriptions, "Reloading Caddy configuration...")

	return commands, descriptions
}

// UpdateLaravelSite returns the commands and descriptions for updating a Laravel site
func UpdateLaravelSite(config UpdateSiteConfig) ([]string, []string, error) {
	// Parse site selection
	var selectedSite string
	if idx := parseInt(config.SiteIndex); idx > 0 && idx <= len(config.Sites) {
		selectedSite = config.Sites[idx-1]
	} else {
		return nil, nil, fmt.Errorf("invalid site selection")
	}

	sitePath := filepath.Join("/var/www", selectedSite)

	// Check if it's a Git repository
	if _, err := os.Stat(filepath.Join(sitePath, ".git")); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("site is not a Git repository: %s", selectedSite)
	}

	var commands []string
	var descriptions []string
	webUser := system.GetWebServerUser()

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

	// 6. Set permissions (using system.GetWebServerUser instead of hardcoded www-data)
	commands = append(commands, fmt.Sprintf("sudo chown -R %s:%s %s", webUser, webUser, sitePath))
	descriptions = append(descriptions, fmt.Sprintf("Setting ownership to %s...", webUser))
	commands = append(commands, fmt.Sprintf("find %s -type d -exec chmod 755 {} + && find %s -type f -exec chmod 644 {} +", sitePath, sitePath))
	descriptions = append(descriptions, "Setting file permissions...")
	commands = append(commands, fmt.Sprintf("chmod -R 775 %s/storage %s/bootstrap/cache", sitePath, sitePath))
	descriptions = append(descriptions, "Setting writable permissions...")

	// 7. Bring site back up
	commands = append(commands, fmt.Sprintf("cd %s && php artisan up", sitePath))
	descriptions = append(descriptions, "Bringing site back online...")

	return commands, descriptions, nil
}

// SetupQueueWorker returns the commands and descriptions for setting up Laravel queue worker
func SetupQueueWorker(config QueueWorkerConfig) ([]string, []string) {
	sitePath := filepath.Join("/var/www", config.SiteName)
	webUser := system.GetWebServerUser()

	// Generate supervisor configuration
	workerName := fmt.Sprintf("laravel-worker-%s", config.SiteName)
	configPath := fmt.Sprintf("/etc/supervisor/conf.d/%s.conf", workerName)

	supervisorConfig := fmt.Sprintf(`[program:%s]
process_name=%%(program_name)s_%%(process_num)02d
command=php %s/artisan queue:work %s --sleep=3 --tries=3 --max-time=3600 --queue=%s
autostart=true
autorestart=true
stopasgroup=true
killasgroup=true
user=%s
numprocs=%s
redirect_stderr=true
stdout_logfile=%s/storage/logs/worker.log
stdout_logfile_maxbytes=100MB
stdout_logfile_backups=2
stopwaitsecs=3600
`, workerName, sitePath, config.Connection, config.QueueName, webUser, config.Processes, sitePath)

	var commands []string
	var descriptions []string

	// 1. Create supervisor configuration
	commands = append(commands, fmt.Sprintf("sudo bash -c 'cat > %s << \"EOF\"\n%sEOF'", configPath, supervisorConfig))
	descriptions = append(descriptions, "Creating Supervisor configuration...")

	// 2. Create log directory if it doesn't exist
	commands = append(commands, fmt.Sprintf("sudo mkdir -p %s/storage/logs", sitePath))
	descriptions = append(descriptions, "Creating log directory...")

	// 3. Set proper permissions (using system.GetWebServerUser)
	commands = append(commands, fmt.Sprintf("sudo chown -R %s:%s %s/storage/logs", webUser, webUser, sitePath))
	descriptions = append(descriptions, fmt.Sprintf("Setting log permissions for %s...", webUser))

	// 4. Reload supervisor configuration
	commands = append(commands, "sudo supervisorctl reread")
	descriptions = append(descriptions, "Reloading Supervisor configuration...")

	// 5. Update supervisor with new configuration
	commands = append(commands, "sudo supervisorctl update")
	descriptions = append(descriptions, "Updating Supervisor...")

	// 6. Start the worker
	commands = append(commands, fmt.Sprintf("sudo supervisorctl start %s:*", workerName))
	descriptions = append(descriptions, "Starting queue worker...")

	return commands, descriptions
}

// ListLaravelSites returns a list of Laravel sites found in /var/www
func ListLaravelSites() ([]string, error) {
	entries, err := os.ReadDir("/var/www")
	if err != nil {
		return nil, err
	}

	var sites []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Check if it looks like a Laravel site
			sitePath := filepath.Join("/var/www", entry.Name())
			if isLaravelSite(sitePath) {
				sites = append(sites, entry.Name())
			}
		}
	}

	return sites, nil
}

// Helper functions

// isLaravelSite checks if a directory contains Laravel indicators
func isLaravelSite(path string) bool {
	// Check for Laravel indicators
	indicators := []string{"artisan", "composer.json", "app/Http/Kernel.php"}

	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(path, indicator)); err == nil {
			return true
		}
	}

	return false
}

// parseInt converts string to int for simple cases
func parseInt(s string) int {
	switch s {
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	case "8":
		return 8
	case "9":
		return 9
	default:
		return 0
	}
}

// SetLaravelPermissions sets proper permissions for a Laravel site
func SetLaravelPermissions(sitePath string) []string {
	webUser := system.GetWebServerUser()
	var commands []string

	// Set ownership
	commands = append(commands, fmt.Sprintf("sudo chown -R %s:%s %s", webUser, webUser, sitePath))

	// Set directory permissions
	commands = append(commands, fmt.Sprintf("find %s -type d -exec chmod 755 {} +", sitePath))

	// Set file permissions
	commands = append(commands, fmt.Sprintf("find %s -type f -exec chmod 644 {} +", sitePath))

	// Set writable permissions for storage and cache
	storagePath := filepath.Join(sitePath, "storage")
	if _, err := os.Stat(storagePath); err == nil {
		commands = append(commands, fmt.Sprintf("chmod -R 775 %s", storagePath))
	}

	cachePath := filepath.Join(sitePath, "bootstrap", "cache")
	if _, err := os.Stat(cachePath); err == nil {
		commands = append(commands, fmt.Sprintf("chmod -R 775 %s", cachePath))
	}

	return commands
}

// CreateCaddySiteConfig creates Caddy configuration for a Laravel site
func CreateCaddySiteConfig(domain, sitePath string) []string {
	var commands []string

	// Create sites directory
	commands = append(commands, "sudo mkdir -p /etc/caddy/sites")

	// Create site configuration
	configPath := fmt.Sprintf("/etc/caddy/sites/%s.caddy", domain)
	config := fmt.Sprintf(`%s {
	import laravel-app %s
}`, domain, sitePath)

	commands = append(commands, fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", config, configPath))

	// Update main Caddyfile to import sites if not already done
	caddyfilePath := "/etc/caddy/Caddyfile"
	checkImportCmd := fmt.Sprintf("grep -q 'import sites/\\*' %s || (echo 'import sites/*' | sudo tee -a %s > /dev/null)", caddyfilePath, caddyfilePath)
	commands = append(commands, checkImportCmd)

	return commands
}
