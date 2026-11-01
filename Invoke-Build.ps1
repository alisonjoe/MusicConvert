# ==============================================================================
# Invoke-Build.ps1 — Universal Agentic Engineering Command Gateway
# ==============================================================================
# Usage (Run from WORKSPACE root):
#   pwsh .\Invoke-Build.ps1 dispatch -Repo <repo> -Title "<task-title>"
#   pwsh .\Invoke-Build.ps1 verify
#   pwsh .\Invoke-Build.ps1 check -Repo <repo> -Branch agent/<slug>
#   pwsh .\Invoke-Build.ps1 cleanup
#   pwsh .\Invoke-Build.ps1 cleanup -Worktree <branch-or-name> [-Force]
#   pwsh .\Invoke-Build.ps1 cleanup -All [-Force]
#   pwsh .\Invoke-Build.ps1 commit -Files "src/file1.go","src/file2.go" -Message "feat: description"
#   pwsh .\Invoke-Build.ps1 codegen [-Check]
# ==============================================================================

[CmdletBinding(DefaultParameterSetName='Default')]
param(
    [Parameter(Position=0, Mandatory=$true)]
    [ValidateSet('dispatch','verify','check','cleanup','codemap','codegen','init','commit')]
    [string]$Target,

    [string]$Repo     = "",
    [string]$Branch   = "",
    [string]$Ticket   = "",
    [string]$Title    = "",
    [Alias("wt")]
    [string]$Worktree = "",
    [string[]]$Files  = @(),
    [Alias("m")]
    [string]$Message  = "",
    [switch]$All,
    [switch]$Force,

    [Parameter(ValueFromRemainingArguments=$true)]
    [string[]]$RemainingArgs
)

$callerLocation = (Get-Location).Path
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
if ($Target -notin @('commit', 'verify')) {
    Set-Location $root
}

switch ($Target) {
    'dispatch' {
        $dispatchArgs = @{}
        if ($Repo)   { $dispatchArgs['Repo']   = $Repo }
        if ($Title)  { $dispatchArgs['Title']  = $Title }
        if ($Ticket) { $dispatchArgs['Ticket'] = $Ticket }
        & ".\scripts\agent-dispatch.ps1" @dispatchArgs
    }
    'verify' {
        if ($callerLocation -ne $root -and -not $Repo) {
            & "$root\scripts\verify-worktree.ps1"
        } else {
            & ".\scripts\verify-worktree.ps1"
        }
    }
    'commit' {
        # 确定工作区目录
        $targetDir = $callerLocation
        if ($Repo -and ($Worktree -or $Branch)) {
            $wtName = if ($Worktree) { $Worktree } else { $Branch }
            $targetDir = Join-Path $root "$Repo\.worktrees\$wtName"
        } elseif ($Repo) {
            $targetDir = Join-Path $root $Repo
        }

        if (-not (Test-Path $targetDir)) {
            Write-Host "❌ Target directory does not exist: $targetDir" -ForegroundColor Red
            exit 1
        }

        Set-Location $targetDir

        # 检查是否在 Git 仓库或工作树中
        $gitRoot = git rev-parse --show-toplevel 2>$null
        if ($LASTEXITCODE -ne 0 -or -not $gitRoot) {
            Write-Host "❌ Current directory is not a Git repository or worktree: $targetDir" -ForegroundColor Red
            exit 1
        }

        # 整理待提交文件列表
        $fileList = @()
        foreach ($f in $Files) {
            if ($f -match ',') {
                $fileList += ($f -split ',') | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" }
            } elseif ($f.Trim() -ne "") {
                $fileList += $f.Trim()
            }
        }

        if ($fileList.Count -eq 0) {
            Write-Host "❌ Commit failed: -Files <list> must be explicitly specified!" -ForegroundColor Red
            Write-Host "   Under agentic safety rules, blanket staging (git add .) is strictly forbidden." -ForegroundColor Yellow
            Write-Host "   Usage: pwsh .\Invoke-Build.ps1 commit -Files `"src/file1.go`",`"src/file2.go`" -Message `"feat: summary`"" -ForegroundColor DarkGray
            exit 1
        }

        if ([string]::IsNullOrWhiteSpace($Message)) {
            Write-Host "❌ Commit failed: -Message (or -m) is required!" -ForegroundColor Red
            exit 1
        }

        function Normalize-GitPath([string]$p) {
            return ($p -replace '\\', '/').TrimStart('./').Trim()
        }

        # 扫描工作区所有脏文件
        $statusOutput = git status --porcelain
        $allDirtyFiles = @()
        if ($statusOutput) {
            foreach ($line in ($statusOutput -split "`r?`n")) {
                if ($line.Length -gt 3) {
                    $rawPath = $line.Substring(3).Trim()
                    if ($rawPath -match '->\s*(.+)$') {
                        $rawPath = $matches[1].Trim()
                    }
                    $rawPath = $rawPath.Trim('"')
                    if ($rawPath) {
                        $allDirtyFiles += $rawPath
                    }
                }
            }
        }

        $normalizedTargetFiles = $fileList | ForEach-Object { Normalize-GitPath $_ }
        $foreignFiles = @()
        foreach ($df in $allDirtyFiles) {
            $normDf = Normalize-GitPath $df
            if ($normDf -notin $normalizedTargetFiles) {
                $foreignFiles += $normDf
            }
        }

        # 若存在非本次清单内的脏文件，触发醒目黄色告警面板并跳过
        if ($foreignFiles.Count -gt 0) {
            Write-Host ""
            Write-Host ("━" * 56) -ForegroundColor Yellow
            Write-Host "⚠️  [Isolation Guard] Detected modified files NOT in commit list:" -ForegroundColor Yellow
            Write-Host ("━" * 56) -ForegroundColor Yellow
            foreach ($ff in $foreignFiles) {
                Write-Host "   - $ff" -ForegroundColor DarkYellow
            }
            Write-Host ""
            Write-Host "   Safely skipping $($foreignFiles.Count) file(s). They will remain untouched." -ForegroundColor Green
            Write-Host "   This strictly prevents cross-session / foreign file commit contamination." -ForegroundColor Green
            Write-Host ("━" * 56) -ForegroundColor Yellow
            Write-Host ""
        }

        # 精准暂存目标文件
        Write-Host "▶ Precisely staging target files ($($fileList.Count) files)..." -ForegroundColor Cyan
        foreach ($targetFile in $fileList) {
            git add -- "$targetFile"
            if ($LASTEXITCODE -ne 0) {
                Write-Host "❌ git add failed: $targetFile" -ForegroundColor Red
                exit 1
            }
        }

        # 检查暂存区是否有内容
        $staged = git diff --cached --name-only
        if (-not $staged) {
            Write-Host "⚠️ Staging area is empty. Specified files had no changes or were already committed." -ForegroundColor Yellow
            return
        }

        # 执行提交
        Write-Host "▶ Executing commit: $Message" -ForegroundColor Cyan
        git commit -m "$Message"
        if ($LASTEXITCODE -eq 0) {
            Write-Host ""
            Write-Host "✅ Precision commit successful!" -ForegroundColor Green
            git log -1 --stat
        } else {
            Write-Host "❌ git commit failed" -ForegroundColor Red
            exit 1
        }
    }
    'check' {
        if (-not $Branch) {
            Write-Host "Usage: pwsh .\Invoke-Build.ps1 check [-Repo <repo>] -Branch <branch>" -ForegroundColor Red; exit 1
        }
        if (-not $Repo) { $Repo = "." }
        & ".\scripts\pre-merge-check.ps1" -Repo $Repo -Branch $Branch
    }
    'cleanup' {
        function Get-WorktreeEntries([string]$RepoPath) {
            $raw = git -C $RepoPath worktree list --porcelain 2>$null
            $items = @()
            $cur = @{}
            foreach ($line in ($raw -split "`r?`n")) {
                if ($line -match '^worktree\s+(.+)$') {
                    $cur = @{ Path = $matches[1].Trim(); Repo = $RepoPath }
                } elseif ($line -match '^branch\s+refs/heads/(.+)$') {
                    $cur['Branch'] = $matches[1].Trim()
                } elseif ($line -eq '' -and $cur.Path) {
                    $items += [PSCustomObject]$cur
                    $cur = @{}
                }
            }
            if ($cur.Path) { $items += [PSCustomObject]$cur }
            return $items
        }

        # Auto-detect repos
        $reposToCheck = @()
        if (Test-Path ".git") { $reposToCheck += "." }
        Get-ChildItem -Directory | Where-Object { Test-Path (Join-Path $_.FullName ".git") } | ForEach-Object { $reposToCheck += $_.Name }

        $allWorktrees = @()
        foreach ($r in $reposToCheck) {
            $allWorktrees += Get-WorktreeEntries $r
        }

        $rootNorm = ($root -replace '\\','/').TrimEnd('/')
        $agentWorktrees = $allWorktrees | Where-Object {
            $pNorm = ($_.Path -replace '\\','/').TrimEnd('/')
            $pNorm -ne $rootNorm -and $pNorm -ne "$rootNorm/$($_.Repo)" -and $_.Branch -notin 'master', 'main'
        }

        if (-not $Worktree -and -not $Branch -and -not $All) {
            Write-Host "══════════════════════════════════════════════════" -ForegroundColor Cyan
            Write-Host "  Active Agent Worktrees" -ForegroundColor Cyan
            Write-Host "══════════════════════════════════════════════════" -ForegroundColor Cyan
            if ($agentWorktrees.Count -eq 0) {
                Write-Host "✅ No active agent worktrees found." -ForegroundColor Green
            } else {
                $agentWorktrees | Format-Table -Property Repo, Branch, Path -AutoSize
                Write-Host "Cleanup Command Hints:" -ForegroundColor Yellow
                Write-Host "  pwsh .\Invoke-Build.ps1 cleanup -Worktree <branch-or-name> [-Force]" -ForegroundColor DarkGray
                Write-Host "  pwsh .\Invoke-Build.ps1 cleanup -All [-Force]" -ForegroundColor DarkGray
            }
            return
        }

        $targetsToDelete = @()
        if ($All) {
            $targetsToDelete = $agentWorktrees
        } else {
            $pattern = if ($Worktree) { $Worktree } else { $Branch }
            $targetsToDelete = $agentWorktrees | Where-Object {
                $_.Path -like "*$pattern*" -or $_.Branch -like "*$pattern*"
            }
        }

        if ($targetsToDelete.Count -eq 0) {
            Write-Host "⚠️ No matching worktree found for: $($Worktree)$($Branch)" -ForegroundColor Yellow
            return
        }

        Write-Host "Target worktrees to be removed:" -ForegroundColor Yellow
        $targetsToDelete | Format-Table -Property Repo, Branch, Path -AutoSize

        if (-not $Force) {
            $confirm = Read-Host "Confirm deletion of above $($targetsToDelete.Count) worktree(s) and branch(es)? (y/N)"
            if ($confirm -notmatch '^(y|yes)$') {
                Write-Host "Cleanup cancelled." -ForegroundColor DarkGray
                return
            }
        }

        foreach ($item in $targetsToDelete) {
            Write-Host "▶ Removing Worktree: [$($item.Repo)] $($item.Path)..." -NoNewline
            git -C $item.Repo worktree remove "$($item.Path)" --force 2>$null
            if ($LASTEXITCODE -eq 0) {
                Write-Host " [Removed]" -ForegroundColor Green
            } else {
                Write-Host " [Worktree removed]" -ForegroundColor Yellow
            }

            if ($item.Branch -and $item.Branch -notin 'master', 'main') {
                Write-Host "▶ Deleting Branch: [$($item.Repo)] $($item.Branch)..." -NoNewline
                git -C $item.Repo branch -D "$($item.Branch)" 2>$null | Out-Null
                Write-Host " [Branch deleted]" -ForegroundColor Green
            }
        }
        Write-Host "✅ Worktree cleanup complete!" -ForegroundColor Green
    }
    'codemap' {
        Write-Host "Open a new session in Antigravity and invoke the 'explorer' agent to generate or refresh CODEMAP.md." -ForegroundColor Cyan
    }
    'init' {
        & ".\scripts\setup-workspace.ps1"
    }
}
