#!/bin/bash
#
# Lumo Agent Installation Script
# Installs lumo-agent as a systemd service on Linux systems
#
# Usage: sudo ./install.sh [OPTIONS]
#
# Options:
#   --binary PATH       Path to lumo-agent binary (default: auto-detect)
#   --config PATH       Path to config file (optional)
#   --user USER         Service user (default: lumo-agent)
#   --group GROUP       Service group (default: lumo-agent)
#   --skip-systemd      Skip systemd service installation
#   --help              Show this help message
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default values
BINARY_PATH=""
CONFIG_PATH=""
SERVICE_USER="lumo-agent"
SERVICE_GROUP="lumo-agent"
SKIP_SYSTEMD=0

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
Lumo Agent Installation Script

Usage: sudo ./install.sh [OPTIONS]

Options:
  --binary PATH       Path to lumo-agent binary (default: auto-detect)
  --config PATH       Path to config file (optional)
  --user USER         Service user (default: lumo-agent)
  --group GROUP       Service group (default: lumo-agent)
  --skip-systemd      Skip systemd service installation
  --help              Show this help message

Examples:
  # Install with auto-detected binary
  sudo ./install.sh

  # Install with specific binary
  sudo ./install.sh --binary /path/to/lumo-agent

  # Install with custom config
  sudo ./install.sh --config /path/to/config.yaml

EOF
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --binary)
            BINARY_PATH="$2"
            shift 2
            ;;
        --config)
            CONFIG_PATH="$2"
            shift 2
            ;;
        --user)
            SERVICE_USER="$2"
            shift 2
            ;;
        --group)
            SERVICE_GROUP="$2"
            shift 2
            ;;
        --skip-systemd)
            SKIP_SYSTEMD=1
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

# Detect OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
    OS_VERSION=$VERSION_ID
else
    print_error "Cannot detect OS version"
    exit 1
fi

print_header "Lumo Agent Installation"
print_info "OS: $OS $OS_VERSION"
print_info "User: $SERVICE_USER"
print_info "Group: $SERVICE_GROUP"

# Auto-detect binary if not provided
if [ -z "$BINARY_PATH" ]; then
    print_info "Auto-detecting lumo-agent binary..."

    # Check common locations
    SEARCH_PATHS=(
        "./lumo-agent"
        "../lumo-agent"
        "../../lumo-agent"
        "./bin/lumo-agent"
        "./build/lumo-agent"
        "$(dirname "$0")/../../lumo-agent"
    )

    for path in "${SEARCH_PATHS[@]}"; do
        if [ -f "$path" ] && [ -x "$path" ]; then
            BINARY_PATH="$path"
            print_info "Found binary: $BINARY_PATH"
            break
        fi
    done

    if [ -z "$BINARY_PATH" ]; then
        print_error "Cannot find lumo-agent binary. Please specify with --binary"
        exit 1
    fi
fi

# Verify binary exists and is executable
if [ ! -f "$BINARY_PATH" ]; then
    print_error "Binary not found: $BINARY_PATH"
    exit 1
fi

if [ ! -x "$BINARY_PATH" ]; then
    print_error "Binary is not executable: $BINARY_PATH"
    exit 1
fi

# Verify binary version
print_info "Verifying binary..."
BINARY_VERSION=$("$BINARY_PATH" version 2>/dev/null | head -n1 || echo "unknown")
print_info "Binary version: $BINARY_VERSION"

# Step 1: Create system user and group
print_header "Step 1: Creating System User and Group"

if ! getent group "$SERVICE_GROUP" > /dev/null 2>&1; then
    print_info "Creating group: $SERVICE_GROUP"
    groupadd --system "$SERVICE_GROUP"
else
    print_info "Group already exists: $SERVICE_GROUP"
fi

if ! getent passwd "$SERVICE_USER" > /dev/null 2>&1; then
    print_info "Creating user: $SERVICE_USER"
    useradd --system \
        --gid "$SERVICE_GROUP" \
        --no-create-home \
        --home-dir "$DATA_DIR" \
        --shell /usr/sbin/nologin \
        --comment "Lumo Agent Service User" \
        "$SERVICE_USER"
else
    print_info "User already exists: $SERVICE_USER"
fi

# Step 2: Create directories
print_header "Step 2: Creating Directories"

for dir in "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"; do
    if [ ! -d "$dir" ]; then
        print_info "Creating directory: $dir"
        mkdir -p "$dir"
    else
        print_info "Directory already exists: $dir"
    fi
done

# Step 3: Install binary
print_header "Step 3: Installing Binary"

print_info "Installing binary to $INSTALL_DIR/lumo-agent"
cp "$BINARY_PATH" "$INSTALL_DIR/lumo-agent"
chmod 755 "$INSTALL_DIR/lumo-agent"
chown root:root "$INSTALL_DIR/lumo-agent"

# Step 4: Install configuration
print_header "Step 4: Installing Configuration"

if [ -n "$CONFIG_PATH" ] && [ -f "$CONFIG_PATH" ]; then
    print_info "Installing config from: $CONFIG_PATH"
    cp "$CONFIG_PATH" "$CONFIG_DIR/config.yaml"
    chmod 640 "$CONFIG_DIR/config.yaml"
else
    if [ ! -f "$CONFIG_DIR/config.yaml" ]; then
        print_info "Creating default config.yaml"
        cat > "$CONFIG_DIR/config.yaml" << 'EOF'
# Lumo Agent Configuration
# See https://github.com/ignacio/lumo for documentation

agent:
  mode: hybrid
  schedule: "*/5 * * * *"
  api_endpoint: ""  # Set to your Lumo API server URL
  token: ""         # Set via LUMO_AGENT_TOKEN environment variable
  tls_enabled: true
  enabled_checks:
    - cpu
    - memory
    - disk
    - process
    - service
    - network
  report_format: toon
  offline_mode: true
  health_check_port: 8080
  metrics_port: 9090

logging:
  level: info
  format: json

diagnostics:
  timeout: 5m
  max_concurrent: 5
EOF
        chmod 640 "$CONFIG_DIR/config.yaml"
    else
        print_info "Config already exists, skipping"
    fi
fi

# Step 5: Set permissions
print_header "Step 5: Setting Permissions"

print_info "Setting ownership and permissions"
chown -R "$SERVICE_USER:$SERVICE_GROUP" "$DATA_DIR"
chown -R "$SERVICE_USER:$SERVICE_GROUP" "$LOG_DIR"
chown -R root:"$SERVICE_GROUP" "$CONFIG_DIR"
chmod 750 "$DATA_DIR"
chmod 750 "$LOG_DIR"
chmod 750 "$CONFIG_DIR"

# Step 6: Install systemd service
if [ $SKIP_SYSTEMD -eq 0 ]; then
    print_header "Step 6: Installing Systemd Service"

    # Check if systemd is available
    if ! command -v systemctl &> /dev/null; then
        print_warn "systemd not found, skipping service installation"
    else
        SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
        SERVICE_FILE="$SCRIPT_DIR/lumo-agent.service"

        if [ ! -f "$SERVICE_FILE" ]; then
            print_error "Service file not found: $SERVICE_FILE"
            exit 1
        fi

        print_info "Installing systemd service"
        cp "$SERVICE_FILE" "$SYSTEMD_DIR/lumo-agent.service"
        chmod 644 "$SYSTEMD_DIR/lumo-agent.service"

        print_info "Reloading systemd daemon"
        systemctl daemon-reload

        print_info "Enabling lumo-agent service"
        systemctl enable lumo-agent.service

        print_info "Service installed successfully"
        echo ""
        print_info "To start the service: sudo systemctl start lumo-agent"
        print_info "To check status: sudo systemctl status lumo-agent"
        print_info "To view logs: sudo journalctl -u lumo-agent -f"
    fi
else
    print_header "Step 6: Skipping Systemd Installation"
fi

# Step 7: Post-installation instructions
print_header "Installation Complete!"

cat << EOF
${GREEN}✓${NC} Lumo Agent has been installed successfully!

Installation Summary:
  Binary:        $INSTALL_DIR/lumo-agent
  Config:        $CONFIG_DIR/config.yaml
  Data Dir:      $DATA_DIR
  Log Dir:       $LOG_DIR
  Service User:  $SERVICE_USER
  Service Group: $SERVICE_GROUP

Next Steps:
  1. Edit the configuration file:
     sudo nano $CONFIG_DIR/config.yaml

  2. Set required environment variables (in service file or config):
     - LUMO_AGENT_API_ENDPOINT: Your Lumo API server URL
     - LUMO_AGENT_TOKEN: Authentication token (JWT)
     - AI provider credentials (if using AI analysis)

  3. Start the service:
     sudo systemctl start lumo-agent

  4. Check the status:
     sudo systemctl status lumo-agent

  5. View logs:
     sudo journalctl -u lumo-agent -f

  6. Test health endpoint:
     curl http://localhost:8080/health

  7. View metrics:
     curl http://localhost:9090/metrics

For more information, visit: https://github.com/ignacio/lumo

EOF
