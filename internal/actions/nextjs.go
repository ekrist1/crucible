package actions

import (
	"fmt"
	"path/filepath"
)

// NextJSSiteConfig contains configuration for creating a NextJS site
type NextJSSiteConfig struct {
	SiteName       string
	Domain         string
	GitRepo        string
	Branch         string
	PackageManager string
	Port           int
}

// CreateNextJSSite returns the commands and descriptions for creating a NextJS site
func CreateNextJSSite(config NextJSSiteConfig) ([]string, []string) {
	sitePath := filepath.Join("/var/www", config.SiteName)
	
	var commands []string
	var descriptions []string
	
	// Set default values
	if config.Branch == "" {
		config.Branch = "main"
	}
	if config.PackageManager == "" {
		config.PackageManager = "npm"
	}
	if config.Port == 0 {
		config.Port = 3000
	}
	
	// 1. Create /var/www directory
	commands = append(commands, fmt.Sprintf("sudo mkdir -p %s", "/var/www"))
	descriptions = append(descriptions, "Creating base directory...")
	
	// 2. Create NextJS site (clone or new)
	if config.GitRepo != "" {
		// Clone from repository
		branchArg := ""
		if config.Branch != "" && config.Branch != "main" && config.Branch != "master" {
			branchArg = fmt.Sprintf(" -b %s", config.Branch)
		}
		commands = append(commands, fmt.Sprintf("sudo git clone%s %s %s", branchArg, config.GitRepo, sitePath))
		descriptions = append(descriptions, fmt.Sprintf("Cloning repository from %s...", config.GitRepo))
	} else {
		// Create new NextJS app
		commands = append(commands, fmt.Sprintf("sudo npx create-next-app@latest %s --typescript --tailwind --eslint --app --src-dir --import-alias '@/*'", sitePath))
		descriptions = append(descriptions, "Creating new NextJS application...")
	}
	
	// 3. Set ownership
	commands = append(commands, fmt.Sprintf("sudo chown -R www-data:www-data %s", sitePath))
	descriptions = append(descriptions, "Setting proper ownership...")
	
	// 4. Install dependencies
	commands = append(commands, fmt.Sprintf("cd %s && sudo -u www-data %s install", sitePath, config.PackageManager))
	descriptions = append(descriptions, "Installing dependencies...")
	
	// 5. Build the application (if cloned from repo)
	if config.GitRepo != "" {
		commands = append(commands, fmt.Sprintf("cd %s && sudo -u www-data %s run build", sitePath, config.PackageManager))
		descriptions = append(descriptions, "Building NextJS application...")
	}
	
	// 6. Create PM2 ecosystem file
	pm2Config := fmt.Sprintf(`{
  "apps": [{
    "name": "%s",
    "script": "%s/node_modules/.bin/next",
    "args": "start -p %d",
    "cwd": "%s",
    "instances": 1,
    "exec_mode": "cluster",
    "env": {
      "NODE_ENV": "production",
      "PORT": "%d"
    },
    "error_file": "/var/log/pm2/%s-error.log",
    "out_file": "/var/log/pm2/%s-out.log",
    "log_file": "/var/log/pm2/%s.log"
  }]
}`, config.SiteName, sitePath, config.Port, sitePath, config.Port, config.SiteName, config.SiteName, config.SiteName)
	
	pm2ConfigPath := fmt.Sprintf("/etc/pm2/ecosystem.%s.config.js", config.SiteName)
	commands = append(commands, fmt.Sprintf("sudo mkdir -p /etc/pm2 /var/log/pm2"))
	descriptions = append(descriptions, "Creating PM2 directories...")
	
	commands = append(commands, fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", pm2Config, pm2ConfigPath))
	descriptions = append(descriptions, "Creating PM2 configuration...")
	
	// 7. Create Caddy configuration
	caddyConfig := fmt.Sprintf(`%s {
    reverse_proxy localhost:%d
    
    # Security headers
    header {
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        X-XSS-Protection "1; mode=block"
        Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
        Referrer-Policy "strict-origin-when-cross-origin"
    }
    
    # Enable compression
    encode gzip
    
    # Handle Next.js specific paths
    handle /_next/* {
        reverse_proxy localhost:%d
    }
    
    handle /api/* {
        reverse_proxy localhost:%d
    }
}`, config.Domain, config.Port, config.Port, config.Port)
	
	caddyConfigPath := fmt.Sprintf("/etc/caddy/sites/%s.caddy", config.Domain)
	commands = append(commands, fmt.Sprintf("sudo mkdir -p /etc/caddy/sites"))
	descriptions = append(descriptions, "Creating Caddy sites directory...")
	
	commands = append(commands, fmt.Sprintf("echo '%s' | sudo tee %s > /dev/null", caddyConfig, caddyConfigPath))
	descriptions = append(descriptions, "Creating Caddy configuration...")
	
	// 8. Start the application with PM2
	commands = append(commands, fmt.Sprintf("sudo pm2 start %s", pm2ConfigPath))
	descriptions = append(descriptions, "Starting NextJS application with PM2...")
	
	commands = append(commands, "sudo pm2 save")
	descriptions = append(descriptions, "Saving PM2 configuration...")
	
	// 9. Reload Caddy to pick up new configuration
	commands = append(commands, "sudo systemctl reload caddy")
	descriptions = append(descriptions, "Reloading Caddy configuration...")
	
	// 10. Verify the site is running
	commands = append(commands, fmt.Sprintf("sleep 3 && curl -s http://localhost:%d > /dev/null && echo '✅ NextJS site is running successfully'", config.Port))
	descriptions = append(descriptions, "Verifying site is running...")
	
	return commands, descriptions
}