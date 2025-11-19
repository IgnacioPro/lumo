#!/bin/bash
#
# Lumo Investor Demo - Reset Script
# This script resets the demo environment to initial state
#
set -e

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="$DEMO_DIR/workspace"

echo "╦  ╦ ╦╔╦╗╔═╗"
echo "║  ║ ║║║║║ ║"
echo "╩═╝╚═╝╩ ╩╚═╝"
echo ""
echo "Resetting Demo Environment..."
echo "=============================="
echo ""

# 1. Stop any running demo processes
echo "[1/5] Stopping demo processes..."

# Kill CPU stress
pkill -f "cpu-stress.sh" 2>/dev/null || true

# Kill memory leak
pkill -f "memory-leak.sh" 2>/dev/null || true

# Kill suspicious process
if [ -f "$WORKSPACE/.active-issues" ]; then
    source "$WORKSPACE/.active-issues"
    if [ -n "$SUSPICIOUS_PID" ]; then
        kill "$SUSPICIOUS_PID" 2>/dev/null || true
    fi
fi

pkill -f "suspicious-process.sh" 2>/dev/null || true

echo "  ✓ Stopped all demo processes"

# 2. Stop and disable systemd service
if command -v systemctl &> /dev/null && [ ! -f /.dockerenv ]; then
    echo "[2/5] Stopping systemd service..."

    if systemctl is-active --quiet lumo-demo-service 2>/dev/null; then
        sudo systemctl stop lumo-demo-service
    fi

    if systemctl is-enabled --quiet lumo-demo-service 2>/dev/null; then
        sudo systemctl disable lumo-demo-service
    fi

    echo "  ✓ Stopped and disabled systemd service"
else
    echo "[2/5] Systemd service (skipped - not available)"
fi

# 3. Clean workspace
echo "[3/5] Cleaning workspace..."

if [ -d "$WORKSPACE" ]; then
    # Calculate current size
    current_size=$(du -sh "$WORKSPACE" 2>/dev/null | cut -f1 || echo "unknown")
    echo "  Current workspace size: $current_size"

    # Remove all contents but keep directory structure
    rm -rf "$WORKSPACE"/{logs,tmp,data}/*
    rm -f "$WORKSPACE"/*.{sh,log,json}
    rm -f "$WORKSPACE"/.{demo-state,active-issues}

    echo "  ✓ Workspace cleaned"
else
    echo "  ℹ Workspace doesn't exist (no cleanup needed)"
fi

# 4. Remove systemd service file
if [ -f /etc/systemd/system/lumo-demo-service.service ]; then
    echo "[4/5] Removing systemd service file..."
    sudo rm -f /etc/systemd/system/lumo-demo-service.service
    sudo systemctl daemon-reload
    echo "  ✓ Systemd service file removed"
else
    echo "[4/5] Systemd service file (not present)"
fi

# 5. Remove log files
echo "[5/5] Cleaning log files..."

rm -f "$DEMO_DIR"/*.log

echo "  ✓ Log files cleaned"

# Summary
echo ""
echo "=================================="
echo "Demo Environment Reset Complete!"
echo "=================================="
echo ""
echo "The demo environment has been returned to initial state:"
echo "  ✓ All demo processes stopped"
echo "  ✓ Workspace cleaned"
echo "  ✓ Systemd service removed"
echo "  ✓ Log files cleaned"
echo ""
echo "To prepare for next demo:"
echo "  1. Run ./setup.sh to recreate demo environment"
echo "  2. Run ./start-issues.sh to activate demo issues"
echo "  3. Run ./run-demo.sh to execute demo"
echo ""
