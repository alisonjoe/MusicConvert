# 测试用例规范: TC-004-daemon-watcher

- **用例编号**: `TC-004`
- **目标功能**: `music-convert` 守护进程、ReleaseBatch 目录扫描、CLI 参数与文件防抖监听
- **测试层级**: Unit & Integration
- **前置条件**:
  - `music-core`, `music-provider`, `music-audio` 均编译通过
  - 测试临时工作目录

---

## 测试步骤与断言标尺 (Steps & Assertions)

### Step 1: ReleaseBatch 目录批扫描断言
- **操作**: 
  1. 建立目录结构：
     - `/music/Jay/Fantasy/CD1/01.wav`
     - `/music/Jay/Fantasy/CD2/01.wav`
  2. 执行 `scan_release_batches("/music")`。
- **预期结果**: CD1 和 CD2 自动聚合为一个以 `/music/Jay/Fantasy` 为 `ReleaseContext` 的 `ReleaseBatch`，不会被拆成两个独立的批次。

### Step 2: CUE 与音频镜像配对测试
- **操作**: 
  1. 建立配对：`album.cue` 与 `album.wav`。
  2. 扫描文件并配对。
- **预期结果**: `MusicFileInfo` 包含匹配的 `cue_path` 与 `file_path`。

### Step 3: CLI 命令行参数解析测试
- **操作**: 
  1. 模拟命令行输入 `--rescan /music/album`、`--rescan-all`、`--config config.yaml`。
  2. 验证参数结构体解析结果。
- **预期结果**: 字段精确匹配，默认值正常回退。

---

## 验收完成标准 (Definition of Done)
- [ ] `cargo test -p music-convert` 全部通过。
- [ ] `cargo check --workspace` 和 `cargo test --workspace` 全绿。
