#!/bin/sh
# scrappy installer — auto-detects OS and arch, downloads latest release
set -eu

REPO="arinbalyan/scrappy"
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -o '"tag_name":"[^"]*"' | head -1 | sed 's/"tag_name":"//;s/"//')

if [ -z "$LATEST" ]; then
  echo "Error: could not determine latest release" >&2
  exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Error: unsupported arch $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux)  FILE="scrappy_linux_${ARCH}.tar.gz" ;;
  darwin) FILE="scrappy_darwin_${ARCH}.tar.gz" ;;
  *) echo "Error: unsupported OS $OS" >&2; exit 1 ;;
esac

URL="https://github.com/$REPO/releases/download/$LATEST/$FILE"
TMP=$(mktemp -d)
echo "Downloading scrappy $LATEST for $OS/$ARCH..."
curl -fsSL "$URL" | tar xz -C "$TMP"

sudo mv "$TMP/scrappy_${OS}_${ARCH}" /usr/local/bin/scrappy
sudo chmod +x /usr/local/bin/scrappy
rm -rf "$TMP"

echo "✓ scrappy $LATEST installed to /usr/local/bin/scrappy"
echo "  Run 'scrappy --help' to get started."