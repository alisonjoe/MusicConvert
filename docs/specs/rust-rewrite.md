# Spec: Rust Rewrite of MusicConvert (Rust 重构规约)

- **Date**: 2026-11-01
- **Status**: Approved
- **Domain Context**: Alignment with `CONTEXT.md` (Metadata Provider Chain, ReleaseBatch / Directory Batch Processing)
- **Related ADRs**: 
  - `docs/adr/0001-deepen-classical-metadata-strategy.md`
  - `docs/adr/0002-metadata-provider-chain.md`
  - `docs/adr/0003-album-batch-processing.md`

---

## 1. User Stories & Goals

1. **As a NAS / Homelab user**, I want MusicConvert to run with a minimal memory footprint and zero GC pauses, so that the music conversion daemon can run continuously and stably on resource-constrained Synology NAS and Linux servers.
2. **As a music collector**, I want my multi-disc albums, CUE sheets, and classical releases to undergo strict Chinese metadata normalization and directory batching (`ReleaseBatch`), so that my output music library maintains clean directory structures without compilation fragmentation or split editions.
3. **As a maintainer**, I want a modular Cargo workspace architecture featuring compile-time type safety and an asynchronous Metadata Provider Chain, so that external API failures, timeouts, and fallbacks are resilient and easy to test with mocks.

---

## 2. Technical & Architecture Decisions

### 2.1 Workspace Architecture & Module Boundaries
采用双轨并存的 Cargo Workspace 架构，保留原 Go 代码与测试用例，通过独立模块逐层演进：

```text
Cargo.toml (Workspace Root)
├── crates/music-core/          # 领域模型、配置解析、SQLite 持久化、中文简繁/标点归一化
├── crates/music-provider/      # 统一 MetadataProvider 责任链 (MusicBrainz, NetEase, iTunes, Spotify, Discogs)
├── crates/music-audio/         # ffmpeg/metaflac 异步命令封装、CUE 解析与分轨转码引擎
└── crates/music-convert/       # CLI 命令行入口、notify 守护监听、ReleaseBatch 调度器
```

### 2.2 Core Tech Stack Decisions
- **Runtime**: `tokio` (multi-threaded async runtime).
- **HTTP Client**: `reqwest` (with `json`, `rustls-tls` for cross-platform musl/synology portability).
- **Database**: `rusqlite` (bundled / local SQLite, retaining identical schema: `processed_files`).
- **CLI & Configuration**: `clap` (derive API) + `serde` / `serde_yaml`.
- **File System Watcher**: `notify` (debounced file system events matching `fsnotify`).
- **External CLI Tools**: `tokio::process::Command` wrapping `ffmpeg`, `metaflac`, `cueprint`.

### 2.3 Forbidden Zones & Blast Radius
- 严格遵循 `agents.config.yaml` 安全规则。
- 保证本地 SQLite 数据库表结构 (`processed_files`) 与现有 Go 程序 100% 兼容，支持相互无缝接管与跳过已处理文件。
- 输出目录结构与命名规则严格遵守 `AGENTS.md`（去除末尾空格/点号、8.3 别名过滤、消歧后缀清理、多碟 `CD1/CD2` 规范）。

---

## 3. Testing Strategy & Verification Decisions

- **Unit Tests**:
  - 配置读取与默认值覆盖。
  - 简繁转换 (`util/chinese.go` 对标实现) 与 Unicode 标点/控制字符清理。
  - SMB / Windows 路径合规清洗（禁止以空格或点号结尾）。
  - CUE 格式解析与音轨时长推断。
- **Integration Tests**:
  - `rusqlite` 幂等性读写测试：插入、重复记录检测与查询。
  - Provider Chain 模拟网络回退流程（Mock HTTP 服务回退）。
  - 音频命令构造参数测试（针对 flac、mp3 转换参数）。
- **Regression Verification**:
  - 与 Go 版本已有逻辑对比验证，使用项目内现有 `test_meta*.flac` 进行真实转码与元数据抽取测试。

---

## 4. Out of Scope

- 完全移除 `ffmpeg`（继续复用 `ffmpeg` 外部工具，避免跨平台音频编解码库的巨大维护成本与群晖硬件加速缺失）。
- Web 管理界面或 REST API（专注保持轻量级本地 CLI 与后台守护进程）。
