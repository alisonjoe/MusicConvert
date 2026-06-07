# AGENTS.md

## 项目说明
`MusicConvert` 是一个用 Go 写的本地音乐转换守护程序。它会监控源目录，识别新增或已有的音频文件与 CUE 文件，提取元数据，转换为 FLAC/MP3，写入标签，整理输出目录结构，并把已处理记录写入本地 SQLite 数据库。

这个项目主要面向 NAS 部署，尤其是群晖。代码里包含了比较多的元数据归一化逻辑，覆盖中文、英文、古典音乐、CUE 专辑、多碟专辑，以及各种标签混乱的源目录。

## 入口文件
- `main.go`：程序入口，负责配置加载、数据库初始化、文件监控、任务投递，以及 `--rescan`、`--rescan-all`、`--test-spotify` 等命令行参数。
- `audio/convert.go`：核心转换与元数据决策逻辑。大多数行为修改都应该落在这里。
- `config/config.go`：YAML 配置解析与配置辅助函数。
- `db/music.go`：保存已处理源文件的 SQLite 持久化层。

## 数据流
1. `main.go` 读取 `config.yaml`。
2. 程序先扫描一次源目录，然后用 `fsnotify` 持续监听。
3. `util.GetMusicFileInfoForFile` 负责把音频文件和匹配的 `.cue` 文件配对。
4. `audio.ToFLAC` 从队列取任务，分别进入单文件转换或 CUE 转换。
5. `audio/convert.go` 按大致如下顺序决定元数据来源：
   - 本地标签和源目录线索
   - CUE 上下文
   - MusicBrainz
   - 如果启用则使用 Spotify
   - 网易云 / OpenAPI / 旧接口回退
   - iTunes 做专辑级补充
   - 配了 token 时，对古典专辑再尝试 Discogs
6. 输出文件写入 `output_root` 下，按歌手和专辑分层，多碟专辑再加 `CD1`、`CD2` 之类的目录。
7. 成功处理后会写入 `musicConvert.db`，避免同一源文件重复转换。

## 关键文件
- `audio/convert.go`：规范化元数据、构建输出路径、处理碟号、年份、古典音乐、缺失曲目报告和标签写入。
- `audio/cueprint.go`：封装 `cueprint`，用于读取 CUE 的曲目数和曲名。
- `audio/ffmpeg.go`：通过 `ffmpeg` 导出或复制元数据。
- `api/musicbrainz.go`：MusicBrainz 搜索与 release 匹配。
- `api/spotify.go`：Spotify 客户端凭证搜索和候选日志。
- `api/itunes.go`：iTunes 专辑元数据、候选日志和专辑名修正。
- `api/netease.go` 和 `api/netease_openapi.go`：网易云搜索、专辑查询、曲目标题获取和歌词相关辅助。
- `api/discogs.go`：当配置了 token 时，对古典音乐 release 做补充。
- `util/file.go`：源文件发现、CUE 配对、文件名清洗、唯一路径生成和源文件删除。
- `util/cmd.go`：带 context 超时的命令执行封装。
- `util/chinese.go`：简繁转换，用于标签、歌手名、专辑名和文件名。

## 运行依赖
程序运行时依赖以下工具必须在 `PATH` 中可用：
- `ffmpeg`
- `ffprobe`
- `metaflac`
- `cueprint`

如果某个功能依赖的工具缺失，应该只让对应功能失败并打印警告，不要在重构时默认它一定存在。

## 配置
`config.yaml` 是运行时控制文件。常见字段：
- `src_path`：监控的源目录。
- `output_root`：输出库根目录。
- `desc_type`：输出编码类型。
- `is_split`：是否拆分 CUE 镜像。
- `is_del_src`：转换成功后是否删除源文件。
- `spotify_client_id` / `spotify_client_secret`
- `spotify_enabled` 和旧字段 `spotify_premium`
- `netease_auth_mode`、`netease_app_id`、`netease_private_key`、`netease_cookie` 等
- `discogs_token`
- `track_overrides`：手工曲序覆盖，key 支持 `artist|album` 或 `album`

优先使用 `spotify_enabled`。`spotify_premium` 只保留兼容用途，不建议继续作为新逻辑的主字段。

## 输出规则
- 文件名和标签优先使用简体中文；中文专辑的歌手、专辑、曲目应尽量保持中文（中文专辑里的英文歌名除外）；不要用拼音曲名替代中文曲名，若中文专辑最终只得到疑似拼音标题应跳过转换；本地路径或上下文已有中文歌手名时，不用英文标签 / iTunes / 在线结果覆盖；遇到拼音源标签或历史拼音命名文件时应优先尝试用网易等中文结果还原，允许轻微专辑名拼写差异（如 `Cross Cover` / `Cross Over`）但仍需标题和歌手校验，`DISC 1` / `CD1` 这类分碟目录或标签不能当专辑名；古典专辑已有可靠中文作曲家 / 作品 / 曲目上下文时，不用纯英文 Discogs / MusicBrainz 结果覆盖中文显示名，可优先使用中文百科等在线中文曲目来源，避免把具体曲目表硬编码到代码里；若古典曲目只能拿到英文 / 日文标题，应保留中文作品名前缀形成中英混合标题，常见体裁名可用通用规则生成中文作品前缀。
- 写入输出前要归一化艺术家名、专辑名和曲名里的 Unicode 标点；古典曲名中的 `N°` / `№` 等编号符号应规范为 `No.`；古典目录名前缀中的来源序号（如 `7_维瓦尔第`、`9贝多芬`）不要进入最终艺术家 / 专辑名，但 `12首...` 这类作品数量不能被误删。
- 同一张专辑应该尽量只落到一个目录里。不要因为标点差异或噪声源目录名把同一张专辑拆成多个输出目录；古典同曲不同版本应在专辑名中加入演奏者 / 指挥等鉴别符，避免不同版本撞目录。
- 多碟专辑使用 `CD1`、`CD2`、`CD3` 等目录；如果源目录用 `PHCP-24028` 这类纯 catalog 子目录分碟，应把其父目录作为专辑上下文、保留当前 catalog 用于 Discogs / MusicBrainz 查询，并按 catalog 子目录顺序推断碟号；不要用相邻 catalog 的曲目表覆盖当前实体 CD，但可用相邻较早 catalog 查盒装级别专辑 / 演奏者元数据；当 catalog 目录直接位于 `PHILIPS首版系列纸盒WAV` 等泛目录下时，应优先把 catalog 自身作为查询上下文而不是泛目录名。
- `missing_tracks.txt` 的规则：
  - 有缺失曲目时保留
  - 曲目完整时删除
  - 流行专辑应优先使用已校验的在线完整曲目表写缺失项；不要用 `source_dir` 的部分曲目导致缺失项显示 `<unknown>`

## 古典音乐规则
项目里对古典音乐和大盒装有专门处理。
- CUE 里的 `TITLE` 和 `REM DATE` 应该作为强专辑上下文。
- 不要让单曲级别的在线命中覆盖已经明确的多碟专辑上下文。
- 遇到古典专辑时，优先按作曲家 / 作品 / 指挥 / 乐团解析，而不是按流行音乐那套规则处理；室内乐专辑没有明确指挥时不要把作曲家或主演奏者误写成 Conductor；本地已有中文室内乐团名时，不要被未本地化的英文 Discogs 艺术家覆盖。
- Discogs 是可选项，只有配置了 token 才有意义。
- `import`、`series`、`bootleg`、`philips`、`trackNN` 之类的泛目录名，不要轻易当成最终歌手或专辑身份，除非没有更好的证据。
- `指挥` / `指揮` 只作为 Conductor 与 Orchestra 的分隔符；同一 CUE 古典专辑内 Orchestra 候选优先选择最长完整名称，不用硬编码别名或删除 `大`、`皇家`、`国立` 等修饰词；全球乐团 / 指挥中文名优先依赖 MusicBrainz 等在线规范名，不维护大规模映射表。

## 元数据来源优先级
- MusicBrainz 是西文音乐的主要外部来源。
- 网易云更适合中文发行和歌词 / 曲目表兜底，但西文专辑上可能很噪；封面补充必须先通过 artist + album 校验，不能只按 album 搜索第一项。
- iTunes 主要用于专辑级补充和标准专辑名，不应该当唯一权威；只有候选专辑与当前上下文精确匹配或明确 deluxe/edition 变体时才采用其 AlbumArtist / CollectionName，避免被搜索结果第一项或仅包含同名词的专辑污染；当前专辑艺术家已有中文名时，不用 iTunes 英文名覆盖。
- Spotify 是可选项，只有在凭证和 API 行为确认可用时才用。
- 对于像 `Track01.wav` 这种泛化文件名，不要在没有足够专辑上下文时进行大范围外网搜索；若已有强专辑上下文但本地 artist 为 Unknown，MusicBrainz 可用标题 + 专辑匹配补全 artist；最终仍无法解析出非泛化曲名时跳过转换并记录日志，避免输出 `TrackNN` 成品。

## 修改规则
- 改动要尽量小，只围绕当前问题修。
- 优先复用现有 helper 和本地约定，不要随意引入新的抽象层。
- 不要回滚用户已经做过的改动，除非用户明确要求。
- 不要在用户没明确要求的情况下做破坏性操作。
- 手工改文件请使用 `apply_patch`。
- 默认使用 ASCII 编写代码和文档，除非文件本身已经是非 ASCII，或者文档内容本身需要中文。

## 验证方式
优先跑最小的相关检查：
- `go test ./...`
- `./scripts/build_synology.sh`
如果改动了元数据匹配或输出命名，最好补 `audio/convert_test.go` 里的定向测试，并用一组代表性样本检查最终目录结构。
**每次任务完成后，必须生成可用于真实测试的二进制文件。**

## 给后续代理的备注
- 这个仓库里有时会残留用户本地生成的二进制和测试文件，除非它们挡住了当前任务，不要去动。
- 不要把当前源目录的命名当成权威。很多真实音乐库目录都很乱，程序的任务就是尽量修正它。
- 新增元数据来源时要保持日志清晰。候选结果的详细日志是刻意保留的，主要用于排查匹配问题；运行日志应同时输出到本地 `logs/MusicConvert-YYYYMMDD-HHMMSS.log` 便于回溯。

## 收尾动作（默认执行）
任务完成后（代码修改 + 验证通过），必须：
- 在「当前实现状态」中更新已完成项（如标记 Bug 已修复、Feature 已实现）
- 如有新行为约束，补充到对应规则节（输出规则 / 古典音乐规则 / 元数据来源优先级）
- **禁止新增 Bug 记录 / 修复日志章节**
- 用 `apply_patch` 修改 AGENTS.md，不手动重写整个文件
- 执行 Git 默认行为（commit，不 push）
- **生成 Linux/amd64 二进制文件（用于真实测试）**

## Git 默认行为
- 每次代码修改（含 Bug 修复、Feature 实现、测试补丁）完成后，必须：
  1. `git status` 检查变更范围
  2. `git add` 相关文件（不要 `git add .`）
  3. `git commit -m "<type>: <short description>"`
- Commit message 规范（简洁即可）：
  - `fix:` 修复 Bug（如 `fix: split CUE multi-disc into CD1/CD2`）
  - `feat:` 新功能
  - `refactor:` 非功能改动
  - `test:` 测试相关
- **禁止**：
  - 私自 `git push`
  - 私自 `git reset --hard`
  - 私自 `git rebase`
  - 提交二进制文件（MusicConvert 等）
  - 提交 `.DS_Store`
  - 提交 `*.db`
  - 私自 `git push / rebase / reset --hard`
- 改动较大时应提示创建新分支
- 如果当前在 `main` 分支且改动较大，应提示用户是否创建新分支。
- 若 AGENTS.md 自身被修改，也应一并提交。
