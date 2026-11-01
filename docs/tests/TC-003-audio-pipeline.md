# 测试用例规范: TC-003-audio-pipeline

- **用例编号**: `TC-003`
- **目标功能**: `music-audio` 外部 CLI 封装、CUE 解析、音频分轨转码与标签写入
- **测试层级**: Unit & Integration
- **前置条件**:
  - `music-core`, `music-provider` 编译正常
  - 本地环境已有 `ffmpeg` / `metaflac` 或 Mock CLI Runner

---

## 测试步骤与断言标尺 (Steps & Assertions)

### Step 1: CUE 镜像解析与音轨分切测试
- **操作**: 
  1. 解析标准 CUE 文本（含 `PERFORMER`, `TITLE`, `FILE`, `TRACK 01 AUDIO`, `INDEX 01`）。
  2. 提取专辑名、艺术家名、音轨标题及音轨序号。
- **预期结果**: CUE 结构体成功解析所有 Track 及其索引偏移，无字段遗漏。

### Step 2: 输出目录规划与多碟 CD1/CD2 结构测试
- **操作**: 
  1. 给定输出根目录、歌手、专辑、年份、碟号（disc_num=1, disc_total=2）。
  2. 调用 `resolve_disc_output_path(root, artist, album, year, disc_num, disc_total)`。
- **预期结果**: 输出路径符合 `{root}/{artist}/{album} [{year}]/CD1` 格式，结尾无空格和点号。

### Step 3: `missing_tracks.txt` 规则断言
- **操作**: 
  1. 完整曲目（1..N 全在）：`sync_missing_tracks_file(dir, total_tracks, &present_tracks)`。
  2. 缺失曲目（例如应有 10 首，实际只有 8 首）：再次同步。
- **预期结果**: 完整时删除该文件；缺失时创建/保留该文件并列出具体缺失序号与标题。

---

## 验收完成标准 (Definition of Done)
- [ ] `cargo test -p music-audio` 全部通过。
- [ ] 真实测试样本 `test_meta.flac` 能够被解析与提取元数据。
