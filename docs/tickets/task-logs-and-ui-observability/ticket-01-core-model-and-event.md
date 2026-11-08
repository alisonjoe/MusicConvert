# Ticket-01: 核心领域模型 TaskLogItem 与 TaskRegistry 日志追加及事件广播

## 状态
- [x] 已完成 (Done)

## 目标
1. 在 `crates/music-convert/src/engine/task.rs` 中定义 `TaskLogItem`，并将其嵌入 `BatchTask`。
2. 在 `crates/music-convert/src/engine/event.rs` 中增加 `EngineEvent::TaskLogAppended`。
3. 在 `crates/music-convert/src/engine/registry.rs` 中增加 `append_task_log` 方法，限制单个任务日志上限为 100 条防止内存膨胀。
4. 编写并运行单元测试验证生命周期与序列化。
