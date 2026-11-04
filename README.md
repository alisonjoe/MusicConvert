# MusicConvert

<div align="center">

**专为 NAS（群晖 Synology、TrueNAS、unRAID）打造的本地音乐库自动化转换与元数据治理引擎**

[![Language](https://img.shields.io/badge/Language-Rust%20%7C%20Go-orange.svg)](https://www.rust-lang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Synology-lightgrey.svg)]()
[![Tests](https://img.shields.io/badge/Tests-Passing-brightgreen.svg)]()

</div>

---

## 📖 简介

**MusicConvert** 是一个面向家庭媒体中心与个人音乐收藏库的自动化音频转换及元数据规范化守护程序。

在 NAS 环境中，下载或归档的音乐源文件常常面临标签混乱、编码各异、整轨 CUE 镜像未切分、多碟专辑被零散拆入不同目录、拼音曲名破坏阅读体验、以及古典音乐作品号遗失等痛点。MusicConvert 能够后台监控源目录，智能识别音频与 CUE 关联，利用多源权威元数据责任链对曲目信息进行富化与本地化校正，高质量转码为 FLAC/MP3，并按标准目录拓扑入库。

> **提示**：本项目已完成纯 **Rust** 异步工作空间重构，具备更低的内存与 CPU 资源开销（几兆内存常驻，非常适合内存敏感的入门级 NAS），同时保留了完整的 Go 历史实现。

---

## ✨ 核心特性

- **📁 多碟专辑智能聚类 (ReleaseBatch)**
  - 自动归纳父级专辑上下文，识别 `CD1`、`CD2`、`Disc 1` 及 `PHCP-24028` 等 Catalog 目录，按多碟目录拓扑规范输出，彻底避免同一专辑因物理碟片被拆散成多个输出目录。
- **🌐 多源元数据责任链 (Provider Chain)**
  - 整合 **MusicBrainz**、**网易云音乐 (NetEase Cloud Music OpenAPI/旧接口)** 以及 **iTunes (Search & Lookup)**。
  - **中文保护铁律**：本地或源目录已包含中文艺术家/专辑时，严禁使用外部英文标签覆盖；支持通过纯 Rust `ferrous-opencc` 引擎进行繁简规范化；支持拼音乱码还原。
- **🎼 古典音乐作品治理**
  - 专为古典乐库优化：规范化 `Op.` / `No.` 作品编号，自动清洗 Discogs 消歧后缀（如 `Indecision (2)`），保留作曲家、指挥家与乐团层级关系，杜绝 8.3 格式短名与 Windows/SMB 兼容性字符非法结尾。
- **✂️ CUE 镜像智能切分与音频转码**
  - 纯 Rust 高精度解析 CUE sheet 时间戳，结合 `ffmpeg` 与 `metaflac` 执行无损切轨与全量 Vorbis Comment / ID3 标签写入，保持与 Swinsian、Navidrome、Audio Station 等媒体库的极佳兼容性。
- **📋 缺失曲目自动追踪 (`missing_tracks.txt`)**
  - 转码过程中自动对比线上完整曲目表：若专辑缺失音轨，自动生成清晰易读的缺失列表；一旦后续补齐曲目，自动清理提示文件，保证音乐库完整性可审计。
- **🗄️ SQLite 增量幂等与生命周期控制**
  - 基于本地 SQLite 数据库持久化处理记录，防重复处理；支持成功后自动清理源文件（`is_del_src`）节省存储空间。
- **⚡ 事件驱动与滚轮日志**
  - 基于 `notify-debouncer-mini` 的防抖文件系统监控，支持平稳处理大规模批量导入；日志自带多轮次大小滚动切分，便于定位回溯。

---

## 🏗️ 架构拓扑

```
MusicConvert (Cargo Workspace)
├── crates/music-core          # 核心领域模型与基础库
│   ├── config.rs              # YAML 配置反序列化
│   ├── db.rs                  # SQLite 幂等去重持久化
│   ├── model.rs               # TrackMetadata, ReleaseMetadata 领域实体
│   └── text/                  # 文本治理 (chinese 繁简转换 / path 规范化)
├── crates/music-provider      # 异步多源元数据引擎
│   ├── core/                  # 内核契约 (MetadataProvider trait, ProviderChain, query)
│   ├── clients/               # 外部服务适配器 (MusicBrainz, NetEase, iTunes)
│   └── rules/                 # 元数据清洗与中文/古典保护规则 (localization)
├── crates/music-audio         # 音频编解码与分轨流水线
│   ├── drivers/               # 外部 CLI 驱动 (ffmpeg, ffprobe, metaflac, async cli runner)
│   ├── cue/                   # 纯 Rust CUE 解析与时间戳计算引擎 (parser)
│   └── policy/                # 治理策略 (missing_tracks 审计 / path_resolver 多碟拓扑)
└── crates/music-convert       # 守护进程与应用程序调度
    ├── app/                   # 命令行入口 (clap CLI 解析)
    ├── monitor/               # 监听源 (ReleaseBatch 目录聚类扫描 / notify 异步文件监听)
    └── engine/                # 编排调度总线 (ConvertOrchestrator)
```

---

## 🛠️ 运行依赖

MusicConvert 依赖以下命令行工具完成实际的音视频编解码与无损标签写入，请确保它们在宿主系统的 `PATH` 中可用：

| 工具 | 作用 | 推荐安装方式 |
| :--- | :--- | :--- |
| **`ffmpeg`** | 音频格式转码与时间戳切轨 | `apk add ffmpeg` / `apt install ffmpeg` / SynoCommunity 套件 |
| **`ffprobe`** | 提取源音频封装格式与流时长 | 通常随 `ffmpeg` 一起预装 |
| **`metaflac`** | FLAC Vorbis Comment 标签写入与清空 | `apk add flac` / `apt install flac` |
| **`cueprint`** *(可选)* | CUE 补充检查工具 | `apt install cuetools` |

---

## ⚙️ 配置说明 (`config.yaml`)

在程序工作目录下提供 `config.yaml`（或通过 `--config` 指定路径）：

```yaml
# 监控的音乐源目录（支持绝对路径）
src_path: "/volume1/music/incoming"

# 转码整理后的最终音乐库根目录
output_root: "/volume1/music/library"

# 目标输出格式：FLAC 或 MP3
desc_type: "FLAC"

# 是否将 CUE 单轨镜像切分为独立音轨 (true/false)
is_split: true

# 转换成功后是否删除源文件（true 为移动/清理模式，false 为归档保留模式）
is_del_src: false

# 外部元数据服务凭据（可选）
spotify_client_id: ""
spotify_client_secret: ""
spotify_enabled: false

# 网易云音乐接口配置（可选，增强中文专辑匹配）
netease_auth_mode: "anonymous" # 可选 "anonymous" 或 "cookie"
netease_cookie: ""
netease_app_id: ""
netease_private_key: ""

# Discogs 个人访问令牌（可选，增强古典音乐目录号匹配）
discogs_token: ""

# 手动曲序/标题覆盖规约（可选）
track_overrides: {}
```

---

## 🚀 命令行与使用场景

### 1. 常用命令

```bash
# 1. 默认运行（读取当前目录 config.yaml 并启动监控守护进程）
./MusicConvert

# 2. 指定自定义配置文件路径
./MusicConvert --config /etc/musicconvert/config.yaml

# 3. 单次扫描并处理指定子目录，处理完成后退出
./MusicConvert --rescan /volume1/music/incoming/Classical_Boxset

# 4. 强制全量重新扫描整个源目录（跳过数据库幂等检查）
./MusicConvert --rescan-all

# 5. 测试 Spotify API 凭据联通性
./MusicConvert --test-spotify
```

### 2. 命令行参数一览

```
选项:
  -c, --config <FILE>     指定配置文件路径 [默认: config.yaml]
  -r, --rescan <PATH>     指定目录路径进行单次针对性扫描
      --rescan-all        强制全量重扫源目录，忽略已处理标记
      --test-spotify      测试配置文件中的 Spotify 客户端凭据连通性
  -h, --help              打印帮助信息
  -V, --version           打印版本信息
```

---

## 📦 编译与部署

### 1. 本地编译与测试

```bash
# 运行工作空间内全部单元测试与集成测试
cargo test --workspace

# 编译本地 Release 二进制
cargo build --release

# 产物位置：target/release/MusicConvert
```

### 2. 群晖 NAS / Linux 交叉编译

项目自带完善的跨平台交叉编译脚本，支持一键构建 Linux amd64、arm64 以及 armv7 架构下的独立二进制文件：

```bash
# 执行 Rust 群晖交叉编译总线脚本
./scripts/build_synology_rust.sh

# 产物生成在项目根目录：
# - MusicConvert_synology_amd64 (Intel/AMD x86_64 群晖)
# - MusicConvert_synology_arm64 (Realtek/Annapurna 64位 ARM 群晖)
# - MusicConvert_synology_armv7 (32位 ARM 群晖)
```

### 3. 群晖 NAS 任务计划部署建议

1. 将编译好的 `MusicConvert_synology_amd64`（或对应架构版本）上传至群晖 NAS 某个目录（如 `/volume1/scripts/MusicConvert`）。
2. 在该目录下放置配置文件 `config.yaml`。
3. 确保系统安装了 `ffmpeg` 与 `flac`（可通过群晖套件中心安装 **SynoCommunity** 的 ffmpeg 套件）。
4. 在群晖 **控制面板 -> 任务计划** 中添加开机启动任务或定时任务：
   ```bash
   #!/bin/sh
   export PATH="/usr/local/bin:/usr/bin:/bin:/volume1/@appstore/ffmpeg/bin:$PATH"
   cd /volume1/scripts/MusicConvert
   nohup ./MusicConvert_synology_amd64 >> logs/MusicConvert.log 2>&1 &
   ```

---

## 🤝 开发与贡献规范

本仓库严格遵循 AI 原生工程协作准则与分层测试驱动开发（TDD）：
- 所有特性修改与 Bug 修复均有对应的测试用例（位于 `crates/*/tests/` 或单元测试模块）。
- 提交代码前，请确保通过以下门禁：
  ```bash
  cargo fmt --check
  cargo clippy --workspace --all-targets -- -D warnings
  cargo test --workspace
  ```

---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 开源发布。
