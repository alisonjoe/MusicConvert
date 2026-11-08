# 技术规约: 任务抓取过程 UI 可视化与实时日志 (task-logs-and-ui-observability)

## 1. 业务目标
将 `MusicConvert` 的元数据拉取与转换生命周期全面可视化，为前端提供端到端透明的诊断流水账，解决排查网络、匹配偏离与转换故障困难的问题。

## 2. 核心架构契约

### 2.1 实体模型契约 (`crates/music-convert/src/engine/task.rs`)
```rust
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TaskLogItem {
    pub timestamp: String, // HH:mm:ss 或 RFC3339
    pub level: String,     // INFO, WARN, ERROR, DEBUG
    pub stage: String,     // LocalProbe, MusicBrainz, NetEase, ITunes, Resolve, Transcode, Tagging
    pub message: String,
}
```
`BatchTask` 新增：
```rust
pub logs: Vec<TaskLogItem>,
```

### 2.2 事件广播契约 (`crates/music-convert/src/engine/event.rs`)
```rust
EngineEvent::TaskLogAppended {
    task_id: String,
    log: TaskLogItem,
}
```

### 2.3 注册表日志操作 (`crates/music-convert/src/engine/registry.rs`)
```rust
impl TaskRegistry {
    pub fn append_task_log(&mut self, task_id: &str, stage: &str, level: &str, message: &str);
}
```

### 2.4 Orchestrator 诊断日志注入阶段
1. `LocalProbe`：本地 CUE 解析或音频 Tag 探针结果（提取出的艺术家、专辑、文件数）。
2. `MetadataQuery`：发起外部元数据请求，记录查询参数。
3. `ProviderChain` 各提供商尝试与响应：
   - `MusicBrainz`：搜索结果、命中 release id、匹配到的 tracks 数。
   - `NetEase`：搜索候选列表、最高分匹配项、封面获取情况。
   - `ITunes`：专辑查验与年份对比。
4. `Resolve`：置信度评分计算与最终采纳元数据来源。
5. `Transcode`：单曲音轨推断、输出路径解析与 ffmpeg 转码启动/耗时。
6. `Tagging`：写入标签与缺失音轨检查结果。

### 2.5 前端交互规格 (`web/dist/index.html`)
- 卡片新增 `📜 抓取与转码日志 (N 条)` 折叠面板，展开呈现暗色风格终端，阶段以彩色标签高亮呈现。
- 顶部导航增加 `🖥️ 实时控制台` 抽屉，支持聚合查看当前所有流式日志，并提供级别过滤与自动滚屏功能。
- 监听 SSE 的 `TaskLogAppended` 事件，实时就地插入相应任务面板。
