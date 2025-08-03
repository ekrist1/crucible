#!/bin/bash

# Build ARM Binaries without CGO (Pure Go)
echo "🔧 Building ARM Binaries (Pure Go - No CGO)"
echo "==========================================="
echo ""

# Build information
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

# Linker flags for static binary
LDFLAGS="-w -s -X 'main.version=$VERSION' -X 'main.buildTime=$BUILD_TIME' -X 'main.gitCommit=$GIT_COMMIT'"

echo "📋 This build disables CGO, so SQLite will use pure Go implementation"
echo "   Pros: Easy cross-compilation, no dependencies"
echo "   Cons: Slightly slower SQLite performance"
echo ""

echo "🏗️  Building both crucible (TUI) and crucible-monitor (agent)..."
echo ""

# Function to build for a specific architecture
build_for_arch() {
    local arch=$1
    local goarch=$2
    local goarm=$3
    
    echo "🔸 Building for $arch..."
    
    # Build crucible (main TUI application)
    echo "   📱 Building crucible TUI..."
    if [ -n "$goarm" ]; then
        env GOOS=linux GOARCH=$goarch GOARM=$goarm CGO_ENABLED=0 \
            go build -a -ldflags="$LDFLAGS" -o "crucible-linux-$arch-nocgo" .
    else
        env GOOS=linux GOARCH=$goarch CGO_ENABLED=0 \
            go build -a -ldflags="$LDFLAGS" -o "crucible-linux-$arch-nocgo" .
    fi
    
    if [ $? -eq 0 ]; then
        echo "   ✅ crucible build successful"
        ls -lh "crucible-linux-$arch-nocgo"
    else
        echo "   ❌ crucible build failed"
        return 1
    fi
    
    # Build crucible-monitor (monitoring agent)
    echo "   📊 Building crucible-monitor agent..."
    if [ -n "$goarm" ]; then
        env GOOS=linux GOARCH=$goarch GOARM=$goarm CGO_ENABLED=0 \
            go build -a -ldflags="$LDFLAGS" -o "crucible-monitor-linux-$arch-nocgo" ./cmd/monitor
    else
        env GOOS=linux GOARCH=$goarch CGO_ENABLED=0 \
            go build -a -ldflags="$LDFLAGS" -o "crucible-monitor-linux-$arch-nocgo" ./cmd/monitor
    fi
    
    if [ $? -eq 0 ]; then
        echo "   ✅ crucible-monitor build successful"
        ls -lh "crucible-monitor-linux-$arch-nocgo"
    else
        echo "   ❌ crucible-monitor build failed"
        return 1
    fi
    
    echo ""
}

# Check if monitor command exists, if not build from main
if [ ! -d "./cmd/monitor" ]; then
    echo "📋 Note: cmd/monitor not found, building monitor from main package"
    echo ""
    
    # Function to build both from main package
    build_for_arch_main() {
        local arch=$1
        local goarch=$2
        local goarm=$3
        
        echo "🔸 Building for $arch..."
        
        # Build crucible (main TUI application) - this is the main package
        echo "   📱 Building crucible TUI..."
        if [ -n "$goarm" ]; then
            env GOOS=linux GOARCH=$goarch GOARM=$goarm CGO_ENABLED=0 \
                go build -a -ldflags="$LDFLAGS" -o "crucible-linux-$arch-nocgo" .
        else
            env GOOS=linux GOARCH=$goarch CGO_ENABLED=0 \
                go build -a -ldflags="$LDFLAGS" -o "crucible-linux-$arch-nocgo" .
        fi
        
        if [ $? -eq 0 ]; then
            echo "   ✅ crucible build successful"
            ls -lh "crucible-linux-$arch-nocgo"
            
            # Copy as monitor too since it's the same binary
            cp "crucible-linux-$arch-nocgo" "crucible-monitor-linux-$arch-nocgo"
            echo "   ✅ crucible-monitor (copy) created"
            ls -lh "crucible-monitor-linux-$arch-nocgo"
        else
            echo "   ❌ crucible build failed"
            return 1
        fi
        
        echo ""
    }
    
    # ARM64 (most common)
    build_for_arch_main "arm64" "arm64" ""
    
    # ARMv7
    build_for_arch_main "armv7" "arm" "7"
    
    # ARMv6
    build_for_arch_main "armv6" "arm" "6"
    
else
    # ARM64 (most common)
    build_for_arch "arm64" "arm64" ""
    
    # ARMv7
    build_for_arch "armv7" "arm" "7"
    
    # ARMv6
    build_for_arch "armv6" "arm" "6"
fi

echo "🏁 Build Summary:"
echo "=================="
echo ""
echo "📱 Crucible TUI Application:"
for binary in crucible-linux-*-nocgo; do
    if [ -f "$binary" ]; then
        echo "✅ $binary ($(file $binary | cut -d: -f2 | cut -d, -f1))"
        echo "   Size: $(du -h $binary | cut -f1)"
    fi
done

echo ""
echo "📊 Crucible Monitor Agent:"
for binary in crucible-monitor-linux-*-nocgo; do
    if [ -f "$binary" ]; then
        echo "✅ $binary ($(file $binary | cut -d: -f2 | cut -d, -f1))"
        echo "   Size: $(du -h $binary | cut -f1)"
    fi
done

echo ""
echo "✅ Pure Go ARM builds completed!"
echo "   These binaries have no external dependencies"
echo ""
echo "💡 Usage:"
echo "   TUI Application: ./crucible-linux-arm64-nocgo"
echo "   Monitor Agent:   ./crucible-monitor-linux-arm64-nocgo"