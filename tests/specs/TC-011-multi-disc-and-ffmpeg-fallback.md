# TC-011: 多碟目录推断、ffmpeg 降级探测与历史去重验收规约

## 1. 规约目标
针对真实 NAS 部署中发现的多碟未分流（如 `CD 01`/`CD 02`）、群晖无 `ffprobe` 导致元数据丢失全归 01、`@eaDir` 扫描风暴、以及重复完成任务在历史卡片中堆叠的问题，建立自动化回归测试。

## 2. 覆盖场景
1. **多碟目录解析**：
   - 支持 `CD 01`, `CD 02`, `CD1`, `Disc 1`, `DISC 02`, `Volume 1`，能正确提取 `disc_num` 与统计 `disc_total`。
   - 多碟专辑的曲目分别落入 `CD1/` 和 `CD2/` 目录中。
2. **文件名音轨与曲名提取**：
   - 包含歌手、专辑前缀的文件名（如 `Lady Gaga - The Fame - 01 - Bad Romance`），必须准确提取 `track_num = 1`，`title = "Bad Romance"`，输出文件为 `01. Bad Romance.flac`，严禁产生 `01. Lady Gaga - The Fame - 01 - Bad Romance.flac`。
3. **Synology 系统私有目录屏蔽**：
   - Scanner 和 Watcher 必须自动忽略 `@eaDir` 及其子文件，防止递归事件风暴。
4. **历史任务去重**：
   - 后端 `TaskRegistry::push_history` 同一 `release_path` 再次完成时，必须自动替换旧记录，保证历史记录中单一专辑仅呈现最新状态。
