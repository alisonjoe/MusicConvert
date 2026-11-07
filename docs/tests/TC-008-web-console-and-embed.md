# TC-008: 嵌入式 Web 控制台与全链路交互端到端验收规约

## 1. 目标
验证在 `MusicConvert` 中构建的 Web 控制台前端与 Rust Axum 后端的完整集成：
1. 前端提供现代化任务大盘：展示活跃任务、单轨转码实时进度条、环形历史完成记录；
2. 待审阅告警卡片与元数据修正抽屉：遇到低置信度（AwaitingReview）任务时，显示醒目待审状态，支持用户直接在线修正歌手名、专辑名、发行年份，并提交 Approve 恢复转码；
3. 单二进制静态嵌入（Single Binary Distribution）：通过 `rust-embed` 或静态托管将前端产物编译嵌入进 `MusicConvert` 二进制，用户通过浏览器直接访问根路径 `/` 即可载入完整控制台，免去外部反代或静态服务器依赖；
4. 全链路端到端闭环验证：从引擎发现存疑批次、挂起待审、SSE 实时推流、前端发起 API 决策，到调度器恢复并转码完成的全流程测试通过。

## 2. 验收标准 (Acceptance Criteria)

- [x] **前端大盘与待审抽屉实现 (`web/`)**:
  - 具备响应式布局（现代暗色/科技感设计，支持移动端/NAS 宽屏适配）；
  - 实时消费 `/api/events` (SSE)，无缝动态更新任务状态与音轨进度条；
  - 待审阅模态框/抽屉，表单包含 `override_artist`、`override_album`、`override_year`，点击确认调用 `/api/tasks/{id}/approve`。
- [x] **静态资源嵌入与 SPA 回退**:
  - Axum 支持在根路径 `/` 及静态资源路径直接 serve 前端 `index.html` 与静态 assets；
  - 非 API 请求自动回退至 `index.html`。
- [x] **自动化测试验收**:
  - 前端构建打包全绿 (`npm run build` / 纯内网离线自包含产物)；
  - Rust 集成测试验证 WebUI 静态资源请求响应 `200 OK`，`index.html` 具备正确 HTML 结构；
  - 模拟低置信度批次从挂起到审批放行全流程端到端自动化通过。
