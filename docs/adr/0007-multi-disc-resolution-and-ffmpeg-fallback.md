# 0007. 多碟目录结构推断、ffmpeg 降级探测与系统私有目录风暴屏蔽

- **日期**: 2026-11-08
- **状态**: 提议已通过并合入 (Accepted)
- **涉及范围**: `crates/music-audio/`, `crates/music-convert/`, `crates/music-provider/`

---

## 1. 背景与业务痛点 (Context)

在真实群晖 NAS（DSM）环境部署后，发现以下四项关键异常：
1. **多碟目录未隔离**：源目录为 `CD 01`/`CD 02` 时，盘号解析逻辑因前导空格 parse 失败回退为 1，且单曲转换层 `disc_total` 硬编码为 1，导致多碟专辑曲目全被扔进根目录混杂并被错误命名为 `01. Lady Gaga - The Fame - ...`。
2. **群晖系统无 ffprobe**：群晖默认仅内置 `/usr/bin/ffmpeg`，缺失 `ffprobe` 导致所有音频文件的本地 Tag 探测全部报错，退化为文件名盲猜。
3. **文件名多段前缀清洗不全**：针对形如 `Artist - Album - 01 - Title` 的文件名，曲名解析未能精准截断前缀，导致曲名被污染。
4. **Synology @eaDir 递归事件风暴**：群晖系统生成的缩略图私有目录 `@eaDir` 产生海量 inotify Access 事件，不断触发重复扫描与重复推送完成记录。
5. **中文专辑检索等待过长**：中文专辑优先进入西文提供商 MusicBrainz 导致国内 NAS 网络握手阻塞 30 秒超时。

---

## 2. 架构决策 (Decision)

1. **通用碟号推断算法**：
   - 在 `music-audio` 中提供 `parse_disc_number_from_name`，基于正则 `(?i)^(?:cd|disc|disk|vol|volume)[\s._-]*(\d{1,3})$` 兼容 `CD 01`, `CD 02`, `CD1`, `Disc 1`, `DISC 02`, `Vol 1` 等变体。
   - `ConvertOrchestrator` 统计批次内所有文件的最大碟号作为 `disc_total` 下发，多碟时由 `resolve_disc_output_dir` 规范化为 `CD1`、`CD2` 子目录。
2. **ffmpeg 优雅降级探测 (Graceful Degradation)**：
   - 在 `ffprobe.rs` 中，若 `ffprobe` 执行失败，自动通过 `ffmpeg -i <file>` 读取 stderr 中的 `Metadata:` 块并提取 Vorbis / ID3 tags。
3. **文件名纯曲名提取增强**：
   - 当文件名按 ` - ` 分割且片段数 $\ge 3$ 时，若检测到纯数字片段，其后内容为纯曲名，彻底分离前面的 `Artist - Album - TrackNo`；若片段数为 2 且首段非数字，则自动按 `Artist - Title` 解构。
4. **Synology @eaDir 与隐藏文件硬拦截**：
   - `WalkDir` 遍历时通过 `filter_entry` 在目录入口处拦截 `@eaDir`、`@*` 和 `.*`，watcher 事件流中彻底丢弃包含 `@eaDir` 或临时后置符的路径。
5. **内存历史按 `release_path` 严格去重与替换**：
   - `TaskRegistry::push_history` 发现已有相同专辑路径的完成任务时就地剔除旧记录保留最新一条，防止卡片重复堆叠。
6. **中文上下文动态优先级倒置**：
   - `MetadataQuery` 命中中文上下文时，`ProviderChain` 动态调整为 `NetEase -> ITunes -> MusicBrainz` 顺序，毫秒级返回中文曲目表。

---

## 3. 产生的影响与收益 (Consequences)

- **正面收益**：
  - 群晖多碟实体 CD 规范隔离至 `CD1/`、`CD2/` 目录下；
  - 群晖无 `ffprobe` 也能完整读取本地音轨号和标签；
  - 彻底斩断群晖私有目录引起的递归扫描风暴与历史卡片重复；
  - 中文专辑检索秒级响应，不再等待 30 秒超时。
- **负面代价 / 权衡**：
  - `ffmpeg -i` 需解析文本 stderr，较 JSON 格式解析稍多微量正则开销，但在单次探测场景下耗时差异可忽略（<5ms）。
