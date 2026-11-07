# TC-006: 调度中枢状态机、事件广播总线与人工审阅挂起机制验证

## 1. 目标
验证 `ConvertOrchestrator` 升级为异步状态机引擎后，具备双层任务模型（`BatchTask` + `TrackItem`）、Tokio 广播总线事件发射（`broadcast::channel`）、内存环形缓冲区以及低置信度人工审阅挂起/放行（`oneshot` 通道唤醒）的能力。

## 2. 验收标准 (Acceptance Criteria)

- [x] **双层任务状态建模**:
  - `BatchTask` 具备唯一 `task_id`、路径、整专元数据与生命周期状态（`Discovered` $\rightarrow$ `MatchingMetadata` $\rightarrow$ `Transcoding` / `AwaitingReview` $\rightarrow$ `Completed` / `Failed`）。
  - `TrackItem` 跟踪单轨编号、标题、百分比进度（0%~100%）与状态。
- [x] **内存状态机与环形缓冲 (TaskRegistry)**:
  - 能够实时按 `task_id` 查询任务，能够获取所有活跃任务列表；
  - 已完成/失败的任务被收纳进容量为 50 的环形缓冲（`VecDeque`），支持历史追溯。
- [x] **广播事件总线 (Event Bus)**:
  - 任何任务状态变迁、进度更新与日志均向 `broadcast::Sender<EngineEvent>` 发射事件；
  - 订阅端（Receiver）能够实时收到完整的生命周期事件序列。
- [x] **人工审阅挂起与唤醒机制 (AwaitingReview & Approve)**:
  - 当元数据置信度存疑或无在线结果时，任务转入 `AwaitingReview` 并在调度器内异步休眠；
  - 外部调用 `registry.approve_task(task_id, override_meta)` 后，调度器被立即唤醒并采用修正后的元数据继续完成转码。
- [x] **测试全绿**: 工作空间 `cargo test --workspace` 必须通过，无编译错误与 Clippy 警告。
