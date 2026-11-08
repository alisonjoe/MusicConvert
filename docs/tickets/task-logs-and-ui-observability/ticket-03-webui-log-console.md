# Ticket-03: 前端任务卡片抓取日志抽屉与全局实时日志 Console

## 状态
- [x] 已完成 (Done)

## 目标
1. 在 `web/dist/index.html` 的任务卡片中新增折叠式日志面板，显示彩色阶段徽章（LocalProbe, MusicBrainz, NetEase, iTunes, Transcode, Tagging）。
2. 在顶部导航增加全局【🖥️ 实时日志】抽屉与终端面板，支持按日志级别过滤、一键清空与自动滚屏跟随。
3. 适配 SSE `TaskLogAppended` 事件，实时就地插入卡片终端与全局控制台。
4. 验证嵌入式静态资源与 SPA 渲染。
