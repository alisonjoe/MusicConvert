# Task: fix-multi-disc-tag-and-nas-sync

- **Repo**: /Users/alisonjoe/WorkSpace/MusicConvert
- **Branch**: agent/fix-multi-disc-tag-and-nas-sync-20261108
- **Base**: main
- **Created**: 2026-11-08 20:30:23

## Implementation Scope & Verification
- [x] 多碟目录名解析 (`parse_disc_number_from_name`)：支持 `CD 01`, `CD 02`, `CD1`, `Disc 1`, `DISC 02`, `Vol 1` 等。
- [x] 修正 `orchestrator.rs` 中 `disc_total` 计算与传递，不再硬编码为 1。
- [x] 增强 `parse_track_and_title_from_filename`，准确分离多段前缀中的歌手、专辑与纯曲名，杜绝生成 `01. Lady Gaga - The Fame - 01 - Bad Romance.flac`。
- [x] 在 `ffprobe.rs` 中实现 `parse_ffmpeg_stderr_tags`，当群晖系统缺失 `ffprobe` 时自动优雅降级使用 `/usr/bin/ffmpeg` 提取元数据。
- [x] 过滤 Synology `@eaDir` 与隐藏/临时文件，杜绝 recursive scan 递归风暴。
- [x] 在 `registry.rs` 的 `push_history` 中依据 `release_path` 严格去重，避免重复历史任务卡片。
- [x] 在 `chain.rs` 中增加中文上下文优先查询网易云策略，避免等待 MusicBrainz 30 秒超时。
- [x] 编写 TC-011 验收单测并全部通过，L2 `Invoke-Build.sh verify` 校验全绿。
