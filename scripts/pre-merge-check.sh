#!/usr/bin/env bash
# ==============================================================================
# pre-merge-check.sh — Universal Pre-Merge Gate (POSIX)
# Local CI substitute before merging into master/main.
# Usage: ./scripts/pre-merge-check.sh [--repo <repo>] --branch <branch>
# ==============================================================================

set -uo pipefail

GO_TOOLCHAIN_BIN="$(find ~/go/pkg/mod/golang.org/toolchain* -name bin 2>/dev/null | sort -V | tail -1 || true)"
export PATH="/opt/homebrew/bin:$HOME/.cargo/bin:$HOME/go/bin:${GO_TOOLCHAIN_BIN:-}:$PATH"

REPO="."
BRANCH=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --repo)   REPO="$2"; shift 2 ;;
    --branch) BRANCH="$2"; shift 2 ;;
    -*) echo "Unknown option: $1"; exit 1 ;;
    *)
      if [[ -z "$BRANCH" ]]; then
        BRANCH="$1"
      fi
      shift ;;
  esac
done

if [[ -z "$BRANCH" ]]; then
  echo "Usage: $0 [--repo <repo>] --branch <branch>"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_PATH="$WORKSPACE"
[[ "$REPO" != "." ]] && REPO_PATH="$WORKSPACE/$REPO"

if [[ ! -d "$REPO_PATH" ]]; then
  echo "❌ Repository path does not exist: $REPO_PATH"
  exit 1
fi

cd "$REPO_PATH"

PASS=0
FAIL=0

run_check() {
  local name="$1"; shift
  echo ""
  echo "───────────────────────────────────────────"
  echo "▶ $name"
  if "$@"; then
    echo "✅ $name passed"
    PASS=$((PASS+1))
  else
    echo "❌ $name failed"
    FAIL=$((FAIL+1))
  fi
}

echo "═══════════════════════════════════════════"
echo "  Pre-Merge Gate: $REPO :: $BRANCH"
echo "═══════════════════════════════════════════"

CURRENT=$(git branch --show-current)
if [[ "$CURRENT" != "master" && "$CURRENT" != "main" ]]; then
  echo "❌ Must be executed on master or main branch (current: $CURRENT)"
  exit 1
fi

if ! git rev-parse --verify "$BRANCH" >/dev/null 2>&1; then
  echo "❌ Branch does not exist: $BRANCH"
  exit 1
fi

BASE="$CURRENT"

# 1. Forbidden Zone Scan
check_forbidden_zones() {
  local changed
  changed=$(git diff --name-only "$BASE...$BRANCH" 2>/dev/null || git diff --name-only "$BASE" "$BRANCH")
  local commit_logs
  commit_logs=$(git log "$BASE..$BRANCH" --oneline 2>/dev/null || true)
  local hits=0

  # Default forbidden patterns
  if echo "$changed" | grep -qE "core/models/"; then
    if ! echo "$commit_logs" | grep -q "\[schema\]"; then
      echo "  ❌ Forbidden zone touched: core/models/ — Requires [schema] in commit log"
      hits=$((hits+1))
    fi
  fi
  if echo "$changed" | grep -qE "\.proto$"; then
    if ! echo "$commit_logs" | grep -q "\[proto\]"; then
      echo "  ❌ Forbidden zone touched: .proto — Requires [proto] in commit log"
      hits=$((hits+1))
    fi
  fi

  if [[ $hits -gt 0 ]]; then
    return 1
  fi
  return 0
}

run_check "1. Forbidden Zone Scan" check_forbidden_zones

# 2. Build & Test
check_build_and_test() {
  local ret=0
  if [[ -f "go.mod" ]]; then
    go build ./... && go vet ./... && go test ./... -timeout 180s || ret=1
  fi
  if [[ -f "Cargo.toml" ]]; then
    cargo check --workspace && cargo test --workspace || ret=1
  fi
  return $ret
}

run_check "2. Build & Test" check_build_and_test

echo ""
echo "═══════════════════════════════════════════"
echo "  Gate Summary: $PASS passed / $FAIL failed"
echo "═══════════════════════════════════════════"

if [[ $FAIL -gt 0 ]]; then
  echo "❌ Pre-merge gate FAILED. Please resolve issues before merging."
  exit 1
fi

echo "✅ Pre-merge gate PASSED. Ready for squash merge."
exit 0
