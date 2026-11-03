#!/usr/bin/env python3
"""
workspace_registry.py — Universal Agentic Workspace Registry Engine (Zero-dependency)
Manages ~/.agents/workspaces.json:
  - Auto-discovers and silently registers Git workspace directories
  - Heuristically detects tech stack and build/test commands
  - Injects .worktrees/ into .git/info/exclude to guarantee ZERO repo pollution
"""

import os
import sys
import json
import subprocess
from pathlib import Path

REGISTRY_PATH = Path.home() / ".agents" / "workspaces.json"

def run_cmd(cmd, cwd=None):
    try:
        res = subprocess.run(cmd, cwd=cwd, shell=True, capture_output=True, text=True)
        return res.returncode == 0, res.stdout.strip()
    except Exception:
        return False, ""

def get_git_root(path):
    ok, out = run_cmd("git rev-parse --show-toplevel", cwd=path)
    if ok and out:
        return Path(out).resolve()
    return None

def detect_tech_stack(root_path):
    """Heuristically inspects project markers to determine tech and verification commands."""
    p = Path(root_path)
    if (p / "go.mod").exists():
        return {
            "tech": "go",
            "build": "go build ./...",
            "test": "go test ./... -timeout 60s"
        }
    if (p / "package.json").exists():
        runner = "npm"
        if (p / "pnpm-lock.yaml").exists():
            runner = "pnpm"
        elif (p / "yarn.lock").exists():
            runner = "yarn"
        return {
            "tech": "node",
            "build": f"{runner} run build" if (p / "src").exists() else "",
            "test": f"{runner} test"
        }
    if (p / "Cargo.toml").exists():
        return {
            "tech": "rust",
            "build": "cargo check",
            "test": "cargo test"
        }
    if (p / "pyproject.toml").exists() or (p / "requirements.txt").exists():
        return {
            "tech": "python",
            "build": "",
            "test": "pytest" if (p / "tests").exists() else "python -m unittest"
        }
    return {
        "tech": "generic",
        "build": "",
        "test": ""
    }

def ensure_exclude_worktrees(git_root):
    """Ensures .worktrees/ is ignored locally in .git/info/exclude without modifying .gitignore."""
    ok, common_git_dir = run_cmd("git rev-parse --git-common-dir", cwd=git_root)
    if not ok or not common_git_dir:
        common_git_dir = ".git"
    
    info_dir = (Path(git_root) / common_git_dir).resolve() / "info"
    info_dir.mkdir(parents=True, exist_ok=True)
    exclude_file = info_dir / "exclude"
    if exclude_file.exists():
        content = exclude_file.read_text(encoding="utf-8")
        if ".worktrees" not in content:
            with open(exclude_file, "a", encoding="utf-8") as f:
                f.write("\n# Agentic Workspace Isolation (Zero-pollution)\n.worktrees/\n")
    else:
        exclude_file.write_text("# Agentic Workspace Isolation (Zero-pollution)\n.worktrees/\n", encoding="utf-8")

def load_registry():
    if not REGISTRY_PATH.exists():
        return {"workspaces": []}
    try:
        with open(REGISTRY_PATH, "r", encoding="utf-8") as f:
            data = json.load(f) or {}
            if "workspaces" not in data:
                data["workspaces"] = []
            return data
    except Exception:
        return {"workspaces": []}

def save_registry(data):
    REGISTRY_PATH.parent.mkdir(parents=True, exist_ok=True)
    with open(REGISTRY_PATH, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)

def resolve_or_register(target_path=None):
    cwd = Path(target_path or os.getcwd()).resolve()
    git_root = get_git_root(cwd)
    if not git_root:
        print(f"❌ Error: {cwd} is not inside a Git repository.")
        sys.exit(1)

    # 1. Guarantee local git exclude isolation (Zero-pollution)
    ensure_exclude_worktrees(git_root)

    # 2. Check registry
    data = load_registry()
    git_root_str = str(git_root)
    matched = None
    for ws in data["workspaces"]:
        if ws.get("path") == git_root_str:
            matched = ws
            break

    if not matched:
        # Silent registration
        inferred = detect_tech_stack(git_root)
        matched = {
            "name": git_root.name,
            "path": git_root_str,
            "tech": inferred["tech"],
            "verification": {
                "build": inferred["build"],
                "test": inferred["test"]
            },
            "registered_at": run_cmd("date '+%Y-%m-%d %H:%M:%S'")[1]
        }
        data["workspaces"].append(matched)
        save_registry(data)
        print(f"⚡ [Registry Engine] Auto-registered workspace: {git_root.name} ({inferred['tech']})")
        print(f"   Config saved silently to: {REGISTRY_PATH}")
    else:
        print(f"⚡ [Registry Engine] Found registered workspace: {matched['name']} ({matched.get('tech', 'generic')})")

    return matched

if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else None
    resolve_or_register(target)
