#!/bin/sh
set -e

REPO="lucharo/society"
BINARY="society"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

info() { printf "${GREEN}%s${NC}\n" "$1"; }
warn() { printf "${YELLOW}%s${NC}\n" "$1"; }
error() { printf "${RED}%s${NC}\n" "$1" >&2; exit 1; }

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    *) error "Unsupported OS: $OS" ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    armv7*|armhf) ARCH="arm" ;;
    *) error "Unsupported architecture: $ARCH" ;;
esac

# Detect download tool
if command -v curl >/dev/null 2>&1; then
    DOWNLOAD="curl -fsSL"
    DOWNLOAD_OUT="curl -fsSL -o"
elif command -v wget >/dev/null 2>&1; then
    DOWNLOAD="wget -qO-"
    DOWNLOAD_OUT="wget -qO"
else
    error "Neither curl nor wget found. Install one and try again."
fi

# Get latest version (try jq first, fall back to grep/sed)
info "Fetching latest release..."
RELEASE_JSON=$($DOWNLOAD "https://api.github.com/repos/${REPO}/releases/latest") || error "Could not fetch release info. Check https://github.com/${REPO}/releases"

if command -v jq >/dev/null 2>&1; then
    VERSION=$(echo "$RELEASE_JSON" | jq -r '.tag_name' | sed 's/^v//')
else
    VERSION=$(echo "$RELEASE_JSON" | grep '"tag_name"' | sed -E 's/.*"v?([^"]+)".*/\1/')
fi

if [ -z "$VERSION" ]; then
    error "Could not determine latest version. Check https://github.com/${REPO}/releases"
fi
info "Latest version: v${VERSION}"

# Build download URL
FILENAME="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

# Choose install directory
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

# Download and extract
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading ${FILENAME}..."
$DOWNLOAD_OUT "$TMPDIR/$FILENAME" "$URL" || error "Download failed. Check https://github.com/${REPO}/releases"

# Verify checksum if SHA256SUMS available
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/v${VERSION}/SHA256SUMS"
if $DOWNLOAD_OUT "$TMPDIR/SHA256SUMS" "$CHECKSUMS_URL" 2>/dev/null; then
    info "Verifying checksum..."
    (
        cd "$TMPDIR"
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum -c SHA256SUMS --ignore-missing 2>/dev/null || error "Checksum verification failed"
        elif command -v shasum >/dev/null 2>&1; then
            shasum -a 256 -c SHA256SUMS --ignore-missing 2>/dev/null || error "Checksum verification failed"
        else
            warn "No checksum tool found, skipping verification"
        fi
    )
else
    warn "Could not download checksums, skipping verification"
fi

info "Extracting..."
tar -xzf "$TMPDIR/$FILENAME" -C "$TMPDIR"

# Install
mv "$TMPDIR/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

# macOS: ad-hoc code sign to avoid Gatekeeper issues
if [ "$OS" = "darwin" ]; then
    codesign -s - "$INSTALL_DIR/$BINARY" 2>/dev/null || true
fi

info "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

# Check PATH
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        warn ""
        warn "Add ${INSTALL_DIR} to your PATH:"
        warn "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.bashrc"
        warn "  # or for zsh:"
        warn "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.zshrc"
        ;;
esac

info ""
info "Run 'society' to get started."
info "Docs: https://society.luischav.es"
