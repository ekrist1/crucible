.PHONY: build clean install test run

# Build the application
build:
	go build -o crucible .

# Clean build artifacts
clean:
	rm -f crucible

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