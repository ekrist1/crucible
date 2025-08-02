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

Crucible is a terminal-based VPS server setup tool built with **Bubble Tea TUI framework**. The application uses a **state machine pattern** with three primary states and **form-based input handling**. Crucible makes it easy to get up and running with a new VPS. Crucible supports framework like Laravel, Nextjs, Python and more.

Crucible-monitor is a part of this application, and is a modern and modular monitoring system for Linux, services (example MySql, Caddy) and framework like Laravel and Nextjs.

### Core Application States
- **`stateMenu`**: Main menu navigation with service status indicators
- **`stateInput`**: Multi-step form input collection (site names, domains, credentials)  
- **`stateProcessing`**: Command execution and result display

### File Structure & Responsibilities

#### Core TUI Application (`internal/tui/`)

**`tui.go`**: Central TUI framework and state management
- Bubble Tea model with state machine (`AppState` enum: Menu, Submenu, Input, Processing, LogViewer, ServiceList)
- Menu hierarchy tracking (`MenuLevel` enum: Main, CoreServices, LaravelManagement, ServerManagement, Settings)
- Service installation status tracking (`serviceStatus map[string]bool`)
- Asynchronous command execution with progress tracking
- Terminal styling with ANSI color codes (0-15 for compatibility)

**`views.go`**: TUI view rendering and display logic
- State-specific view rendering (menu, input, processing)
- Form input display with cursor position visualization
- Report pattern implementation for structured output
- Service status indicators and progress spinners

**`form_handlers.go`**: Multi-step form workflows and input processing
- Enhanced input handling with cursor navigation (arrow keys, backspace, delete)
- Laravel site creation workflow (name → domain → git repo)
- Site update selection from available Laravel installations
- MySQL backup configuration (credentials → host → path)
- Character insertion/deletion with proper cursor positioning

**`menu_handlers.go`**: Menu navigation and selection logic
- Multi-level menu system with submenu support
- Service installation menu options
- Laravel management operations
- Server management utilities

**`settings.go`**: Settings menu and configuration management
- API key management interface
- Email configuration wizard
- Settings persistence to .env files
- Configuration validation and testing
- Secure credential storage

#### Monitoring System (`internal/monitor/`)

**`alerts/types.go`**: Core alert system types and interfaces (196 lines)
- Alert types: `Alert`, `AlertRule`, `AlertConditions`, `AlertManager`
- Severity levels: Info, Warning, Critical
- Alert statuses: Firing, Resolved, Acknowledged, Suppressed
- Evaluation context and metrics data structures

**`alerts/engine.go`**: Alert evaluation engine and rule processing (437 lines)
- Rule evaluation with threshold-based conditions
- System metrics monitoring (CPU, memory, disk, load)
- Service status checking and HTTP endpoint monitoring
- Alert lifecycle management (firing, resolving, deduplication)
- Concurrent rule evaluation with goroutines

**`alerts/config.go`**: Configuration management and YAML parsing (291 lines)
- YAML configuration loading with duration parsing
- Environment variable override support
- Rule conversion from config to internal types
- Default configuration creation

**`alerts/keymanager.go`**: Secure API key management (294 lines)
- .env file operations for credential storage
- Interactive API key setup and validation
- Resend API key testing functionality
- Secure file permissions management

**`alerts/notifiers/email.go`**: Email notification implementation (337 lines)
- Resend API integration for email delivery
- HTML and text email template generation
- Alert severity-based styling and icons
- Email configuration and recipient management

**`alerts/notifiers/types.go`**: Notifier interface and alert types
- Notifier interface definition
- Alert types compatible with email system
- Email configuration structures

#### Legacy Core Application

**`main.go`**: Application entry point and legacy TUI logic
- Original Bubble Tea implementation (being refactored)
- Service installation status tracking
- Basic input handling and form validation

**`install.go`**: System service installation functions
- OS detection (Ubuntu/Fedora) with different package managers
- Component installation: PHP 8.4/8.5, Composer, MySQL, Caddy, Git
- System command execution with proper error handling

**`forms.go`**: Legacy form workflows (being migrated to `internal/tui/`)
- Laravel site creation workflow
- Site update selection
- MySQL backup configuration
- Shared utility functions for Laravel operations

**`utils.go`**: System monitoring and utility functions  
- Service status checking and system health monitoring
- PHP version upgrade functionality
- Caddy configuration updates for new PHP versions

#### Configuration Files

**`configs/alerts.yaml`**: Alert system configuration
- 12 pre-configured monitoring rules (CPU, memory, disk, services)
- Email notification templates and settings
- Rate limiting and notification channel configuration

**`.env`**: Environment configuration (created by Settings menu)
- Resend API key storage
- Email configuration settings
- Alert system credentials

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

## Important Notes

Remember to update the README.md when there are key changes to the application.

The MONITORING.md contains key information about how to use and configure the monitoring service.

The MONITORING_ROADMAP.md contains the roadmap of the monitoring service. Update the roadmap when implemening new features.

Use the context7 MCP if you don't have knowledge about spesific Bubble Tea TUI implementation practises. 

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

# Go Coding Standards and Best Practices

This document outlines best practices for writing clean, idiomatic, and maintainable Go code. These guidelines align with the Go community's conventions, as seen in the standard library, [Effective Go](https://go.dev/doc/effective_go), and tools like `golangci-lint`. They are designed to help new Go developers write consistent and efficient code.

## 1. Code Formatting
- **Use `gofmt`**: Always run `gofmt` to format your code. It enforces a consistent style (e.g., tabs for indentation, no trailing whitespace).
  - Run: `gofmt -w .` in your project directory.
  - Use `goimports` to automatically manage imports: `goimports -w .`.
- **Line Length**: Aim for lines under 100 characters for readability, but don't sacrifice clarity for strict adherence.
- **Naming**:
  - Use **camelCase** for local variables and unexported fields (e.g., `alertManager`, `config`).
  - Use **PascalCase** for exported names (e.g., `AlertManager`, `NewAlertManager`).
  - Keep names concise but descriptive (e.g., `am` for `AlertManager` in local scope).
  - Avoid stuttering (e.g., don’t name a field `AlertRuleID` in `AlertRule`; just use `ID`).

## 2. Code Organization
- **Package Structure**:
  - Keep packages small and focused (e.g., `alerts` for alert management, `notifiers` for notification logic).
  - Use meaningful package names that describe functionality (e.g., `alerts` instead of `alertmanager`).
  - Avoid deep nesting; prefer flat directory structures (e.g., `internal/monitor/alerts` is fine, but avoid excessive depth).
- **File Organization**:
  - Group related types and functions together (e.g., `AlertManager` and its methods in one file).
  - Start files with package-level documentation: `// Package alerts manages alert rules and notifications`.
  - Order contents: package comment, imports, constants, types, functions, methods.
- **Imports**:
  - Group imports in three blocks (with blank lines between): standard library, third-party, internal packages.
  - Example:
    ```go
    import (
        "fmt"
        "log"
        "time"

        "github.com/google/uuid"

        "crucible/internal/monitor/alerts/notifiers"
    )
    ```
  - Avoid import aliases unless necessary (e.g., to resolve conflicts).

## 3. Error Handling
- **Handle Errors Explicitly**: Always check errors; don’t ignore them.
  - Bad: `_, err := doSomething() // ignoring error`.
  - Good:
    ```go
    result, err := doSomething()
    if err != nil {
        return fmt.Errorf("failed to do something: %w", err)
    }
    ```
- **Use `fmt.Errorf` with `%w`**: Wrap errors to provide context while preserving the original error for inspection.
- **Centralize Error Handling**: For repetitive checks, consider a helper function or middleware pattern.
- **Avoid Panic**: Use `panic` only for unrecoverable errors (e.g., programmer mistakes). Recover with `defer` if needed.

## 4. Structs and Interfaces
- **Structs**:
  - Use pointers for structs when passing to functions to avoid copying and allow modification (e.g., `func NewAlertManager(config *Config) *AlertManager`).
  - Initialize structs with meaningful defaults:
    ```go
    am := &AlertManager{
        rules:        make(map[string]*AlertRule),
        activeAlerts: make(map[string]*Alert),
        notifiers:    make([]Notifier, 0),
    }
    ```
  - Avoid embedding structs unless the relationship is clear (e.g., "is-a" vs. "has-a").
- **Interfaces**:
  - Define interfaces at the point of use (e.g., `Notifier` in `notifiers` package).
  - Keep interfaces small and focused (e.g., `Send(alert *Alert) error` for `Notifier`).
  - Name interfaces descriptively, often ending in `-er` (e.g., `Notifier`, `Reader`).
  - Use implicit interface satisfaction; don’t explicitly declare implementation.

## 5. Concurrency
- **Use Goroutines Judiciously**: Use `go` for parallel tasks, but synchronize with `sync.WaitGroup` or channels.
  - Example from your code:
    ```go
    var wg sync.WaitGroup
    for _, rule := range am.rules {
        wg.Add(1)
        go func(r *AlertRule) {
            defer wg.Done()
            am.evaluateRule(r, ctx)
        }(rule)
    }
    wg.Wait()
    ```
  - Pass a copy of loop variables to goroutines to avoid race conditions (e.g., `r *AlertRule` above).
- **Avoid Shared State**: Prefer channels for communication over shared memory. Use `sync.Mutex` if shared state is unavoidable.
- **Thread Safety**: Ensure maps are not accessed concurrently (e.g., `am.rules` may need a mutex if accessed outside `EvaluateRules`).

## 6. Functions and Methods
- **Keep Functions Small**: Aim for single-responsibility functions (e.g., `checkSystemCondition`, `sendNotifications`).
- **Use Named Returns Sparingly**: Only use named return values when they clarify intent (e.g., `func (am *AlertManager) GetRules() (rules map[string]*AlertRule)`).
- **Receiver Types**: Use pointer receivers (`*AlertManager`) for methods that modify the receiver or for large structs:
  ```go
  func (am *AlertManager) AddRule(rule *AlertRule) {
      am.rules[rule.ID] = rule
  }
  ```
- **Return Early**: Reduce nesting with early returns:
  ```go
  if !rule.Enabled {
      return
  }
  ```

## 7. Documentation
- **Comment Exported Items**: Every exported function, type, or method should have a comment starting with its name:
  ```go
  // NewAlertManager creates a new alert manager instance.
  func NewAlertManager(config *Config) *AlertManager { ... }
  ```
- **Be Concise**: Comments should explain *why*, not *what* (e.g., avoid “loops through rules”; explain intent if complex).
- **Use Godoc**: Run `godoc -http=:6060` to preview documentation locally.

## 8. Testing
- **Write Tests**: Place tests in files named `*_test.go` in the same package (e.g., `alerts_test.go`).
- **Use Table-Driven Tests**: For multiple test cases:
  ```go
  tests := []struct {
      name     string
      input    *Config
      expected int
  }{
      {"EmptyConfig", &Config{}, 0},
      {"EmailEnabled", &Config{Email: EmailConfig{Enabled: true}}, 1},
  }
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
          am := NewAlertManager(tt.input)
          if len(am.notifiers) != tt.expected {
              t.Errorf("got %d notifiers, want %d", len(am.notifiers), tt.expected)
          }
      })
  }
  ```
- **Test Coverage**: Aim for high coverage (`go test -cover`). Focus on critical paths (e.g., `EvaluateRules`).
- **Use `testing.T` Helpers**: Like `t.Fatal` for stopping tests on failure.

## 9. Performance
- **Use `make` for Slices/Maps**: Pre-allocate capacity when possible:
  ```go
  alerts := make([]*Alert, 0, len(am.activeAlerts))
  ```
- **Avoid Unnecessary Allocations**: Use pointers for structs; reuse buffers where applicable.
- **Profile Code**: Use `pprof` (`go test -cpuprofile`) to identify bottlenecks.

## 10. Logging and Debugging
- **Use Standard `log`**: For simple logging, as in your code:
  ```go
  log.Printf("Alert fired: %s - %s", alert.Name, alert.Message)
  ```
- **Structured Logging**: For production, consider `go.uber.org/zap` or `logrus` for JSON logs.
- **Contextual Logging**: Include relevant details (e.g., alert ID, rule name).

## 11. Dependency Management
- **Use Go Modules**: Initialize with `go mod init <module-name>` (e.g., `crucible`).
- **Minimize Dependencies**: Only import what’s needed (e.g., `github.com/google/uuid` for IDs).
- **Vendoring**: Optional, but use `go mod vendor` for air-gapped environments.

## 12. Idiomatic Go
- **Simplicity**: Favor clear, straightforward code over clever solutions.
- **Zero Values**: Leverage Go’s zero values (e.g., `nil` for pointers, `0` for ints) instead of explicit initialization when possible.
- **Avoid Naked Returns**: They can obscure intent.
- **Use `defer`**: For cleanup (e.g., `defer wg.Done()` in your code).

## 13. Tooling
- **Linters**: Use `golangci-lint` to catch issues (e.g., unused variables, shadowing).
  - Install: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
  - Run: `golangci-lint run`
- **Static Analysis**: Use `go vet` and `staticcheck` for additional checks.
- **Build and Test**: Use `go build` and `go test` regularly.

## 14. Applying to Your `alerts` Package
Your `alerts` package already follows many best practices, but here are specific suggestions:
- **Add Documentation**: Add comments for exported types (`AlertManager`, `GenerateID`).
- **Thread Safety**: Add a `sync.RWMutex` for `rules` and `activeAlerts` if accessed concurrently outside `EvaluateRules`.
- **Error Handling**: In `sendNotifications`, consider aggregating errors instead of logging and continuing:
  ```go
  var errs []error
  for _, notifier := range am.notifiers {
      if err := notifier.Send(alert); err != nil {
          errs = append(errs, fmt.Errorf("notifier %s: %w", notifier.Name(), err))
      }
  }
  if len(errs) > 0 {
      return errors.Join(errs...)
  }
  ```
- **Testing**: Add tests for `EvaluateRules` and `checkSystemCondition` with mock `EvaluationContext`.

## Resources
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide)
- [Go Proverbs](https://go-proverbs.github.io/)

Follow these practices, and your Go code will be idiomatic, maintainable, and efficient. Start small, use `gofmt`, and write tests early!