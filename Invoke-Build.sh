#!/usr/bin/env bash
# ==============================================================================
# Invoke-Build.sh — Universal Agentic Engineering Command Gateway (POSIX)
# Usage (Run from WORKSPACE root or worktree):
#   ./Invoke-Build.sh dispatch --title "<task-title>" [--repo <repo>]
#   ./Invoke-Build.sh commit -Files "path/to/file1,path/to/file2" -Message "<msg>"
#   ./Invoke-Build.sh verify
#   ./Invoke-Build.sh check --branch agent/<slug> [--repo <repo>]
#   ./Invoke-Build.sh cleanup [--worktree <name>] [--force]
#   ./Invoke-Build.sh init
# ==============================================================================

set -uo pipefail

TARGET="${1:-}"
if [[ -z "$TARGET" ]]; then
  echo "Usage: $0 <dispatch|commit|verify|check|cleanup|init> [options]"
  exit 1
fi
shift || true

CALLER_DIR="$(pwd)"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR"

REPO=""
BRANCH=""
TITLE=""
TICKET=""
WORKTREE=""
FORCE=0
ALL=0
FILES=""
MESSAGE=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --repo|-Repo)         REPO="$2"; shift 2 ;;
    --branch|-Branch)     BRANCH="$2"; shift 2 ;;
    --ticket|-Ticket)     TICKET="$2"; shift 2 ;;
    --title|-Title)       TITLE="$2"; shift 2 ;;
    --worktree|-Worktree|-wt) WORKTREE="$2"; shift 2 ;;
    --files|-Files)       FILES="$2"; shift 2 ;;
    --message|-Message|-m) MESSAGE="$2"; shift 2 ;;
    --force|-Force|-f)    FORCE=1; shift ;;
    --all|-All)           ALL=1; shift ;;
    *) shift ;;
  esac
done

case "$TARGET" in
  dispatch)
    cd "$ROOT"
    DISPATCH_ARGS=()
    [[ -n "$REPO" ]]   && DISPATCH_ARGS+=(--repo "$REPO")
    [[ -n "$TITLE" ]]  && DISPATCH_ARGS+=(--title "$TITLE")
    [[ -n "$BRANCH" ]] && DISPATCH_ARGS+=(--branch "$BRANCH")
    [[ -n "$TICKET" ]] && DISPATCH_ARGS+=(--ticket "$TICKET")
    "$ROOT/scripts/agent-dispatch.sh" "${DISPATCH_ARGS[@]}"
    ;;

  verify)
    if [[ "$CALLER_DIR" != "$ROOT" && -z "$REPO" ]]; then
      "$ROOT/scripts/verify-worktree.sh"
    else
      cd "$ROOT"
      "$ROOT/scripts/verify-worktree.sh"
    fi
    ;;

  commit)
    TARGET_DIR="$CALLER_DIR"
    if [[ -n "$REPO" && ( -n "$WORKTREE" || -n "$BRANCH" ) ]]; then
      WT_NAME="${WORKTREE:-$BRANCH}"
      TARGET_DIR="$ROOT/$REPO/.worktrees/$WT_NAME"
    elif [[ -n "$REPO" ]]; then
      TARGET_DIR="$ROOT/$REPO"
    fi

    if [[ ! -d "$TARGET_DIR" ]]; then
      echo "❌ Target directory does not exist: $TARGET_DIR"
      exit 1
    fi

    cd "$TARGET_DIR"

    if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
      echo "❌ Current directory is not a Git repository or worktree: $TARGET_DIR"
      exit 1
    fi

    if [[ -z "$FILES" ]]; then
      echo "❌ Commit failed: -Files <list> must be explicitly specified!"
      echo "   Under agentic safety rules, blanket staging (git add .) is strictly forbidden."
      echo "   Example: ./Invoke-Build.sh commit -Files \"src/file1.go,src/file2.go\" -Message \"feat: summary\""
      exit 1
    fi

    if [[ -z "$MESSAGE" ]]; then
      echo "❌ Commit failed: -Message \"<summary>\" is required!"
      exit 1
    fi

    # Parse target files
    IFS=',' read -ra TARGET_ARRAY <<< "$FILES"
    CLEAN_TARGETS=()
    for f in "${TARGET_ARRAY[@]}"; do
      trimmed=$(echo "$f" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
      [[ -n "$trimmed" ]] && CLEAN_TARGETS+=("$trimmed")
    done

    # Scan dirty files
    DIRTY_FILES=()
    while IFS= read -r line; do
      if [[ ${#line} -gt 3 ]]; then
        raw="${line:3}"
        # handle rename
        if [[ "$raw" =~ -\>[[:space:]]*(.+)$ ]]; then
          raw="${BASH_REMATCH[1]}"
        fi
        raw=$(echo "$raw" | tr -d '"' | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')
        [[ -n "$raw" ]] && DIRTY_FILES+=("$raw")
      fi
    done < <(git status --porcelain 2>/dev/null || true)

    # Check for foreign dirty files
    FOREIGN_FILES=()
    for df in "${DIRTY_FILES[@]}"; do
      is_target=0
      for tf in "${CLEAN_TARGETS[@]}"; do
        if [[ "$df" == "$tf" || "$df" == *"/$tf" || "$tf" == *"/$df" ]]; then
          is_target=1
          break
        fi
      done
      if [[ $is_target -eq 0 ]]; then
        FOREIGN_FILES+=("$df")
      fi
    done

    if [[ ${#FOREIGN_FILES[@]} -gt 0 ]]; then
      echo ""
      echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
      echo "⚠️  [Isolation Guard] Detected modified files NOT in commit list:"
      echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
      for ff in "${FOREIGN_FILES[@]}"; do
        echo "   - $ff"
      done
      echo ""
      echo "   Safely skipping ${#FOREIGN_FILES[@]} file(s). They will remain untouched."
      echo "   This strictly prevents cross-session / foreign file commit contamination."
      echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
      echo ""
    fi

    echo "▶ Precisely staging target files (${#CLEAN_TARGETS[@]} files)..."
    for tf in "${CLEAN_TARGETS[@]}"; do
      git add -- "$tf"
    done

    STAGED=$(git diff --cached --name-only)
    if [[ -z "$STAGED" ]]; then
      echo "⚠️ Staging area is empty. Specified files had no changes or were already committed."
      exit 0
    fi

    echo "▶ Executing commit: $MESSAGE"
    git commit -m "$MESSAGE"
    echo ""
    echo "✅ Precision commit successful!"
    git log -1 --stat
    ;;

  check)
    if [[ -z "$BRANCH" ]]; then
      echo "Usage: $0 check [--repo <repo>] --branch <branch>"
      exit 1
    fi
    cd "$ROOT"
    "$ROOT/scripts/pre-merge-check.sh" --repo "${REPO:-.}" --branch "$BRANCH"
    ;;

  cleanup)
    cd "$ROOT"
    RAW=$(git worktree list --porcelain 2>/dev/null || true)
    if [[ -z "$RAW" ]]; then
      echo "✅ No active worktrees found."
      exit 0
    fi

    PATTERN="${WORKTREE:-$BRANCH}"
    if [[ -z "$PATTERN" && $ALL -eq 0 ]]; then
      echo "══════════════════════════════════════════════════"
      echo "  Active Agent Worktrees"
      echo "══════════════════════════════════════════════════"
      git worktree list
      echo ""
      echo "Cleanup Command Hints:"
      echo "  ./Invoke-Build.sh cleanup --worktree <name> [--force]"
      echo "  ./Invoke-Build.sh cleanup --all [--force]"
      exit 0
    fi

    # Remove target worktrees
    echo "▶ Processing cleanup..."
    while IFS= read -r line; do
      if [[ "$line" =~ ^worktree[[:space:]]+(.+)$ ]]; then
        WT_PATH="${BASH_REMATCH[1]}"
        if [[ "$WT_PATH" == *".worktrees"* ]]; then
          if [[ $ALL -eq 1 || -n "$PATTERN" && "$WT_PATH" == *"$PATTERN"* ]]; then
            echo "▶ Removing worktree: $WT_PATH"
            git worktree remove --force "$WT_PATH" 2>/dev/null || rm -rf "$WT_PATH"
          fi
        fi
      fi
    done <<< "$RAW"
    echo "✅ Cleanup complete."
    ;;

  init)
    cd "$ROOT"
    "$ROOT/scripts/setup-workspace.sh"
    ;;

  *)
    echo "❌ Unknown command target: $TARGET"
    echo "Valid targets: dispatch, commit, verify, check, cleanup, init"
    exit 1
    ;;
esac
