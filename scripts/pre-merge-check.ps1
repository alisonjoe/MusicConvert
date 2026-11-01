<#
.SYNOPSIS
    Universal Pre-Merge Gate — Local CI substitute before merging into master/main.
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)]
    [string]$Repo,

    [Parameter(Mandatory=$true)]
    [string]$Branch
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$WORKSPACE = (Resolve-Path (Join-Path $ScriptDir "..")).Path
$RepoPath  = if ($Repo -eq ".") { $WORKSPACE } else { Join-Path $WORKSPACE $Repo }

if (-not (Test-Path $RepoPath)) {
    Write-Error "Repository path does not exist: $RepoPath"
    exit 1
}

Set-Location $RepoPath
$PASS = 0; $FAIL = 0

function Run-Check([string]$Name, [scriptblock]$Body) {
    Write-Host ""
    Write-Host ("─" * 45) -ForegroundColor DarkGray
    Write-Host "▶ $Name"
    try {
        & $Body
        Write-Host "✅ $Name passed" -ForegroundColor Green
        $script:PASS++
    } catch {
        Write-Host "❌ $Name failed: $_" -ForegroundColor Red
        $script:FAIL++
    }
}

Write-Host ("═" * 45) -ForegroundColor Cyan
Write-Host "  Pre-Merge Gate: $Repo :: $Branch" -ForegroundColor Cyan
Write-Host ("═" * 45) -ForegroundColor Cyan

# 1. Verify executed on master/main
$current = git branch --show-current
if ($current -ne "master" -and $current -ne "main") {
    Write-Error "Must be executed on master or main branch (current: $current)"
    exit 1
}

if (-not (git rev-parse --verify $Branch 2>$null)) {
    Write-Error "Branch does not exist: $Branch"
    exit 1
}

$base = $current

# 2. Forbidden Zone Scan
Run-Check "1. Forbidden Zone Scan" {
    $changed = git diff --name-only "$base...$Branch" 2>$null
    if (-not $changed) { $changed = git diff --name-only $base $Branch }
    
    # Read agents.config.yaml if present
    $configPath = Join-Path $WORKSPACE "agents.config.yaml"
    $hits = 0

    # Inspect commit messages for waiver tags
    $commitLogs = git log "$base..$Branch" --oneline 2>$null

    # Default common forbidden patterns if not overridden
    $patterns = @(
        @{ Pattern = "core/models/"; Reason = "Core DB Models"; Tag = "[schema]" },
        @{ Pattern = "\.proto$"; Reason = "Protobuf Contracts"; Tag = "[proto]" }
    )

    foreach ($p in $patterns) {
        if ($changed -match $p.Pattern) {
            if ($commitLogs -notmatch [regex]::Escape($p.Tag)) {
                Write-Host "  ❌ Forbidden zone touched without waiver: $($p.Pattern) ($($p.Reason)) — Requires $($p.Tag) in commit log" -ForegroundColor Red
                $hits++
            } else {
                Write-Host "  ⚠️ Forbidden zone touched with waiver: $($p.Pattern) (Waiver tag $($p.Tag) confirmed)" -ForegroundColor Yellow
            }
        }
    }

    if ($hits -gt 0) {
        throw "Touched $hits forbidden zone(s) without approved waiver tag in commit log."
    }
    Write-Host "  Forbidden zone scan passed."
}

# 3. Build & Test Verification
Run-Check "2. Build & Test Verification" {
    if (Test-Path "go.mod") {
        go build ./...; if ($LASTEXITCODE -ne 0) { throw "go build failed" }
        go vet ./...;   if ($LASTEXITCODE -ne 0) { throw "go vet failed" }
        go test ./... -timeout 60s; if ($LASTEXITCODE -ne 0) { throw "go test failed" }
    } elseif (Test-Path "package.json") {
        npm test; if ($LASTEXITCODE -ne 0) { throw "npm test failed" }
    } elseif (Test-Path "Cargo.toml") {
        cargo test; if ($LASTEXITCODE -ne 0) { throw "cargo test failed" }
    } else {
        Write-Host "  No standard test runner detected, skipping unit tests."
    }
}

Write-Host ""
Write-Host ("═" * 45) -ForegroundColor Cyan
Write-Host ("  Result: ✅ {0} passed / ❌ {1} failed" -f $PASS, $FAIL) -ForegroundColor Cyan
Write-Host ("═" * 45) -ForegroundColor Cyan

if ($FAIL -gt 0) {
    Write-Host "`nMerge blocked. Fix issues and rerun:" -ForegroundColor Red
    Write-Host "  pwsh scripts\pre-merge-check.ps1 -Repo $Repo -Branch $Branch"
    exit 1
}

Write-Host "`n✅ All checks passed! Ready to merge into $base." -ForegroundColor Green
Write-Host "`nNext steps:" -ForegroundColor White
Write-Host "  cd $Repo; git merge --squash $Branch"
Write-Host "  pwsh .\Invoke-Build.ps1 cleanup -Worktree $Branch -Force"
