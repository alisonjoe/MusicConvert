# Coding Standards & Quality Guardrails (代码规范与质量护栏)

> **使用指南**：
> 1. 本文件放置于仓库 `docs/coding-standards.md` 或根目录 `CODING_STANDARDS.md`。
> 2. **本文件是 Reviewer Agent 的独立检查清单（`/code-review` 专用）**。编写代码的 Coder 专注于业务实现与单测，而只读的 Reviewer Agent 依据本文件逐项审计。
> 3. 任何在会话或 Code Review 中被人类纠正的错误，通过 `/retro` 自动追加到本文件中，杜绝同一错误发生第二次。

---

## 一、防 AI 幻觉与“AI 味”代码反模式 (Anti-AI Smells)

大模型在生成代码时最容易出现的固有毛病，Reviewer 应列为高优先级拦截项：

### 1. 静默吞异常 (Silent Error Swallowing)
- **现象**：`catch (e) { return null; }` 或 `catch (e) { console.log(e); }`，将致命异常降级为静默无声，导致后续逻辑出现不可预测的连环崩溃。
- **规范**：
  - 必须**保留原始堆栈**，要么显式向上抛出，要么将其包装为强类型业务错误（如 `Result.Err(CustomError)`）。
  - 严禁捕获异常后不处理直接返回假空值（`return false` / `return null` / `return {}`）。

### 2. 过度防御与僵尸检查 (Paranoid Defensive Programming)
- **现象**：在强类型语言已经保证非空（Non-null）或内部纯函数上下文中，反复插入冗余的 `if (!param) return;`。
- **规范**：
  - **在边界处防御，在内部信任契约**。仅在系统边界（API 请求入参、用户表单、环境变量读取、持久化数据反序列化）做严格校验；内部纯逻辑信任类型系统，杜绝无谓的 null 检查。

### 3. 臆想式过度抽象 (Speculative Generality)
- **现象**：需求只要求支持本地上传，AI 自作主张抽象出 `IStorageProvider`、`StorageFactory`、`PluginRegistrationEngine` 等 5 层架构。
- **规范**：
  - **Rule of Three（三次法则）**：在出现第 3 处确定性重复之前，优先选择内联（Inline）和扁平实现。
  - 拒绝为了“未来可能出现的扩展”提前预留钩子和空接口。

### 4. 僵尸代码与假注释 (Zombie & Echo Comments)
- **现象**：被注释掉的废弃代码段，或为每一行显而易见的代码写无意义的翻译式注释（如 `// set id to 1 \n user.id = 1`）。
- **规范**：
  - 严禁提交被注释掉的旧逻辑（Git 历史才是唯一的版本归档地）。
  - 注释只记录 **“为什么这么做（Why / Trade-off）”**，绝不复述 **“做了什么（What）”**。

---

## 二、架构与模块设计规范 (Deep Module Architecture)

基于 John Ousterhout《软件设计的哲学》与 Fowler 重构基线：

### 1. 追求“深模块”，消灭“浅模块” (Deep vs. Shallow Modules)
- **深模块**：接口极小极其简单，但背后隐藏了强大的复杂性（例如一个简单的 `readFile(path)` 屏蔽了整个 VFS 和缓冲区细节）。
- **浅模块（需打回）**：接口的复杂度和其实现几乎一模一样，只是做了一层无意义的透传（Middle Man）。
- **审查规则**：若一个类或函数只做了一层薄薄的参数转发，直接干掉，让调用方直接使用底层契约。

### 2. 避免散弹式手术与发散修改 (Shotgun Surgery & Divergent Change)
- **散弹式手术**：在 Diff 中，为了修改一个业务规则，散落修改了多个不同的文件。
  - *整改要求*：将这部分高频协同变动的逻辑内聚归档到同一个领域模块中。
- **发散修改**：一个文件同时因为多个完全不相干的需求而被频繁改动。
  - *整改要求*：按照业务正交性拆分为单一职责的小模块。

### 3. 单向依赖与无环依赖 (Dependency Rule)
- 领域核心层（Domain / Pure Core）严禁依赖基础设施层（HTTP / Database / CLI / UI）。
- 维持 **“纯净核心 + 外部命令式壳（Pure Core, Imperative Shell）”**：核心算法和状态机不带副作用，所有网络与 I/O 交互在边界壳层完成。

---

## 三、类型系统与防御规范 (Type Safety & Domain Modeling)

### 1. 严禁原始类型偏执 (Primitive Obsession)
- **现象**：在全工程到处传递原始的 `string`、`number` 来代表具有特殊语义的概念（如用户 ID、金额、手机号、状态机状态）。
- **规范**：
  - 为核心领域概念定义**标称类型（Nominal Type）**、**枚举（Enum）**或**值对象（Value Object）**（如 `type UserId = string & { readonly __brand: unique symbol }`，或 `class Money`）。
  - 杜绝布尔盲区（Boolean Blindness）：不要出现带有多个连续布尔参数的函数调用 `doTask(true, false, true)`，强制使用具名选项对象或枚举替代。

### 2. 杜绝危险类型逃逸 (No Type Escapes)
- 严禁出现未经安全守卫的全局逃逸（如 TypeScript 的 `any`、`as any`、`@ts-ignore`，或 Python 的无标注字典）。
- 必须使用类型守卫（Type Guard / Zod / Pydantic 等）在系统输入口将未知数据收窄（Narrowing）为强契约类型。

---

## 四、命名与统一领域语言 (Ubiquitous Language)

### 1. 拒绝神秘命名与模糊词 (Mysterious & Vague Names)
- **严禁使用的垃圾名字**：`data`, `info`, `item`, `res`, `manager`, `helper`, `process`, `handle`。
- **规范**：
  - 变量与函数名必须精确反映其业务意图或内部状态（例如用 `PendingInvoiceList` 替代 `invoiceData`，用 `calculateOverdueLateFee` 替代 `handleFee`）。
  - 名字越具体，代码注释的必要性就越低。

### 2. 统一领域术语 (Ubiquitous Language)
- 严格遵循项目根目录 `CONTEXT.md` 约定的术语表。同一个概念在前后端、数据库和命名中必须保持 100% 词汇一致。

---

## 五、单向门高危风险防护 (One-Way Doors)

涉及以下变更的文件属于“单向门（高危不可逆）”，Reviewer 必须强制在报告中高亮标记：
1. **数据库与数据模型**：表结构变更、数据迁移脚本、破坏性字段重命名；
2. **通信与外部契约**：Protobuf、OpenAPI/Swagger 接口、公共 SDK 导出方法；
3. **安全与认证凭证**：Token 加解密、权限鉴权中间件、密码哈希逻辑、绝对禁止硬编码任何敏感密钥与连接串。

---

## 六、测试与工程验证规范 (Test Quality)

### 1. 测试真实行为，不测试实现细节 (Behavior over Implementation)
- **测试只打在公开接缝（Public Seams）**：测试用例必须通过模块的公共接口、公开方法或端点进行验证。
- **严禁编写“镜像假测试”**：严禁把刚刚写的私有函数实现逻辑用断言照抄一遍。重构内部逻辑不破坏行为时，单测绝对不能挂。

### 2. 拒绝过度脆弱 Mock 与时间依赖 (No Flaky Tests)
- 仅对不可控的外部依赖（网络 I/O、第三方计费 API、系统时钟）进行 Mock；系统内部各层之间优先使用真实内存实例或 Fake 实现。
- 严禁在测试代码中写入硬编码的 `sleep(1000)` 等待；必须使用时间旅行工具（Mock Clock）或事件轮询门禁。

### 3. 单测必须包含边界与负向用例 (Negative Cases)
- 每个功能除了“正向流程（Happy Path）”，必须覆盖至少 2 个负向分支（空边界、非法输入、网络超时与降级）。

---

## 七、团队经验与防回归规则 (通过 `/retro` 动态追加)
<!-- 由 /retro 会话自动更新，记录团队特定的防踩坑规则 -->
- [Initial Guard] 新增功能必须在 `tests/` 下提供可自动化运行的回归测试。
