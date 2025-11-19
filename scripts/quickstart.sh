#!/usr/bin/env bash
# quickstart.sh - Quick installation script for Lumo
# Usage: curl -sSL https://raw.githubusercontent.com/ignacio/lumo/main/scripts/quickstart.sh | bash
# Or: wget -qO- https://raw.githubusercontent.com/ignacio/lumo/main/scripts/quickstart.sh | bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# GitHub repository
REPO="ignacio/lumo"
INSTALL_DIR="${LUMO_INSTALL_DIR:-$HOME/.local/bin}"

# Banner
echo -e "${BLUE}${BOLD}"
cat << "EOF"
╦  ╦ ╦╔╦╗╔═╗
║  ║ ║║║║║ ║
╩═╝╚═╝╩ ╩╚═╝
EOF
echo -e "${NC}"
echo -e "${BLUE}Lumo Quick Installer${NC}"
echo -e "${BLUE}====================${NC}"
echo ""

# Check if running with sudo (not recommended)
if [ "$EUID" -eq 0 ]; then
    echo -e "${YELLOW}⚠️  Warning: Running as root is not recommended${NC}"
    echo -e "${YELLOW}   Install to user directory instead: export LUMO_INSTALL_DIR=\$HOME/.local/bin${NC}"
    echo ""
fi

# Detect OS and architecture
detect_platform() {
    local os arch

    # Detect OS
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        MINGW*|MSYS*|CYGWIN*)
            echo -e "${RED}✗ Windows is not supported by this installer${NC}"
            echo "  Please download binaries from: https://github.com/$REPO/releases"
            exit 1
            ;;
        *)
            echo -e "${RED}✗ Unsupported operating system: $(uname -s)${NC}"
            exit 1
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        armv7l|armv6l)  arch="arm" ;;
        *)
            echo -e "${RED}✗ Unsupported architecture: $(uname -m)${NC}"
            exit 1
            ;;
    esac

    echo "$os-$arch"
}

# Get latest release version
get_latest_version() {
    local version
    version=$(curl -sSf https://api.github.com/repos/$REPO/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$version" ]; then
        echo -e "${RED}✗ Failed to get latest version${NC}"
        exit 1
    fi

    echo "$version"
}

# Download and extract binary
download_binary() {
    local version="$1"
    local platform="$2"
    local archive_name="lumo-${version}-${platform}.tar.gz"
    local download_url="https://github.com/$REPO/releases/download/${version}/${archive_name}"
    local tmp_dir=$(mktemp -d)

    echo -e "${BLUE}📥 Downloading Lumo ${version} for ${platform}...${NC}"

    if command -v curl >/dev/null 2>&1; then
        curl -sSfL "$download_url" -o "$tmp_dir/$archive_name"
    elif command -v wget >/dev/null 2>&1; then
        wget -q "$download_url" -O "$tmp_dir/$archive_name"
    else
        echo -e "${RED}✗ Neither curl nor wget found${NC}"
        echo "  Please install curl or wget and try again"
        exit 1
    fi

    if [ $? -ne 0 ]; then
        echo -e "${RED}✗ Failed to download binary${NC}"
        echo "  URL: $download_url"
        rm -rf "$tmp_dir"
        exit 1
    fi

    echo -e "${GREEN}✓ Downloaded successfully${NC}"
    echo ""

    # Extract archive
    echo -e "${BLUE}📦 Extracting archive...${NC}"
    tar -xzf "$tmp_dir/$archive_name" -C "$tmp_dir"

    echo "$tmp_dir/${platform}"
}

# Install binaries
install_binaries() {
    local extract_dir="$1"

    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"

    echo -e "${BLUE}📥 Installing binaries to ${INSTALL_DIR}...${NC}"

    # Install lumo CLI
    if [ -f "$extract_dir/lumo" ]; then
        cp "$extract_dir/lumo" "$INSTALL_DIR/lumo"
        chmod +x "$INSTALL_DIR/lumo"
        echo -e "${GREEN}✓ Installed lumo CLI${NC}"
    else
        echo -e "${RED}✗ lumo binary not found in archive${NC}"
        exit 1
    fi

    # Install lumo-agent
    if [ -f "$extract_dir/lumo-agent" ]; then
        cp "$extract_dir/lumo-agent" "$INSTALL_DIR/lumo-agent"
        chmod +x "$INSTALL_DIR/lumo-agent"
        echo -e "${GREEN}✓ Installed lumo-agent${NC}"
    fi

    echo ""
}

# Check if directory is in PATH
check_path() {
    case ":$PATH:" in
        *":$INSTALL_DIR:"*)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

# Add to PATH instructions
show_path_instructions() {
    local shell_rc=""

    # Detect shell config file
    if [ -n "$BASH_VERSION" ]; then
        shell_rc="$HOME/.bashrc"
    elif [ -n "$ZSH_VERSION" ]; then
        shell_rc="$HOME/.zshrc"
    else
        shell_rc="$HOME/.profile"
    fi

    echo -e "${YELLOW}⚠️  ${INSTALL_DIR} is not in your PATH${NC}"
    echo ""
    echo "Add it to your PATH by running:"
    echo -e "${BOLD}  echo 'export PATH=\"\$PATH:${INSTALL_DIR}\"' >> ${shell_rc}${NC}"
    echo -e "${BOLD}  source ${shell_rc}${NC}"
    echo ""
}

# Verify installation
verify_installation() {
    if ! check_path; then
        # Try with full path
        if [ -x "$INSTALL_DIR/lumo" ]; then
            "$INSTALL_DIR/lumo" version >/dev/null 2>&1
            return $?
        fi
        return 1
    fi

    # In PATH, try directly
    command -v lumo >/dev/null 2>&1 && lumo version >/dev/null 2>&1
    return $?
}

# Main installation flow
main() {
    echo -e "${BLUE}🔍 Detecting platform...${NC}"
    PLATFORM=$(detect_platform)
    echo -e "${GREEN}✓ Platform: ${PLATFORM}${NC}"
    echo ""

    echo -e "${BLUE}🔍 Getting latest version...${NC}"
    VERSION=$(get_latest_version)
    echo -e "${GREEN}✓ Latest version: ${VERSION}${NC}"
    echo ""

    # Download and extract
    EXTRACT_DIR=$(download_binary "$VERSION" "$PLATFORM")

    # Install binaries
    install_binaries "$EXTRACT_DIR"

    # Cleanup
    rm -rf "$(dirname "$EXTRACT_DIR")"

    # Verify installation
    echo -e "${BLUE}🔍 Verifying installation...${NC}"
    if verify_installation; then
        echo -e "${GREEN}✓ Installation verified${NC}"
    else
        echo -e "${YELLOW}⚠️  Could not verify installation${NC}"
    fi
    echo ""

    # Check PATH
    if ! check_path; then
        show_path_instructions
    fi

    # Success message
    echo -e "${GREEN}${BOLD}✅ Installation complete!${NC}"
    echo ""
    echo -e "${BLUE}Next steps:${NC}"

    if check_path; then
        echo "  1. Run setup wizard:     ${BOLD}lumo init${NC}"
        echo "  2. Try local diagnostic: ${BOLD}lumo diagnose localhost${NC}"
        echo "  3. View help:            ${BOLD}lumo --help${NC}"
    else
        echo "  1. Add Lumo to PATH (see instructions above)"
        echo "  2. Run setup wizard:     ${BOLD}$INSTALL_DIR/lumo init${NC}"
        echo "  3. Try local diagnostic: ${BOLD}$INSTALL_DIR/lumo diagnose localhost${NC}"
    fi
    echo ""
    echo -e "${BLUE}Documentation:${NC} https://github.com/$REPO"
    echo -e "${BLUE}Report issues:${NC} https://github.com/$REPO/issues"
    echo ""
}

# Run main installation
main
