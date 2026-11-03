# AGENTS.md — 全局 AI 原生工程协作守则

> **适用范围**：本文件是本工作区内**所有** AI 智能体（Agent）的唯一最高准则（Single Source of Truth）。
> 同时适用于主对话智能体以及所有子智能体（`invoke_subagent`）。
>
> **核心原则**：工作区根目录 = 调度编排与上下文锚点；
> 真正的代码编写与修改，**必须严格限制在独立的 Git Worktree（工作树）内进行**。
> 项目特定配置（关联仓库、安全禁区、服务端口、构建测试命令）统一在 `agents.config.yaml` 中声明式定义。

---

## 1. 声明式项目结构与配置

本工作区采用**声明式工程配置**。在采取任何动作之前，必须先查看工作区根目录的 `agents.config.yaml`：

```
WORKSPACE/ (工作区根目录)
├─ agents.config.yaml          # 项目专属声明式配置（仓库关联、安全禁区、测试命令、文档同步）
├─ .agents/                    # 智能体定义目录（工作区私有，不提交到生产代码仓）
│  ├─ AGENTS.md                # 本全局通用工程守则
│  └─ agents/<name>/agent.md   # 专属角色定义（explorer 勘探者、reviewer 审查者、coder 编码者等）
├─ scripts/                    # 自动化脚本（Windows 使用 PowerShell，macOS/Linux 使用 Bash）
│  ├─ setup-workspace.sh/ps1   # Git Hooks 安装与环境初始化
│  ├─ agent-dispatch.sh/ps1    # 隔离工作树创建与任务派发
│  ├─ verify-worktree.sh/ps1   # L2 级工作树内部自测核验
│  └─ pre-merge-check.sh/ps1   # L3 级合并前主干门禁检查
├─ Invoke-Build.sh / .ps1      # 统一跨平台自动化调度总入口
├─ CONTEXT.md                  # 领域通用词汇表、业务实体边界与状态机定义
├─ docs/
│  ├─ CODEMAP.md               # 代码拓扑全景图与物理架构地图
│  ├─ adr/                     # 架构决策记录（0001-xxx.md）
│  └─ tests/                   # 预先编写的测试用例规约（TC-XXX.md）
└─ <repo-directories>/         # 实际目标项目代码库（支持单仓 Monorepo 或多仓 Multi-repo）
```

### 路径引用规范
在所有 Agent 的指令执行和工具调用中，**所有文件路径必须相对于工作区根目录**：
```text
✅ <repo-name>/src/core/services/
❌ src/core/services/           # 严禁此种相对路径，在多仓或子模块架构下会产生歧义
```

---

## 2. 核心安全禁区（单点故障防护）

在 `agents.config.yaml` 的 `security.forbidden_zones` 中定义的路径属于受保护的安全红线区域（例如：数据库迁移脚本、网络通信协议、核心加解密组件、底层公共库等）。

**安全禁区铁律**：
- Agent 仅允许**评估变更意图并分析改动影响半径（Blast Radius）**；绝对禁止未经人类批准直接擅自修改。
- 如果某项开发任务确实需要改动禁区文件：
  1. 立即停止代码写入操作。
  2. 向用户主动汇报：涉及的具体文件、拟修改内容、预期影响半径以及上下游兼容性影响。
  3. 停下并等待用户的明确授权。
  4. 获得确认后，提交信息中必须附带指定的豁免标签（例如：`[schema]`、`[proto]`、`[shared]`）。

---

## 3. Git 与 Worktree 物理隔离协议（强制执行）

> **铁律：Agent 绝对禁止直接修改 `master` / `main` 或受保护的发布分支！**

### 3.1 Worktree 工作树流转标准
每个开发任务必须运行在独立的 Git Worktree 中，保证主分支时刻干净且处于可发布状态：

1. **动手改代码前**，必须确认当前工作目录位于专属工作树内：
   - Windows 环境：`pwsh .\Invoke-Build.ps1 dispatch -Title "<task-slug>"`
   - macOS / Linux 环境：`./Invoke-Build.sh dispatch --title "<task-slug>"`
   - 路径格式示例：`<repo>/.worktrees/<task-slug>/...`
2. **限定修改范围**：仅允许修改当前任务分配的目标模块，严禁大范围跨模块漫游修改。
3. **工作树自验证**：在汇报完成前，必须在工作树内执行 L2 自验证流程。
4. **禁止擅自合并/推送**：严禁私自运行 `git merge`、`git push` 或 `git checkout master`，分支合并权归人类所有。
5. **交付汇报**：输出完整的改动 Diff 摘要、验证证据截图/日志以及需要人类决策的事项。
6. **主干审查合并**：由用户在 `master` 上审查改动并完成合并或清理工作树。

### 3.2 多会话物理隔离原则（一会话一工作树）
- **严禁多个并发活跃会话（Session）共用同一个物理 Worktree 目录**，避免未提交改动互相覆盖、单元测试竞争以及并发构建锁冲突。
- 派发工具会自动识别目录冲突并追加独立的会话后缀（如 `agent/<slug>-s2`）。

### 3.3 提交规范与精准暂存铁律
- **严禁全量暂存（Blanket Staging 绝不允许）**：
  - **绝对禁止执行** `git add .`、`git add -A` 或 `git commit -a` / `git commit -am`！
  - 必须实行**按文件清单的精准暂存**：只添加当前任务明确改动的文件，彻底杜绝把未验收的中间产物或其他会话的脏文件带入提交。
  - **推荐提交命令**（内置隔离保护与未跟踪脏文件告警）：
    - Windows：`pwsh .\Invoke-Build.ps1 commit -Files "path/file1","path/file2" -Message "<msg>"`
    - POSIX：`./Invoke-Build.sh commit -Files "path/file1,path/file2" -Message "<msg>"`
- 提交信息规范：`<模块名>: <动词开头的简明描述>`（例如：`auth: 支持刷新令牌自动轮换`）。
- 严禁任何形式的强推（Force-Push）。

---

## 4. 质量工程与严谨实践

### 4.1 分层测试驱动开发（Pragmatic TDD）
- **强制 TDD 领域（红-绿-重构循环）**：
  - 核心业务逻辑、计算引擎、数据解析器、协议序列化以及状态机流转。
  - *标准循环*：编写处于失败状态的单测 `*_test.*`（红灯）→ 编写最简实现代码使测试通过（绿灯）→ 在保持测试全绿的前提下做整洁重构（重构）。
- **契约优先与端到端验收领域（非 TDD）**：
  - UI 页面布局、视觉样式呈现、实时长连接推流等不适合堆叠脆弱 Mock 的模块。
  - 通过无头浏览器自动化、真机探索运行（如 ARTEMIS / Playwright）以及契约对比进行验收。

### 4.2 科学排错循环（`diagnosing-bugs`）
当单元测试失败、运行时发生崩溃或用户报告 Bug 时：
- **严禁凭空瞎猜或盲目试错（Shotgun Debugging）**。
- **严格遵循 4 步诊断闭环**：
  1. *复现（Reproduce）*：编写一个最小可复现脚本或失败单测，确保稳定红灯。
  2. *假说（Hypothesize）*：结合代码与堆栈日志，提出不超过 3 个可证伪的排查假说。
  3. *隔离（Isolate）*：针对性插入打桩断言或定向排错日志，逐个排除变量。
  4. *修复与防回归（Verify & Prevent）*：编写防回归测试用例，实施最小接缝修复，确认全绿后清除调试打桩代码。

---

## 5. 日常跨平台自动化命令（双轨适配）

> **操作系统感知铁律（OS Awareness Rule）**：
> - **Windows 环境**：统一使用 `pwsh .\Invoke-Build.ps1 <command>`。
> - **macOS / Linux 环境**：严格使用 `./Invoke-Build.sh <command>`（或 `bash scripts/*.sh`）。
> - **绝对禁令**：严禁在 macOS 或 Linux 终端中尝试执行 `pwsh` 或 `.ps1` 命令！

### Windows (PowerShell) 常用命令
```powershell
# 1. 任务派发：创建工作树并初始化任务清单
pwsh .\Invoke-Build.ps1 dispatch -Repo <repo> -Title "<任务简述>"

# 2. 精准提交：仅暂存指定目标文件，自动拦截无关脏文件
pwsh .\Invoke-Build.ps1 commit -Files "path/file1","path/file2" -Message "feat: 功能描述"

# 3. L2 级工作树内部自测核验（在工作树内执行）
pwsh .\Invoke-Build.ps1 verify

# 4. L3 级主干合并前门禁（在主干上执行验证）
pwsh .\Invoke-Build.ps1 check -Repo <repo> -Branch agent/<slug>

# 5. 工作树清理与回收
pwsh .\Invoke-Build.ps1 cleanup
pwsh .\Invoke-Build.ps1 cleanup -Worktree <name> -Force
```

### macOS / Linux (POSIX Bash) 常用命令
```bash
# 1. 任务派发：创建工作树并初始化任务清单
./Invoke-Build.sh dispatch --title "<任务简述>" [--repo <repo>]

# 2. 精准提交：仅暂存指定目标文件，自动拦截无关脏文件
./Invoke-Build.sh commit -Files "path/file1,path/file2" -Message "feat: 功能描述"

# 3. L2 级工作树内部自测核验（在工作树内执行）
./Invoke-Build.sh verify

# 4. L3 级主干合并前门禁（在主干上执行验证）
./Invoke-Build.sh check --branch agent/<slug> [--repo <repo>]

# 5. 工作树清理与回收
./Invoke-Build.sh cleanup
./Invoke-Build.sh cleanup --worktree <name> --force
```

---

## 6. 四大核心知识资产矩阵

| 资产文件 | 关注维度 | 核心目的 | 何时查阅 | 何时更新 |
| :--- | :--- | :--- | :--- | :--- |
| **`CONTEXT.md`** | **业务语义** | 统一领域通用语言、业务实体边界、状态机流转与反向术语说明 | 阶段 1 & 3（概念建模与命名） | 阶段 1（澄清需求）或当核心业务概念发生分化时 |
| **`docs/CODEMAP.md`** | **物理拓扑** | 模块职责边界、顶层目录结构、服务出入口、依赖流向与安全禁区 | 阶段 1 & 2（评估改动影响半径） | 阶段 5（当路由、通信协议或模块依赖拓扑变化时增量维护） |
| **`docs/adr/`** | **决策原因** | 不可逆技术决策、选型对比（为何选方案 A 弃方案 B）、特殊妥协方案依据 | 架构推演与技术评审时 | 阶段 1（确定架构决策）或引入重要技术变通时 |
| **`tests/**/TC-XXX.md`** | **验收标准** | 可量化测试方案、动态选择器定位、断言逻辑与完成标准（DoD） | 阶段 3（TDD 目标）& 阶段 4（质量核验） | 阶段 2（动代码前必须预先编写完成） |

---

## 7. 端到端五阶段工程交付生命周期

```text
1. 阶段 1：意图对齐与深度质询 (/grill-with-docs)
   - 彻底澄清模糊需求，严禁未想清楚就直接动手写代码；
   - 查阅 docs/CODEMAP.md 核实模块归属、依赖流向与禁区范围；
   - 将敲定的领域词汇沉淀到 CONTEXT.md，不可逆选型沉淀到 docs/adr/。

2. 阶段 2：规格拆解、测试先行与禁区核验 (/to-tickets)
   - 输出覆盖单元测试、集成测试与 UI/E2E 层的完整验证计划；
   - 在写任何业务代码之前，预先编写并落盘具体的测试规约（tests/**/TC-XXX.md）；
   - 将大任务切分为 15~30 分钟粒度的轻量任务（保持上下文 <30k tokens）；
   - 若改动涉及 agents.config.yaml 定义的禁区，必须先向用户申请审批。

3. 阶段 3：工作树物理隔离与实用编码
   - 开启独立的 Git Worktree（通过 Invoke-Build dispatch）；
   - 在核心业务层、解析器与状态机上严格落实 TDD（红-绿-重构）；
   - 若遇到意外崩溃：严格执行科学排错 4 步法，禁止盲目碰运气。

4. 阶段 4：双轨校验门禁 (L2 代码自测 + L2.5 动态验证 + L3 主干门禁)
   - L2 级工作树内部自测：执行 Invoke-Build verify；
   - L2.5 级动态验证：界面操作回放、接口协议探测或真机探索；
   - L3 级主干预合并检查：在主干执行 Invoke-Build check；
   - 提交人类用户审查 Diff，确认无误后合并主干并清理工作树。

5. 阶段 5：成果交付、知识同步与归档
   - 生成带测试输出和证据的 walkthrough.md；
   - 若架构或路由发生变动，增量刷新 docs/CODEMAP.md；
   - 将成果文档同步到企业知识库（若 agents.config.yaml 有配置）。
```


---

## 9. 智能体自治编排协议（Orchestrator Protocol）

在从单步指令（Conductor）跃迁至流水线编排（Orchestrator）体系中，主智能体充当**编排调度中枢**，子智能体（`explorer`, `coder`, `reviewer`）通过标准化的结构化交接文件（Handoff Artifacts）实现无缝信息传递与状态自愈：

```
                ┌──────────────────────────────────────┐
                │        Orchestrator (主调度中枢)       │
                └──────────────────┬───────────────────┘
                                   │
              1. 调度派发          │ 5. 自愈驱动 / 最终验收
          (./Invoke-Build.sh       │ (最多重试 3 次，
             orchestrate)          │  全绿后交付人类)
                                   ▼
┌──────────────────────────────────┴──────────────────────────────────┐
│                      Worktree 隔离执行流水线                         │
│                                                                     │
│  [Explorer 工位] ──HANDOFF_EXPLORER.md──> [Coder 实施与自测]        │
│                                                   │                 │
│                                            HANDOFF_CODER.md         │
│                                                   ▼                 │
│  [合流交付 / 汇报] <─── AUDIT_REPORT.md (PASS) ─── [Reviewer 门禁]   │
│                                  │                                  │
│                                  └─── (BLOCK / FAIL 自动打回重做) ───┘
└─────────────────────────────────────────────────────────────────────┘
```

### 9.1 工位交接协议契约（Handoff Contracts）
每个工位必须在工作树根目录输出确定的交付工件，严禁口头模糊交接：
1. **Explorer 交付物（`HANDOFF_EXPLORER.md`）**：
   - 包含：影响文件清单（Target Files）、架构拓扑影响、安全禁区核验结果（Forbidden Zones Check）、推荐测试策略。
2. **Coder 交付物（`HANDOFF_CODER.md`）**：
   - 包含：精准修改文件清单、TDD 红绿单测证据、L2 `verify` 执行输出日志。
3. **Reviewer 交付物（`AUDIT_REPORT.md`）**：
   - 包含：结论判定（`PASS` / `BLOCK` / `CAUTION`）、安全与架构合规项、未通过时具体可执行的整改建议（Actionable Feedback）。

### 9.2 自愈反馈环路（Self-Healing Retry Loop）
- 当 L2 `verify` 失败或 Reviewer 给出 `BLOCK` 时，Orchestrator **禁止直接放弃或转交人类**。
- Orchestrator 提取具体失败日志与审计意见，重新派发给 Coder 进行自愈修复（默认上限 3 轮）。
- 只有经过 3 轮自愈仍无法全绿，或触发核心禁区授权请求时，才向人类发起挂起求助。

---

## 10. 智能体行为红线（安全护栏）

### 必须严格执行（ALWAYS）
- ✅ 开始行动前，必须先阅读 `agents.config.yaml` 与 `AGENTS.md`。
- ✅ 自动感知当前操作系统（Windows vs macOS/Linux），严禁调用不匹配的终端命令。
- ✅ 严格在独立的 Git Worktree 中开展修改（一会话一工作树）。
- ✅ 坚持清单式精准暂存（`Invoke-Build commit -Files ...`），严防带入外部脏文件。
- ✅ 修改代码前必须查阅 `docs/CODEMAP.md` 评估影响面。
- ✅ 编写业务代码前，必须预先落盘测试用例规约（`TC-XXX.md`）。
- ✅ 对核心业务逻辑和状态机落实测试驱动开发（TDD）。
- ✅ 子智能体协同严格遵循 `HANDOFF_*.md` 结构化交接契约。
- ✅ 遇错执行科学排查（假说检验法），优先触发编排自愈循环。
- ✅ 必须通过双轨校验（L2+L3）全绿后，方可声明任务完成。
- ✅ 任务合并完成后，必须自动执行工单状态闭环（本地 ticket 文件标记 completed 并打勾 [x]；若接入 GitHub/GitLab 则联动关闭对应 issue）。
- ✅ 任务完成后，使用 `Invoke-Build cleanup` 安全回收工作树。

### 严厉禁止的行为（NEVER）
- ❌ 在 macOS 或 Linux 终端环境中输出 `pwsh` 或 `.ps1` 指令。
- ❌ 多个并发会话共享同一个物理工作树目录。
- ❌ 执行无脑全量暂存（`git add .`、`git add -A`、`git commit -a`）。
- ❌ 直接在 `master`、`main` 或任何受保护的发布分支上直接做改动。
- ❌ 未经用户明确授权和附带豁免标签私自修改核心安全禁区。
- ❌ 面对 Bug 缺乏证据链，凭空瞎猜乱试代码。
- ❌ 在缺少测试证据和 Handoff 契约的情况下私自流转或声称“完成”。
- ❌ 自动执行 `git merge`、`git push` 或删除未经核验的分支。
- ❌ 在单元测试未跑通、构建未全绿的情况下声称“任务完成”。
