#!/usr/bin/env bash
# promote.sh <from_branch> <to_branch> [--force]
# Merges from_branch into to_branch if the last commit is 7+ days old (or --force).
set -euo pipefail

FROM=$1
TO=$2
FORCE=${3:-}

git fetch origin "$FROM" "$TO"

# Check if FROM has commits ahead of TO
AHEAD=$(git rev-list --count "origin/$TO..origin/$FROM" 2>/dev/null || echo 0)
if [ "$AHEAD" -eq 0 ]; then
  echo "no new commits on $FROM since last $TO promotion — skipping"
  exit 0
fi

if [ "$FORCE" != "--force" ]; then
  # Check age of the latest commit on FROM
  LAST_TS=$(git log -1 --format=%ct "origin/$FROM")
  NOW=$(date +%s)
  AGE_DAYS=$(( (NOW - LAST_TS) / 86400 ))
  echo "latest $FROM commit is ${AGE_DAYS}d old"
  if [ "$AGE_DAYS" -lt 7 ]; then
    echo "not ready — only ${AGE_DAYS}d old, need 7d"
    exit 0
  fi
fi

# Create a PR for visibility (or just merge directly)
PR_URL=$(gh pr list --head "$FROM" --base "$TO" --json url --jq '.[0].url' 2>/dev/null || echo "")
if [ -z "$PR_URL" ]; then
  gh pr create \
    --head "$FROM" \
    --base "$TO" \
    --title "promote: $FROM → $TO" \
    --body "Automated promotion triggered by $(date -u +%Y-%m-%d)" \
    --fill 2>/dev/null || true
fi

# Merge
git checkout -B temp-promote "origin/$FROM"
git push origin "temp-promote:$TO" 2>/dev/null || {
  echo "fast-forward failed — creating merge commit"
  git fetch origin "$TO"
  git checkout -B temp-merge "origin/$TO"
  git merge --no-ff "origin/$FROM" -m "promote: $FROM → $TO"
  git push origin "temp-merge:$TO"
}
git branch -D temp-promote 2>/dev/null || true
git branch -D temp-merge 2>/dev/null || true

echo "promoted $FROM → $TO"
