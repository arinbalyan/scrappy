#!/usr/bin/env bash
set -euo pipefail

# Scrappy installer
# Usage: curl -fsSL https://raw.githubusercontent.com/arinbalyan/scrappy/main/scripts/install.sh | bash

REPO="${SCRAPPY_REPO:-arinbalyan/scrappy}"
VERSION="${SCRAPPY_VERSION:-latest}"

os() {
  case "$(uname -s)" in
    Linux*)  echo linux ;;
    Darwin*) echo darwin ;;
    *)       echo unsupported ;;
  esac
}

arch() {
  case "$(uname -m)" in
    x86_64)  echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *)       echo unsupported ;;
  esac
}

OS="$(os)"
ARCH="$(arch)"
BINARY="scrappy_${OS}_${ARCH}"

if [[ "$OS" == "unsupported" || "$ARCH" == "unsupported" ]]; then
  echo "Unsupported platform: $(uname -s) $(uname -m)"
  exit 1
fi

if [[ "$OS" == "darwin" ]]; then
  EXT="tar.gz"
else
  EXT="tar.gz"
fi

URL="https://github.com/${REPO}/releases/${VERSION}/download/${BINARY}.${EXT}"

echo "Installing Scrappy ${VERSION} for ${OS}/${ARCH}..."
echo "  → ${URL}"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" -o "$TMP/scrappy.${EXT}"

if [[ "$EXT" == "tar.gz" ]]; then
  tar xzf "$TMP/scrappy.${EXT}" -C "$TMP"
fi

INSTALL_DIR="${SCRAPPY_INSTALL_DIR:-/usr/local/bin}"
BIN_PATH="${INSTALL_DIR}/${BINARY}"

echo "Installing to ${BIN_PATH} ..."
mv "$TMP/${BINARY}" "$BIN_PATH"
chmod +x "$BIN_PATH"

echo "✓ Done. Run '${BINARY} --help' to get started."
