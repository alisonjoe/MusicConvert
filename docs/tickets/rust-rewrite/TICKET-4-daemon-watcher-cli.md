# [Ticket-4] Daemon Watcher, ReleaseBatch Orchestration & CLI

- **Spec**: `docs/specs/rust-rewrite.md`
- **Status**: completed
- **Merged in**: 5ae4dd8 (main)
- **Scope**:
  - [x] 创建可执行子 crate `crates/music-convert`，采用 `clap` 构建 CLI 命令（支持 `--rescan`, `--rescan-all`, `--config`）。
  - [x] 基于 `notify` 实现带防抖 (debounce) 的本地文件监听守护循环。
  - [x] 实现基于物理目录及分碟兄弟目录合并的 `ReleaseBatch` 批调度引擎（对齐 ADR 0003）。
  - [x] 编写 Synology NAS 静态链接交叉编译脚本（`scripts/build_synology_rust.sh`，生成 `x86_64-unknown-linux-musl` 与 `aarch64` 二进制）。
- **Blocked by**: Ticket-3
- **Affected Paths**:
  - `crates/music-convert/**`
  - `scripts/build_synology_rust.sh`
- **Pre-authored Test Spec**: `docs/tests/TC-004-daemon-watcher.md`
- **Verification Plan**:
  - [x] `cargo test --workspace` 全绿。
  - [x] 执行本地文件模拟监听事件，验证批处理调度到 SQLite 入库端到端完整闭环。
