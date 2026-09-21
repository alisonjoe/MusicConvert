# 0003: 从单曲处理向专辑级批处理演进 (Album Batch Processing)

## 状态 (Status)
已采纳 (Accepted) - 2026-09-21

## 背景 (Context)
系统原先采用“单曲级 (File-by-File)”的元数据获取与处理模式。这导致了严重的文件碎片化问题：
1. 同一张专辑内的不同单曲，可能会因为抓取到不同版本的元数据（如普通版 vs 豪华版）而被分散到不同的输出目录。
2. 群星合辑 (Various Artists) 会为每首歌曲单独创建歌手文件夹，导致目录灾难。
3. 多碟专辑（CD1/CD2）因独立查询，常无法正确合体。
4. 外部 API（如 MusicBrainz）返回的拼音/外文名未能得到强制的中文清洗兜底，导致输出路径不符合本地化预期。

## 决策 (Decision)
1. **引入 ReleaseBatch (目录打包) 模型**: 扫描器将不再返回扁平的单曲列表，而是基于物理目录结构（并自动合并同属一级的 CD1/CD2 兄弟目录）将文件打包为 `ReleaseBatch`。
2. **全局元数据决策**: 转换引擎 `audio/convert.go` 接收一个 `ReleaseBatch`，仅对代表性曲目（或专辑名）执行**一次** Provider Chain 查询，并强制使用查得的最佳中文结果（Localization Layer）。
3. **强制下发**: 确定的 `AlbumArtist`、`AlbumName`、`Cover` 等元数据将被统一强制赋予该 Batch 内的所有单曲，仅保留单曲各自的 `TrackTitle` 和 `TrackNumber`。

## 影响 (Consequences)
- **正面**: 彻底解决专辑碎裂、合辑灾难和多碟无法合并的问题，确保目录输出的绝对整洁和高度一致。
- **负面**: 彻底重构底层调度循环，`main.go`、`util/file.go` 和 `audio/convert.go` 均需大幅改造，增加了短期的代码复杂度。
