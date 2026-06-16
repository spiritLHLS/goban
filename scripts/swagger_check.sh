#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

(cd server && go test ./internal/docs)
node scripts/check_api_routes.js

if command -v swag >/dev/null 2>&1; then
  tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/goban-swag.XXXXXX")"
  trap 'rm -rf "$tmp_dir"' EXIT
  (cd server && swag init --parseInternal --parseDependency --output "$tmp_dir" --generatedTime=false)
  test -s "$tmp_dir/docs.go"
  echo "swag init check passed"
else
  echo "swag not installed; embedded OpenAPI and route coverage checks passed"
fi
