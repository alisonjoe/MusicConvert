# [Ticket-2] Tracer Bullet: Asynchronous Metadata Provider Chain

- **Spec**: `docs/specs/rust-rewrite.md`
- **Status**: completed
- **Merged in**: 5ae4dd8 (main)
- **Scope**:
  - [x] 创建 `crates/music-provider` 子 crate。
  - [x] 定义异步 `MetadataProvider` trait 与责任链 `ProviderChain`（严格对齐 ADR 0002）。
  - [x] 实现核心 Provider：MusicBrainz、NetEase（网易云）、iTunes，并支持 Spotify / Discogs 可选集成。
  - [x] 实现中文本地化保护层（有中文上下文时不被外文覆盖、拼音曲名还原校验、古典乐作品前缀保留）。
- **Blocked by**: Ticket-1
- **Affected Paths**:
  - `crates/music-provider/**`
- **Pre-authored Test Spec**: `docs/tests/TC-002-provider-chain.md`
- **Verification Plan**:
  - [x] `cargo test -p music-provider` 单元测试及 Mock 外部响应的责任链级联回退测试全绿。
