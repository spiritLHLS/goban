#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

# Run docs package tests
(cd server && go test ./internal/docs)

# Check API route alignment (frontend <-> backend <-> OpenAPI spec)
node scripts/check_api_routes.js

# Regenerate swagger docs if swag is available
if command -v swag >/dev/null 2>&1; then
  echo "Regenerating swagger docs in server/docs..."
  (cd server && swag init --parseInternal --parseDependency --output docs --generatedTime=false --quiet)

  # Check whether the regenerated docs differ from what's committed
  if git diff --exit-code server/docs/ >/dev/null 2>&1; then
    echo "Swagger docs are up to date"
  else
    echo "Swagger docs are outdated and have been regenerated locally"
    if [ "${AUTOFIX:-false}" = "true" ]; then
      echo "Auto-fixing: committing and pushing regenerated docs..."
      git config user.name  "github-actions[bot]"
      git config user.email "github-actions[bot]@users.noreply.github.com"
      git add server/docs/
      git commit -m "docs: auto-regenerate swagger documentation [skip ci]"
      git push
      echo "Auto-fixed and pushed swagger docs"
    else
      echo "Run with AUTOFIX=true to auto-commit changes, or manually run:"
      echo "  cd server && swag init --parseInternal --parseDependency --output docs --generatedTime=false"
      exit 1
    fi
  fi
else
  echo "swag not installed; embedded OpenAPI and route coverage checks passed"
fi
