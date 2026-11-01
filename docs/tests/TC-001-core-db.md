# 测试用例规范: TC-001-core-db

- **用例编号**: `TC-001`
- **目标功能**: `music-core` 核心配置解析、SQLite 数据库幂等读写、简繁转换与路径清洗
- **测试层级**: Unit & Integration
- **前置条件**:
  - Rust 编译环境就绪
  - 临时内存 SQLite 数据库 (`:memory:`) 或测试临时目录

---

## 测试步骤与断言标尺 (Steps & Assertions)

### Step 1: 配置解析与默认值测试
- **操作**: 从 YAML 文本中反序列化 `Config`，并测试部分字段缺省时的默认值回退。
- **预期结果**: `src_path`, `output_root`, `desc_type`（默认 flac）, `spotify_enabled` 正确加载。

### Step 2: SQLite 数据持久化与去重测试
- **操作**: 
  1. 初始化 `Database::open` 创建表 `processed_files`。
  2. 写入已处理记录 `mark_processed(path, hash)`。
  3. 查询 `is_processed(path, hash)`，再次重复插入同一记录。
- **预期结果**: 首次判断返回 `false`，写入后返回 `true`，重复插入不报错且保持唯一性。

### Step 3: 中文简繁转换与路径清洗断言
- **操作**:
  1. 繁体中文转简体中文：输入“遙遠的她”，转为“遥远的她”。
  2. Unicode 标点与控制字符清洗：归一化全角/半角标点，去除末尾空格与点号（如 `"Mono Inc. "` 清洗为 `"Mono Inc"`）。
  3. 过滤 8.3 别名与消歧后缀：`"Indecision (2)"` -> `"Indecision"`。
- **预期结果**: 所有字符串按预期清洗，路径符合 Windows/SMB 规范。

---

## 验收完成标准 (Definition of Done)
- [ ] `cargo test -p music-core` 全部通过。
- [ ] 覆盖空字符串、异常路径与极端边界。
