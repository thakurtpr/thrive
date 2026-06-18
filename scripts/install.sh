#!/usr/bin/env sh
set -e

REPO="thakurprasadrout/thrive"
VERSION="${THRIVE_VERSION:-}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)        ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

if [ -z "$VERSION" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Could not resolve latest version. Set THRIVE_VERSION=vX.Y.Z" >&2
        exit 1
    fi
fi

BINARY="thrive-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"

echo "Installing thrive ${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp)
curl -fsSL -o "$TMP" "$URL"
chmod +x "$TMP"

if [ "$(id -u)" -ne 0 ] && [ ! -w "$INSTALL_DIR" ]; then
    sudo mv "$TMP" "${INSTALL_DIR}/thrive"
else
    mv "$TMP" "${INSTALL_DIR}/thrive"
fi

echo "thrive installed to ${INSTALL_DIR}/thrive"
"${INSTALL_DIR}/thrive" --version
