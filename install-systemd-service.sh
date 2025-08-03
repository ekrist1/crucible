#!/bin/bash

# Install Crucible Monitor as Systemd Service
echo "🔧 Installing Crucible Monitor Systemd Service"
echo "=============================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "❌ This script must be run as root (use sudo)"
    exit 1
fi

echo "📋 Step 1: Create crucible user and directories"
# Create system user for crucible
if ! id "crucible" &>/dev/null; then
    useradd --system --shell /bin/false --home /opt/crucible --create-home crucible
    echo "✅ Created crucible system user"
else
    echo "✅ Crucible user already exists"
fi

# Create directories
mkdir -p /opt/crucible
mkdir -p /var/lib/crucible
mkdir -p /etc/crucible

echo "📋 Step 2: Copy files to production location"
# Copy binary
cp ./crucible-monitor /opt/crucible/
chmod +x /opt/crucible/crucible-monitor

# Copy configuration files
cp -r ./configs /opt/crucible/
cp ./.env /opt/crucible/

# Set ownership
chown -R crucible:crucible /opt/crucible
chown -R crucible:crucible /var/lib/crucible

echo "📋 Step 3: Install systemd service"
# Install systemd service
cp ./systemd/crucible-monitor.service /etc/systemd/system/
systemctl daemon-reload

echo "📋 Step 4: Enable and start service"
systemctl enable crucible-monitor
systemctl start crucible-monitor

echo ""
echo "✅ Crucible Monitor installed as systemd service!"
echo ""
echo "🔧 Management commands:"
echo "   Start:   sudo systemctl start crucible-monitor"
echo "   Stop:    sudo systemctl stop crucible-monitor"
echo "   Status:  sudo systemctl status crucible-monitor"
echo "   Logs:    sudo journalctl -u crucible-monitor -f"
echo "   Restart: sudo systemctl restart crucible-monitor"
echo ""
echo "📋 Configuration files:"
echo "   Service: /opt/crucible/"
echo "   Config:  /opt/crucible/configs/"
echo "   Env:     /opt/crucible/.env"
echo "   Data:    /var/lib/crucible/"