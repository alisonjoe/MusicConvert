#!/usr/bin/env bash
# ==============================================================================
# agent-dispatch.sh — Universal Agentic Worktree Dispatcher (POSIX)
# Usage: ./scripts/agent-dispatch.sh --title "Task Title" [--repo <repo>] [--ticket <file>]
# ==============================================================================

set -euo pipefail

REPO=""
TITLE=""
BRANCH=""
TICKET=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --repo)   REPO="$2"; shift 2 ;;
    --title)  TITLE="$2"; shift 2 ;;
    --branch) BRANCH="$2"; shift 2 ;;
    --ticket) TICKET="$2"; shift 2 ;;
    *) echo "Unknown parameter: $1"; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="$(cd "$SCRIPT_DIR/.." && pwd)"

# 1. Resolve repo
if [[ -z "$REPO" ]]; then
  GIT_TOPLEVEL=$(git rev-parse --show-toplevel 2>/dev/null || true)
  if [[ -n "$GIT_TOPLEVEL" ]]; then
    REPO="$GIT_TOPLEVEL"
  elif [[ -d "$WORKSPACE/.git" ]]; then
    REPO="$WORKSPACE"
  else
    REPOS=()
    while IFS= read -r r; do
      [[ -n "$r" ]] && REPOS+=("$r")
    done < <(find "$WORKSPACE" -mindepth 1 -maxdepth 1 -type d -exec test -e "{}/.git" ';' -print -prune 2>/dev/null || true)

    if [[ ${#REPOS[@]} -eq 1 ]]; then
      REPO="${REPOS[0]}"
    elif [[ ${#REPOS[@]} -gt 1 ]]; then
      echo "❌ Multiple repositories detected. Please specify --repo <name>:"
      for r in "${REPOS[@]}"; do echo "   - $(basename "$r")"; done
      exit 1
    else
      echo "❌ No Git repository found in $WORKSPACE"
      exit 1
    fi
  fi
fi

REPO_PATH="$REPO"
[[ "$REPO" == "." ]] && REPO_PATH="$WORKSPACE"

if [[ ! -d "$REPO_PATH" ]]; then
  echo "❌ Repository path does not exist: $REPO_PATH"
  exit 1
fi

# 2. Resolve title from ticket
if [[ -z "$TITLE" && -n "$TICKET" && -f "$TICKET" ]]; then
  TITLE=$(grep -m 1 '^#\s\+' "$TICKET" | sed 's/^#\s*//' || true)
fi

if [[ -z "$TITLE" ]]; then
  echo "❌ Please specify task title via --title \"<string>\" or --ticket <file>"
  exit 1
fi

SLUG=$(echo "$TITLE" | tr '[:upper:] ' '[:lower:]-' | tr -cd 'a-z0-9-')
DATE_STR=$(date +%Y%m%d)
BASE_BRANCH="agent/$SLUG-$DATE_STR"
BRANCH_NAME="$BASE_BRANCH"
CANDIDATE_SLUG="$SLUG"
WORKTREE_DIR="$REPO_PATH/.worktrees/$CANDIDATE_SLUG"

# Detect collision and auto-suffix (-s2, -s3...) to prevent concurrent session contamination
SEQ=2
while [[ -d "$WORKTREE_DIR" ]] || git -C "$REPO_PATH" rev-parse --verify --quiet "refs/heads/$BRANCH_NAME" >/dev/null 2>&1; do
  CANDIDATE_SLUG="${SLUG}-s${SEQ}"
  BRANCH_NAME="agent/${CANDIDATE_SLUG}-${DATE_STR}"
  WORKTREE_DIR="$REPO_PATH/.worktrees/$CANDIDATE_SLUG"
  SEQ=$(( SEQ + 1 ))
done

if [[ "$CANDIDATE_SLUG" != "$SLUG" ]]; then
  echo ""
  echo "⚡ [Collision Guard] Active worktree or branch for [$SLUG] already exists!"
  echo "   Allocating isolated session worktree: $CANDIDATE_SLUG ($BRANCH_NAME)"
  echo ""
fi

echo "══════════════════════════════════════════════════"
echo "  Agent Worktree Dispatch (POSIX)"
echo "  Repo:   $REPO"
echo "  Branch: $BRANCH_NAME"
echo "  Path:   $WORKTREE_DIR"
echo "══════════════════════════════════════════════════"

cd "$REPO_PATH"
BASE=$(git branch --show-current 2>/dev/null || true)
if [[ -z "$BASE" ]]; then
  if git show-ref --verify --quiet "refs/heads/master"; then
    BASE="master"
  else
    BASE="main"
  fi
fi

mkdir -p .worktrees
echo "▶ Creating git worktree from $BASE..."
git worktree add -b "$BRANCH_NAME" "$WORKTREE_DIR" "$BASE"

# Write TASK.md
TASK_FILE="$WORKTREE_DIR/TASK.md"
cat << EOF > "$TASK_FILE"
# Task: $TITLE

- **Repo**: $REPO
- **Branch**: $BRANCH_NAME
- **Base**: $BASE
- **Created**: $(date '+%Y-%m-%d %H:%M:%S')

## Implementation Scope & Verification
- Keep changes strictly scoped to this task.
- Follow TDD (Red-Green-Refactor) for business logic.
- Verify using:
  - POSIX (macOS/Linux): \`../../Invoke-Build.sh verify\`
  - Windows: \`pwsh ..\\..\\Invoke-Build.ps1 verify\`
- Commit using (Precision Staging):
  - POSIX (macOS/Linux): \`../../Invoke-Build.sh commit -Files "path/file" -Message "feat: description"\`
  - Windows: \`pwsh ..\\..\\Invoke-Build.ps1 commit -Files "path/file" -Message "feat: description"\`
- NEVER use blanket staging (git add . / git add -A / git commit -a).
EOF

echo "✅ Worktree ready: $WORKTREE_DIR"
echo "   Task ticket initialized at: $TASK_FILE"
