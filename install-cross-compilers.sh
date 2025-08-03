#!/bin/bash

# Install Cross-Compilation Tools for ARM
echo "🔧 Installing ARM Cross-Compilation Tools"
echo "========================================"
echo ""

# Detect Linux distribution
if [ -f /etc/fedora-release ]; then
    DISTRO="fedora"
elif [ -f /etc/debian_version ]; then
    DISTRO="debian"
else
    echo "❌ Unsupported distribution"
    exit 1
fi

echo "📋 Detected distribution: $DISTRO"
echo ""

if [ "$DISTRO" = "fedora" ]; then
    echo "🔸 Installing ARM cross-compilers on Fedora..."
    sudo dnf install -y \
        gcc-aarch64-linux-gnu \
        gcc-arm-linux-gnueabihf \
        glibc-devel.aarch64 \
        glibc-devel.armv7hl
    
elif [ "$DISTRO" = "debian" ]; then
    echo "🔸 Installing ARM cross-compilers on Debian/Ubuntu..."
    sudo apt update
    sudo apt install -y \
        gcc-aarch64-linux-gnu \
        gcc-arm-linux-gnueabihf \
        libc6-dev-arm64-cross \
        libc6-dev-armhf-cross
fi

echo ""
echo "✅ Cross-compilers installed!"
echo ""
echo "🧪 Testing cross-compilers..."
echo "ARM64 compiler:"
aarch64-linux-gnu-gcc --version | head -1

echo "ARMv7 compiler:"
arm-linux-gnueabihf-gcc --version | head -1

echo ""
echo "💡 Now you can run: ./build-arm.sh"