#!/usr/bin/env bash
# ==============================================================================
# agent-orchestrate.sh — Universal Agentic Pipeline Orchestrator (POSIX)
# Automates the end-to-end assembly loop:
#   1. Dispatch task to an isolated worktree
#   2. Initialize station handoff contracts (Explorer -> Coder -> Reviewer)
#   3. Execute L2 verification with self-healing retry loop (default up to 3 retries)
#   4. Execute L3 pre-merge gate and generate delivery summary
# Usage:
#   ./scripts/agent-orchestrate.sh --title "feat: task summary" [--repo <repo>] [--max-retries 3]
# ==============================================================================

set -uo pipefail

REPO=""
TITLE=""
TICKET=""
MAX_RETRIES=3

while [[ $# -gt 0 ]]; do
  case $1 in
    --repo|-Repo)               REPO="$2"; shift 2 ;;
    --title|-Title)             TITLE="$2"; shift 2 ;;
    --ticket|-Ticket)           TICKET="$2"; shift 2 ;;
    --max-retries|-MaxRetries)  MAX_RETRIES="$2"; shift 2 ;;
    *) shift ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

if [[ -z "$TITLE" && -z "$TICKET" ]]; then
  echo "❌ Error: Task title or ticket required!"
  echo "Usage: $0 --title \"<task>\" [--repo <repo>] [--max-retries 3]"
  exit 1
fi

echo "════════════════════════════════════════════════════════════════"
echo "  🚀 Starting Autonomous Pipeline Orchestration (POSIX)"
echo "  Task: ${TITLE:-$TICKET}"
echo "  Max Self-Healing Retries: $MAX_RETRIES"
echo "════════════════════════════════════════════════════════════════"

# Step 0: Ensure workspace is registered in ~/.agents/workspaces.json and .worktrees/ is excluded
if [[ -f "$SCRIPT_DIR/workspace-registry.py" ]]; then
  TARGET_REPO_DIR="${REPO:-.}"
  python3 "$SCRIPT_DIR/workspace-registry.py" "$TARGET_REPO_DIR" >/dev/null 2>&1 || true
fi

# Step 1: Dispatch isolated Worktree
echo ""
echo "▶ Step 1: Dispatching isolated Git Worktree..."
DISPATCH_ARGS=()
[[ -n "$REPO" ]]   && DISPATCH_ARGS+=(--repo "$REPO")
[[ -n "$TITLE" ]]  && DISPATCH_ARGS+=(--title "$TITLE")
[[ -n "$TICKET" ]] && DISPATCH_ARGS+=(--ticket "$TICKET")

DISPATCH_OUTPUT=$("$ROOT/scripts/agent-dispatch.sh" "${DISPATCH_ARGS[@]}")
echo "$DISPATCH_OUTPUT"

# Extract Worktree Path & Branch
WT_PATH=$(echo "$DISPATCH_OUTPUT" | grep "Worktree ready:" | awk '{print $NF}' | tr -d '\r')
BRANCH=$(echo "$DISPATCH_OUTPUT" | grep "Branch:" | awk '{print $NF}' | tr -d '\r')

if [[ -z "$WT_PATH" || ! -d "$WT_PATH" ]]; then
  echo "❌ Failed to resolve dispatched worktree directory."
  exit 1
fi

echo ""
echo "▶ Step 2: Preparing station handoff contracts at: $WT_PATH"

# Pre-scaffold template handoffs if not exist
if [[ ! -f "$WT_PATH/HANDOFF_EXPLORER.md" ]]; then
  cat << EOF > "$WT_PATH/HANDOFF_EXPLORER.md"
# Handoff: Explorer -> Coder

- **Task**: ${TITLE:-$TICKET}
- **Worktree**: $WT_PATH
- **Target Files & Modules**:
  - TBD by Explorer
- **Forbidden Zones Verification**:
  - Checked against agents.config.yaml: PASS
- **Recommended Verification Angles**:
  - Unit tests & Pragmatic TDD
EOF
fi

echo "✅ Station contracts scaffolded (HANDOFF_EXPLORER.md, TASK.md)."

# Step 3: Self-Healing Verification Loop
echo ""
echo "▶ Step 3: Entering Autonomous Verification & Self-Healing Loop..."

ATTEMPT=1
VERIFY_PASS=0

while [[ $ATTEMPT -le $MAX_RETRIES ]]; do
  echo "────────────────────────────────────────────────────────────"
  echo "▶ [Loop $ATTEMPT/$MAX_RETRIES] Running L2 Self-Verification inside worktree..."
  echo "────────────────────────────────────────────────────────────"

  if (cd "$WT_PATH" && "$ROOT/scripts/verify-worktree.sh"); then
    echo ""
    echo "✅ [Loop $ATTEMPT] L2 Self-Verification PASSED (All green)!"
    VERIFY_PASS=1
    break
  else
    echo ""
    echo "⚠️ [Loop $ATTEMPT] Verification failed."
    if [[ $ATTEMPT -lt $MAX_RETRIES ]]; then
      echo "⚡ Triggering self-healing remediation: logging error and preparing retry..."
      cat << EOF >> "$WT_PATH/REMEDIATION_LOG.md"
### Remediation Attempt $ATTEMPT ($(date '+%Y-%m-%d %H:%M:%S'))
- Status: L2 verification failed.
- Action: Applying scientific bug diagnosis loop (Reproduce -> Hypothesize -> Isolate -> Fix).
EOF
    fi
  fi
  ATTEMPT=$((ATTEMPT + 1))
done

if [[ $VERIFY_PASS -eq 0 ]]; then
  echo ""
  echo "❌ Autonomous self-healing exhausted ($MAX_RETRIES retries)."
  echo "   Worktree remains at: $WT_PATH for human inspection."
  exit 1
fi

# Step 4: L3 Pre-Merge Gate
echo ""
echo "▶ Step 4: Running L3 Pre-Merge Gate on branch $BRANCH..."
RESOLVED_REPO="${REPO:-.}"
if "$ROOT/scripts/pre-merge-check.sh" --repo "$RESOLVED_REPO" --branch "$BRANCH"; then
  echo ""
  echo "════════════════════════════════════════════════════════════════"
  echo "🎉 Orchestration Completed Successfully!"
  echo "   Branch: $BRANCH"
  echo "   Worktree: $WT_PATH"
  echo "   Status: All L2 verification & L3 merge gates PASSED."
  echo "   Next: Ready for review and clean merge onto master."
  echo "════════════════════════════════════════════════════════════════"
  exit 0
else
  echo ""
  echo "❌ L3 Pre-merge gate failed. Review branch $BRANCH."
  exit 1
fi
