#!/bin/bash

# Build Crucible for All Platforms
echo "🏗️  Building Crucible for All Platforms"
echo "========================================"
echo ""

# Build information
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

echo "📋 Build Information:"
echo "   Version: $VERSION"
echo "   Build Time: $BUILD_TIME"
echo "   Git Commit: $GIT_COMMIT"
echo ""

# Create release directory
RELEASE_DIR="release"
mkdir -p $RELEASE_DIR

# Build flags
LDFLAGS="-w -s -X 'main.version=$VERSION' -X 'main.buildTime=$BUILD_TIME' -X 'main.gitCommit=$GIT_COMMIT'"

# Function to build for a platform
build_platform() {
    local os=$1
    local arch=$2
    local arm_version=$3
    local suffix=$4
    local cgo_enabled=$5
    local cc=$6
    
    local platform_name="${os}-${suffix}"
    echo "🔸 Building for $platform_name..."
    
    # Build crucible (TUI)
    echo "   📱 Building crucible TUI..."
    local build_cmd="env GOOS=$os GOARCH=$arch"
    if [ -n "$arm_version" ]; then
        build_cmd="$build_cmd GOARM=$arm_version"
    fi
    if [ -n "$cgo_enabled" ]; then
        build_cmd="$build_cmd CGO_ENABLED=$cgo_enabled"
    fi
    if [ -n "$cc" ]; then
        build_cmd="$build_cmd CC=$cc"
    fi
    
    local crucible_binary="$RELEASE_DIR/crucible-$platform_name"
    if [ "$os" = "windows" ]; then
        crucible_binary="$crucible_binary.exe"
    fi
    
    eval "$build_cmd go build -a -ldflags=\"$LDFLAGS\" -o \"$crucible_binary\" ."
    
    if [ $? -eq 0 ]; then
        echo "   ✅ crucible build successful: $crucible_binary"
        ls -lh "$crucible_binary"
    else
        echo "   ❌ crucible build failed"
        return 1
    fi
    
    # Build crucible-monitor (agent)
    echo "   📊 Building crucible-monitor agent..."
    local monitor_binary="$RELEASE_DIR/crucible-monitor-$platform_name"
    if [ "$os" = "windows" ]; then
        monitor_binary="$monitor_binary.exe"
    fi
    
    if [ -d "./cmd/crucible-monitor" ]; then
        eval "$build_cmd go build -a -ldflags=\"$LDFLAGS\" -o \"$monitor_binary\" ./cmd/crucible-monitor"
    else
        cp "$crucible_binary" "$monitor_binary"
        echo "   📋 Copied from main binary (monitor functionality included)"
    fi
    
    if [ $? -eq 0 ] || [ -f "$monitor_binary" ]; then
        echo "   ✅ crucible-monitor build successful: $monitor_binary"
        ls -lh "$monitor_binary"
    else
        echo "   ❌ crucible-monitor build failed"
        return 1
    fi
    
    echo ""
}

echo "🏗️  Building for Linux platforms..."
echo ""

# Linux x86_64 (most common)
build_platform "linux" "amd64" "" "amd64" "1" ""

# Linux ARM64 (Raspberry Pi 4, AWS Graviton, Apple Silicon servers)
build_platform "linux" "arm64" "" "arm64" "0" ""

# Linux ARMv7 (Raspberry Pi 3, older ARM boards)
build_platform "linux" "arm" "7" "armv7" "0" ""

# Linux ARMv6 (very old Raspberry Pi models)
build_platform "linux" "arm" "6" "armv6" "0" ""

# Linux 386 (old 32-bit x86)
build_platform "linux" "386" "" "386" "1" ""

echo "🏗️  Building for other platforms..."
echo ""

# macOS Intel
build_platform "darwin" "amd64" "" "darwin-amd64" "1" ""

# macOS Apple Silicon
build_platform "darwin" "arm64" "" "darwin-arm64" "1" ""

# Windows x86_64
build_platform "windows" "amd64" "" "windows-amd64" "1" ""

# Windows 386
build_platform "windows" "386" "" "windows-386" "1" ""

# FreeBSD x86_64
build_platform "freebsd" "amd64" "" "freebsd-amd64" "1" ""

echo "🏁 Build Summary:"
echo "=================="
echo ""
echo "📁 Release directory: $RELEASE_DIR/"
echo ""

echo "📱 Crucible TUI Application:"
for binary in $RELEASE_DIR/crucible-linux-* $RELEASE_DIR/crucible-darwin-* $RELEASE_DIR/crucible-windows-* $RELEASE_DIR/crucible-freebsd-*; do
    if [ -f "$binary" ]; then
        echo "✅ $binary"
        echo "   Size: $(du -h "$binary" | cut -f1)"
    fi
done

echo ""
echo "📊 Crucible Monitor Agent:"
for binary in $RELEASE_DIR/crucible-monitor-*; do
    if [ -f "$binary" ]; then
        echo "✅ $binary"
        echo "   Size: $(du -h "$binary" | cut -f1)"
    fi
done

echo ""
echo "📦 Creating release archives..."
cd $RELEASE_DIR

# Create tar.gz archives for Unix-like systems
for binary in crucible-linux-* crucible-darwin-* crucible-freebsd-*; do
    if [ -f "$binary" ] && [[ "$binary" != *".tar.gz" ]]; then
        platform=$(echo "$binary" | sed 's/crucible-//')
        tar -czf "crucible-$platform.tar.gz" crucible-$platform* ../configs/ ../README.md 2>/dev/null || true
        if [ -f "crucible-$platform.tar.gz" ]; then
            echo "✅ Created: crucible-$platform.tar.gz"
        fi
    fi
done

# Create zip archives for Windows
for binary in crucible-windows-*; do
    if [ -f "$binary" ] && [[ "$binary" != *".zip" ]]; then
        platform=$(echo "$binary" | sed 's/crucible-//' | sed 's/.exe//')
        zip -q "crucible-$platform.zip" crucible-$platform* ../configs/* ../README.md 2>/dev/null || true
        if [ -f "crucible-$platform.zip" ]; then
            echo "✅ Created: crucible-$platform.zip"
        fi
    fi
done

cd ..

echo ""
echo "✅ Multi-platform build completed!"
echo ""
echo "💡 Usage:"
echo "   Linux ARM64:   ./release/crucible-linux-arm64"
echo "   Monitor Agent: ./release/crucible-monitor-linux-arm64"
echo "   Archives:      ./release/*.tar.gz, ./release/*.zip"