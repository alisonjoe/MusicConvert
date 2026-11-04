# 4. Migrate Legacy Go to Archive and Consolidate Pure Rust Workspace

Date: 2026-11-04

## Status

Accepted

## Context

MusicConvert 已完成纯 Rust 异步工作空间（Cargo Workspace）的全量功能实现与树状领域重构（`crates/music-core`, `crates/music-provider`, `crates/music-audio`, `crates/music-convert`）。
此前根目录下同时平铺着历史 Go 源码（`main.go`, `api/`, `audio/`, `config/`, `db/`, `model/`, `util/`）与 Rust 工作空间。
双轨并存造成：
1. 根目录物理结构混杂；
2. 门禁每次需触发外部 MusicBrainz 网络接口运行 Go 测试，严重拖慢自测与 CI 效率；
3. 构建产物与发布脚本割裂（`build_synology.sh` 编译 Go，`build_synology_rust.sh` 编译 Rust）。

经过与人类用户的深度质询（`/grill-me`），达成“渐进式替代”共识：
- 根目录收敛为纯粹的 Rust Cargo Workspace；
- 将历史 Go 源码完整物理归拢至 `legacy/go/`，保留 git commit 历史供比对参考，但不参与日常门禁；
- 统一由 Rust 接管构建发布，`scripts/build_synology.sh` 默认构建 Rust 生产二进制。

## Decision

1. 使用 `git mv` 将 `main.go`, `go.mod`, `go.sum`, `api/`, `audio/`, `config/`, `db/`, `model/`, `util/` 迁入 `legacy/go/`；
2. 更新 `agents.config.yaml`，将技术栈收敛为 `rust`，命令聚焦 `cargo check` 与 `cargo test`；
3. 更新 `scripts/verify-worktree.sh` 与 `scripts/pre-merge-check.sh`，优先并默认执行 Rust 门禁，实现秒级增量自检；
4. 将 `scripts/build_synology.sh` 统一为编译 Rust 跨平台生产产物，旧 Go 构建脚本归档至 `legacy/go/scripts/build_synology_go.sh`；
5. 更新 `README.md`、`docs/CODEMAP.md` 与根目录 `AGENTS.md`。

## Consequences

- 根目录极其清爽，只包含纯粹的标准 Rust 工程布局与全局配置；
- 本地与 CI 门禁耗时从 30s+ 骤降至 <1s（秒级缓存命中）或 3s（全量自测）；
- Go 历史实现依然完整保留在 `legacy/go/`，任何需要对照算法或网络逆向逻辑的场景均可随时查阅；
- 群晖与 Linux 的生产部署完全由 Rust 接管，享受低内存占用与高并发收益。
