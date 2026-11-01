# [Ticket-1] Prefactor: Cargo Workspace, Core Domain & SQLite Persistence

- **Spec**: `docs/specs/rust-rewrite.md`
- **Status**: completed
- **Merged in**: 5ae4dd8 (main)
- **Scope**:
  - [x] 初始化根目录 Cargo Workspace 配置及 `crates/music-core` 子 crate。
  - [x] 实现与现有 Go `config.yaml` 完全对齐的强类型配置解析模块（支持所有核心字段与默认值）。
  - [x] 实现与现有 SQLite 数据库完全兼容的持久化层 (`rusqlite`)，保证 `processed_files` 表结构与防重逻辑 100% 互通。
  - [x] 实现工具层：简繁体转换 (`chinese.rs`)、Unicode 标点归一化及 Windows/SMB 目录路径清洗（禁止以空格或点号结尾，过滤 8.3 别名）。
  - [x] 更新 `agents.config.yaml` 加入 `cargo check` 和 `cargo test` 门禁。
- **Blocked by**: None
- **Affected Paths**:
  - `Cargo.toml`
  - `crates/music-core/**`
  - `agents.config.yaml`
- **Pre-authored Test Spec**: `docs/tests/TC-001-core-db.md`
- **Verification Plan**:
  - [x] `cargo test -p music-core` 全绿通过。
  - [x] 测试覆盖：配置反序列化、SQLite 读写幂等性、简繁转换与路径清洗断言。
