#!/bin/bash

# Build Static ARM Binaries for Crucible
echo "🔧 Building Static ARM Binaries for Crucible"
echo "============================================="
echo ""

# Set build variables
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

echo "📋 Build Information:"
echo "   Version: $VERSION"
echo "   Build Time: $BUILD_TIME"
echo "   Git Commit: $GIT_COMMIT"
echo ""

# Build flags for static linking
LDFLAGS="-w -s -extldflags '-static' -X 'main.version=$VERSION' -X 'main.buildTime=$BUILD_TIME' -X 'main.gitCommit=$GIT_COMMIT'"
MONITOR_LDFLAGS="-w -s -extldflags '-static'"  # Monitor uses constants, not variables for version info

echo "🏗️  Building both crucible (TUI) and crucible-monitor (agent)..."
echo "    Using CGO for better SQLite performance"
echo ""

# Function to build for a specific architecture
build_for_arch() {
    local arch=$1
    local goarch=$2
    local goarm=$3
    local cc=$4
    
    echo "🔸 Building for $arch ($goarch)..."
    
    # Build crucible (main TUI application)
    echo "   📱 Building crucible TUI..."
    if [ -n "$goarm" ]; then
        env GOOS=linux GOARCH=$goarch GOARM=$goarm CGO_ENABLED=1 CC=$cc \
            go build -a -ldflags="$LDFLAGS" -o "crucible-linux-$arch" .
    else
        env GOOS=linux GOARCH=$goarch CGO_ENABLED=1 CC=$cc \
            go build -a -ldflags="$LDFLAGS" -o "crucible-linux-$arch" .
    fi
    
    if [ $? -eq 0 ]; then
        echo "   ✅ crucible build successful: crucible-linux-$arch"
        ls -lh "crucible-linux-$arch"
    else
        echo "   ❌ crucible build failed (may need cross-compiler: $cc)"
        return 1
    fi
    
    # Build crucible-monitor (monitoring agent)
    echo "   📊 Building crucible-monitor agent..."
    if [ -d "./cmd/crucible-monitor" ]; then
        # Build from separate monitor command
        if [ -n "$goarm" ]; then
            env GOOS=linux GOARCH=$goarch GOARM=$goarm CGO_ENABLED=1 CC=$cc \
                go build -a -ldflags="$MONITOR_LDFLAGS" -o "crucible-monitor-linux-$arch" ./cmd/crucible-monitor
        else
            env GOOS=linux GOARCH=$goarch CGO_ENABLED=1 CC=$cc \
                go build -a -ldflags="$MONITOR_LDFLAGS" -o "crucible-monitor-linux-$arch" ./cmd/crucible-monitor
        fi
    else
        # Copy from main binary since monitor is part of main
        cp "crucible-linux-$arch" "crucible-monitor-linux-$arch"
        echo "   📋 Copied from main binary (monitor functionality included)"
    fi
    
    if [ $? -eq 0 ] || [ -f "crucible-monitor-linux-$arch" ]; then
        echo "   ✅ crucible-monitor build successful: crucible-monitor-linux-$arch"
        ls -lh "crucible-monitor-linux-$arch"
    else
        echo "   ❌ crucible-monitor build failed"
        return 1
    fi
    
    echo ""
}

# Check if cross-compilers are available
echo "🔍 Checking for cross-compilation tools..."
for compiler in aarch64-linux-gnu-gcc arm-linux-gnueabihf-gcc; do
    if command -v $compiler >/dev/null 2>&1; then
        echo "✅ Found: $compiler"
    else
        echo "❌ Missing: $compiler"
        echo "   Install with: ./install-cross-compilers.sh"
    fi
done
echo ""

# ARM64 (most common for modern ARM servers like Raspberry Pi 4, AWS Graviton)
build_for_arch "arm64" "arm64" "" "aarch64-linux-gnu-gcc"

# ARMv7 (32-bit ARM, older Raspberry Pi models)
build_for_arch "armv7" "arm" "7" "arm-linux-gnueabihf-gcc"

# ARMv6 (very old ARM devices)
build_for_arch "armv6" "arm" "6" "arm-linux-gnueabihf-gcc"

echo "🏁 Build Summary:"
echo "=================="
echo ""
echo "📱 Crucible TUI Application:"
for binary in crucible-linux-arm*; do
    if [ -f "$binary" ]; then
        echo "✅ $binary ($(file $binary | cut -d: -f2 | cut -d, -f1))"
        echo "   Size: $(du -h $binary | cut -f1)"
    fi
done

echo ""
echo "📊 Crucible Monitor Agent:"
for binary in crucible-monitor-linux-arm*; do
    if [ -f "$binary" ]; then
        echo "✅ $binary ($(file $binary | cut -d: -f2 | cut -d, -f1))"
        echo "   Size: $(du -h $binary | cut -f1)"
    fi
done

echo ""
echo "💡 Usage:"
echo "   Copy the appropriate binaries to your ARM device:"
echo "   scp crucible-linux-arm64 user@arm-server:/opt/crucible/"
echo "   scp crucible-monitor-linux-arm64 user@arm-server:/opt/crucible/"
echo "   ssh user@arm-server 'chmod +x /opt/crucible/crucible-*'"
echo ""
echo "   TUI Application: ./crucible-linux-arm64"
echo "   Monitor Agent:   ./crucible-monitor-linux-arm64"