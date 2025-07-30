# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building and Running
```bash
# Build the application
make build
# or
go build -o crucible

# Run with required sudo privileges
make run
# or  
sudo ./crucible

# Development build with race detection
make dev

# Install/update dependencies
make install
# or
go mod tidy
```

### Code Quality
```bash
# Run all checks (format, vet, build)
make check

# Format code
make fmt

# Vet code  
make vet

# Test build (TUI apps can't run traditional tests)
make test
```

## Architecture Overview

Crucible is a terminal-based Laravel server setup tool built with **Bubble Tea TUI framework**. The application uses a **state machine pattern** with three primary states and **form-based input handling**.

### Core Application States
- **`stateMenu`**: Main menu navigation with service status indicators
- **`stateInput`**: Multi-step form input collection (site names, domains, credentials)  
- **`stateProcessing`**: Command execution and result display

### File Structure & Responsibilities

**`main.go`**: Central TUI logic and state management
- Bubble Tea model with state machine (`appState` enum)
- Service installation status tracking (`serviceStatus map[string]bool`)
- Input handling with real-time form validation
- Terminal styling with ANSI color codes (0-15 for compatibility)

**`install.go`**: System service installation functions
- OS detection (Ubuntu/Fedora) with different package managers
- Component installation: PHP 8.4/8.5, Composer, MySQL, Caddy, Git
- System command execution with proper error handling

**`forms.go`**: Multi-step form workflows and input processing
- Laravel site creation workflow (name → domain → git repo)
- Site update selection from available Laravel installations
- MySQL backup configuration (credentials → host → path)
- Shared utility functions for Laravel operations

**`utils.go`**: System monitoring and utility functions  
- Service status checking and system health monitoring
- PHP version upgrade functionality
- Caddy configuration updates for new PHP versions

**`laravel.go`**: Legacy placeholder (functions moved to `forms.go`)

### Key Design Patterns

**State Management**: Uses pointer receivers (`*model`) for status updates while maintaining value receivers for TUI operations to avoid conflicts with Bubble Tea's model interface.

**Service Status Tracking**: Real-time installation status with visual indicators (✅/⬜) that refresh after operations and on manual refresh (`r` key).

**Form Flow**: Sequential input collection with validation at each step, allowing cancellation (`Esc`) and graceful error handling.

**Cross-Platform Support**: OS detection and conditional command execution for Ubuntu (`apt`) and Fedora (`dnf`) package managers.

## Laravel Integration

Creates sites in `/var/www/[site-name]` with proper ownership (`www-data:www-data`) and permissions. Generates Caddy configurations with:
- PHP-FPM Unix socket integration  
- Security headers and Laravel URL rewriting
- Modular configuration in `/etc/caddy/sites/[domain].caddy`

## Important Context

- **Requires sudo**: All operations need elevated privileges for system package installation
- **Terminal compatibility**: Uses ANSI color codes instead of hex colors for broader terminal support
- **No traditional tests**: TUI applications use build validation instead of unit tests
- **Logging**: Structured logging with Charm Log for all operations and errors
- **Security focus**: Runs `mysql_secure_installation` and implements Laravel security best practices

## TUI Output Formatting Guidelines

### Avoiding Misalignment Issues

**Problem**: Using multiple `logger.Info()` or `logger.Warn()` calls for structured output creates misalignment due to repeated timestamps, log levels, and prefixes on every line.

**Solution**: For cohesive reports or structured output, use the **report pattern**:

1. **Build styled strings** using Lipgloss instead of logging directly
2. **Store in model.report []string** field for structured display
3. **Render in View()** function when processing is complete
4. **Use helper functions** that return styled strings instead of logging

### Report Pattern Implementation

```go
// ✅ CORRECT: Build report as styled strings
func (m model) showSystemStatus() (tea.Model, tea.Cmd) {
    m.report = []string{}
    m.report = append(m.report, infoStyle.Render("=== SYSTEM STATUS ==="))
    m.report = append(m.report, m.getServiceStatus("PHP", "php", "--version"))
    // Render in viewProcessing() when complete
    return m, nil
}

func (m model) getServiceStatus(name, command string, args ...string) string {
    // Return styled string instead of logging
    return infoStyle.Render(fmt.Sprintf("✅ %s: %s", name, version))
}
```

```go
// ❌ INCORRECT: Multiple logger calls create misalignment
func (m model) showSystemStatus() (tea.Model, tea.Cmd) {
    m.logger.Info("=== SYSTEM STATUS ===")
    m.logger.Info("✅ PHP: 8.4.1")
    m.logger.Info("✅ MySQL: 8.0.35")
    // Creates: [timestamp] INFO [prefix] message (repeated)
    return m, nil
}
```

### When to Use Each Approach

- **Use logger**: For individual operations, errors, and debug information
- **Use report pattern**: For structured output, status reports, and multi-line displays that need visual alignment
- **Available styles**: `infoStyle` (green), `warnStyle` (yellow), `titleStyle`, `selectedStyle`, `choiceStyle`