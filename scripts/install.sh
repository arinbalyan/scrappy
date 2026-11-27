#!/usr/bin/env bash
set -euo pipefail

BASE_URL="https://github.com/arinbalyan/scrappy/releases/latest/download"
case "$(uname -s)" in
  Linux) archive="scrappy_linux_amd64.tar.gz" ;;
  Darwin)
    case "$(uname -m)" in
      arm64) archive="scrappy_darwin_arm64.tar.gz" ;;
      *) archive="scrappy_darwin_amd64.tar.gz" ;;
    esac
    ;;
  *) echo "unsupported platform" >&2; exit 1 ;;
esac

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$BASE_URL/$archive" -o "$tmp/$archive"
curl -fsSL "$BASE_URL/SHA256SUMS" -o "$tmp/SHA256SUMS"
(cd "$tmp" && grep " $archive$" SHA256SUMS | sha256sum --check --status)
tar -xzf "$tmp/$archive" -C "$tmp"
install -m 0755 "$tmp/scrappy" "${SCRAPPY_INSTALL_DIR:-$HOME/.local/bin}/scrappy"

