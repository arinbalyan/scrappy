#!/usr/bin/env bash
# sync-all.sh — promote dev through branches via PRs, one at a time.
# Handles both fast-forward and squash merges.
set -euo pipefail

cd "$(dirname "$0")"

promote() {
  local FROM=$1 TO=$2
  local REVIEWER=${3:-}

  echo ""
  echo "=== $FROM → $TO ==="
  git fetch origin "$FROM" "$TO" 2>/dev/null || true
  if git diff --quiet "origin/$TO" "origin/$FROM"; then
    echo "no content differences — skipping"
    return
  fi

  # Check if there are actual commits ahead (non-squash case)
  local AHEAD
  AHEAD=$(git rev-list --count "origin/$TO..origin/$FROM" 2>/dev/null || echo 0)

  local COUNT
  if [ "$AHEAD" -gt 0 ]; then
    COUNT=$(git log --oneline "origin/$TO..origin/$FROM" | wc -l)
  else
    COUNT="(squashed)"
  fi

  local PR_URL
  PR_URL=$(gh pr list --head "$FROM" --base "$TO" --json url --jq '.[0].url' 2>/dev/null || echo "")
  if [ -z "$PR_URL" ]; then
    if [ -n "$REVIEWER" ]; then
      gh pr create --head "$FROM" --base "$TO" \
        --title "promote: $FROM → $TO" \
        --body "Automated promotion. $COUNT commits ahead." \
        --reviewer "$REVIEWER" --fill 2>/dev/null || true
    else
      gh pr create --head "$FROM" --base "$TO" \
        --title "promote: $FROM → $TO" \
        --body "Automated promotion. $COUNT commits ahead." \
        --fill 2>/dev/null || true
    fi
    PR_URL=$(gh pr list --head "$FROM" --base "$TO" --json url --jq '.[0].url' 2>/dev/null || echo "")
  fi

  if [ -z "$PR_URL" ]; then
    echo "failed to create PR — skipping"
    return
  fi
  echo "PR: $PR_URL"

  if [ -n "$REVIEWER" ]; then
    gh pr review "$PR_URL" --approve 2>/dev/null || true
    sleep 1
  fi

  gh pr merge "$PR_URL" --merge 2>/dev/null || true

  echo "waiting for merge..."
  for i in $(seq 1 60); do
    sleep 2
    STATE=$(gh pr view "$PR_URL" --json state --jq '.state' 2>/dev/null || echo "")
    if [ "$STATE" = "MERGED" ]; then
      echo "merged"
      git fetch origin "$TO" 2>/dev/null || true
      break
    fi
  done

  echo "done: $FROM → $TO"
}

git fetch origin dev beta staging main 2>/dev/null || git fetch --all

promote dev beta
promote beta staging
promote staging main arinbalyan

echo ""
echo "Chain complete."
