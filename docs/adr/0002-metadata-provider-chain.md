# 0002: 抽象元数据提供者责任链 (Metadata Provider Chain)

## 状态 (Status)
已采纳 (Accepted) - 2026-09-21

## 背景 (Context)
在将古典策略拆离后，`audio/convert.go` 的主干逻辑中依旧充斥着对于不同外网 API（MusicBrainz, Spotify, Netease, iTunes）的硬编码级联查询。
诸如 `shouldUseMusicBrainz`、`if config.SpotifyEnabled` 以及各类容错回退机制，导致主干逻辑臃肿、扩展性差且极其难以独立测试（Mock）。

## 决策 (Decision)
为了构建深层模块 (Deep Module) 并解耦主流程与外部网络依赖，我们决定：
1. 引入**责任链模式 (Chain of Responsibility)**，定义统一的 `MetadataProvider` 接口。
2. 将 MusicBrainz, Spotify, Netease 等不同数据源的调用及返回数据映射，分别封装成实现了该接口的独立 Provider 结构。
3. `convert.go` 仅保留对 `chain.Fetch(context)` 的极简调用，不再感知具体 API 的优先级与回退细节。

## 替代方案 (Discarded Alternatives)
- **维持现状 (Status Quo)**: 每次增加或调整数据源都需要修改高风险的 `convert.go` 主流程，加剧“散弹式修改”。放弃。

## 影响 (Consequences)
- **正面**: 大幅降低主流程复杂度，实现了“开闭原则”(Open-Closed Principle)，未来添加新的数据源（如 QQ 音乐）只需新增一个 Provider 并注册到责任链中即可。
- **负面**: 需要重构现有的外网请求和上下文组装逻辑，前期改造成本较高。
