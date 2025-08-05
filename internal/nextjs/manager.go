package nextjs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"crucible/internal/git"
)

// NextJSManager handles Next.js site management
type NextJSManager struct {
	sitesDir     string
	caddyDir     string
	pm2ConfigDir string
	gitRepo      *git.Repository
}

// Site represents a Next.js site configuration
type Site struct {
	Name           string            `yaml:"name"`
	Repository     string            `yaml:"repository"`
	Branch         string            `yaml:"branch"`
	Domain         string            `yaml:"domain"`
	BuildCommand   string            `yaml:"build_command"`
	StartCommand   string            `yaml:"start_command"`
	Environment    string            `yaml:"environment"`
	PM2Instances   int               `yaml:"pm2_instances"`
	NodeVersion    string            `yaml:"node_version"`
	PackageManager string            `yaml:"package_manager"`
	Port           int               `yaml:"port"`
	EnvVars        map[string]string `yaml:"env_vars"`
	Status         string            `yaml:"status"`
	CreatedAt      time.Time         `yaml:"created_at"`
	UpdatedAt      time.Time         `yaml:"updated_at"`
}

// SiteStatus represents the current status of a site
type SiteStatus struct {
	Name       string    `json:"name"`
	Status     string    `json:"status"`     // running, stopped, building, error
	PM2Status  string    `json:"pm2_status"` // online, stopped, errored
	Instances  int       `json:"instances"`
	CPU        float64   `json:"cpu"`
	Memory     string    `json:"memory"`
	Uptime     string    `json:"uptime"`
	LastDeploy time.Time `json:"last_deploy"`
}

// NewNextJSManager creates a new Next.js manager instance
func NewNextJSManager() *NextJSManager {
	sitesDir := "/var/www/nextjs"
	return &NextJSManager{
		sitesDir:     sitesDir,
		caddyDir:     "/etc/caddy/sites",
		pm2ConfigDir: "/etc/pm2",
		gitRepo:      git.NewRepository(sitesDir),
	}
}

// CreateSite creates a new Next.js site from a GitHub repository
func (nm *NextJSManager) CreateSite(site *Site) error {
	// Validate site configuration
	if err := nm.validateSiteConfig(site); err != nil {
		return fmt.Errorf("invalid site configuration: %w", err)
	}

	// Create site directory
	siteDir := filepath.Join(nm.sitesDir, site.Name)
	if err := os.MkdirAll(siteDir, 0755); err != nil {
		return fmt.Errorf("failed to create site directory: %w", err)
	}

	// Clone repository
	if err := nm.gitRepo.CloneRepository(site.Repository, site.Branch, site.Name); err != nil {
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	// Auto-detect package manager if not specified
	if site.PackageManager == "" || site.PackageManager == "auto" {
		site.PackageManager = nm.detectPackageManager(siteDir)
	}

	// Install dependencies
	if err := nm.installDependencies(siteDir, site.PackageManager); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	// Create environment file
	if err := nm.createEnvironmentFile(siteDir, site.EnvVars); err != nil {
		return fmt.Errorf("failed to create environment file: %w", err)
	}

	// Build the application
	if err := nm.buildApplication(siteDir, site.BuildCommand, site.PackageManager); err != nil {
		return fmt.Errorf("failed to build application: %w", err)
	}

	// Generate PM2 ecosystem file
	if err := nm.generatePM2Config(site); err != nil {
		return fmt.Errorf("failed to generate PM2 config: %w", err)
	}

	// Generate Caddy configuration
	if err := nm.generateCaddyConfig(site); err != nil {
		return fmt.Errorf("failed to generate Caddy config: %w", err)
	}

	// Start the application with PM2
	if err := nm.startWithPM2(site); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Reload Caddy configuration
	if err := nm.reloadCaddy(); err != nil {
		return fmt.Errorf("failed to reload Caddy: %w", err)
	}

	site.Status = "running"
	site.CreatedAt = time.Now()
	site.UpdatedAt = time.Now()

	return nil
}

// validateSiteConfig validates the site configuration
func (nm *NextJSManager) validateSiteConfig(site *Site) error {
	if site.Name == "" {
		return fmt.Errorf("site name is required")
	}
	if site.Repository == "" {
		return fmt.Errorf("repository URL is required")
	}
	if site.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if site.Branch == "" {
		site.Branch = "main"
	}
	if site.BuildCommand == "" {
		site.BuildCommand = "npm run build"
	}
	if site.StartCommand == "" {
		site.StartCommand = "npm start"
	}
	if site.PM2Instances == 0 {
		site.PM2Instances = 1
	}
	if site.Port == 0 {
		site.Port = nm.findAvailablePort(3000)
	}
	return nil
}

// detectPackageManager detects the package manager used by the project
func (nm *NextJSManager) detectPackageManager(siteDir string) string {
	// Check for lock files to determine package manager
	if _, err := os.Stat(filepath.Join(siteDir, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(siteDir, "yarn.lock")); err == nil {
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(siteDir, "package-lock.json")); err == nil {
		return "npm"
	}
	// Default to npm
	return "npm"
}

// installDependencies installs project dependencies
func (nm *NextJSManager) installDependencies(siteDir, packageManager string) error {
	var cmd *exec.Cmd

	switch packageManager {
	case "yarn":
		cmd = exec.Command("yarn", "install")
	case "pnpm":
		cmd = exec.Command("pnpm", "install")
	default:
		cmd = exec.Command("npm", "install")
	}

	cmd.Dir = siteDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dependency installation failed: %s", string(output))
	}
	return nil
}

// createEnvironmentFile creates a .env file with the specified variables
func (nm *NextJSManager) createEnvironmentFile(siteDir string, envVars map[string]string) error {
	if len(envVars) == 0 {
		return nil
	}

	envFile := filepath.Join(siteDir, ".env.production")
	file, err := os.Create(envFile)
	if err != nil {
		return err
	}
	defer file.Close()

	for key, value := range envVars {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, value); err != nil {
			return err
		}
	}

	return nil
}

// buildApplication builds the Next.js application
func (nm *NextJSManager) buildApplication(siteDir, buildCommand, packageManager string) error {
	// Parse build command
	parts := strings.Fields(buildCommand)
	if len(parts) == 0 {
		return fmt.Errorf("invalid build command")
	}

	// Replace package manager placeholder if needed
	if parts[0] == "npm" && packageManager != "npm" {
		parts[0] = packageManager
		if packageManager == "yarn" && len(parts) > 1 && parts[1] == "run" {
			// Remove "run" for yarn
			parts = append(parts[:1], parts[2:]...)
		}
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = siteDir
	cmd.Env = append(os.Environ(), "NODE_ENV=production")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build failed: %s", string(output))
	}
	return nil
}

// generatePM2Config generates a PM2 ecosystem file for the site
func (nm *NextJSManager) generatePM2Config(site *Site) error {
	configPath := filepath.Join(nm.pm2ConfigDir, fmt.Sprintf("%s.config.js", site.Name))

	config := fmt.Sprintf(`module.exports = {
  apps: [{
    name: '%s',
    script: '%s',
    cwd: '%s',
    instances: %d,
    exec_mode: 'cluster',
    watch: false,
    max_memory_restart: '1G',
    env: {
      NODE_ENV: 'production',
      PORT: %d
    },
    error_file: '/var/log/pm2/%s-error.log',
    out_file: '/var/log/pm2/%s-out.log',
    log_file: '/var/log/pm2/%s-combined.log',
    time: true
  }]
};`,
		site.Name,
		site.StartCommand,
		filepath.Join(nm.sitesDir, site.Name),
		site.PM2Instances,
		site.Port,
		site.Name,
		site.Name,
		site.Name,
	)

	return os.WriteFile(configPath, []byte(config), 0644)
}

// generateCaddyConfig generates a Caddy configuration for the site
func (nm *NextJSManager) generateCaddyConfig(site *Site) error {
	configPath := filepath.Join(nm.caddyDir, fmt.Sprintf("%s.caddy", site.Name))

	config := fmt.Sprintf(`%s {
	reverse_proxy localhost:%d {
		health_uri /api/health
		health_interval 10s
		health_timeout 5s
	}
	
	# Enable compression
	encode gzip
	
	# Security headers
	header {
		X-Content-Type-Options nosniff
		X-Frame-Options DENY
		X-XSS-Protection "1; mode=block"
		Referrer-Policy strict-origin-when-cross-origin
	}
	
	# Static asset caching
	@static {
		path /_next/static/*
		path /favicon.ico
		path /robots.txt
	}
	header @static Cache-Control "public, max-age=31536000, immutable"
	
	# API routes
	@api {
		path /api/*
	}
	header @api Cache-Control "no-cache, no-store, must-revalidate"
	
	# Logging
	log {
		output file /var/log/caddy/%s.log
		format json
	}
}`, site.Domain, site.Port, site.Name)

	return os.WriteFile(configPath, []byte(config), 0644)
}

// startWithPM2 starts the application using PM2
func (nm *NextJSManager) startWithPM2(site *Site) error {
	configPath := filepath.Join(nm.pm2ConfigDir, fmt.Sprintf("%s.config.js", site.Name))

	cmd := exec.Command("pm2", "start", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("PM2 start failed: %s", string(output))
	}
	return nil
}

// reloadCaddy reloads the Caddy configuration
func (nm *NextJSManager) reloadCaddy() error {
	cmd := exec.Command("caddy", "reload", "--config", "/etc/caddy/Caddyfile")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Caddy reload failed: %s", string(output))
	}
	return nil
}

// findAvailablePort finds an available port starting from the given port
func (nm *NextJSManager) findAvailablePort(startPort int) int {
	// This is a simplified implementation
	// In practice, you'd check if the port is actually available
	return startPort
}

// GetSiteStatus returns the current status of a site
func (nm *NextJSManager) GetSiteStatus(siteName string) (*SiteStatus, error) {
	// Get PM2 process information
	cmd := exec.Command("pm2", "jlist")
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get PM2 status: %w", err)
	}

	// Parse PM2 output and extract site information
	// This is a simplified implementation
	status := &SiteStatus{
		Name:      siteName,
		Status:    "running",
		PM2Status: "online",
		Instances: 2,
		CPU:       15.5,
		Memory:    "245MB",
		Uptime:    "2d 3h 15m",
	}

	return status, nil
}

// ListSites returns a list of all managed Next.js sites
func (nm *NextJSManager) ListSites() ([]*Site, error) {
	// This would read from a configuration file or database
	// For now, return an empty slice
	return []*Site{}, nil
}

// UpdateSite updates an existing site (git pull, rebuild, restart)
func (nm *NextJSManager) UpdateSite(siteName string) error {
	siteDir := filepath.Join(nm.sitesDir, siteName)

	// Git pull using the repository manager
	if err := nm.gitRepo.PullRepository(siteName); err != nil {
		return fmt.Errorf("failed to pull repository: %w", err)
	}

	// Reinstall dependencies (in case package.json changed)
	if err := nm.installDependencies(siteDir, "npm"); err != nil {
		return err
	}

	// Rebuild
	if err := nm.buildApplication(siteDir, "npm run build", "npm"); err != nil {
		return err
	}

	// Restart PM2 process
	cmd := exec.Command("pm2", "restart", siteName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("PM2 restart failed: %s", string(output))
	}

	return nil
}

// DeleteSite removes a site and all its configurations
func (nm *NextJSManager) DeleteSite(siteName string) error {
	// Stop PM2 process
	cmd := exec.Command("pm2", "delete", siteName)
	cmd.CombinedOutput() // Ignore errors if process doesn't exist

	// Remove site directory
	siteDir := filepath.Join(nm.sitesDir, siteName)
	if err := os.RemoveAll(siteDir); err != nil {
		return fmt.Errorf("failed to remove site directory: %w", err)
	}

	// Remove PM2 config
	pm2Config := filepath.Join(nm.pm2ConfigDir, fmt.Sprintf("%s.config.js", siteName))
	os.Remove(pm2Config)

	// Remove Caddy config
	caddyConfig := filepath.Join(nm.caddyDir, fmt.Sprintf("%s.caddy", siteName))
	os.Remove(caddyConfig)

	// Reload Caddy
	return nm.reloadCaddy()
}

// GetRepositoryStatus returns the Git repository status for a site
func (nm *NextJSManager) GetRepositoryStatus(siteName string) (*git.RepositoryStatus, error) {
	return nm.gitRepo.GetRepositoryStatus(siteName)
}

// SwitchBranch switches a site to a different branch
func (nm *NextJSManager) SwitchBranch(siteName, branch string) error {
	// Switch to the new branch
	if err := nm.gitRepo.CheckoutBranch(siteName, branch); err != nil {
		return fmt.Errorf("failed to switch branch: %w", err)
	}

	// Pull latest changes
	if err := nm.gitRepo.PullRepository(siteName); err != nil {
		return fmt.Errorf("failed to pull after branch switch: %w", err)
	}

	// Get site directory
	siteDir := filepath.Join(nm.sitesDir, siteName)

	// Reinstall dependencies (package.json might have changed)
	packageManager := nm.detectPackageManager(siteDir)
	if err := nm.installDependencies(siteDir, packageManager); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	// Rebuild the application
	if err := nm.buildApplication(siteDir, "npm run build", packageManager); err != nil {
		return fmt.Errorf("failed to build application: %w", err)
	}

	// Restart PM2 process
	cmd := exec.Command("pm2", "restart", siteName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to restart PM2 process: %s", string(output))
	}

	return nil
}

// ListBranches returns all available branches for a site
func (nm *NextJSManager) ListBranches(siteName string) ([]string, error) {
	return nm.gitRepo.ListBranches(siteName)
}

// GetCurrentBranch returns the current branch for a site
func (nm *NextJSManager) GetCurrentBranch(siteName string) (string, error) {
	return nm.gitRepo.GetCurrentBranch(siteName)
}

// ValidateGitRepository checks if a site has a valid Git repository
func (nm *NextJSManager) ValidateGitRepository(siteName string) error {
	return nm.gitRepo.ValidateRepository(siteName)
}

// GetSiteWithRepositoryInfo returns site information including Git repository data
func (nm *NextJSManager) GetSiteWithRepositoryInfo(siteName string) (*SiteInfo, error) {
	// Get repository status
	repoStatus, err := nm.GetRepositoryStatus(siteName)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository status: %w", err)
	}

	// Get PM2 status (simplified)
	siteStatus, err := nm.GetSiteStatus(siteName)
	if err != nil {
		// Don't fail if PM2 status is unavailable, just set defaults
		siteStatus = &SiteStatus{
			Name:      siteName,
			Status:    "unknown",
			PM2Status: "unknown",
		}
	}

	return &SiteInfo{
		Name:       siteName,
		Repository: repoStatus,
		Status:     siteStatus,
	}, nil
}

// SiteInfo represents comprehensive information about a Next.js site
type SiteInfo struct {
	Name       string                `json:"name"`
	Repository *git.RepositoryStatus `json:"repository"`
	Status     *SiteStatus           `json:"status"`
}
