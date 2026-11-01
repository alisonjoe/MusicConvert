#!/usr/bin/env bash
# ==============================================================================
# setup-workspace.sh — Universal Agentic Workspace Setup (POSIX)
# Deploys pre-commit hooks, verifies agent structure, and generates entry points.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORKSPACE="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "══════════════════════════════════════════════════"
echo "  Agentic Workspace Setup Wizard (POSIX)"
echo "══════════════════════════════════════════════════"

# 1. Verify agents structure
echo ""
echo "[1/4] Verifying Agents Matrix..."
for agent in explorer reviewer coder; do
  if [[ -f "$WORKSPACE/.agents/agents/$agent/agent.md" ]]; then
    echo "  ✅ $agent"
  else
    echo "  ⚠️ $agent (missing agent.md)"
  fi
done

# 2. Ensure agents.config.yaml exists
echo ""
echo "[2/4] Checking agents.config.yaml..."
if [[ ! -f "$WORKSPACE/agents.config.yaml" ]]; then
  if [[ -f "$WORKSPACE/agents.config.yaml.template" ]]; then
    cp "$WORKSPACE/agents.config.yaml.template" "$WORKSPACE/agents.config.yaml"
    echo "  ✅ Generated agents.config.yaml from template."
  else
    echo "  ⚠️ agents.config.yaml not found."
  fi
else
  echo "  ✅ agents.config.yaml already exists."
fi

# 3. Ensure Invoke-Build entry points exist
echo ""
echo "[3/4] Checking Invoke-Build entry points..."
if [[ ! -f "$WORKSPACE/Invoke-Build.sh" && -f "$WORKSPACE/Invoke-Build.sh.template" ]]; then
  cp "$WORKSPACE/Invoke-Build.sh.template" "$WORKSPACE/Invoke-Build.sh"
  chmod +x "$WORKSPACE/Invoke-Build.sh"
  echo "  ✅ Generated Invoke-Build.sh."
fi
if [[ ! -f "$WORKSPACE/Invoke-Build.ps1" && -f "$WORKSPACE/Invoke-Build.ps1.template" ]]; then
  cp "$WORKSPACE/Invoke-Build.ps1.template" "$WORKSPACE/Invoke-Build.ps1"
  echo "  ✅ Generated Invoke-Build.ps1."
fi

# 4. Deploy Git Pre-Commit Hook
echo ""
echo "[4/4] Deploying Git Pre-Commit Hooks..."
if [[ -d "$WORKSPACE/.git/hooks" ]]; then
  cat << 'EOF' > "$WORKSPACE/.git/hooks/pre-commit"
#!/bin/sh
BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null)
if [ "$BRANCH" = "master" ] || [ "$BRANCH" = "main" ]; then
  echo "❌ FORBIDDEN: Direct commits to $BRANCH are prohibited."
  echo "   Please create an isolated worktree first (Invoke-Build dispatch)."
  exit 1
fi
exit 0
EOF
  chmod +x "$WORKSPACE/.git/hooks/pre-commit"
  echo "  ✅ Pre-commit hook installed to $WORKSPACE/.git/hooks/pre-commit"
fi

echo ""
echo "══════════════════════════════════════════════════"
echo "  🎉 Setup Complete! Workspace is AI-Ready."
echo "══════════════════════════════════════════════════"
