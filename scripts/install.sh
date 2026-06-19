#!/usr/bin/env sh
set -e

REPO="thakurtpr/thrive"
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

# goreleaser archives: thrive_VERSION_OS_ARCH.tar.gz (strip leading 'v' from tag)
VER="${VERSION#v}"
ARCHIVE="thrive_${VER}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

echo "Installing thrive ${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL -o "${TMP}/${ARCHIVE}" "$URL"
tar -xzf "${TMP}/${ARCHIVE}" -C "$TMP" thrive

if [ "$(id -u)" -ne 0 ] && [ ! -w "$INSTALL_DIR" ]; then
    sudo mv "${TMP}/thrive" "${INSTALL_DIR}/thrive"
else
    mv "${TMP}/thrive" "${INSTALL_DIR}/thrive"
fi

chmod +x "${INSTALL_DIR}/thrive"
echo "thrive installed to ${INSTALL_DIR}/thrive"
"${INSTALL_DIR}/thrive" --version
