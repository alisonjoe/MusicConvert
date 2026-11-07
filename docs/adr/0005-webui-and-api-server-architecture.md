# 5. WebUI Dashboard and Realtime Orchestration Architecture

Date: 2026-11-07

## Status

Accepted (via /wayfinder MISSION-WEBUI)

## Context

MusicConvert 已经完成了高性能纯 Rust 工作空间重构并移除了历史 Go 依赖。
当前程序作为后台 Daemon 运行，存在以下局限性：
1. **黑盒运行不可见**: 转码进度、多碟批量合并状态只能通过排查日志文件获知；
2. **缺乏人工纠错接缝**: 当多源元数据匹配置信度存疑或存在歧义时，当前逻辑只能静默采用默认值，无法由用户介入挑选候选或手动矫正；
3. **部署环境严苛**: 目标环境主要是群晖 Synology、TrueNAS 与 unRAID 等 NAS，必须保持单二进制零依赖分发，且长连接需兼容群晖内置 Nginx 反向代理。

## Decision

1. **交付形态**: 采用纯单二进制嵌入式交付（`rust-embed`），前端静态资源在构建期打包进 Rust 二进制文件；
2. **Web 服务框架**: 选型 Tokio 官方生态的 `axum`，保持极低内存开销与异步同构；
3. **实时传输协议**: 采用 Server-Sent Events (SSE) 负责服务端单向高频进度与日志流推送，搭配 REST API 负责客户端操作指令控制，确保穿透群晖反代；
4. **调度中枢状态机升级**:
   - 建立 **双层任务模型**：`BatchTask`（整专与专辑级元数据上下文）+ `TrackItem`（单轨切分与转码进度百分比）；
   - **人工审阅策略**：采用**模式 A（存疑挂起）**，高置信度全自动放行，仅在元数据歧义或低置信度时置入 `AwaitingReview`，基于 `tokio::sync::oneshot` 挂起与唤醒；
   - **事件总线与缓存**：基于 `tokio::sync::broadcast` 发射细粒度事件，结合内存环形缓冲区（RingBuffer，保留活跃任务 + 最近 50 条历史）实现客户端首屏极速加载；
5. **前端交互技术栈**: 采用 Vue 3 + Vite + Tailwind CSS，产物 gzip 压制在 150KB 内；
6. **鉴权模型**: 局域网默认免密，支持配置可选的 `webui.auth_token` Bearer 鉴权。

## Consequences

- 调度引擎（`ConvertOrchestrator`）从阻塞式线性处理跃迁为异步事件驱动状态机；
- 用户可通过 Web 浏览器（默认端口 `:8080`）获得现代化可视化仪表盘与转码监控；
- 用户可在网页端对存疑专辑进行人工挑选与元数据校验，极大降低错标率；
- 依然保持单一二进制分发，不增加任何 Node.js 运行时或额外静态文件夹部署负担。
