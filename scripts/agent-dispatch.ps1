<#
.SYNOPSIS
    Universal Agent Dispatcher — Opens isolated git worktrees and writes task tickets.
.DESCRIPTION
    Creates a new worktree under .worktrees/<task-slug> for a given repo,
    isolated from master/main, and initializes the task environment.
#>

[CmdletBinding()]
param(
    [Parameter(Position=0, Mandatory=$false)]
    [string]$Repo = "",

    [Parameter(Position=1, Mandatory=$false)]
    [string]$Title = "",

    [Parameter(Mandatory=$false)]
    [string]$Ticket = ""
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$WORKSPACE = (Resolve-Path (Join-Path $ScriptDir "..")).Path

# 1. Resolve repo
if (-not $Repo) {
    # Check if mono-repo or single repo
    if (Test-Path (Join-Path $WORKSPACE ".git")) {
        $Repo = "."
    } else {
        # Check subdirectories with .git
        $gitRepos = Get-ChildItem $WORKSPACE -Directory | Where-Object { Test-Path (Join-Path $_.FullName ".git") }
        if ($gitRepos.Count -eq 1) {
            $Repo = $gitRepos[0].Name
        } else {
            Write-Host "❌ Multiple repos detected. Please specify -Repo <name>:" -ForegroundColor Red
            $gitRepos | ForEach-Object { Write-Host "   - $($_.Name)" -ForegroundColor Yellow }
            exit 1
        }
    }
}

$RepoPath = if ($Repo -eq ".") { $WORKSPACE } else { Join-Path $WORKSPACE $Repo }
if (-not (Test-Path $RepoPath)) {
    Write-Error "Repository path does not exist: $RepoPath"
    exit 1
}

# 2. Resolve title / task slug
if (-not $Title -and $Ticket -and (Test-Path $Ticket)) {
    $firstLine = Get-Content $Ticket -TotalCount 10 | Where-Object { $_ -match '^#\s+(.+)$' } | Select-Object -First 1
    if ($firstLine -match '^#\s+(.+)$') {
        $Title = $matches[1].Trim()
    }
}

if (-not $Title) {
    Write-Error "Please specify task title via -Title <string>"
    exit 1
}

# Convert title to clean slug
$slug = $Title.ToLower() -replace '[^\w\d]+', '-' -replace '^-|-$', ''
$dateStr = Get-Date -Format "yyyyMMdd"
$baseBranchName = "agent/$slug-$dateStr"
$branchName = $baseBranchName
$worktreeDir = Join-Path $RepoPath ".worktrees\$slug"

# Detect collision and auto-suffix (-s2, -s3...) to prevent concurrent session contamination
$seq = 2
$candidateSlug = $slug
while ((Test-Path $worktreeDir) -or (git -C $RepoPath rev-parse --verify --quiet "refs/heads/$branchName" 2>$null)) {
    $candidateSlug = "$slug-s$seq"
    $branchName = "agent/$candidateSlug-$dateStr"
    $worktreeDir = Join-Path $RepoPath ".worktrees\$candidateSlug"
    $seq++
}

if ($candidateSlug -ne $slug) {
    Write-Host ""
    Write-Host "⚡ [Collision Guard] Active worktree or branch for [$slug] already exists!" -ForegroundColor Yellow
    Write-Host "   Allocating isolated session worktree: $candidateSlug ($branchName)" -ForegroundColor Yellow
    Write-Host ""
}

Write-Host ("═" * 50) -ForegroundColor Cyan
Write-Host "  Agent Worktree Dispatch" -ForegroundColor Cyan
Write-Host "  Repo:   $Repo" -ForegroundColor White
Write-Host "  Branch: $branchName" -ForegroundColor White
Write-Host "  Path:   $worktreeDir" -ForegroundColor White
Write-Host ("═" * 50) -ForegroundColor Cyan

# 3. Detect base branch
Set-Location $RepoPath
$baseBranch = git branch --show-current
if (-not $baseBranch) {
    git show-ref --verify --quiet "refs/heads/master"
    $baseBranch = if ($LASTEXITCODE -eq 0) { "master" } else { "main" }
}

# 4. Create worktree
Write-Host "▶ Creating git worktree from $baseBranch..." -ForegroundColor Cyan
git worktree add -b $branchName $worktreeDir $baseBranch
if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to create worktree."
    exit 1
}

# 5. Write TASK.md
$taskFile = Join-Path $worktreeDir "TASK.md"
$taskContent = @"
# Task: $Title

- **Repo**: $Repo
- **Branch**: $branchName
- **Base**: $baseBranch
- **Created**: $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")

## Implementation Scope & Verification
- Keep changes strictly scoped to this task.
- Follow TDD (Red-Green-Refactor) for business logic.
- Verify using:
  - Windows: ``pwsh ..\..\Invoke-Build.ps1 verify``
  - POSIX (macOS/Linux): ``../../Invoke-Build.sh verify``
- Commit using (Precision Staging):
  - Windows: ``pwsh ..\..\Invoke-Build.ps1 commit -Files "path/file" -Message "feat: description"``
  - POSIX (macOS/Linux): ``../../Invoke-Build.sh commit -Files "path/file" -Message "feat: description"``
- NEVER use blanket staging (git add . / git add -A / git commit -a).
"@

Set-Content -Path $taskFile -Value $taskContent -Encoding utf8
Write-Host "✅ Worktree ready: $worktreeDir" -ForegroundColor Green
Write-Host "   Task ticket initialized at: $taskFile" -ForegroundColor DarkGray
