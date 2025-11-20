#!/bin/bash
#
# Lumo Investor Demo - Environment Setup
# This script sets up a demo environment with induced issues to showcase Lumo's capabilities
#
set -e

DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="$DEMO_DIR/demo-setup.log"

echo "╦  ╦ ╦╔╦╗╔═╗"
echo "║  ║ ║║║║║ ║"
echo "╩═╝╚═╝╩ ╩╚═╝"
echo ""
echo "Investor Demo - Environment Setup"
echo "=================================="
echo ""

log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
}

log "Starting demo environment setup..."

# Detect OS
OS="unknown"
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="macos"
else
    log "ERROR: Unsupported OS: $OSTYPE"
    exit 1
fi

log "Detected OS: $OS"

# Check if running in Docker
if [ -f /.dockerenv ]; then
    log "Running in Docker container"
    DOCKER_MODE=true
else
    DOCKER_MODE=false
fi

# 1. Create demo workspace
log "Creating demo workspace..."
WORKSPACE="$DEMO_DIR/workspace"
mkdir -p "$WORKSPACE"/{logs,tmp,data}

# 2. Simulate log files (for disk space demo)
log "Creating sample log files..."
for i in {1..50}; do
    date=$(date -d "$i days ago" +%Y-%m-%d 2>/dev/null || date -v-${i}d +%Y-%m-%d 2>/dev/null || echo "2025-11-$i")
    logfile="$WORKSPACE/logs/app-${date}.log"

    # Create logs of varying sizes (1-10 MB each)
    size=$((RANDOM % 10 + 1))
    dd if=/dev/urandom of="$logfile" bs=1M count=$size status=none 2>/dev/null || true

    # Add realistic log content
    echo "[$date 00:00:00] INFO Application started" >> "$logfile"
    echo "[$date 12:34:56] ERROR Connection timeout to database" >> "$logfile"
    echo "[$date 23:59:59] WARN High memory usage detected" >> "$logfile"
done

log "Created $(find $WORKSPACE/logs -type f | wc -l) log files ($(du -sh $WORKSPACE/logs | cut -f1))"

# 3. Create temporary files
log "Creating temporary files..."
for i in {1..100}; do
    tmpfile="$WORKSPACE/tmp/temp_${RANDOM}.tmp"
    dd if=/dev/zero of="$tmpfile" bs=1M count=1 status=none 2>/dev/null || true
done

log "Created $(find $WORKSPACE/tmp -type f | wc -l) temp files ($(du -sh $WORKSPACE/tmp | cut -f1))"

# 4. Create data files
log "Creating sample data files..."
cat > "$WORKSPACE/data/database.sql" << 'EOF'
-- Sample database dump
-- Size: Large (for demo purposes)
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255),
    email VARCHAR(255),
    created_at TIMESTAMP
);

INSERT INTO users (username, email, created_at) VALUES
('user1', 'user1@example.com', NOW()),
('user2', 'user2@example.com', NOW()),
('user3', 'user3@example.com', NOW());
EOF

# Expand database file to realistic size
for i in {1..1000}; do
    echo "INSERT INTO users (username, email, created_at) VALUES ('user$i', 'user$i@example.com', NOW());" >> "$WORKSPACE/data/database.sql"
done

log "Created database dump ($(du -sh $WORKSPACE/data/database.sql | cut -f1))"

# 5. Create CPU stress script
log "Creating CPU stress simulation..."
cat > "$WORKSPACE/cpu-stress.sh" << 'EOF'
#!/bin/bash
# CPU stress test for demo
echo "Starting CPU stress (4 threads)..."

stress_cpu() {
    while true; do
        echo "scale=5000; 4*a(1)" | bc -l > /dev/null 2>&1
    done
}

# Start 4 background processes
for i in {1..4}; do
    stress_cpu &
done

echo "CPU stress started. PIDs: $(jobs -p | tr '\n' ' ')"
echo "To stop: pkill -f cpu-stress.sh"
wait
EOF

chmod +x "$WORKSPACE/cpu-stress.sh"

# 6. Create memory leak script
log "Creating memory leak simulation..."
cat > "$WORKSPACE/memory-leak.sh" << 'EOF'
#!/bin/bash
# Memory leak simulation for demo
echo "Starting memory leak simulation..."

leak_array=()
counter=0

while true; do
    # Allocate memory in chunks
    leak_array+=("$(head -c 10M /dev/urandom | base64)")
    counter=$((counter + 1))

    if [ $((counter % 10)) -eq 0 ]; then
        echo "Allocated ${counter}0 MB..."
    fi

    sleep 1
done
EOF

chmod +x "$WORKSPACE/memory-leak.sh"

# 7. Create failing service script
log "Creating failing service simulation..."
cat > "$WORKSPACE/demo-service.sh" << 'EOF'
#!/bin/bash
# Simulated failing service
echo "Demo service starting..."
sleep 2
echo "ERROR: Demo service crashed (simulated failure)"
exit 1
EOF

chmod +x "$WORKSPACE/demo-service.sh"

# 8. Create systemd service file (if available)
if command -v systemctl &> /dev/null && [ "$DOCKER_MODE" = false ]; then
    log "Creating systemd service for demo..."

    sudo tee /etc/systemd/system/lumo-demo-service.service > /dev/null << EOF
[Unit]
Description=Lumo Demo Service (Intentionally Failing)
After=network.target

[Service]
Type=simple
ExecStart=$WORKSPACE/demo-service.sh
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    log "Systemd service created: lumo-demo-service"
else
    log "Skipping systemd service (not available or Docker mode)"
fi

# 9. Create demo metrics baseline
log "Capturing baseline metrics..."
cat > "$WORKSPACE/demo-metrics.json" << EOF
{
  "timestamp": "$(date -Iseconds)",
  "environment": "investor-demo",
  "baseline": {
    "disk_usage": "$(df -h . | tail -1 | awk '{print $5}')",
    "workspace_size": "$(du -sh $WORKSPACE | cut -f1)",
    "log_count": $(find $WORKSPACE/logs -type f | wc -l),
    "tmp_count": $(find $WORKSPACE/tmp -type f | wc -l),
    "total_files": $(find $WORKSPACE -type f | wc -l)
  }
}
EOF

log "Baseline metrics saved to demo-metrics.json"

# 10. Create demo state file
log "Creating demo state file..."
cat > "$WORKSPACE/.demo-state" << EOF
DEMO_INITIALIZED=true
DEMO_VERSION=1.0.0
SETUP_TIME=$(date -Iseconds)
WORKSPACE=$WORKSPACE
OS=$OS
DOCKER_MODE=$DOCKER_MODE
EOF

# 11. Display summary
echo ""
echo "=================================="
echo "Demo Environment Setup Complete!"
echo "=================================="
echo ""
echo "Workspace: $WORKSPACE"
echo "Disk Usage: $(du -sh $WORKSPACE | cut -f1)"
echo ""
echo "Simulated Issues Created:"
echo "  ✓ 50 log files (~250 MB total)"
echo "  ✓ 100 temp files (~100 MB total)"
echo "  ✓ Large database dump"
echo "  ✓ CPU stress script (not running)"
echo "  ✓ Memory leak script (not running)"
if command -v systemctl &> /dev/null && [ "$DOCKER_MODE" = false ]; then
    echo "  ✓ Failing systemd service"
fi
echo ""
echo "Next Steps:"
echo "  1. Review demo script: cat README.md"
echo "  2. Start demo issues: ./start-issues.sh"
echo "  3. Run demo: ./run-demo.sh"
echo "  4. Reset environment: ./reset.sh"
echo ""
echo "Setup log: $LOG_FILE"
echo ""

log "Demo environment setup complete!"
