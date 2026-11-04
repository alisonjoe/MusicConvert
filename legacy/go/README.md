# Legacy Go Implementation (历史 Go 实现归档)

本目录完整保存了 MusicConvert 初始版本的 Go 实现源码，仅作为只读参考资产、算法核对与回归排查之用。

- **状态**: 归档 (Archived & Read-Only)
- **主力运行态**: 仓库根目录 Rust Cargo Workspace (`crates/`)
- **包含内容**:
  - `main.go`: 原程序入口
  - `api/`: MusicBrainz / NetEase / iTunes / Spotify 历史客户端实现
  - `audio/`: 原基于 ffmpeg / shnsplit / cueprint 的音频处理
  - `config/`, `db/`, `model/`, `util/`: 原配置、SQLite 与工具库
  - `scripts/build_synology_go.sh`: 原 Go 交叉编译脚本
