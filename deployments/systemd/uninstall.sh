#!/bin/bash
#
# Lumo Agent Uninstallation Script
# Removes lumo-agent systemd service and files
#
# Usage: sudo ./uninstall.sh [OPTIONS]
#
# Options:
#   --purge             Remove all data and configuration files
#   --keep-config       Keep configuration files
#   --keep-data         Keep data directory
#   --keep-logs         Keep log files
#   --user USER         Service user to remove (default: lumo-agent)
#   --group GROUP       Service group to remove (default: lumo-agent)
#   --yes               Skip confirmation prompts
#   --help              Show this help message
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
PURGE=0
KEEP_CONFIG=0
KEEP_DATA=0
KEEP_LOGS=0
SERVICE_USER="lumo-agent"
SERVICE_GROUP="lumo-agent"
SKIP_CONFIRM=0

# Installation paths
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/lumo-agent"
DATA_DIR="/var/lib/lumo-agent"
LOG_DIR="/var/log/lumo-agent"
SYSTEMD_DIR="/etc/systemd/system"

# Print functions
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo ""
    echo "=================================="
    echo "$1"
    echo "=================================="
    echo ""
}

# Help message
show_help() {
    cat << EOF
Lumo Agent Uninstallation Script

Usage: sudo ./uninstall.sh [OPTIONS]

Options:
  --purge             Remove all data and configuration files
  --keep-config       Keep configuration files
  --keep-data         Keep data directory
  --keep-logs         Keep log files
  --user USER         Service user to remove (default: lumo-agent)
  --group GROUP       Service group to remove (default: lumo-agent)
  --yes               Skip confirmation prompts
  --help              Show this help message

Examples:
  # Standard uninstall (keeps config, data, logs)
  sudo ./uninstall.sh

  # Complete removal (purge everything)
  sudo ./uninstall.sh --purge --yes

  # Remove but keep configuration
  sudo ./uninstall.sh --keep-config

EOF
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --purge)
            PURGE=1
            shift
            ;;
        --keep-config)
            KEEP_CONFIG=1
            shift
            ;;
        --keep-data)
            KEEP_DATA=1
            shift
            ;;
        --keep-logs)
            KEEP_LOGS=1
            shift
            ;;
        --user)
            SERVICE_USER="$2"
            shift 2
            ;;
        --group)
            SERVICE_GROUP="$2"
            shift 2
            ;;
        --yes)
            SKIP_CONFIRM=1
            shift
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   print_error "This script must be run as root (use sudo)"
   exit 1
fi

# If purge is set, don't keep anything
if [ $PURGE -eq 1 ]; then
    KEEP_CONFIG=0
    KEEP_DATA=0
    KEEP_LOGS=0
fi

print_header "Lumo Agent Uninstallation"

# Confirmation
if [ $SKIP_CONFIRM -eq 0 ]; then
    echo "This will remove the following:"
    echo "  - Lumo Agent binary"
    echo "  - Systemd service"
    [ $KEEP_CONFIG -eq 0 ] && echo "  - Configuration files ($CONFIG_DIR)"
    [ $KEEP_DATA -eq 0 ] && echo "  - Data directory ($DATA_DIR)"
    [ $KEEP_LOGS -eq 0 ] && echo "  - Log directory ($LOG_DIR)"
    echo "  - Service user ($SERVICE_USER)"
    echo "  - Service group ($SERVICE_GROUP)"
    echo ""
    [ $KEEP_CONFIG -eq 1 ] && print_info "Keeping configuration files"
    [ $KEEP_DATA -eq 1 ] && print_info "Keeping data directory"
    [ $KEEP_LOGS -eq 1 ] && print_info "Keeping log directory"
    echo ""
    read -p "Are you sure you want to continue? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Uninstallation cancelled"
        exit 0
    fi
fi

# Step 1: Stop and disable service
print_header "Step 1: Stopping and Disabling Service"

if command -v systemctl &> /dev/null; then
    if systemctl is-active --quiet lumo-agent.service; then
        print_info "Stopping lumo-agent service"
        systemctl stop lumo-agent.service || true
    else
        print_info "Service is not running"
    fi

    if systemctl is-enabled --quiet lumo-agent.service 2>/dev/null; then
        print_info "Disabling lumo-agent service"
        systemctl disable lumo-agent.service || true
    else
        print_info "Service is not enabled"
    fi
else
    print_warn "systemctl not found, skipping service management"
fi

# Step 2: Remove systemd service file
print_header "Step 2: Removing Systemd Service File"

if [ -f "$SYSTEMD_DIR/lumo-agent.service" ]; then
    print_info "Removing service file: $SYSTEMD_DIR/lumo-agent.service"
    rm -f "$SYSTEMD_DIR/lumo-agent.service"

    if command -v systemctl &> /dev/null; then
        print_info "Reloading systemd daemon"
        systemctl daemon-reload || true
        systemctl reset-failed || true
    fi
else
    print_info "Service file not found, skipping"
fi

# Step 3: Remove binary
print_header "Step 3: Removing Binary"

if [ -f "$INSTALL_DIR/lumo-agent" ]; then
    print_info "Removing binary: $INSTALL_DIR/lumo-agent"
    rm -f "$INSTALL_DIR/lumo-agent"
else
    print_info "Binary not found, skipping"
fi

# Step 4: Remove directories
print_header "Step 4: Removing Directories"

if [ $KEEP_CONFIG -eq 0 ] && [ -d "$CONFIG_DIR" ]; then
    print_info "Removing configuration directory: $CONFIG_DIR"
    rm -rf "$CONFIG_DIR"
elif [ -d "$CONFIG_DIR" ]; then
    print_info "Keeping configuration directory: $CONFIG_DIR"
fi

if [ $KEEP_DATA -eq 0 ] && [ -d "$DATA_DIR" ]; then
    print_info "Removing data directory: $DATA_DIR"
    rm -rf "$DATA_DIR"
elif [ -d "$DATA_DIR" ]; then
    print_info "Keeping data directory: $DATA_DIR"
fi

if [ $KEEP_LOGS -eq 0 ] && [ -d "$LOG_DIR" ]; then
    print_info "Removing log directory: $LOG_DIR"
    rm -rf "$LOG_DIR"
elif [ -d "$LOG_DIR" ]; then
    print_info "Keeping log directory: $LOG_DIR"
fi

# Step 5: Remove user and group
print_header "Step 5: Removing Service User and Group"

if getent passwd "$SERVICE_USER" > /dev/null 2>&1; then
    print_info "Removing user: $SERVICE_USER"
    userdel "$SERVICE_USER" 2>/dev/null || print_warn "Could not remove user (may have running processes)"
else
    print_info "User not found, skipping"
fi

if getent group "$SERVICE_GROUP" > /dev/null 2>&1; then
    print_info "Removing group: $SERVICE_GROUP"
    groupdel "$SERVICE_GROUP" 2>/dev/null || print_warn "Could not remove group (may be in use)"
else
    print_info "Group not found, skipping"
fi

# Step 6: Clean up any remaining files
print_header "Step 6: Cleaning Up"

# Remove any systemd journal logs
if command -v journalctl &> /dev/null; then
    print_info "Rotating systemd journal to remove old logs"
    journalctl --rotate 2>/dev/null || true
    journalctl --vacuum-time=1s 2>/dev/null || true
fi

print_header "Uninstallation Complete!"

cat << EOF
${GREEN}✓${NC} Lumo Agent has been uninstalled successfully!

Removed:
  ${GREEN}✓${NC} Binary
  ${GREEN}✓${NC} Systemd service
EOF

if [ $KEEP_CONFIG -eq 0 ]; then
    echo "  ${GREEN}✓${NC} Configuration files"
else
    echo "  ${YELLOW}○${NC} Configuration files (kept at: $CONFIG_DIR)"
fi

if [ $KEEP_DATA -eq 0 ]; then
    echo "  ${GREEN}✓${NC} Data directory"
else
    echo "  ${YELLOW}○${NC} Data directory (kept at: $DATA_DIR)"
fi

if [ $KEEP_LOGS -eq 0 ]; then
    echo "  ${GREEN}✓${NC} Log directory"
else
    echo "  ${YELLOW}○${NC} Log directory (kept at: $LOG_DIR)"
fi

echo "  ${GREEN}✓${NC} Service user and group"
echo ""

if [ $KEEP_CONFIG -eq 1 ] || [ $KEEP_DATA -eq 1 ] || [ $KEEP_LOGS -eq 1 ]; then
    echo "${YELLOW}Note:${NC} Some files were preserved. To completely remove all files, run:"
    echo "  sudo ./uninstall.sh --purge --yes"
    echo ""
fi

EOF
