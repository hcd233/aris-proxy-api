# 数据集导出复用 session 领域不新建 domain/dataset 聚合

## Status

accepted

## Context

训练数据集导出功能需要：按条件筛选会话、批量拉取消息/工具、转换为 ShareGPT 格式、流式输出 JSONL。架构上面临一个边界决策：是否为"数据集导出"新建独立的 `domain/dataset` 领域（聚合根 + repository + service），还是复用现有 `domain/session` 领域 + 在 `application/dataset` 层编排。

项目已有严格的 DDD 分层（domain → application → infrastructure），且已有 `application/audit`、`application/session` 等模块作为先例。

## Decision

复用 `domain/session` 领域，不新建 `domain/dataset` 聚合。在 `application/dataset` 新建 query handler 和 converter 编排导出逻辑。扩展 `SessionReadRepository` 新增 `ListSessionsForExport` 方法，复用已有的 `FindMessagesByIDs`/`FindToolsByIDs` 批量查询。

## Considered Options

1. **新建 `domain/dataset` 领域**（DatasetExport 聚合根 + repository + service）— 完整 DDD 分层，但导出记录没有复杂领域不变量（纯流式无持久化），引入完整新领域过重。
2. **复用 session 领域 + `application/dataset`**（选定）— 导出是 session 数据的只读消费，不需要独立聚合保护不变量。符合项目 ponytail 原则。
3. **最简 audit 模式复用**（仿 CronCallAudit 作为操作审计）— 但纯流式导出不持久化记录，连审计表都不需要。

## Consequences

- `application/dataset` 只含 query handler 和 converter，无 command handler、无聚合根。
- `SessionReadRepository` 新增 `ListSessionsForExport` 方法，签名与 `ListAllSessions`/`ListSessionsByOwnerNames` 类似但按 score 阈值过滤并返回导出所需的完整投影。
- 如果未来需要导出记录持久化（导出历史、可重复下载），可以再引入 `domain/dataset` 聚合，此时决策可被 ADR-NNNN 取代。
