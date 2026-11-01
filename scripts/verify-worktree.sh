#!/usr/bin/env bash
# ==============================================================================
# verify-worktree.sh — Universal L2 Self-Verification (POSIX)
# Runs build, lint, and tests inside the active worktree.
# ==============================================================================

set -uo pipefail

GO_TOOLCHAIN_BIN="$(find ~/go/pkg/mod/golang.org/toolchain* -name bin 2>/dev/null | sort -V | tail -1 || true)"
export PATH="/opt/homebrew/bin:$HOME/.cargo/bin:$HOME/go/bin:${GO_TOOLCHAIN_BIN:-}:$PATH"

CURRENT_DIR="$(pwd)"
echo "══════════════════════════════════════════════════"
echo "  L2 Self-Verification: $CURRENT_DIR"
echo "══════════════════════════════════════════════════"

PASSED=0
FAILED=0

run_step() {
  local name="$1"; shift
  echo ""
  echo "▶ $name"
  if "$@"; then
    echo "✅ $name passed"
    PASSED=$((PASSED+1))
  else
    echo "❌ $name failed"
    FAILED=$((FAILED+1))
  fi
}

CHECKED=0

if [[ -f "go.mod" ]]; then
  CHECKED=1
  run_step "Go Build" go build ./...
  run_step "Go Vet"   go vet ./...
  run_step "Go Test"  go test ./... -timeout 180s
fi

if [[ -f "Cargo.toml" ]]; then
  CHECKED=1
  run_step "Cargo Check" cargo check --workspace
  run_step "Cargo Test"  cargo test --workspace
fi

if [[ -f "package.json" ]]; then
  CHECKED=1
  if grep -q '"lint"' package.json; then
    run_step "NPM/PNPM Lint" npm run lint
  fi
  if grep -q '"test"' package.json; then
    run_step "NPM/PNPM Test" npm test
  fi
fi

if [[ -f "pyproject.toml" || -f "requirements.txt" ]]; then
  CHECKED=1
  if command -v pytest >/dev/null 2>&1; then
    run_step "Pytest" pytest
  fi
fi

if [[ $CHECKED -eq 0 ]]; then
  echo "⚠️ No standard project marker detected (go.mod, package.json, Cargo.toml, pyproject.toml)."
fi

echo ""
echo "══════════════════════════════════════════════════"
echo "  Verification Result: $PASSED passed / $FAILED failed"
echo "══════════════════════════════════════════════════"

if [[ $FAILED -gt 0 ]]; then
  exit 1
fi
exit 0
