# Session Retrospective: MusicConvert Rust Rewrite

- **Date**: 2026-11-01
- **Task**: 用 Rust 重构 MusicConvert (`/workflow` & `/goal`)
- **Status**: Completed (L2 + L3 Verified)

---

## 1. 摩擦点与痛点复盘 (Friction Points Encountered)

### 痛点 1: Worktree 物理隔离与用户视野脱节
- **现象**: 用户在主会话触发 `/goal` 完成后，在根目录未看到任何 Rust 文件，产生“为什么我一个 Rust 代码都没看到”的疑惑。
- **根本原因**:
  1. `.agents/AGENTS.md` 铁律禁止 Agent 直接向 `main` 提交或擅自合并主干（合并权归人类），因此所有代码均在独立的 Git Worktree（`.worktrees/rust-rewrite`）与独立分支（`agent/rust-rewrite-20261101`）中创建。
  2. `.worktrees` 目录以 `.` 开头，属于操作系统和 IDE 默认隐藏/折叠的目录，在常规资源管理器中不可见。
  3. 任务交付报告（Goal Complete）中未在显著位置声明“代码所在的工作树物理路径”以及“合并到 main 的引导”。
- **机器/流程级防固（Machine-Enforced Fix）**:
  - [x] 在交付总结的顶层强制加入**代码物理路径指示卡**与**一键合并选项**，消除长时任务完成后的认知隔阂。

### 痛点 2: 非交互 Shell 下跨技术栈工具链 PATH 缺失
- **现象**: `verify-worktree.sh` 和 `pre-merge-check.sh` 运行初始抛出 `go: command not found`。
- **根本原因**: Agent 运行在无环境状态的非交互子 shell，Homebrew 与 Go 路径（`/opt/homebrew/bin`, `~/go/...`）未被自动继承。
- **机器级防固（Machine-Enforced Fix）**:
  - [x] 在 `scripts/verify-worktree.sh` 和 `scripts/pre-merge-check.sh` 顶部增加自愈式工具链探测：
    ```bash
    GO_TOOLCHAIN_BIN="$(find ~/go/pkg/mod/golang.org/toolchain* -name bin 2>/dev/null | sort -V | tail -1 || true)"
    export PATH="/opt/homebrew/bin:$HOME/.cargo/bin:$HOME/go/bin:${GO_TOOLCHAIN_BIN:-}:$PATH"
    ```
  - [x] 修改门禁脚本支持多栈共存检测（如果同时存在 `go.mod` 与 `Cargo.toml`，连续执行两套自测而非 `elif` 互斥）。

### 痛点 3: `.gitignore` 通配符误伤子 crate 源码目录
- **现象**: 执行 `git commit` 时提示 `The following paths are ignored by one of your .gitignore files: crates/music-convert`。
- **根本原因**: `.gitignore` 中原有的二进制忽略规则为泛化的 `music-convert`，导致位于子路径的 `crates/music-convert` 被命中。
- **机器级防固（Machine-Enforced Fix）**:
  - [x] 修改 `.gitignore`，增加根目录锚定斜杠 `/music-convert`、`/MusicConvert`、`/target/` 与 `/.worktrees/`，杜绝误杀源码目录。

---

## 2. 知识资产与规约沉淀 (Knowledge & Architecture Updates)

- [x] 更新 `docs/CODEMAP.md`：增加 4 个 Rust 子 crate（`music-core`, `music-provider`, `music-audio`, `music-convert`）的架构职责与语言映射。
- [x] 更新 `AGENTS.md`：在常规验证步骤中增加 `cargo test --workspace` 与 `./scripts/build_synology_rust.sh`。
- [x] 落盘全套技术规约与测试用例：
  - `docs/specs/rust-rewrite.md`
  - `docs/tickets/rust-rewrite/` (TICKET-1 至 TICKET-4)
  - `docs/tests/` (TC-001 至 TC-004)
