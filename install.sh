#!/bin/bash

# Crucible Installation Script
set -e

echo "🔧 Installing Crucible - Laravel Server Setup Tool"
echo "=================================================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21 or higher first."
    echo "   Visit: https://golang.org/doc/install"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | grep -oE '[0-9]+\.[0-9]+')
if [[ "$(printf '%s\n' "$GO_VERSION" "1.21" | sort -V | head -n1)" != "1.21" ]]; then
    echo "❌ Go version $GO_VERSION is too old. Please upgrade to Go 1.21 or higher."
    exit 1
fi

echo "✅ Go version $GO_VERSION detected"

# Install dependencies
echo "📦 Installing dependencies..."
go mod tidy

# Build the application
echo "🔨 Building Crucible..."
go build -o crucible .

# Make executable
chmod +x crucible

echo "✅ Crucible built successfully!"
echo ""
echo "Usage:"
echo "  sudo ./crucible    # Run with sudo privileges (required)"
echo ""
echo "Features:"
echo "  - Install PHP 8.4/8.5, Composer, MySQL, Caddy, Git"
echo "  - Create and manage Laravel sites"
echo "  - Update sites from Git repositories"
echo "  - Backup MySQL databases"
echo "  - Monitor system status"
echo ""
echo "Note: Run with sudo privileges to install system packages and configure services."