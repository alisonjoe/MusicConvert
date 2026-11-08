# 6. 元数据抓取过程 UI 可视化与任务实时日志架构决策

日期: 2026-11-08

## 状态
已接受 (Accepted)

## 上下文 (Context)
在目前的音乐转换流程中，多源元数据拉取（MusicBrainz, NetEase, iTunes）、置信度评估、单曲音轨号推断及转码操作均在后台隐式执行。用户在 Web 控制台只能看到粗粒度的状态徽章（如“查询元数据”或“转码中”），无法直观获知：
1. 本地文件探测到了哪些信息；
2. 责任链中的每一个 Provider 请求了什么、是否超时、命中了哪个候选、评分是多少；
3. 为什么触发了人工审核或免审放行；
4. 单曲转码时的详细推断逻辑与耗时。
当发生网络风控、元数据不匹配或排队阻塞时，排查极其困难。因此需要将元数据抓取过程全面 UI 化，并提供结构化日志记录与实时流式推送。

## 决策 (Decision)
1. **领域模型扩展**：
   - 在 `music_convert::engine::task` 中新增 `TaskLogItem` 实体，包含 `timestamp`、`level`、`stage`（阶段标签）、`message`。
   - `BatchTask` 持有 `logs: Vec<TaskLogItem>` 集合，随任务内存状态生命周期持久保留，并在 `GET /api/tasks` 中全量下发。
2. **事件总线扩展**：
   - `EngineEvent` 新增 `TaskLogAppended { task_id: String, log: TaskLogItem }` 事件。
   - 通过 SSE `/api/events` 实时广播单条日志，前端无需全量轮询即可毫秒级动态追加。
3. **全生命周期日志注入**：
   - 在 `ConvertOrchestrator` 与 `ProviderChain` 中打桩结构化诊断日志，覆盖 `LocalProbe`、`MusicBrainz`、`NetEase`、`ITunes`、`Resolve`、`Transcode`、`Tagging` 各阶段。
4. **前端可视化**：
   - **卡片级**：在每个任务卡片提供折叠式仿终端控制台，展示该批次的抓取流程与详细流水账，附带彩色阶段徽章。
   - **全局级**：在顶部导航增加【🖥️ 实时控制台】全局抽屉，支持全局日志聚合、按级别过滤、一键清屏与自动滚动。

## 后果 (Consequences)
- **正面**：抓取与转换流程彻底透明化，用户可一眼定位元数据未命中的原因或网络故障点，大幅提升可观测性与排障效率。
- **负面**：单任务常驻内存增加少量字节（日志上限默认单任务 100 条），在内存中由 RingBuffer 控制上限，对系统整体开销极低。
