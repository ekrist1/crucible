.PHONY: build build-monitor clean install test run run-monitor

# Build the main TUI application
build:
	go build -o crucible .

# Build the monitoring agent
build-monitor:
	go build -o crucible-monitor ./cmd/crucible-monitor

# Build both applications
build-all: build build-monitor

# Clean build artifacts
clean:
	rm -f crucible crucible-monitor

# Install dependencies
install:
	go mod tidy
	go mod download

# Test the application (build only since TUI can't run in CI)
test:
	go build -o crucible .
	@echo "Build successful - TUI application ready"

# Run the application (requires TTY)
run: build
	sudo ./crucible

# Run the monitoring agent
run-monitor: build-monitor
	./crucible-monitor

# Development build with race detection
dev:
	go build -race -o crucible .

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# All checks
check: fmt vet test