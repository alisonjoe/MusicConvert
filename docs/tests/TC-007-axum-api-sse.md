# TC-007: Axum REST API 与 SSE 实时事件流服务集成规约

## 1. 目标
验证在 `music-convert` 中集成的轻量高效嵌入式 HTTP 服务（基于 `axum`），能够向 Web 前端或外部管理系统暴露标准的 REST API 和 Server-Sent Events (SSE) 事件流，完成任务监控、历史查询及人工审阅交互闭环。

## 2. 接口契约规范 (API Contract)

### 2.1 健康检查与系统信息
- `GET /api/health`
  - 返回: `{"status": "ok", "version": "0.1.0"}`

### 2.2 任务管理接口
- `GET /api/tasks`
  - 查询参数: 可选 `status=active|history|all` (默认 `all`)
  - 返回: `{"active": [BatchTask], "history": [BatchTask]}`
- `GET /api/tasks/:id`
  - 返回: `BatchTask` 实体或 `404 Not Found`
- `POST /api/tasks/:id/approve`
  - Body: `ReviewDecision` JSON (可选 `override_artist`, `override_album`, `override_year`, `override_tracks`)
  - 返回: `{"success": true, "message": "Task approved"}` 或 `400/404`
- `POST /api/tasks/:id/cancel`
  - 返回: `{"success": true, "message": "Task cancelled"}` 或 `404`

### 2.3 SSE 实时推送流
- `GET /api/events`
  - Content-Type: `text/event-stream`
  - 自动将 `TaskRegistry.subscribe()` 广播出来的 `EngineEvent` 格式化为 SSE 消息 (`event: <event_type>`, `data: <json>`)

## 3. 验收标准 (Acceptance Criteria)

- [x] **REST 路由完整性**:
  - `/api/health` 正确响应 `200 OK` 与系统状态。
  - `/api/tasks` 能够准确聚合输出 `TaskRegistry` 中的活跃任务与环形历史缓冲区列表。
  - `/api/tasks/{id}` 能够按 ID 检索单条任务。
- [x] **人工审阅审批 (Approve / Cancel)**:
  - 针对处于 `AwaitingReview` 的任务，通过 `POST /api/tasks/{id}/approve` 发送覆盖元数据，能够成功唤醒后台调度器。
  - 通过 `POST /api/tasks/{id}/cancel` 取消任务，任务能够正确进入已取消状态并移入历史缓冲。
- [x] **SSE 事件流持续推流**:
  - 客户端连接 `GET /api/events` 后，当 `TaskRegistry` 发出事件时，客户端能实时以 SSE 协议格式收到对应事件。
- [x] **测试全绿**:
  - `cargo test -p music-convert --test tc_007_axum_api_sse_test` 与 `cargo test --workspace` 全量通过，零 Clippy 警告。
