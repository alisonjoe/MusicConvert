# Spec: Web 控制台源目录管理、已转换列表与手动转换 (Web Control Features)

- **Date**: 2026-11-08
- **Status**: Approved
- **Domain Context**: Alignment with `CONTEXT.md` (ReleaseBatch, Orchestrator, SQLite Database)
- **Module Scope**:
  - `crates/music-core/src/db.rs`: SQLite 查询与记录检索增强
  - `crates/music-convert/src/server/`: REST API 端点扩展 (文件系统浏览、配置查看/修改、已转换查询/重转、手动扫描触发)
  - `crates/music-convert/src/server/assets.rs`: Vue 3 控制台 UI 界面交互扩展

---

## 1. User Stories & Goals

1. **源目录选择与管理 (Source Folder Selection)**:
   - 作为用户，我希望在 Web 控制台直观查看当前监控的源目录 `src_path`，并通过目录浏览器或输入框切换源文件夹，无需手工登录后台修改 `config.yaml` 或重启进程。
2. **已转换列表与重新转换 (Converted Records & Re-conversion)**:
   - 作为用户，我希望在 Web 控制台查看历史已转换完成的音乐列表（支持搜索与分页），并在需要重新生成或更新标签时，一键触发“重新转换”，系统自动撤销已转换标记并重新执行转码富化。
3. **手动立即扫描转换 (Manual Scan & Convert Trigger)**:
   - 作为用户，我希望在页面上一键点击“立即手动扫描转换”，立即对当前源目录发起全量/增量聚类扫描并推入转换队列，直观感知进度与结果。

---

## 2. Technical Architecture & Endpoints

### 2.1 数据持久层扩展 (`crates/music-core/src/db.rs`)
- `list_records(limit: usize, offset: usize, search: Option<&str>) -> Result<Vec<MusicInfo>, DbError>`:
  支持按时间倒序 `created DESC` 查询已处理记录，支持可选的路径/曲名模糊匹配 (`LIKE %search%`)。
- `count_records(search: Option<&str>) -> Result<usize, DbError>`:
  查询总记录条数，用于前端分页展示。
- `delete_by_music(music: &str) -> Result<usize, DbError>`:
  精确删除单条已处理记录。

### 2.2 服务端运行时状态注入 (`crates/music-convert/src/server/state.rs`)
- `AppState` 扩展持有：
  - `db: Arc<Database>`: SQLite 实例引用。
  - `scan_trigger: mpsc::Sender<ScanCommand>` (或事件触发管道): 用于向后台扫描调度线程派发手动扫描指令。
  - `current_src_path: Arc<RwLock<PathBuf>>`: 动态支持运行时源路径热更新。

### 2.3 REST API 接口契约
| 路径 | 方法 | 功能描述 | 请求参数 | 响应格式 |
| :--- | :--- | :--- | :--- | :--- |
| `/api/config` | GET | 获取当前运行时基础配置 (src_path, output_root 等) | 无 | `{ "src_path": "...", "output_root": "..." }` |
| `/api/config/src-path` | POST | 更新当前源目录并热重载文件监控 | `{ "src_path": "..." }` | `{ "success": true, "src_path": "..." }` |
| `/api/fs/browse` | GET | 受控文件系统目录浏览 | `?path=/volume1` | `{ "current": "...", "directories": ["dir1", "dir2"] }` |
| `/api/converted` | GET | 分页检索已转换音乐记录 | `?limit=50&offset=0&search=...` | `{ "total": 120, "items": [...] }` |
| `/api/converted/reconvert` | POST | 撤销数据库记录并投递重转 | `{ "music": "path/to/song.flac" }` 或 `{ "prefix": "..." }` | `{ "success": true, "message": "..." }` |
| `/api/scan` | POST | 手动触发全量/增量源目录扫描转换 | `{ "path": optional }` | `{ "success": true, "message": "Scan triggered" }` |

### 2.4 Web 控制台前端交互设计
- **顶部操作栏**:
  - 当前源目录 Badge 及「📂 切换源目录」交互按钮。
  - 「⚡ 立即手动扫描」主操作按钮，带加载指示器与执行结果 Toast。
- **目录选择弹窗 (Folder Picker Modal)**:
  - 树形/层级下钻浏览服务器目录，支持快速点击子文件夹，也可直接手动输入路径提交。
- **已转换音乐 (Converted Library) 视图**:
  - 独立 Tab 标签页，包含搜索栏、数据表格（文件名/路径、转换时间、状态）。
  - 操作列配备「🔄 重新转换」按钮，点击确认后发起重转，无缝切换到实时任务流追踪进度。

---

## 3. Testing & Verification Strategy

1. **Unit Tests (Core & DB)**:
   - `crates/music-core/src/db.rs`: 验证 `list_records`、`count_records`、`delete_by_music` 的分页、搜索与精准删除。
2. **Integration Tests (Server API)**:
   - `crates/music-convert/tests/`: 测试 `/api/config`、`/api/converted`、`/api/fs/browse`、`/api/scan` 的 HTTP 请求/响应。
3. **Workspace Full Verification**:
   - `cargo test --workspace` 必须全绿通过。
   - `./scripts/build_synology.sh` 编译无 warning。
