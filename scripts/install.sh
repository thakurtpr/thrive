#!/bin/bash
set -e

# THRIVE Installer - Supports Linux and macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/thakurprasadrout/thrive/main/scripts/install.sh | sh

THRIVE_VERSION="${THRIVE_VERSION:-0.1.0}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
THRIVE_DIR="${THRIVE_DIR:-/var/lib/thrive}"

echo "Installing Thrive ${THRIVE_VERSION}..."

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Detect OS
OS=$(uname -s)

# Check if running as root or has sudo
if [ "$(id -u)" -ne 0 ]; then
    SUDO=sudo
else
    SUDO=
fi

# macOS installation
install_macos() {
    echo "Detected macOS..."

    # Check for Docker
    if ! command -v docker &> /dev/null; then
        echo "Docker is required on macOS for container operations."
        echo "Please install Docker Desktop from: https://docker.com/products/docker-desktop"
        echo ""
        echo "Installing CLI only (for management functions)..."
    fi

    # Build Linux binary on macOS
    echo "Building Thrive for Linux..."
    if ! command -v go &> /dev/null; then
        echo "Go is required. Install via: brew install go"
        exit 1
    fi

    GOTOOLCHAIN=auto GOOS=linux CGO_ENABLED=1 go build -o thrive ./cmd/thrive
    $SUDO cp thrive "$INSTALL_DIR/thrive"
    $SUDO chmod +x "$INSTALL_DIR/thrive"

    # Create wrapper script for macOS that can use Docker
    if command -v docker &> /dev/null; then
        echo "Setting up Docker integration..."
        # Ensure Docker container is running
        docker compose -f "$INSTALL_DIR/../thrive/docker-compose.yml" up -d 2>/dev/null || true
    fi

    echo ""
    echo "✅ Thrive installed successfully to $INSTALL_DIR/thrive"
    echo ""
    echo "Usage on macOS:"
    echo "  thrive run alpine:3.19 -- echo hello  # Runs via Docker"
    echo "  thrive ps                           # List containers"
    echo "  thrive images                       # List images"
    echo ""
    echo "For container operations, ensure Docker Desktop is running."
}

# Linux installation
install_linux() {
    echo "Detected Linux..."

    # Create directories
    echo "Creating directories..."
    $SUDO mkdir -p "$THRIVE_DIR/images" "$THRIVE_DIR/chunks" "$THRIVE_DIR/secrets" "$THRIVE_DIR/containers"
    $SUDO mkdir -p "$INSTALL_DIR"

    # Build from source
    echo "Building from source..."
    if ! command -v go &> /dev/null; then
        echo "Go is required. Install via: sudo apt install golang-go"
        exit 1
    fi

    GOTOOLCHAIN=auto GOOS=linux CGO_ENABLED=1 go build -o thrive ./cmd/thrive
    $SUDO cp thrive "$INSTALL_DIR/thrive"
    $SUDO chmod +x "$INSTALL_DIR/thrive"

    # Install runtime dependencies
    echo "Installing dependencies..."
    $SUDO apt-get update -qq 2>/dev/null || true
    $SUDO apt-get install -y -qq fuse libseccomp2 2>/dev/null || true

    # Create systemd service (optional)
    if command -v systemctl &> /dev/null; then
        echo "Setting up systemd service..."
        $SUDO tee /etc/systemd/system/thrive.service > /dev/null <<EOF
[Unit]
Description=Thrive Container Runtime
After=network.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/thrive daemon
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
        $SUDO systemctl daemon-reload 2>/dev/null || true
    fi

    echo ""
    echo "✅ Thrive installed successfully to $INSTALL_DIR/thrive"
    echo ""
    echo "To get started:"
    echo "  thrive run alpine:3.19 -- echo hello"
    echo "  thrive --help"
}

# Main
case "$OS" in
    Linux)
        install_linux
        ;;
    Darwin)
        install_macos
        ;;
    *)
        echo "Unsupported OS: $OS"
        echo "Thrive supports Linux and macOS."
        exit 1
        ;;
esac