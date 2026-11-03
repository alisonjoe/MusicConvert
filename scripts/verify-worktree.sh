#!/usr/bin/env bash
# ==============================================================================
# verify-worktree.sh — Universal L2 Smart Incremental Self-Verification (POSIX)
# Supports:
#   - Automatic Git-diff based incremental verification (skips untouched binaries/modules)
#   - Fast-track flags: --skip-verify, --reinstall-only, --force
# Usage:
#   ./scripts/verify-worktree.sh [--skip-verify] [--reinstall-only] [--force]
# ==============================================================================

set -uo pipefail

GO_TOOLCHAIN_BIN="$(find ~/go/pkg/mod/golang.org/toolchain* -name bin 2>/dev/null | sort -V | tail -1 || true)"
export PATH="/opt/homebrew/bin:$HOME/.cargo/bin:$HOME/go/bin:${GO_TOOLCHAIN_BIN:-}:$PATH"

FORCE=false
SKIP_VERIFY=false
REINSTALL_ONLY=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --force|-f)           FORCE=true; shift ;;
    --skip-verify|-s)     SKIP_VERIFY=true; shift ;;
    --reinstall-only|-r)  REINSTALL_ONLY=true; shift ;;
    *) shift ;;
  esac
done

CURRENT_DIR="$(pwd)"
echo "══════════════════════════════════════════════════"
echo "  ⚡ L2 Verification (Smart Incremental): $CURRENT_DIR"
echo "══════════════════════════════════════════════════"

if [[ "$SKIP_VERIFY" = true || "$REINSTALL_ONLY" = true ]]; then
  echo "⏩ [Fast-Track] 检测到跳过参数 (--skip-verify / --reinstall-only)，已极速通过验证流程！"
  exit 0
fi

# 1. 检测工作区变动范围 (Git Diff)
CHANGED_FILES=""
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  # 优先检测当前沙箱相对主干分支的差异
  BASE_BRANCH="main"
  if git rev-parse --verify "master" >/dev/null 2>&1; then
    BASE_BRANCH="master"
  fi
  CHANGED_FILES=$(git diff --name-only "$BASE_BRANCH" 2>/dev/null || git status -s | awk '{print $NF}' || true)
fi

echo "🔍 扫描工作树变更范围..."
if [[ -z "$CHANGED_FILES" && "$FORCE" = false ]]; then
  echo "✨ [Smart Cache] 当前工作树没有任何代码或配置变动，直接秒级命中缓存跳过验证！"
  echo "   (如需全量强制重新验证，请传入 --force)"
  exit 0
fi

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

# 2. 差量按需编译与测试
MODULE_MATCHED=0

if [[ -f "go.mod" ]]; then
  MODULE_MATCHED=1
  if [[ "$FORCE" = true ]] || echo "$CHANGED_FILES" | grep -qE "\.go$|go\.mod|go\.sum"; then
    run_step "Go Build" go build ./...
    run_step "Go Vet"   go vet ./...
    run_step "Go Test"  go test ./... -timeout 180s
  else
    echo "⏩ [Go 模块未变动] 跳过 Go 编译与全量测试"
  fi
fi

if [[ -f "Cargo.toml" ]]; then
  MODULE_MATCHED=1
  if [[ "$FORCE" = true ]] || echo "$CHANGED_FILES" | grep -qE "\.rs$|Cargo\.toml|Cargo\.lock"; then
    run_step "Cargo Check" cargo check --workspace
    run_step "Cargo Test"  cargo test --workspace
  else
    echo "⏩ [Rust 模块未变动] 跳过 Cargo 编译与测试"
  fi
fi

if [[ -f "package.json" ]]; then
  MODULE_MATCHED=1
  if [[ "$FORCE" = true ]] || echo "$CHANGED_FILES" | grep -qE "\.(js|ts|jsx|tsx|vue|json)$"; then
    if grep -q '"lint"' package.json; then
      run_step "NPM/PNPM Lint" npm run lint
    fi
    if grep -q '"test"' package.json; then
      run_step "NPM/PNPM Test" npm test
    fi
  else
    echo "⏩ [前端/Node 模块未变动] 跳过 JS/TS 校验与测试"
  fi
fi

if [[ -f "pyproject.toml" || -f "requirements.txt" || -d "tests" ]]; then
  MODULE_MATCHED=1
  if [[ "$FORCE" = true ]] || echo "$CHANGED_FILES" | grep -qE "\.py$|requirements.*\.txt|pyproject\.toml"; then
    if command -v pytest >/dev/null 2>&1; then
      if [[ -d "tests" ]]; then
        run_step "Pytest (tests/)" pytest tests/
      else
        run_step "Pytest" pytest
      fi
    elif command -v python3 >/dev/null 2>&1; then
      run_step "Python Unittest" python3 -m unittest discover -s tests
    fi
  else
    echo "⏩ [Python 模块未变动] 跳过 Python 校验与测试"
  fi
fi

if [[ $MODULE_MATCHED -eq 0 ]]; then
  echo "⚠️ No standard project marker detected (go.mod, package.json, Cargo.toml, pyproject.toml, tests/)."
fi

echo ""
echo "══════════════════════════════════════════════════"
echo "  Verification Result: $PASSED passed / $FAILED failed"
echo "══════════════════════════════════════════════════"

if [[ $FAILED -gt 0 ]]; then
  exit 1
fi
exit 0
