# Task: rust-rewrite

- **Repo**: .
- **Branch**: agent/rust-rewrite-20261101
- **Base**: main
- **Created**: 2026-11-01 16:20:44

## Implementation Scope & Verification
- Keep changes strictly scoped to this task.
- Follow TDD (Red-Green-Refactor) for business logic.
- Verify using:
  - POSIX (macOS/Linux): `../../Invoke-Build.sh verify`
  - Windows: `pwsh ..\..\Invoke-Build.ps1 verify`
- Commit using (Precision Staging):
  - POSIX (macOS/Linux): `../../Invoke-Build.sh commit -Files "path/file" -Message "feat: description"`
  - Windows: `pwsh ..\..\Invoke-Build.ps1 commit -Files "path/file" -Message "feat: description"`
- NEVER use blanket staging (git add . / git add -A / git commit -a).
