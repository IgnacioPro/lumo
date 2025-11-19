#!/usr/bin/env bash
# build-release.sh - Build Lumo binaries for multiple platforms
# Usage: ./scripts/build-release.sh [version]
# Example: ./scripts/build-release.sh v1.0.0

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get version from argument or git tag
VERSION="${1:-}"
if [ -z "$VERSION" ]; then
    # Try to get from git tag
    VERSION=$(git describe --tags --exact-match 2>/dev/null || echo "")
    if [ -z "$VERSION" ]; then
        # Use latest tag or default
        VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0-dev")
        echo -e "${YELLOW}⚠️  No version specified and not on a tagged commit${NC}"
        echo -e "${YELLOW}⚠️  Using: $VERSION${NC}"
    fi
fi

# Get git commit hash
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo -e "${BLUE}🚀 Building Lumo Release Binaries${NC}"
echo -e "${BLUE}=================================${NC}"
echo "Version: $VERSION"
echo "Commit:  $COMMIT"
echo "Date:    $DATE"
echo ""

# Create build directory
BUILD_DIR="build/release"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Platforms to build
declare -a PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "linux/arm"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# LDFLAGS for version info
LDFLAGS="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE"

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS="${PLATFORM%/*}"
    GOARCH="${PLATFORM#*/}"

    echo -e "${GREEN}📦 Building for $GOOS/$GOARCH...${NC}"

    # Set binary extension for Windows
    EXT=""
    if [ "$GOOS" = "windows" ]; then
        EXT=".exe"
    fi

    # Create platform-specific directory
    PLATFORM_DIR="$BUILD_DIR/$GOOS-$GOARCH"
    mkdir -p "$PLATFORM_DIR"

    # Build lumo CLI
    echo "   Building lumo CLI..."
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "$LDFLAGS" \
        -o "$PLATFORM_DIR/lumo$EXT" \
        ./cmd/lumo

    # Build lumo-agent
    echo "   Building lumo-agent..."
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "$LDFLAGS" \
        -o "$PLATFORM_DIR/lumo-agent$EXT" \
        ./cmd/lumo-agent

    # Copy additional files
    cp LICENSE "$PLATFORM_DIR/" 2>/dev/null || true
    cp README.md "$PLATFORM_DIR/" 2>/dev/null || true

    # Create archive
    ARCHIVE_NAME="lumo-$VERSION-$GOOS-$GOARCH"
    if [ "$GOOS" = "windows" ]; then
        # Windows: create zip
        echo "   Creating $ARCHIVE_NAME.zip..."
        (cd "$BUILD_DIR" && zip -q -r "$ARCHIVE_NAME.zip" "$GOOS-$GOARCH")
        ARCHIVE_PATH="$BUILD_DIR/$ARCHIVE_NAME.zip"
    else
        # Unix: create tar.gz
        echo "   Creating $ARCHIVE_NAME.tar.gz..."
        tar czf "$BUILD_DIR/$ARCHIVE_NAME.tar.gz" -C "$BUILD_DIR" "$GOOS-$GOARCH"
        ARCHIVE_PATH="$BUILD_DIR/$ARCHIVE_NAME.tar.gz"
    fi

    # Generate checksums
    echo "   Generating checksums..."
    (cd "$BUILD_DIR" && sha256sum "$(basename "$ARCHIVE_PATH")" > "$(basename "$ARCHIVE_PATH").sha256")
    (cd "$BUILD_DIR" && md5sum "$(basename "$ARCHIVE_PATH")" > "$(basename "$ARCHIVE_PATH").md5")

    # Show file info
    SIZE=$(du -h "$ARCHIVE_PATH" | cut -f1)
    echo -e "   ${GREEN}✓${NC} Created $ARCHIVE_NAME ($SIZE)"
    echo ""
done

# Generate combined checksums
echo -e "${GREEN}📝 Generating combined checksums...${NC}"
(cd "$BUILD_DIR" && sha256sum *.tar.gz *.zip 2>/dev/null > SHA256SUMS.txt || true)
(cd "$BUILD_DIR" && md5sum *.tar.gz *.zip 2>/dev/null > MD5SUMS.txt || true)

# Display summary
echo -e "${GREEN}✅ Build Complete!${NC}"
echo ""
echo "Built binaries are in: $BUILD_DIR"
echo ""
echo "Files created:"
ls -lh "$BUILD_DIR" | grep -E '\.(tar\.gz|zip)$' | awk '{print "  " $9 " (" $5 ")"}'
echo ""
echo -e "${BLUE}Checksums:${NC}"
cat "$BUILD_DIR/SHA256SUMS.txt"
echo ""

# Optional: Test a binary
if command -v file >/dev/null 2>&1; then
    echo -e "${BLUE}Sample binary info:${NC}"
    file "$BUILD_DIR/linux-amd64/lumo"
fi

echo ""
echo -e "${GREEN}🎉 All builds successful!${NC}"
echo ""
echo -e "${YELLOW}Next steps:${NC}"
echo "  1. Test binaries: ./$BUILD_DIR/<platform>/lumo version"
echo "  2. Create git tag: git tag $VERSION && git push origin $VERSION"
echo "  3. GitHub Actions will automatically create the release"
