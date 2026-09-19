# 0001: 抽离古典音乐元数据策略 (Deepen Classical Metadata Strategy)

## 状态 (Status)
已修正 (Amended) - 2026-09-19

## 背景 (Context)
在 `MusicConvert` 的核心文件 `audio/convert.go` 中，存在着大量专门针对古典音乐的元数据解析逻辑。
原计划拆分出独立的 `audio/classical` 子包，但在实际执行中发现由于 Go 语言包管理的限制，强行拆分会引发与通用文本清洗函数和正则表达式的深层循环依赖（Cyclic Dependency）。

## 决策 (Decision)
为了避免大范围破坏现有的正则表达式变量并保持重构的安全，我们采用**同包深层模块化 (In-package Deep Module)** 的策略：
1. 在 `audio` 目录内建立专属的 `classical_strategy.go` 和 `source_context.go` 文件。
2. 保持所有的相关函数在同一个包内，允许它们共享未导出的变量。
3. 但在物理文件和逻辑设计上，强制做到“责任分离”与高内聚，分别补充相应的脱机单元测试。

## 替代方案 (Discarded Alternatives)
- **硬切分为不同 Package**：会导致必须将大量的通用工具层再次独立出一个 `textutil` 等缓冲包，重构成本极大且容易引发 Bug。放弃。
- **维持现状**：代码过于臃肿，放弃。

## 影响 (Consequences)
- **正面**：成功让 `convert.go` 从 80KB 瘦身至 54KB，大幅提升局部性；同时避免了任何循环依赖，保护了现存所有的流行音乐处理逻辑。
