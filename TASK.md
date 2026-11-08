# Task: fix-rwlock-deadlock

- **Repo**: /Users/alisonjoe/WorkSpace/MusicConvert
- **Branch**: agent/fix-rwlock-deadlock-20261108
- **Base**: main
- **Created**: 2026-11-08 22:18:15

## Implementation Scope & Verification
- [x] 修复 `orchestrator.rs` 中 TaskRegistry 写锁未释放即调用 `log_task` 导致的非重入死锁
- [x] 修复 `ffprobe.rs` 中内嵌封面（Video 流）的 `title: cover.jpg` 覆盖容器曲目标题的 Bug
- [x] 补充单元测试验证内嵌封面流标签过滤
- [x] 通过 L2 `verify` 全量门禁校验 (2 passed / 0 failed)
