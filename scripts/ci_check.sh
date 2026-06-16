#!/usr/bin/env bash
set -uo pipefail

ERRORS=0

log_pass() { printf '\033[32m[PASS]\033[0m %s\n' "$*"; }
log_fail() {
  printf '\033[31m[FAIL]\033[0m %s\n' "$*"
  ERRORS=$((ERRORS + 1))
}

echo "=== GitHub Actions ==="
for file in .github/workflows/*.yml; do
  grep -q 'FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true' "$file" && log_pass "$file uses Node 24 actions" || log_fail "$file missing Node 24 env"
  grep -q 'timeout-minutes:' "$file" && log_pass "$file has job timeout" || log_fail "$file missing job timeout"
  grep -q '^concurrency:' "$file" && log_pass "$file has concurrency group" || log_fail "$file missing concurrency"
done

echo
echo "=== Security Defaults ==="
if grep -R -n 'Access-Control-Allow-Origin.*\*' server --include='*.go' >/dev/null 2>&1; then
  log_fail "CORS wildcard detected"
else
  log_pass "CORS wildcard not detected"
fi

if grep -R -n 'admin123' docker-compose.yml Dockerfile start.sh start.bat README.md README.en.md server web --exclude='package-lock.json' >/dev/null 2>&1; then
  log_fail "weak default password marker detected"
else
  log_pass "weak default password marker not detected"
fi

for pattern in '*.env' '*.db' '*.csv' 'result.json' 'screenshots/' '__pycache__/'; do
  grep -q "$pattern" .gitignore && log_pass ".gitignore covers $pattern" || log_fail ".gitignore missing $pattern"
done

if command -v gitleaks >/dev/null 2>&1; then
  if gitleaks detect --source . --redact --no-banner >/dev/null 2>&1; then
    log_pass "gitleaks history scan passed"
  else
    log_fail "gitleaks history scan found leaks"
  fi
else
  log_pass "gitleaks not installed; history scan skipped"
fi

echo
echo "=== Backend Markers ==="
grep -R -n 'EncryptString\|DecryptString' server/internal --include='*.go' >/dev/null 2>&1 && log_pass "Cookie encryption usage detected" || log_fail "Cookie encryption usage missing"
grep -R -n 'ALLOWED_ORIGINS' server/internal server/main.go --include='*.go' >/dev/null 2>&1 && log_pass "CORS origin configuration detected" || log_fail "CORS origin configuration missing"
grep -R -n 'daily_report_limit' server web --include='*.go' --include='*.vue' >/dev/null 2>&1 && log_pass "daily report limit detected" || log_fail "daily report limit missing"
grep -R -n 'OpenAPIJSON' server/internal/docs --include='*.go' >/dev/null 2>&1 && log_pass "OpenAPI spec detected" || log_fail "OpenAPI spec missing"
grep -R -n 'make swagger-check' .github/workflows --include='*.yml' >/dev/null 2>&1 && log_pass "swagger-check workflow step detected" || log_fail "swagger-check workflow step missing"

echo
echo "=== API Contract ==="
if command -v node >/dev/null 2>&1; then
  if node scripts/check_api_routes.js; then
    log_pass "frontend/backend/OpenAPI routes are aligned"
  else
    log_fail "frontend/backend/OpenAPI route check failed"
  fi
else
  log_fail "node is required for API route check"
fi

echo
if [[ "$ERRORS" -eq 0 ]]; then
  log_pass "All checks passed"
else
  log_fail "$ERRORS check(s) failed"
  exit 1
fi
