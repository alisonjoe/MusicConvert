# TC-010: 任务抓取日志生成、事件流式推送与状态查询规约

## 测试目标
验证 `TaskLogItem` 能够正确随任务生成并记录阶段日志，`TaskRegistry::append_task_log` 能够触发 `EngineEvent::TaskLogAppended` 广播，并通过 Axum REST API（`GET /api/tasks`）与 SSE（`/api/events`）无损流向前端。

## 测试步骤与断言
1. **领域模型单测**：
   - 创建 `BatchTask`，验证初始 `logs` 为空。
   - 通过 `registry.append_task_log(&task_id, "MusicBrainz", "INFO", "Matched release")` 追加日志。
   - 检查任务对象中 `logs.len() == 1`，字段解析完整。
2. **容量保护单测**：
   - 连续追加 120 条日志，验证单个任务日志上限限制为 100 条（FIFO 淘汰旧日志）。
3. **事件总线广播**：
   - 订阅 `registry.subscribe()`，追加日志时验证能收到 `TaskLogAppended` 事件。
4. **集成测试**：
   - 在 `orchestrator.process_batch` 运行过程中验证各阶段日志（LocalProbe, MusicBrainz/NetEase/iTunes, Transcode）均成功注入到任务中。
