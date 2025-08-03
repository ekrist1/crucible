#!/bin/bash

# Start Monitor with Environment Variables
echo "🚀 Starting Crucible Monitor with Environment Variables"
echo "======================================================"
echo ""

# Load environment variables from .env file
if [ ! -f ".env" ]; then
    echo "❌ .env file not found"
    exit 1
fi

echo "📋 Loading environment variables from .env..."
set -a
source .env
set +a

echo "✅ Environment loaded:"
echo "   RESEND_API_KEY: ${RESEND_API_KEY:0:8}..."
echo "   ALERT_FROM_EMAIL: $ALERT_FROM_EMAIL"
echo "   ALERT_FROM_NAME: $ALERT_FROM_NAME"
echo ""

# Stop any existing monitoring agent
echo "🛑 Stopping existing monitoring agent..."
sudo pkill -f crucible-monitor 2>/dev/null || true
sleep 2

echo "🚀 Starting monitoring agent with explicit environment variables..."
echo "   This ensures all environment variables are passed correctly"
echo ""

# Run with sudo but preserve environment variables
sudo -E env \
    RESEND_API_KEY="$RESEND_API_KEY" \
    ALERT_FROM_EMAIL="$ALERT_FROM_EMAIL" \
    ALERT_FROM_NAME="$ALERT_FROM_NAME" \
    ./crucible-monitor

echo ""
echo "✅ Monitor started with environment variables!"