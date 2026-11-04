# TC-005: 历史 Go 源码归档与纯 Rust 工作空间收敛验证

## 1. 目标
验证 Go 历史代码物理归拢至 `legacy/go/` 后，整个仓库的物理拓扑、Rust Cargo Workspace、构建发布脚本以及 L2/L3 门禁均能正常、快速且无残留地执行。

## 2. 验收标准 (Acceptance Criteria)

- [ ] **物理拓扑干净**: 根目录下不再存在 `main.go`, `go.mod`, `go.sum` 及 `api/`, `audio/`, `config/`, `db/`, `model/`, `util/` 散落目录，所有 Go 代码完好保留在 `legacy/go/` 中。
- [ ] **Rust Workspace 独立性**: `cargo check --workspace` 与 `cargo test --workspace` 在根目录下无缝运行，不依赖任何 Go 运行环境。
- [ ] **快速自检通过**: `./Invoke-Build.sh verify` 在秒级内通过，门禁完全聚焦 Rust。
- [ ] **统一发布脚本**: `./scripts/build_synology.sh` 能够独立调用 Rust 编译器生成生产级二进制 `MusicConvert_synology_amd64`、`arm64`、`armv7` 以及 `target/release/MusicConvert`。
- [ ] **L3 预合并门禁**: `./Invoke-Build.sh check --branch agent/migrate-legacy-go-...` 全绿通过。
