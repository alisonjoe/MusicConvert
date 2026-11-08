# Task: web-config-and-force-reconvert

- **Repo**: /Users/alisonjoe/WorkSpace/MusicConvert
- **Branch**: agent/web-config-and-force-reconvert-20261109
- **Base**: main
- **Created**: 2026-11-09 00:14:45

## Implementation Scope & Verification
- [x] **配置可视化与持久化**：
  - `music-core/config.rs`：新增 `force_reconvert` 字段，支持 0/1/bool 解析；新增 `save_to_file` 方法写回 YAML。
  - `music-convert/src/server/`：新增 `POST /api/config` 路由与处理器，支持在线修改配置、热更新运行时调度器并持久化写回 `config.yaml`。
- [x] **界面开启强制转换 (Force Reconvert)**：
  - 调度编排器新增 `process_batch_ext(batch, force: bool)`，当 `force || cfg.force_reconvert` 为真时跳过 SQLite `is_processed` 检查，全量重新处理整个目录所有曲目。
  - `POST /api/scan` 支持 `{ "force": bool }` 参数。
  - 前端控制台在【手动扫描转换】左侧增加【⚡ 强制重转】快捷勾选框与【⚙️ 系统设置】模态弹窗，提供在线配置编辑与持久化保存能力。
- [x] **修复曲目乱序 (如 B.B. King 截图)**：
  - 在 `orchestrator.rs` 中增加单曲自然排序逻辑（Natural Sort），先按 Disc 升序，再按 TrackNumber / 文件名升序，彻底解决无序遍历导致 `#13` 置顶的乱序问题。
- [x] **双轨门禁测试验证**：
  - 更新 `tc_009_web_control_api_test.rs`，验证 `POST /api/config` 持久化与 `POST /api/scan (force)`。
  - 全量 `cargo test --workspace` 测试通过（38+ tests passed, 0 failed）。
