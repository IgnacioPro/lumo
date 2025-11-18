#!/bin/bash
#
# RPM Build Script for Lumo Agent
# Builds RPM packages for RHEL/CentOS/Fedora
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
ARCH=${ARCH:-x86_64}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build/rpm"

print_info "Building Lumo Agent RPM"
print_info "Version: $VERSION-$RELEASE"
print_info "Architecture: $ARCH"

# Check for required tools
if ! command -v rpmbuild &> /dev/null; then
    print_error "rpmbuild not found. Install with: sudo yum install rpm-build"
    exit 1
fi

# Create RPM build directories
print_info "Creating RPM build directories"
mkdir -p "$BUILD_DIR"/{BUILD,RPMS,SOURCES,SPECS,SRPMS}

# Build the binary
print_info "Building lumo-agent binary"
cd "$PROJECT_ROOT"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$BUILD_DIR/lumo-agent" \
    ./cmd/lumo-agent

# Create source tarball
print_info "Creating source tarball"
TARBALL_DIR="lumo-agent-$VERSION"
TARBALL_PATH="$BUILD_DIR/SOURCES/lumo-agent-$VERSION.tar.gz"

mkdir -p "$BUILD_DIR/$TARBALL_DIR"
cp "$BUILD_DIR/lumo-agent" "$BUILD_DIR/$TARBALL_DIR/"
cp "$PROJECT_ROOT/deployments/systemd/lumo-agent.service" "$BUILD_DIR/$TARBALL_DIR/"
cp "$PROJECT_ROOT/LICENSE" "$BUILD_DIR/$TARBALL_DIR/" 2>/dev/null || echo "LICENSE" > "$BUILD_DIR/$TARBALL_DIR/LICENSE"
cp "$PROJECT_ROOT/README.md" "$BUILD_DIR/$TARBALL_DIR/" 2>/dev/null || echo "README" > "$BUILD_DIR/$TARBALL_DIR/README.md"
cp "$PROJECT_ROOT/CLAUDE.md" "$BUILD_DIR/$TARBALL_DIR/" 2>/dev/null || true
cp "$PROJECT_ROOT/DEVELOPMENT.md" "$BUILD_DIR/$TARBALL_DIR/" 2>/dev/null || true

cd "$BUILD_DIR"
tar czf "$TARBALL_PATH" "$TARBALL_DIR"
rm -rf "$BUILD_DIR/$TARBALL_DIR"

# Copy spec file
print_info "Copying spec file"
cp "$SCRIPT_DIR/lumo-agent.spec" "$BUILD_DIR/SPECS/"

# Build RPM
print_info "Building RPM package"
cd "$BUILD_DIR"
rpmbuild --define "_topdir $BUILD_DIR" \
    --define "version $VERSION" \
    --define "release $RELEASE" \
    -ba SPECS/lumo-agent.spec

# Find and display the built RPM
RPM_FILE=$(find "$BUILD_DIR/RPMS" -name "lumo-agent-*.rpm" -type f)
SRPM_FILE=$(find "$BUILD_DIR/SRPMS" -name "lumo-agent-*.src.rpm" -type f)

print_info "Build complete!"
echo ""
echo "Packages created:"
echo "  RPM:  $RPM_FILE"
echo "  SRPM: $SRPM_FILE"
echo ""
echo "Install with:"
echo "  sudo rpm -ivh $RPM_FILE"
echo ""
echo "Or with yum:"
echo "  sudo yum localinstall $RPM_FILE"
