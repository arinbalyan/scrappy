#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

chmod +x scripts/hooks/pre-push scripts/hooks/commit-msg
ln -sf ../../scripts/hooks/pre-push .git/hooks/pre-push
ln -sf ../../scripts/hooks/commit-msg .git/hooks/commit-msg

echo "Git hooks installed (pre-push, commit-msg)."

echo "Installing pre-commit hooks if pre-commit exists..."
if command -v pre-commit >/dev/null 2>&1; then
  pre-commit install
  pre-commit install --hook-type pre-push
  echo "pre-commit hooks installed."
else
  echo "pre-commit not found. Install with: pipx install pre-commit (or pip install pre-commit)"
fi
