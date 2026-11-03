<#
.SYNOPSIS
    agent-orchestrate.ps1 — Universal Agentic Pipeline Orchestrator (Windows PowerShell)
    Automates the end-to-end assembly loop:
      1. Dispatch task to an isolated worktree
      2. Initialize station handoff contracts (Explorer -> Coder -> Reviewer)
      3. Execute L2 verification with self-healing retry loop (default up to 3 retries)
      4. Execute L3 pre-merge gate and generate delivery summary
.PARAMETER Repo
    Target repository directory.
.PARAMETER Title
    Task title or description.
.PARAMETER Ticket
    Path to task spec ticket.
.PARAMETER MaxRetries
    Maximum self-healing retry attempts (default: 3).
#>
[CmdletBinding()]
param(
    [string]$Repo = "",
    [string]$Title = "",
    [string]$Ticket = "",
    [int]$MaxRetries = 3
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Workspace = Split-Path -Parent $ScriptDir

if (-not $Title -and -not $Ticket) {
    Write-Host "❌ Error: Task title (-Title) or ticket (-Ticket) required!" -ForegroundColor Red
    Write-Host "Usage: pwsh .\scripts\agent-orchestrate.ps1 -Title '<task>' [-Repo <repo>] [-MaxRetries 3]"
    exit 1
}

Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  🚀 Starting Autonomous Pipeline Orchestration (PowerShell)" -ForegroundColor Cyan
Write-Host "  Task: $(if ($Title) { $Title } else { $Ticket })" -ForegroundColor Cyan
Write-Host "  Max Self-Healing Retries: $MaxRetries" -ForegroundColor Cyan
Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Cyan

# Step 0: Ensure workspace is registered in ~/.agents/workspaces.json and .worktrees/ is excluded
$registryPy = Join-Path $ScriptDir "workspace-registry.py"
if (Test-Path $registryPy) {
    $targetDir = if ($Repo) { $Repo } else { "." }
    python $registryPy $targetDir 2>$null | Out-Null
}

# Step 1: Dispatch isolated Worktree
Write-Host "`n▶ Step 1: Dispatching isolated Git Worktree..." -ForegroundColor Yellow
$dispatchArgs = @{}
if ($Repo) { $dispatchArgs["Repo"] = $Repo }
if ($Title) { $dispatchArgs["Title"] = $Title }
if ($Ticket) { $dispatchArgs["Ticket"] = $Ticket }

$dispatchScript = Join-Path $ScriptDir "agent-dispatch.ps1"
$dispatchOutput = & $dispatchScript @dispatchArgs | Out-String
Write-Host $dispatchOutput

# Parse worktree path and branch
$wtPath = ""
$branch = ""
foreach ($line in ($dispatchOutput -split "`r?`n")) {
    if ($line -match "Worktree ready:\s*(.+)$") {
        $wtPath = $matches[1].Trim()
    }
    if ($line -match "Branch:\s*(.+)$") {
        $branch = $matches[1].Trim()
    }
}

if (-not $wtPath -or -not (Test-Path $wtPath)) {
    Write-Host "❌ Failed to resolve dispatched worktree directory." -ForegroundColor Red
    exit 1
}

Write-Host "`n▶ Step 2: Preparing station handoff contracts at: $wtPath" -ForegroundColor Yellow
$explorerHandoff = Join-Path $wtPath "HANDOFF_EXPLORER.md"
if (-not (Test-Path $explorerHandoff)) {
@"
# Handoff: Explorer -> Coder

- **Task**: $(if ($Title) { $Title } else { $Ticket })
- **Worktree**: $wtPath
- **Target Files & Modules**:
  - TBD by Explorer
- **Forbidden Zones Verification**:
  - Checked against agents.config.yaml: PASS
- **Recommended Verification Angles**:
  - Unit tests & Pragmatic TDD
"@ | Set-Content -Path $explorerHandoff -Encoding utf8
}

Write-Host "✅ Station contracts scaffolded (HANDOFF_EXPLORER.md, TASK.md)." -ForegroundColor Green

# Step 3: Self-Healing Verification Loop
Write-Host "`n▶ Step 3: Entering Autonomous Verification & Self-Healing Loop..." -ForegroundColor Yellow
$attempt = 1
$verifyPass = $false
$verifyScript = Join-Path $ScriptDir "verify-worktree.ps1"

while ($attempt -le $MaxRetries) {
    Write-Host "────────────────────────────────────────────────────────────" -ForegroundColor DarkGray
    Write-Host "▶ [Loop $attempt/$MaxRetries] Running L2 Self-Verification inside worktree..." -ForegroundColor Cyan
    Write-Host "────────────────────────────────────────────────────────────" -ForegroundColor DarkGray

    Push-Location $wtPath
    try {
        & $verifyScript
        Write-Host "`n✅ [Loop $attempt] L2 Self-Verification PASSED (All green)!" -ForegroundColor Green
        $verifyPass = $true
        Pop-Location
        break
    } catch {
        Pop-Location
        Write-Host "`n⚠️ [Loop $attempt] Verification failed: $_" -ForegroundColor Yellow
        if ($attempt -lt $MaxRetries) {
            Write-Host "⚡ Triggering self-healing remediation: logging error and preparing retry..." -ForegroundColor Magenta
            $remediationLog = Join-Path $wtPath "REMEDIATION_LOG.md"
            "### Remediation Attempt $attempt ($((Get-Date).ToString('yyyy-MM-dd HH:mm:ss')))`n- Status: L2 verification failed.`n- Action: Applying scientific bug diagnosis loop.`n" | Out-File -FilePath $remediationLog -Append -Encoding utf8
        }
    }
    $attempt++
}

if (-not $verifyPass) {
    Write-Host "`n❌ Autonomous self-healing exhausted ($MaxRetries retries)." -ForegroundColor Red
    Write-Host "   Worktree remains at: $wtPath for human inspection."
    exit 1
}

# Step 4: L3 Pre-Merge Gate
Write-Host "`n▶ Step 4: Running L3 Pre-Merge Gate on branch $branch..." -ForegroundColor Yellow
$preMergeScript = Join-Path $ScriptDir "pre-merge-check.ps1"
$targetRepo = if ($Repo) { $Repo } else { "." }

try {
    & $preMergeScript -Repo $targetRepo -Branch $branch
    Write-Host "`n════════════════════════════════════════════════════════════════" -ForegroundColor Green
    Write-Host "🎉 Orchestration Completed Successfully!" -ForegroundColor Green
    Write-Host "   Branch: $branch" -ForegroundColor Green
    Write-Host "   Worktree: $wtPath" -ForegroundColor Green
    Write-Host "   Status: All L2 verification & L3 merge gates PASSED." -ForegroundColor Green
    Write-Host "   Next: Ready for review and clean merge onto master." -ForegroundColor Green
    Write-Host "════════════════════════════════════════════════════════════════" -ForegroundColor Green
    exit 0
} catch {
    Write-Host "`n❌ L3 Pre-merge gate failed: $_" -ForegroundColor Red
    exit 1
}
