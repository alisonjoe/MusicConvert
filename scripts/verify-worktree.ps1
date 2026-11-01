<#
.SYNOPSIS
    Universal L2 Self-Verification — Runs build, lint, and tests inside the active worktree.
#>

[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"

$currentDir = Get-Location
Write-Host ("═" * 50) -ForegroundColor Cyan
Write-Host "  L2 Self-Verification: $currentDir" -ForegroundColor Cyan
Write-Host ("═" * 50) -ForegroundColor Cyan

$passed = 0
$failed = 0

function Run-Step([string]$Name, [scriptblock]$Action) {
    Write-Host "`n▶ $Name" -ForegroundColor White
    try {
        & $Action
        Write-Host "✅ $Name passed" -ForegroundColor Green
        $script:passed++
    } catch {
        Write-Host "❌ $Name failed: $_" -ForegroundColor Red
        $script:failed++
    }
}

# Auto-detection of project technology stack if config not explicitly provided
if (Test-Path "go.mod") {
    Run-Step "Go Build" { go build ./...; if ($LASTEXITCODE -ne 0) { throw "go build failed" } }
    Run-Step "Go Vet"   { go vet ./...; if ($LASTEXITCODE -ne 0) { throw "go vet failed" } }
    Run-Step "Go Test"  { go test ./... -timeout 60s; if ($LASTEXITCODE -ne 0) { throw "go test failed" } }
} elseif (Test-Path "package.json") {
    $pkgJson = Get-Content "package.json" -Raw | ConvertFrom-Json
    if ($pkgJson.scripts.lint) {
        Run-Step "NPM/PNPM Lint" { npm run lint; if ($LASTEXITCODE -ne 0) { throw "lint failed" } }
    }
    if ($pkgJson.scripts.test) {
        Run-Step "NPM/PNPM Test" { npm test; if ($LASTEXITCODE -ne 0) { throw "test failed" } }
    }
} elseif (Test-Path "Cargo.toml") {
    Run-Step "Cargo Check" { cargo check; if ($LASTEXITCODE -ne 0) { throw "cargo check failed" } }
    Run-Step "Cargo Test"  { cargo test; if ($LASTEXITCODE -ne 0) { throw "cargo test failed" } }
} elseif (Test-Path "pyproject.toml" -or (Test-Path "requirements.txt")) {
    if (Get-Command pytest -ErrorAction SilentlyContinue) {
        Run-Step "Pytest" { pytest; if ($LASTEXITCODE -ne 0) { throw "pytest failed" } }
    }
} else {
    Write-Host "⚠️ No standard project marker detected (go.mod, package.json, Cargo.toml, pyproject.toml)." -ForegroundColor Yellow
}

Write-Host "`n" + ("═" * 50) -ForegroundColor Cyan
Write-Host ("  Verification Result: {0} passed / {1} failed" -f $passed, $failed) -ForegroundColor $(if ($failed -eq 0) { "Green" } else { "Red" })
Write-Host ("═" * 50) -ForegroundColor Cyan

if ($failed -gt 0) {
    exit 1
}
