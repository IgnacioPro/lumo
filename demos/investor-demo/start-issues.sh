#!/bin/bash
#
# Lumo Investor Demo - Start Simulated Issues
# This script activates the simulated issues for the demo
#
set -e

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="$DEMO_DIR/workspace"

echo "╦  ╦ ╦╔╦╗╔═╗"
echo "║  ║ ║║║║║ ║"
echo "╩═╝╚═╝╩ ╩╚═╝"
echo ""
echo "Starting Demo Issues..."
echo "======================"
echo ""

# Check if workspace exists
if [ ! -d "$WORKSPACE" ]; then
    echo "ERROR: Workspace not found. Run ./setup.sh first"
    exit 1
fi

# 1. Start CPU stress (optional - commented out by default to avoid system impact)
# Uncomment for full demo effect
# echo "[1/4] Starting CPU stress..."
# nohup "$WORKSPACE/cpu-stress.sh" > "$WORKSPACE/cpu-stress.log" 2>&1 &
# echo "  ✓ CPU stress running (PID: $!)"

echo "[1/4] CPU stress (skipped - uncomment in script to enable)"

# 2. Start memory leak (optional - commented out by default)
# Uncomment for full demo effect
# echo "[2/4] Starting memory leak..."
# nohup "$WORKSPACE/memory-leak.sh" > "$WORKSPACE/memory-leak.log" 2>&1 &
# echo "  ✓ Memory leak running (PID: $!)"

echo "[2/4] Memory leak (skipped - uncomment in script to enable)"

# 3. Start/fail systemd service
if command -v systemctl &> /dev/null && [ ! -f /.dockerenv ]; then
    echo "[3/4] Starting failing systemd service..."
    if sudo systemctl is-active --quiet lumo-demo-service 2>/dev/null; then
        sudo systemctl restart lumo-demo-service
    else
        sudo systemctl start lumo-demo-service 2>/dev/null || true
    fi

    sleep 2

    if sudo systemctl is-failed --quiet lumo-demo-service 2>/dev/null; then
        echo "  ✓ Service is in failed state (as expected)"
    else
        echo "  ⚠ Service status: $(sudo systemctl is-active lumo-demo-service)"
    fi
else
    echo "[3/4] Systemd service (skipped - not available)"
fi

# 4. Create additional "problem" indicators
echo "[4/4] Creating additional issue indicators..."

# Create a process that looks suspicious
cat > "$WORKSPACE/suspicious-process.sh" << 'EOF'
#!/bin/bash
# Suspicious looking process for demo
while true; do
    sleep 60
done
EOF

chmod +x "$WORKSPACE/suspicious-process.sh"
nohup "$WORKSPACE/suspicious-process.sh" > /dev/null 2>&1 &
SUSPICIOUS_PID=$!

echo "  ✓ Suspicious process started (PID: $SUSPICIOUS_PID)"

# Record active issues
cat > "$WORKSPACE/.active-issues" << EOF
TIMESTAMP=$(date -Iseconds)
ISSUES_STARTED=true
SUSPICIOUS_PID=$SUSPICIOUS_PID
EOF

echo ""
echo "=================================="
echo "Demo Issues Started!"
echo "=================================="
echo ""
echo "Active Issues:"
echo "  ✓ Excessive disk usage (~350 MB in workspace)"
echo "  ✓ Old log files (50 files older than 30 days)"
echo "  ✓ Temporary files accumulation (100 files)"
if command -v systemctl &> /dev/null && [ ! -f /.dockerenv ]; then
    echo "  ✓ Failed systemd service (lumo-demo-service)"
fi
echo "  ✓ Suspicious process (PID: $SUSPICIOUS_PID)"
echo ""
echo "These issues will be detected and fixed by Lumo during the demo."
echo ""
echo "To verify issues:"
echo "  - Check disk usage: du -sh $WORKSPACE"
echo "  - Check service: sudo systemctl status lumo-demo-service"
echo "  - Check processes: ps aux | grep suspicious"
echo ""
echo "Next: Run ./run-demo.sh to demonstrate Lumo fixing these issues"
echo ""
