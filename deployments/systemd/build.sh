#!/bin/bash
#
# Cross-Platform Build Script for Lumo Agent
# Builds binaries for multiple platforms and architectures
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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
    echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════${NC}"
    echo ""
}

# Configuration
VERSION=${VERSION:-$(git describe --tags --always 2>/dev/null || echo "dev")}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="${BUILD_DIR:-$PROJECT_ROOT/build}"
DIST_DIR="$BUILD_DIR/dist"

# Build targets (GOOS/GOARCH)
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Help message
show_help() {
    cat << EOF
Cross-Platform Build Script for Lumo Agent

Usage: ./build.sh [OPTIONS]

Options:
  --version VERSION   Set version (default: git describe or "dev")
  --targets TARGETS   Comma-separated list of targets (default: all)
  --output DIR        Output directory (default: build/dist)
  --skip-checksums    Skip generating checksums
  --skip-packages     Skip building RPM/DEB packages
  --clean             Clean build directory before building
  --help              Show this help message

Available targets:
  linux/amd64, linux/arm64, linux/arm
  darwin/amd64, darwin/arm64
  windows/amd64

Examples:
  # Build all targets
  ./build.sh

  # Build specific targets
  ./build.sh --targets linux/amd64,linux/arm64

  # Build with custom version
  ./build.sh --version 1.0.0

  # Clean build
  ./build.sh --clean

  # Build binaries only (skip packages)
  ./build.sh --skip-packages

EOF
}

# Parse arguments
SKIP_CHECKSUMS=0
SKIP_PACKAGES=0
CLEAN=0
CUSTOM_TARGETS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --targets)
            CUSTOM_TARGETS="$2"
            shift 2
            ;;
        --output)
            DIST_DIR="$2"
            shift 2
            ;;
        --skip-checksums)
            SKIP_CHECKSUMS=1
            shift
            ;;
        --skip-packages)
            SKIP_PACKAGES=1
            shift
            ;;
        --clean)
            CLEAN=1
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

# Parse custom targets
if [ -n "$CUSTOM_TARGETS" ]; then
    IFS=',' read -ra TARGETS <<< "$CUSTOM_TARGETS"
fi

print_header "Lumo Agent Cross-Platform Build"

print_info "Configuration:"
print_info "  Version:    $VERSION"
print_info "  Commit:     $COMMIT"
print_info "  Build Date: $BUILD_DATE"
print_info "  Output:     $DIST_DIR"
print_info "  Targets:    ${TARGETS[*]}"
echo ""

# Clean build directory
if [ $CLEAN -eq 1 ]; then
    print_info "Cleaning build directory"
    rm -rf "$BUILD_DIR"
fi

# Create build directories
mkdir -p "$DIST_DIR"
mkdir -p "$BUILD_DIR/checksums"

# Build function
build_binary() {
    local target=$1
    local goos=$(echo $target | cut -d'/' -f1)
    local goarch=$(echo $target | cut -d'/' -f2)

    local output_name="lumo-agent"
    local output_dir="$DIST_DIR/lumo-agent-${VERSION}-${goos}-${goarch}"

    if [ "$goos" = "windows" ]; then
        output_name="lumo-agent.exe"
    fi

    print_info "Building for $goos/$goarch"

    # Build binary
    CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build \
        -ldflags="-s -w -X main.Version=$VERSION -X main.Commit=$COMMIT -X main.BuildDate=$BUILD_DATE" \
        -o "$output_dir/$output_name" \
        "$PROJECT_ROOT/cmd/lumo-agent"

    # Create release package
    if [ "$goos" != "windows" ]; then
        # Copy systemd service for Linux
        if [ "$goos" = "linux" ]; then
            cp "$SCRIPT_DIR/lumo-agent.service" "$output_dir/"
        fi

        # Create README
        cat > "$output_dir/README.txt" << EOF
Lumo Agent v$VERSION

Platform: $goos/$goarch
Build Date: $BUILD_DATE
Commit: $COMMIT

Installation:
  1. Copy lumo-agent to /usr/local/bin/
  2. Make it executable: chmod +x /usr/local/bin/lumo-agent
  3. For systemd (Linux): See lumo-agent.service

For more information:
  https://github.com/ignacio/lumo

EOF

        # Create tarball
        cd "$DIST_DIR"
        tar czf "lumo-agent-${VERSION}-${goos}-${goarch}.tar.gz" \
            "lumo-agent-${VERSION}-${goos}-${goarch}"

        # Remove directory (keep tarball)
        rm -rf "lumo-agent-${VERSION}-${goos}-${goarch}"

        print_info "Created: lumo-agent-${VERSION}-${goos}-${goarch}.tar.gz"
    else
        # Create ZIP for Windows
        cat > "$output_dir/README.txt" << EOF
Lumo Agent v$VERSION

Platform: $goos/$goarch
Build Date: $BUILD_DATE
Commit: $COMMIT

Installation:
  1. Extract lumo-agent.exe to a directory
  2. Add the directory to your PATH
  3. Run: lumo-agent.exe version

For more information:
  https://github.com/ignacio/lumo

EOF

        cd "$DIST_DIR"
        if command -v zip &> /dev/null; then
            zip -r "lumo-agent-${VERSION}-${goos}-${goarch}.zip" \
                "lumo-agent-${VERSION}-${goos}-${goarch}" >/dev/null
            rm -rf "lumo-agent-${VERSION}-${goos}-${goarch}"
            print_info "Created: lumo-agent-${VERSION}-${goos}-${goarch}.zip"
        else
            print_warn "zip not found, skipping Windows archive"
        fi
    fi
}

# Build all targets
print_header "Building Binaries"

for target in "${TARGETS[@]}"; do
    build_binary "$target"
done

# Generate checksums
if [ $SKIP_CHECKSUMS -eq 0 ]; then
    print_header "Generating Checksums"

    cd "$DIST_DIR"

    if command -v sha256sum &> /dev/null; then
        sha256sum *.tar.gz *.zip 2>/dev/null > "lumo-agent-${VERSION}-checksums-sha256.txt" || true
        print_info "Created: lumo-agent-${VERSION}-checksums-sha256.txt"
    fi

    if command -v md5sum &> /dev/null; then
        md5sum *.tar.gz *.zip 2>/dev/null > "lumo-agent-${VERSION}-checksums-md5.txt" || true
        print_info "Created: lumo-agent-${VERSION}-checksums-md5.txt"
    fi
fi

# Build packages
if [ $SKIP_PACKAGES -eq 0 ]; then
    print_header "Building Packages"

    # Build RPM
    if [ -f "$SCRIPT_DIR/packaging/rpm/build-rpm.sh" ]; then
        print_info "Building RPM package"
        VERSION=$VERSION "$SCRIPT_DIR/packaging/rpm/build-rpm.sh" || print_warn "RPM build failed"
    fi

    # Build DEB
    if [ -f "$SCRIPT_DIR/packaging/deb/build-deb.sh" ]; then
        print_info "Building DEB package"
        VERSION=$VERSION "$SCRIPT_DIR/packaging/deb/build-deb.sh" || print_warn "DEB build failed"
    fi
fi

# Summary
print_header "Build Complete!"

print_info "Artifacts created in: $DIST_DIR"
echo ""
ls -lh "$DIST_DIR" | grep -E '\.(tar\.gz|zip|rpm|deb)$' || ls -lh "$DIST_DIR"
echo ""

print_info "Next steps:"
echo "  1. Test the binaries"
echo "  2. Create a GitHub release"
echo "  3. Upload artifacts to release"
echo "  4. Update documentation with release notes"
echo ""
