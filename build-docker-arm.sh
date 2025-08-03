#!/bin/bash

# Build ARM Docker Images
echo "🐳 Building ARM Docker Images"
echo "============================="
echo ""

# Enable Docker BuildKit for multi-platform builds
export DOCKER_BUILDKIT=1

echo "📋 Building for multiple ARM platforms..."
echo ""

# Build for ARM64
echo "🔸 Building ARM64 Docker image..."
docker buildx build --platform linux/arm64 \
    -f Dockerfile.arm \
    -t crucible-monitor:arm64 \
    --load .

# Build for ARMv7
echo "🔸 Building ARMv7 Docker image..."
docker buildx build --platform linux/arm/v7 \
    -f Dockerfile.arm \
    -t crucible-monitor:armv7 \
    --load .

echo ""
echo "✅ Docker ARM builds completed!"
echo ""
echo "📋 Available images:"
docker images | grep crucible-monitor

echo ""
echo "💡 Usage:"
echo "   docker save crucible-monitor:arm64 | gzip > crucible-arm64.tar.gz"
echo "   scp crucible-arm64.tar.gz user@arm-server:"
echo "   ssh user@arm-server 'docker load < crucible-arm64.tar.gz'"