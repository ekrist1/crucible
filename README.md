# 🔧 Crucible - Laravel Server Setup Tool

Crucible is a terminal-based tool built with Bubble Tea that simplifies setting up Ubuntu or Fedora servers for Laravel applications. It provides an interactive menu to install and configure all necessary components for a production Laravel environment.

## Features

- **PHP Management**: Install PHP 8.4 with option to upgrade to PHP 8.5
- **Composer**: Install PHP Composer for dependency management
- **MySQL**: Install MySQL server with best practice configurations
- **Caddy Server**: Install and configure Caddy with Laravel-optimized settings
- **Git CLI**: Install Git for version control
- **Laravel Site Management**: 
  - Create new Laravel sites in `/var/www`
  - Pull Laravel projects from GitHub repositories
  - Update existing Laravel sites with `git pull`
- **Database Backup**: Backup MySQL databases over SSH to local machines
- **System Status**: Monitor the health of all installed services
- **Logging**: Uses Charm Log for comprehensive logging

## Supported Operating Systems

- Ubuntu (18.04+)
- Fedora (30+)

## Prerequisites

- Go 1.21 or higher
- Root/sudo access on the target server
- SSH access for database backups (optional)

## Installation

1. Clone or download the project:
```bash
git clone <repository-url>
cd crucible
```

2. Install dependencies:
```bash
go mod tidy
```

3. Build the application:
```bash
go build -o crucible
```

4. Run with sudo privileges:
```bash
sudo ./crucible
```

## Usage

Launch Crucible and use the interactive menu to:

1. **Install Components**: Start by installing PHP, Composer, MySQL, Caddy, and Git
2. **Create Laravel Sites**: Set up new Laravel applications with domain configuration
3. **Manage Sites**: Update existing sites and manage deployments
4. **Monitor System**: Check service status and site health
5. **Backup Data**: Create and transfer MySQL backups

### Navigation

- Use **↑/↓** or **k/j** to navigate the menu
- Press **Enter** or **Space** to select an option
- Press **q** or **Ctrl+C** to quit

## Laravel Site Structure

Sites are created in `/var/www/[site-name]` with proper permissions:
- Owner: `www-data:www-data`
- Directories: `755`
- Files: `644`
- Storage/Cache: `775`

## Caddy Configuration

Crucible creates optimized Caddy configurations for Laravel:

- PHP-FPM integration with Unix sockets
- Security headers (XSS, CSRF protection)
- Gzip compression
- Static file serving
- Laravel-specific URL rewriting

### Configuration Files

- Main Caddyfile: `/etc/caddy/Caddyfile`
- Laravel snippet: `/etc/caddy/snippets/laravel.caddy`
- Site configs: `/etc/caddy/sites/[domain].caddy`

## MySQL Best Practices

Crucible configures MySQL with security best practices:
- Runs `mysql_secure_installation`
- Enables automatic startup
- Optimizes for Laravel workloads

## Development

The project is structured as follows:

- `main.go`: Main application and UI logic
- `install.go`: Component installation functions
- `laravel.go`: Laravel site management
- `utils.go`: Backup, status, and utility functions

### Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea): TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss): Terminal styling
- [Charm Log](https://github.com/charmbracelet/log): Structured logging

## Security Considerations

- Always run with appropriate privileges
- Secure MySQL installations with strong passwords
- Configure firewalls appropriately
- Use HTTPS in production with proper SSL certificates
- Regularly update system packages and PHP versions

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure you're running with sudo privileges
2. **Package Not Found**: Update package repositories before installation
3. **Service Start Failures**: Check system logs with `journalctl`
4. **Caddy Configuration Errors**: Validate syntax with `caddy validate`

### Logs

Crucible uses structured logging. Check the output for detailed information about operations and any errors encountered.

### System Status

Use the "System Status" option in the menu to check:
- Service installation status
- Service health
- Available Laravel sites
- Disk usage

## Contributing

Contributions are welcome! Please ensure:
- Code follows Go conventions
- All functions include proper error handling
- Log important operations and errors
- Test on both Ubuntu and Fedora

## License

This project is open source and available under the MIT License.

## Support

For issues and questions:
1. Check the troubleshooting section
2. Review system logs
3. Ensure proper permissions and prerequisites