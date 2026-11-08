# Ticket-02: Orchestrator 与 Provider 全生命周期抓取诊断日志注入

## 状态
- [x] 已完成 (Done)

## 目标
1. 在 `ConvertOrchestrator` 中引入结构化日志打桩辅助方法 `self.log_task(task_id, stage, level, message).await`。
2. 在 `ProviderChain` 与 `orchestrator.rs` 中埋点记录完整抓取决策：
   - 本地探针识别（艺术家、专辑、曲目数）
   - MusicBrainz 查询与结果（命中/未命中、release id）
   - 网易云音乐搜索与候选匹配详情（候选专辑名、匹配分、封面获取）
   - iTunes 补充与年份校验
   - 置信度计算与采纳决议
   - 单曲音轨推断、输出路径生成与转码结果
3. 单元测试覆盖全生命周期打桩。
