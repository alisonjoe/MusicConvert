# ==============================================================================
# setup-workspace.ps1 — Universal Agentic Workspace Setup
# Deploys pre-commit hooks, verifies agent structure, and generates entry points.
# ==============================================================================

$ErrorActionPreference = "Stop"
$WORKSPACE = Split-Path $PSScriptRoot -Parent

Write-Host ("═" * 50) -ForegroundColor Cyan
Write-Host "  Agentic Workspace Setup Wizard" -ForegroundColor Cyan
Write-Host ("═" * 50) -ForegroundColor Cyan

# 1. Verify agents structure
$agentsDir = Join-Path $WORKSPACE ".agents\agents"
$requiredAgents = @("explorer", "reviewer", "coder")

Write-Host "`n[1/4] Verifying Agents Matrix..." -ForegroundColor Yellow
foreach ($agent in $requiredAgents) {
    $agentFile = Join-Path $agentsDir "$agent\agent.md"
    if (Test-Path $agentFile) {
        Write-Host "  ✅ $agent" -ForegroundColor Green
    } else {
        Write-Host "  ⚠️ $agent (missing agent.md)" -ForegroundColor Yellow
    }
}

# 2. Ensure agents.config.yaml exists
Write-Host "`n[2/4] Checking agents.config.yaml..." -ForegroundColor Yellow
$cfgPath = Join-Path $WORKSPACE "agents.config.yaml"
$cfgTpl = Join-Path $WORKSPACE "agents.config.yaml.template"
if (-not (Test-Path $cfgPath)) {
    if (Test-Path $cfgTpl) {
        Copy-Item -Path $cfgTpl -Destination $cfgPath
        Write-Host "  ✅ Generated agents.config.yaml from template." -ForegroundColor Green
    } else {
        Write-Host "  ⚠️ agents.config.yaml not found." -ForegroundColor Yellow
    }
} else {
    Write-Host "  ✅ agents.config.yaml already exists." -ForegroundColor Green
}

# 3. Ensure Invoke-Build.ps1 exists at root
Write-Host "`n[3/4] Checking Invoke-Build.ps1..." -ForegroundColor Yellow
$ibPath = Join-Path $WORKSPACE "Invoke-Build.ps1"
$ibTpl = Join-Path $WORKSPACE "Invoke-Build.ps1.template"
if (-not (Test-Path $ibPath)) {
    if (Test-Path $ibTpl) {
        Copy-Item -Path $ibTpl -Destination $ibPath
        Write-Host "  ✅ Generated Invoke-Build.ps1." -ForegroundColor Green
    }
} else {
    Write-Host "  ✅ Invoke-Build.ps1 ready." -ForegroundColor Green
}

# 4. Deploy Git Pre-Commit Hook
Write-Host "`n[4/4] Deploying Git Pre-Commit Hooks..." -ForegroundColor Yellow

$preCommitContent = @'
#!/bin/sh
# Universal Agentic Pre-Commit Hook
STAGED=$(git diff --cached --name-only --diff-filter=ACMR)

# 1. Protect direct commits on master/main
BRANCH=$(git symbolic-ref --short HEAD 2>/dev/null)
if [ "$BRANCH" = "master" ] || [ "$BRANCH" = "main" ]; then
  echo "❌ FORBIDDEN: Direct commits to $BRANCH are prohibited."
  echo "   Please create an isolated worktree via:"
  echo "   pwsh .\\Invoke-Build.ps1 dispatch -Title <task-name>"
  exit 1
fi

exit 0
'@

# Find repos
$repoPaths = @()
if (Test-Path (Join-Path $WORKSPACE ".git")) {
    $repoPaths += $WORKSPACE
}
Get-ChildItem $WORKSPACE -Directory | Where-Object { Test-Path (Join-Path $_.FullName ".git") } | ForEach-Object {
    $repoPaths += $_.FullName
}

foreach ($r in $repoPaths) {
    $hooksDir = Join-Path $r ".git\hooks"
    if (Test-Path $hooksDir) {
        $hookPath = Join-Path $hooksDir "pre-commit"
        [IO.File]::WriteAllBytes($hookPath, [Text.Encoding]::UTF8.GetBytes($preCommitContent.Replace("`r`n", "`n")))
        Write-Host "  ✅ Hook deployed to: $r" -ForegroundColor Green
    }
}

Write-Host "`n" + ("═" * 50) -ForegroundColor Green
Write-Host "  ✅ Workspace Setup Complete!" -ForegroundColor Green
Write-Host ("═" * 50) -ForegroundColor Green
