package actions

import (
	"fmt"
	"strings"
	"time"
)

// SecurityAuditResult represents the result of a security assessment
type SecurityAuditResult struct {
	SSHRootLogin       bool
	SSHPasswordAuth    bool
	SSHDefaultPort     bool
	SSHKeyAuth         bool
	FirewallEnabled    bool
	FirewallConfigured bool
	Fail2banInstalled  bool
	Fail2banActive     bool
	AutoUpdatesEnabled bool
	SSLConfigured      bool
	SecurityScore      int
	Recommendations    []string
}

// SecurityConfig contains configuration for security hardening
type SecurityConfig struct {
	SSHPort             int
	SSHKeys             []string
	DisableRootLogin    bool
	DisablePasswordAuth bool
	FirewallRules       []FirewallRule
	Fail2banJails       []string
	EnableAutoUpdates   bool
	AdminEmail          string
}

// FirewallRule represents a UFW firewall rule
type FirewallRule struct {
	Port        string
	Protocol    string
	Direction   string
	Action      string
	Description string
}

// SecurityAssessment performs a comprehensive security audit
func SecurityAssessment() ([]string, []string) {
	var commands []string
	var descriptions []string

	// Add educational header
	commands = append(commands, "echo '=== CRUCIBLE SECURITY ASSESSMENT ==='")
	descriptions = append(descriptions, "Starting comprehensive security analysis...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// SSH Configuration Analysis
	commands = append(commands, "echo '📡 SSH SECURITY ANALYSIS'")
	descriptions = append(descriptions, "Analyzing SSH daemon configuration...")

	commands = append(commands, "echo '   Why: SSH is the primary remote access method - securing it prevents unauthorized access'")
	descriptions = append(descriptions, "")

	// Check SSH root login
	commands = append(commands, "if grep -E '^PermitRootLogin|^#PermitRootLogin' /etc/ssh/sshd_config >/dev/null 2>&1; then grep -E '^PermitRootLogin|^#PermitRootLogin' /etc/ssh/sshd_config | head -1; else echo '⚠️  PermitRootLogin: Configuration not found (using defaults)'; fi")
	descriptions = append(descriptions, "Checking if root login is disabled...")

	// Check SSH password authentication
	commands = append(commands, "if grep -E '^PasswordAuthentication|^#PasswordAuthentication' /etc/ssh/sshd_config >/dev/null 2>&1; then grep -E '^PasswordAuthentication|^#PasswordAuthentication' /etc/ssh/sshd_config | head -1; else echo '⚠️  PasswordAuthentication: Configuration not found (using defaults)'; fi")
	descriptions = append(descriptions, "Checking if password authentication is disabled...")

	// Check SSH port
	commands = append(commands, "if grep -E '^Port|^#Port' /etc/ssh/sshd_config >/dev/null 2>&1; then grep -E '^Port|^#Port' /etc/ssh/sshd_config | head -1; else echo '📋 Port: Using default port 22'; fi")
	descriptions = append(descriptions, "Checking if SSH port has been changed from default...")

	// Check for SSH keys
	commands = append(commands, "if [ -f ~/.ssh/authorized_keys ]; then echo \"✅ SSH keys: $(wc -l < ~/.ssh/authorized_keys) key(s) configured\"; else echo '❌ SSH keys: No authorized keys found - password authentication required'; fi")
	descriptions = append(descriptions, "Checking if SSH key authentication is configured...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// Firewall Analysis
	commands = append(commands, "echo '🔥 FIREWALL SECURITY ANALYSIS'")
	descriptions = append(descriptions, "Analyzing firewall configuration...")

	commands = append(commands, "echo '   Why: Firewalls block unauthorized network connections and reduce attack surface'")
	descriptions = append(descriptions, "")

	// Check UFW firewall with proper error handling
	commands = append(commands, "if command -v ufw >/dev/null 2>&1; then ufw status verbose 2>/dev/null || echo '⚠️  UFW installed but not configured properly'; else echo '❌ UFW Firewall: Not installed - server is unprotected'; fi")
	descriptions = append(descriptions, "Checking UFW firewall status and rules...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// Intrusion Detection Analysis  
	commands = append(commands, "echo '👁️  INTRUSION DETECTION ANALYSIS'")
	descriptions = append(descriptions, "Analyzing intrusion detection systems...")

	commands = append(commands, "echo '   Why: Intrusion detection blocks brute force attacks and suspicious activity'")
	descriptions = append(descriptions, "")

	// Check Fail2ban with proper error handling
	commands = append(commands, "if command -v fail2ban-client >/dev/null 2>&1; then if systemctl is-active fail2ban >/dev/null 2>&1; then echo '✅ Fail2ban: Active and running'; fail2ban-client status 2>/dev/null | grep 'Jail list:' || echo '⚠️  No jails configured'; else echo '⚠️  Fail2ban: Installed but not running'; fi; else echo '❌ Fail2ban: Not installed - no protection against brute force attacks'; fi")
	descriptions = append(descriptions, "Checking Fail2ban intrusion detection status...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// System Updates Analysis
	commands = append(commands, "echo '🔄 AUTOMATIC UPDATES ANALYSIS'")
	descriptions = append(descriptions, "Analyzing automatic security updates...")

	commands = append(commands, "echo '   Why: Automatic updates ensure security patches are installed promptly'")
	descriptions = append(descriptions, "")

	// Check automatic updates
	commands = append(commands, "if systemctl is-active unattended-upgrades >/dev/null 2>&1; then echo '✅ Automatic Updates: Enabled and active'; else if [ -f /etc/apt/apt.conf.d/20auto-upgrades ]; then echo '⚠️  Automatic Updates: Configured but service not running'; else echo '❌ Automatic Updates: Not configured - manual updates required'; fi; fi")
	descriptions = append(descriptions, "Checking automatic security updates configuration...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// Network Security Analysis
	commands = append(commands, "echo '🌐 NETWORK SECURITY ANALYSIS'")
	descriptions = append(descriptions, "Analyzing network exposure...")

	commands = append(commands, "echo '   Why: Knowing what services are exposed helps identify potential attack vectors'")
	descriptions = append(descriptions, "")

	// Check open ports with better formatting
	commands = append(commands, "echo 'Open Network Ports:'; ss -tulpn | grep LISTEN | head -10 | while read line; do echo \"  $line\"; done; total=$(ss -tulpn | grep LISTEN | wc -l); if [ $total -gt 10 ]; then echo \"  ... and $((total - 10)) more ports\"; fi")
	descriptions = append(descriptions, "Scanning for open network ports...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// User Account Analysis
	commands = append(commands, "echo '👤 USER ACCOUNT ANALYSIS'")
	descriptions = append(descriptions, "Analyzing user accounts...")

	commands = append(commands, "echo '   Why: Unnecessary user accounts increase security risks'")
	descriptions = append(descriptions, "")

	// Check system users with better output
	commands = append(commands, "echo 'Non-system user accounts:'; users=$(awk -F: '$3 >= 1000 && $3 < 65534 {print $1}' /etc/passwd); if [ -z \"$users\" ]; then echo '  No regular user accounts found'; else echo \"$users\" | while read user; do echo \"  📋 $user\"; done; fi")
	descriptions = append(descriptions, "Checking regular user accounts...")

	// Add security summary and recommendations
	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	commands = append(commands, "echo '📊 SECURITY ASSESSMENT SUMMARY'")
	descriptions = append(descriptions, "Generating security recommendations...")

	// Create a comprehensive assessment script
	summaryScript := `#!/bin/bash
echo ""
echo "=== SECURITY STATUS SUMMARY ==="
echo ""

# Track security scores
secure_count=0
total_checks=5

echo "🔐 IMPLEMENTED SECURITY MEASURES:"

# Check SSH root login
if grep -E '^PermitRootLogin no' /etc/ssh/sshd_config >/dev/null 2>&1; then
    echo "  ✅ SSH root login disabled"
    secure_count=$((secure_count + 1))
fi

# Check SSH password auth
if grep -E '^PasswordAuthentication no' /etc/ssh/sshd_config >/dev/null 2>&1; then
    echo "  ✅ SSH password authentication disabled"
    secure_count=$((secure_count + 1))
fi

# Check SSH keys
if [ -f ~/.ssh/authorized_keys ] && [ -s ~/.ssh/authorized_keys ]; then
    echo "  ✅ SSH key authentication configured"
    secure_count=$((secure_count + 1))
fi

# Check UFW
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "Status: active"; then
    echo "  ✅ UFW firewall active and configured"
    secure_count=$((secure_count + 1))
fi

# Check Fail2ban
if command -v fail2ban-client >/dev/null 2>&1 && systemctl is-active fail2ban >/dev/null 2>&1; then
    echo "  ✅ Fail2ban intrusion detection active"
    secure_count=$((secure_count + 1))
fi

# Check automatic updates
if systemctl is-active unattended-upgrades >/dev/null 2>&1; then
    echo "  ✅ Automatic security updates enabled"
fi

echo ""
echo "⚠️  RECOMMENDED SECURITY IMPROVEMENTS:"

# SSH recommendations
if ! grep -E '^PermitRootLogin no' /etc/ssh/sshd_config >/dev/null 2>&1; then
    echo "  🔴 HIGH: Disable SSH root login (prevents direct root access)"
fi

if ! grep -E '^PasswordAuthentication no' /etc/ssh/sshd_config >/dev/null 2>&1; then
    echo "  🔴 HIGH: Disable SSH password authentication (use keys only)"
fi

if [ ! -f ~/.ssh/authorized_keys ] || [ ! -s ~/.ssh/authorized_keys ]; then
    echo "  🟡 MEDIUM: Configure SSH key authentication (more secure than passwords)"
fi

if ! command -v ufw >/dev/null 2>&1; then
    echo "  🔴 HIGH: Install and configure UFW firewall (blocks unauthorized access)"
elif ! ufw status | grep -q "Status: active"; then
    echo "  🟡 MEDIUM: Enable UFW firewall (currently installed but inactive)"
fi

if ! command -v fail2ban-client >/dev/null 2>&1; then
    echo "  🟡 MEDIUM: Install Fail2ban (protects against brute force attacks)"
elif ! systemctl is-active fail2ban >/dev/null 2>&1; then
    echo "  🟡 MEDIUM: Start Fail2ban service (currently installed but inactive)"
fi

if ! systemctl is-active unattended-upgrades >/dev/null 2>&1; then
    echo "  🟡 MEDIUM: Enable automatic security updates (keeps system patched)"
fi

echo ""
echo "📈 SECURITY SCORE: $secure_count/$total_checks"
if [ $secure_count -ge 4 ]; then
    echo "🟢 Status: GOOD - Your server has strong security"
elif [ $secure_count -ge 2 ]; then
    echo "🟡 Status: MODERATE - Some improvements needed"
else
    echo "🔴 Status: CRITICAL - Immediate security attention required"
fi

echo ""
echo "🚀 NEXT STEPS:"
echo "  1. Use 'Quick Security Hardening' to implement recommendations"
echo "  2. Review the 'Security Status Dashboard' regularly"
echo "  3. Monitor security logs for suspicious activity"
echo ""
`

	commands = append(commands, fmt.Sprintf("cat > /tmp/security_summary.sh << 'EOF'\n%sEOF", summaryScript))
	descriptions = append(descriptions, "Creating security assessment script...")

	commands = append(commands, "chmod +x /tmp/security_summary.sh")
	descriptions = append(descriptions, "")

	commands = append(commands, "/tmp/security_summary.sh")
	descriptions = append(descriptions, "Analyzing security posture and generating recommendations...")

	commands = append(commands, "rm -f /tmp/security_summary.sh")
	descriptions = append(descriptions, "")

	return commands, descriptions
}

// SSHHardening implements comprehensive SSH security
func SSHHardening(config SecurityConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	// Backup original SSH config
	backupFile := fmt.Sprintf("/etc/ssh/sshd_config.backup.%s", time.Now().Format("20060102-150405"))
	commands = append(commands, fmt.Sprintf("cp /etc/ssh/sshd_config %s", backupFile))
	descriptions = append(descriptions, "Backing up SSH configuration...")

	// Change SSH port if specified
	if config.SSHPort != 0 && config.SSHPort != 22 {
		commands = append(commands, fmt.Sprintf("sed -i 's/^#Port 22/Port %d/' /etc/ssh/sshd_config", config.SSHPort))
		commands = append(commands, fmt.Sprintf("sed -i 's/^Port 22/Port %d/' /etc/ssh/sshd_config", config.SSHPort))
		descriptions = append(descriptions, fmt.Sprintf("Changing SSH port to %d...", config.SSHPort))
	}

	// Disable root login if requested
	if config.DisableRootLogin {
		commands = append(commands, "sed -i 's/^#PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config")
		commands = append(commands, "sed -i 's/^PermitRootLogin yes/PermitRootLogin no/' /etc/ssh/sshd_config")
		descriptions = append(descriptions, "Disabling SSH root login...")
	}

	// Configure password authentication
	if config.DisablePasswordAuth {
		commands = append(commands, "sed -i 's/^#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config")
		commands = append(commands, "sed -i 's/^PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config")
		descriptions = append(descriptions, "Disabling SSH password authentication...")
	}

	// Additional SSH hardening
	sshHardeningConfigs := []string{
		"Protocol 2",
		"MaxAuthTries 3",
		"ClientAliveInterval 300",
		"ClientAliveCountMax 2",
		"X11Forwarding no",
		"UseDNS no",
		"PermitEmptyPasswords no",
		"AllowUsers root", // Will be updated based on actual users
	}

	for _, sshConfig := range sshHardeningConfigs {
		configKey := strings.Split(sshConfig, " ")[0]
		commands = append(commands, fmt.Sprintf("grep -q '^%s' /etc/ssh/sshd_config || echo '%s' >> /etc/ssh/sshd_config", configKey, sshConfig))
		descriptions = append(descriptions, fmt.Sprintf("Configuring SSH: %s...", sshConfig))
	}

	// Test SSH configuration
	commands = append(commands, "sshd -t")
	descriptions = append(descriptions, "Testing SSH configuration...")

	// Restart SSH service
	commands = append(commands, "systemctl restart ssh")
	descriptions = append(descriptions, "Restarting SSH service...")

	return commands, descriptions
}

// ConfigureFirewall sets up UFW with sensible defaults
func ConfigureFirewall(config SecurityConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	// Install UFW if not present
	commands = append(commands, "which ufw > /dev/null || apt-get update && apt-get install -y ufw")
	descriptions = append(descriptions, "Installing UFW firewall if needed...")

	// Reset UFW to defaults
	commands = append(commands, "ufw --force reset")
	descriptions = append(descriptions, "Resetting firewall to defaults...")

	// Set default policies
	commands = append(commands, "ufw default deny incoming")
	commands = append(commands, "ufw default allow outgoing")
	descriptions = append(descriptions, "Setting default firewall policies...")

	// Allow SSH on custom port or default
	sshPort := "22"
	if config.SSHPort != 0 {
		sshPort = fmt.Sprintf("%d", config.SSHPort)
	}
	commands = append(commands, fmt.Sprintf("ufw allow %s/tcp comment 'SSH'", sshPort))
	descriptions = append(descriptions, fmt.Sprintf("Allowing SSH on port %s...", sshPort))

	// Allow HTTP and HTTPS for web services
	commands = append(commands, "ufw allow 80/tcp comment 'HTTP'")
	commands = append(commands, "ufw allow 443/tcp comment 'HTTPS'")
	descriptions = append(descriptions, "Allowing HTTP and HTTPS traffic...")

	// Add custom rules if specified
	for _, rule := range config.FirewallRules {
		ruleCmd := fmt.Sprintf("ufw %s %s/%s comment '%s'", rule.Action, rule.Port, rule.Protocol, rule.Description)
		commands = append(commands, ruleCmd)
		descriptions = append(descriptions, fmt.Sprintf("Adding firewall rule: %s...", rule.Description))
	}

	// Enable firewall
	commands = append(commands, "ufw --force enable")
	descriptions = append(descriptions, "Enabling firewall...")

	// Show firewall status
	commands = append(commands, "ufw status verbose")
	descriptions = append(descriptions, "Displaying firewall status...")

	return commands, descriptions
}

// InstallFail2ban sets up Fail2ban with basic jails
func InstallFail2ban(config SecurityConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	// Install Fail2ban
	commands = append(commands, "apt-get update && apt-get install -y fail2ban")
	descriptions = append(descriptions, "Installing Fail2ban...")

	// Create jail.local configuration
	jailConfig := `[DEFAULT]
bantime = 1800
findtime = 600
maxretry = 3
backend = systemd
banaction = ufw
action = %(action_mwl)s

[sshd]
enabled = true
port = ssh
logpath = %(sshd_log)s
backend = %(sshd_backend)s

[nginx-http-auth]
enabled = true
port = http,https
logpath = /var/log/nginx/error.log

[nginx-limit-req]
enabled = true
port = http,https
logpath = /var/log/nginx/error.log
maxretry = 10

[nginx-botsearch]
enabled = true
port = http,https
logpath = /var/log/nginx/access.log
maxretry = 2`

	// Update SSH port in jail configuration if custom port is used
	if config.SSHPort != 0 && config.SSHPort != 22 {
		jailConfig = strings.Replace(jailConfig, "port = ssh", fmt.Sprintf("port = %d", config.SSHPort), 1)
	}

	commands = append(commands, fmt.Sprintf("cat > /etc/fail2ban/jail.local << 'EOF'\n%s\nEOF", jailConfig))
	descriptions = append(descriptions, "Creating Fail2ban jail configuration...")

	// Start and enable Fail2ban
	commands = append(commands, "systemctl enable fail2ban")
	commands = append(commands, "systemctl restart fail2ban")
	descriptions = append(descriptions, "Starting and enabling Fail2ban...")

	// Show Fail2ban status
	commands = append(commands, "fail2ban-client status")
	descriptions = append(descriptions, "Checking Fail2ban jail status...")

	return commands, descriptions
}

// EnableAutoUpdates configures automatic security updates
func EnableAutoUpdates() ([]string, []string) {
	var commands []string
	var descriptions []string

	// Install unattended-upgrades if not present
	commands = append(commands, "apt-get update && apt-get install -y unattended-upgrades apt-listchanges")
	descriptions = append(descriptions, "Installing automatic updates package...")

	// Configure unattended upgrades
	unattendedConfig := `APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Download-Upgradeable-Packages "1";
APT::Periodic::AutocleanInterval "7";
APT::Periodic::Unattended-Upgrade "1";`

	commands = append(commands, fmt.Sprintf("cat > /etc/apt/apt.conf.d/20auto-upgrades << 'EOF'\n%s\nEOF", unattendedConfig))
	descriptions = append(descriptions, "Configuring automatic updates...")

	// Configure which packages to auto-update
	upgradeConfig := `Unattended-Upgrade::Allowed-Origins {
	"${distro_id}:${distro_codename}";
	"${distro_id}:${distro_codename}-security";
	"${distro_id}ESMApps:${distro_codename}-apps-security";
	"${distro_id}ESM:${distro_codename}-infra-security";
};

Unattended-Upgrade::Package-Blacklist {
};

Unattended-Upgrade::DevRelease "false";
Unattended-Upgrade::Remove-Unused-Dependencies "true";
Unattended-Upgrade::Automatic-Reboot "false";
Unattended-Upgrade::Automatic-Reboot-WithUsers "false";
Unattended-Upgrade::Automatic-Reboot-Time "02:00";`

	commands = append(commands, fmt.Sprintf("cat > /etc/apt/apt.conf.d/50unattended-upgrades << 'EOF'\n%s\nEOF", upgradeConfig))
	descriptions = append(descriptions, "Configuring security update policies...")

	// Enable and start the service
	commands = append(commands, "systemctl enable unattended-upgrades")
	commands = append(commands, "systemctl start unattended-upgrades")
	descriptions = append(descriptions, "Enabling automatic security updates...")

	// Test the configuration
	commands = append(commands, "unattended-upgrade --dry-run --debug")
	descriptions = append(descriptions, "Testing automatic updates configuration...")

	return commands, descriptions
}

// ComprehensiveSecurityHardening runs all Phase 1 security measures
func ComprehensiveSecurityHardening(config SecurityConfig) ([]string, []string) {
	var commands []string
	var descriptions []string

	// Add a header
	descriptions = append(descriptions, "=== PHASE 1: CORE SECURITY HARDENING ===")

	// 1. SSH Hardening
	sshCommands, sshDescriptions := SSHHardening(config)
	commands = append(commands, sshCommands...)
	descriptions = append(descriptions, sshDescriptions...)

	// 2. Firewall Configuration
	fwCommands, fwDescriptions := ConfigureFirewall(config)
	commands = append(commands, fwCommands...)
	descriptions = append(descriptions, fwDescriptions...)

	// 3. Fail2ban Installation
	f2bCommands, f2bDescriptions := InstallFail2ban(config)
	commands = append(commands, f2bCommands...)
	descriptions = append(descriptions, f2bDescriptions...)

	// 4. Automatic Updates
	updateCommands, updateDescriptions := EnableAutoUpdates()
	commands = append(commands, updateCommands...)
	descriptions = append(descriptions, updateDescriptions...)

	// Final security status check
	commands = append(commands, "echo '=== SECURITY HARDENING COMPLETED ==='")
	descriptions = append(descriptions, "Security hardening completed successfully!")

	return commands, descriptions
}

// GetSecurityStatus returns current security status
func GetSecurityStatus() ([]string, []string) {
	var commands []string
	var descriptions []string

	// Header
	commands = append(commands, "echo '=== SECURITY STATUS DASHBOARD ==='")
	descriptions = append(descriptions, "Generating security status overview...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// SSH Status
	commands = append(commands, "echo '🔐 SSH CONFIGURATION STATUS'")
	descriptions = append(descriptions, "Checking SSH security settings...")

	commands = append(commands, "if [ -f /etc/ssh/sshd_config ]; then echo 'Current SSH configuration:'; grep -E '^(Port|PermitRootLogin|PasswordAuthentication)' /etc/ssh/sshd_config | while read line; do echo \"  ✅ $line\"; done; else echo '⚠️  SSH configuration file not found'; fi")
	descriptions = append(descriptions, "Displaying active SSH security settings...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// Firewall Status
	commands = append(commands, "echo '🔥 FIREWALL STATUS'")
	descriptions = append(descriptions, "Checking firewall protection...")

	commands = append(commands, "if command -v ufw >/dev/null 2>&1; then if ufw status | grep -q 'Status: active'; then echo '✅ Firewall: Active and protecting'; ufw status numbered | grep -E '^\\[[0-9]' | head -5; total=$(ufw status numbered | grep -E '^\\[[0-9]' | wc -l); if [ $total -gt 5 ]; then echo \"  ... and $((total - 5)) more rules\"; fi; else echo '⚠️  Firewall: Installed but inactive'; fi; else echo '❌ Firewall: UFW not installed'; fi")
	descriptions = append(descriptions, "Checking firewall rules and status...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// Fail2ban Status
	commands = append(commands, "echo '👁️  INTRUSION DETECTION STATUS'")
	descriptions = append(descriptions, "Checking intrusion detection...")

	commands = append(commands, "if command -v fail2ban-client >/dev/null 2>&1; then if systemctl is-active fail2ban >/dev/null 2>&1; then echo '✅ Fail2ban: Active and monitoring'; fail2ban-client status 2>/dev/null | grep -E 'Jail list:|Currently banned:' || echo '⚠️  Status check failed'; else echo '⚠️  Fail2ban: Installed but not running'; fi; else echo '❌ Fail2ban: Not installed'; fi")
	descriptions = append(descriptions, "Checking Fail2ban protection status...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// Auto Updates Status
	commands = append(commands, "echo '🔄 AUTOMATIC UPDATES STATUS'")
	descriptions = append(descriptions, "Checking automatic update configuration...")

	commands = append(commands, "if systemctl is-active unattended-upgrades >/dev/null 2>&1; then echo '✅ Automatic Updates: Service active'; if [ -f /etc/apt/apt.conf.d/20auto-upgrades ]; then echo '✅ Configuration: Properly configured'; else echo '⚠️  Configuration: Service running but config missing'; fi; else echo '❌ Automatic Updates: Service not active'; fi")
	descriptions = append(descriptions, "Verifying automatic security updates...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	// Recent Security Events
	commands = append(commands, "echo '📊 RECENT SECURITY ACTIVITY'")
	descriptions = append(descriptions, "Checking recent security events...")

	commands = append(commands, "if [ -f /var/log/fail2ban.log ]; then echo 'Recent Fail2ban activity:'; tail -n 5 /var/log/fail2ban.log | while read line; do echo \"  📋 $line\"; done; else echo '📋 No Fail2ban activity logs found'; fi")
	descriptions = append(descriptions, "Displaying recent security events...")

	commands = append(commands, "echo ''")
	descriptions = append(descriptions, "")

	commands = append(commands, "echo '✅ STATUS CHECK COMPLETE'")
	descriptions = append(descriptions, "Security status check completed")

	return commands, descriptions
}

// GenerateSecurityReport creates a comprehensive security report
func GenerateSecurityReport() ([]string, []string) {
	var commands []string
	var descriptions []string

	reportScript := `#!/bin/bash
echo "=========================================="
echo "       CRUCIBLE SECURITY REPORT"
echo "     Generated: $(date)"
echo "=========================================="
echo ""

# System Information
echo "=== SYSTEM INFORMATION ==="
echo "Hostname: $(hostname)"
echo "OS: $(lsb_release -d | cut -f2)"
echo "Kernel: $(uname -r)"
echo "Uptime: $(uptime -p)"
echo ""

# Security Score Calculation
score=0
total=10

echo "=== SECURITY ASSESSMENT ==="

# SSH Security
echo "--- SSH Security ---"
if grep -q "^PermitRootLogin no" /etc/ssh/sshd_config; then
  echo "✅ Root login disabled"
  score=$((score + 2))
else
  echo "❌ Root login enabled (CRITICAL)"
fi

if grep -q "^PasswordAuthentication no" /etc/ssh/sshd_config; then
  echo "✅ Password authentication disabled"
  score=$((score + 2))
else
  echo "⚠️  Password authentication enabled"
fi

if ! grep -q "^Port 22" /etc/ssh/sshd_config && ! grep -q "^#Port 22" /etc/ssh/sshd_config; then
  echo "✅ SSH port changed from default"
  score=$((score + 1))
else
  echo "⚠️  SSH using default port 22"
fi

# Firewall Status
echo "--- Firewall Status ---"
if ufw status | grep -q "Status: active"; then
  echo "✅ Firewall enabled"
  score=$((score + 2))
else
  echo "❌ Firewall disabled (CRITICAL)"
fi

# Fail2ban Status
echo "--- Intrusion Detection ---"
if systemctl is-active --quiet fail2ban; then
  echo "✅ Fail2ban active"
  score=$((score + 2))
else
  echo "❌ Fail2ban not active"
fi

# Auto Updates
echo "--- Automatic Updates ---"
if systemctl is-active --quiet unattended-upgrades; then
  echo "✅ Automatic security updates enabled"
  score=$((score + 1))
else
  echo "⚠️  Automatic updates disabled"
fi

echo ""
echo "=== SECURITY SCORE ==="
echo "Score: $score/$total"
percentage=$((score * 100 / total))
echo "Percentage: $percentage%"

if [ $percentage -ge 80 ]; then
  echo "Status: ✅ SECURE"
elif [ $percentage -ge 60 ]; then
  echo "Status: ⚠️  MODERATE"
else
  echo "Status: ❌ CRITICAL - IMMEDIATE ACTION REQUIRED"
fi

echo ""
echo "=========================================="
echo "        END OF SECURITY REPORT"
echo "=========================================="
`

	commands = append(commands, fmt.Sprintf("cat > /tmp/security_report.sh << 'EOF'\n%s\nEOF", reportScript))
	commands = append(commands, "chmod +x /tmp/security_report.sh")
	commands = append(commands, "/tmp/security_report.sh")
	commands = append(commands, "rm -f /tmp/security_report.sh")

	descriptions = append(descriptions, "Generating comprehensive security report...")
	descriptions = append(descriptions, "Executing security assessment...")
	descriptions = append(descriptions, "Displaying security report...")
	descriptions = append(descriptions, "Cleaning up temporary files...")

	return commands, descriptions
}
