#!/usr/bin/env bash
# scripts/setup-labels.sh
#
# Creates / updates all GitHub labels defined in .github/labels.yml.
# Requires: gh CLI authenticated, yq (https://github.com/mikefarah/yq)
#
# Usage:
#   ./scripts/setup-labels.sh
#   ./scripts/setup-labels.sh --dry-run

set -euo pipefail

DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true

LABELS_FILE=".github/labels.yml"

if ! command -v gh &>/dev/null; then
  echo "ERROR: gh CLI not found. Install: https://cli.github.com"
  exit 1
fi

if ! command -v yq &>/dev/null; then
  echo "ERROR: yq not found. Install: https://github.com/mikefarah/yq#install"
  exit 1
fi

REPO=$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || echo "unknown")
echo "Target repo: ${REPO}"
echo

COUNT=$(yq '. | length' "$LABELS_FILE")
echo "Applying ${COUNT} labels..."
echo

for i in $(seq 0 $((COUNT - 1))); do
  NAME=$(yq ".[${i}].name"        "$LABELS_FILE" | tr -d '"')
  COLOR=$(yq ".[${i}].color"      "$LABELS_FILE" | tr -d '"')
  DESC=$(yq  ".[${i}].description" "$LABELS_FILE" | tr -d '"')

  if $DRY_RUN; then
    echo "[dry-run] label: ${NAME}  color: #${COLOR}"
  else
    gh label create "$NAME" \
      --color "$COLOR" \
      --description "$DESC" \
      --force   # update if exists
    echo "  ✓ ${NAME}"
  fi
done

echo
echo "Done."
