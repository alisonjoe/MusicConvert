# Ticket: 多碟目录推断、ffmpeg 降级探测与 @eaDir 过滤

## 状态
- [x] 已完成 (Done)

## 关联 PR / Commit
- `25fee97` (fix: multi-disc resolution, ffmpeg probe fallback and deduplication)
- 对应测试规约：`tests/specs/TC-011-multi-disc-and-ffmpeg-fallback.md`
- 对应 ADR：`docs/adr/0007-multi-disc-resolution-and-ffmpeg-fallback.md`

## 验收结果
- [x] 多碟专辑（如 `CD 01`/`CD 02`）正确识别盘号，曲目规范隔离输出到 `CD1/`、`CD2/` 目录。
- [x] 群晖 NAS 缺失 `ffprobe` 时，自动平滑 fallback 至 `/usr/bin/ffmpeg` 提取标签。
- [x] 强化文件名曲名解析，彻底剔除 `Artist - Album` 前缀，杜绝输出 `01. Lady Gaga - The Fame - 01 - ...`。
- [x] 过滤 `@eaDir` 与隐藏文件，阻止扫描风暴。
- [x] 内存历史记录按 `release_path` 严格去重与替换。
- [x] 中文专辑优先走网易云，消除 30 秒超时卡死。
- [x] L2/L3 双轨门禁全绿，群晖二进制构建并在真实 NAS 上部署运行。
