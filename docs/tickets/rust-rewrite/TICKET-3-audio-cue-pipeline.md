# [Ticket-3] Audio Pipeline & CUE Splitter Engine

- **Spec**: `docs/specs/rust-rewrite.md`
- **GitHub Issue**: #3
- **Status**: completed
- **Merged in**: 5ae4dd8 (main)
- **Scope**:
  - [x] 创建 `crates/music-audio` 子 crate。
  - [x] 基于 `tokio::process::Command` 封装异步外部 CLI 执行器（`ffmpeg`、`metaflac`、`ffprobe`、`cueprint`），自带超时与降级容错。
  - [x] 实现 CUE 镜像解析与音轨分切管道。
  - [x] 实现标准元数据 Tag 写入器及输出目录规划器（支持多碟 `CD1/CD2` 结构、`missing_tracks.txt` 判定与生成）。
- **Blocked by**: Ticket-2
- **Affected Paths**:
  - `crates/music-audio/**`
- **Pre-authored Test Spec**: `docs/tests/TC-003-audio-pipeline.md`
- **Verification Plan**:
  - [x] `cargo test -p music-audio` 全绿。
  - [x] 对现有测试样本 `test_meta.flac` 执行元数据提取与转码命令参数校验。
