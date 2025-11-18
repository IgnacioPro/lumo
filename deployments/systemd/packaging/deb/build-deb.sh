#!/bin/bash
#
# DEB Build Script for Lumo Agent
# Builds DEB packages for Ubuntu/Debian
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Configuration
VERSION=${VERSION:-1.0.0}
RELEASE=${RELEASE:-1}
ARCH=${ARCH:-amd64}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build/deb"

print_info "Building Lumo Agent DEB"
print_info "Version: $VERSION-$RELEASE"
print_info "Architecture: $ARCH"

# Check for required tools
if ! command -v dpkg-deb &> /dev/null; then
    print_error "dpkg-deb not found. Install with: sudo apt-get install dpkg-dev"
    exit 1
fi

# Create build directory
print_info "Creating build directory"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/lumo-agent-$VERSION"

# Build the binary
print_info "Building lumo-agent binary"
cd "$PROJECT_ROOT"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$BUILD_DIR/lumo-agent-$VERSION/lumo-agent" \
    ./cmd/lumo-agent

# Copy service file
cp "$PROJECT_ROOT/deployments/systemd/lumo-agent.service" "$BUILD_DIR/lumo-agent-$VERSION/"

# Create default config
print_info "Creating default configuration"
cat > "$BUILD_DIR/lumo-agent-$VERSION/config.yaml" << 'EOF'
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

# Copy debian directory
print_info "Copying debian package files"
cp -r "$SCRIPT_DIR/debian" "$BUILD_DIR/lumo-agent-$VERSION/"

# Make maintainer scripts executable
chmod +x "$BUILD_DIR/lumo-agent-$VERSION/debian/postinst"
chmod +x "$BUILD_DIR/lumo-agent-$VERSION/debian/prerm"
chmod +x "$BUILD_DIR/lumo-agent-$VERSION/debian/postrm"
chmod +x "$BUILD_DIR/lumo-agent-$VERSION/debian/rules"

# Build the package
print_info "Building DEB package"
cd "$BUILD_DIR/lumo-agent-$VERSION"

# Use dpkg-buildpackage if available, otherwise use dpkg-deb
if command -v dpkg-buildpackage &> /dev/null; then
    dpkg-buildpackage -us -uc -b
    DEB_FILE="$BUILD_DIR/lumo-agent_${VERSION}-${RELEASE}_${ARCH}.deb"
else
    # Manual build with dpkg-deb (simplified)
    mkdir -p DEBIAN
    cp debian/control DEBIAN/
    cp debian/postinst DEBIAN/
    cp debian/prerm DEBIAN/
    cp debian/postrm DEBIAN/

    # Update control file with version
    sed -i "s/Version:.*/Version: $VERSION-$RELEASE/" DEBIAN/control

    cd ..
    dpkg-deb --build "lumo-agent-$VERSION"
    DEB_FILE="$BUILD_DIR/lumo-agent-$VERSION.deb"
fi

# Find the built package
if [ ! -f "$DEB_FILE" ]; then
    DEB_FILE=$(find "$BUILD_DIR" -name "lumo-agent*.deb" -type f | head -n1)
fi

print_info "Build complete!"
echo ""
echo "Package created:"
echo "  DEB: $DEB_FILE"
echo ""
echo "Install with:"
echo "  sudo dpkg -i $DEB_FILE"
echo ""
echo "Or with apt:"
echo "  sudo apt install $DEB_FILE"
echo ""
echo "If dependencies are missing:"
echo "  sudo apt-get install -f"
