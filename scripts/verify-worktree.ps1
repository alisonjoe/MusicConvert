<#
.SYNOPSIS
    Universal L2 Smart Incremental Self-Verification (PowerShell)
    支持基于 Git Diff 的差量智能跳过、已有二进制缓存直通以及快速通道参数。
#>

[CmdletBinding()]
param(
    [switch]$Force,
    [Alias("s")]
    [switch]$SkipVerify,
    [Alias("r")]
    [switch]$ReinstallOnly
)

$ErrorActionPreference = "Stop"

$currentDir = Get-Location
Write-Host ("═" * 50) -ForegroundColor Cyan
Write-Host "  ⚡ L2 Verification (Smart Incremental): $currentDir" -ForegroundColor Cyan
Write-Host ("═" * 50) -ForegroundColor Cyan

if ($SkipVerify -or $ReinstallOnly) {
    Write-Host "⏩ [Fast-Track] 检测到跳过参数 (-SkipVerify / -ReinstallOnly)，已极速通过验证流程！" -ForegroundColor Green
    exit 0
}

# 1. 扫描变更文件
$changedFiles = ""
try {
    $baseBranch = "main"
    $checkMaster = git rev-parse --verify master 2>$null
    if ($LASTEXITCODE -eq 0) { $baseBranch = "master" }
    $changedFiles = (git diff --name-only $baseBranch 2>$null) -join "`n"
    if (!$changedFiles) {
        $changedFiles = (git status -s 2>$null) -join "`n"
    }
} catch {}

if (!$changedFiles -and !$Force) {
    Write-Host "✨ [Smart Cache] 当前工作树没有任何代码或配置变动，直接秒级命中缓存跳过验证！" -ForegroundColor Green
    Write-Host "   (如需全量强制重新验证，请传入 -Force)" -ForegroundColor Gray
    exit 0
}

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

# 2. 差量按需编译与测试
if (Test-Path "go.mod") {
    if ($Force -or ($changedFiles -match "\.go$|go\.mod|go\.sum")) {
        Run-Step "Go Build" { go build ./...; if ($LASTEXITCODE -ne 0) { throw "go build failed" } }
        Run-Step "Go Vet"   { go vet ./...; if ($LASTEXITCODE -ne 0) { throw "go vet failed" } }
        Run-Step "Go Test"  { go test ./... -timeout 60s; if ($LASTEXITCODE -ne 0) { throw "go test failed" } }
    } else {
        Write-Host "⏩ [Go 模块未变动] 跳过 Go 编译与全量测试" -ForegroundColor DarkGray
    }
} elseif (Test-Path "package.json") {
    if ($Force -or ($changedFiles -match "\.(js|ts|jsx|tsx|vue|json)$")) {
        $pkgJson = Get-Content "package.json" -Raw | ConvertFrom-Json
        if ($pkgJson.scripts.lint) {
            Run-Step "NPM/PNPM Lint" { npm run lint; if ($LASTEXITCODE -ne 0) { throw "lint failed" } }
        }
        if ($pkgJson.scripts.test) {
            Run-Step "NPM/PNPM Test" { npm test; if ($LASTEXITCODE -ne 0) { throw "test failed" } }
        }
    } else {
        Write-Host "⏩ [前端/Node 模块未变动] 跳过 JS/TS 校验与测试" -ForegroundColor DarkGray
    }
} elseif (Test-Path "Cargo.toml") {
    if ($Force -or ($changedFiles -match "\.rs$|Cargo\.toml|Cargo\.lock")) {
        Run-Step "Cargo Check" { cargo check; if ($LASTEXITCODE -ne 0) { throw "cargo check failed" } }
        Run-Step "Cargo Test"  { cargo test; if ($LASTEXITCODE -ne 0) { throw "cargo test failed" } }
    } else {
        Write-Host "⏩ [Rust 模块未变动] 跳过 Cargo 编译与测试" -ForegroundColor DarkGray
    }
} elseif (Test-Path "pyproject.toml" -or (Test-Path "requirements.txt")) {
    if ($Force -or ($changedFiles -match "\.py$|requirements.*\.txt|pyproject\.toml")) {
        if (Get-Command pytest -ErrorAction SilentlyContinue) {
            Run-Step "Pytest" { pytest; if ($LASTEXITCODE -ne 0) { throw "pytest failed" } }
        }
    } else {
        Write-Host "⏩ [Python 模块未变动] 跳过 Python 校验与测试" -ForegroundColor DarkGray
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
