# 测试用例规范: TC-002-provider-chain

- **用例编号**: `TC-002`
- **目标功能**: `music-provider` 异步元数据责任链与数据源回退降级
- **测试层级**: Unit & Integration
- **前置条件**:
  - `music-core` 编译正常
  - HTTP Mock 服务（Wiremock 或内置 Mock Provider）

---

## 测试步骤与断言标尺 (Steps & Assertions)

### Step 1: Provider 责任链顺序降级测试
- **操作**: 
  1. 注册两个 Provider：Provider A（模拟网络失败/无结果）、Provider B（返回有效专辑及曲目列表）。
  2. 调用 `chain.fetch_release(&query)`。
- **预期结果**: Chain 自动跳过 Provider A 并成功获取 Provider B 的元数据，结果标记来源为 Provider B。

### Step 2: 中文本地化保护层断言
- **操作**: 
  1. 上下文具备可靠中文艺术家（如“周杰伦”）。
  2. 外部来源返回英文别名（如“Jay Chou”）。
  3. 执行本地化合并过滤 `localize_metadata(&query, &mut metadata)`。
- **预期结果**: 中文艺术家名得到强制保留，不被英文覆盖。

### Step 3: Classical 体裁与 No. 编号清洗
- **操作**: 
  1. 提供包含“Symphony N° 9”的古典专辑曲目。
  2. 经 Provider 映射归一化。
- **预期结果**: 曲目名自动规范为“Symphony No. 9”，保留作品编号。

---

## 验收完成标准 (Definition of Done)
- [ ] `cargo test -p music-provider` 全部通过。
- [ ] 覆盖全部 Provider 降级与兜底逻辑。
