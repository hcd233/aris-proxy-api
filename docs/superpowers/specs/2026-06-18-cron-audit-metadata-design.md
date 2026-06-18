# Cron 审计日志 Metadata 设计

## 概述

为 `cron_call_audits` 表新增 `metadata` 字段（JSONB），使每个定时任务在执行时写入本次执行的统计元数据，并在前端审计日志表格中以紧凑标签形式展示。

## 背景

当前 cron 审计日志记录 9 个字段（ID、CreatedAt、CronName、TraceID、StartedAt、EndedAt、DurationMs、Status、Message），缺少执行结果的量化信息。例如 ThinkExtractCron 执行后无法知道本次提取了多少条消息、扫描了多少条。

## 各任务 Metadata 字段定义

### SessionDeduplicateCron（每小时执行）
| 字段 | 类型 | 含义 |
|------|------|------|
| `deduped_sessions_count` | number | 本次被判定为冗余并删除的会话数 |
| `checked_sessions_count` | number | 本次扫描检查的会话总数 |

### SoftDeletePurgeCron（每周日凌晨 4 点）
| 字段 | 类型 | 含义 |
|------|------|------|
| `purged_messages_count` | number | 本次硬删除的消息数 |
| `purged_tools_count` | number | 本次硬删除的工具调用记录数 |
| `retention_days` | number | 配置的软删除保留天数 |

### ThinkExtractCron（每小时整点）
| 字段 | 类型 | 含义 |
|------|------|------|
| `extracted_messages_count` | number | 本次成功提取到思考内容的消息数 |
| `scanned_messages_count` | number | 本次扫描的消息总数 |

### BlockedHitSyncCron（每 5 分钟）
| 字段 | 类型 | 含义 |
|------|------|------|
| `synced_hits_count` | number | 本次同步的违禁词命中总数 |

## 涉及改动

### 1. 数据库 Migration

```sql
ALTER TABLE cron_call_audits ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}';
```

### 2. Model

`internal/infrastructure/database/model/cron.go` — `CronCallAudit` 结构体加 `Metadata` 字段：

```go
type CronCallAudit struct {
    // ... existing fields ...
    Metadata datatypes.JSON `gorm:"column:metadata;type:jsonb;default:'{}';not null" json:"metadata"`
}
```

### 3. Runner（审计日志创建入口）

`internal/cron/lock_runner.go` — 修改 `wrapCronFunc` 和 `saveCronCallAudit`：

- `cronFunc` 签名从 `func(ctx context.Context) error` 改为 `func(ctx context.Context) (map[string]any, error)`
- `saveCronCallAudit` 接收 `metadata map[string]any` 参数，序列化后存入 `Metadata` 字段
- `skipped` 和 `panic` 分支传 `nil`

### 4. 各 Cron 任务实现

4 个 cron 文件各自在成功执行后返回对应 metadata：

- `session_dedup.go` — 返回 `checked_sessions_count`、`deduped_sessions_count`
- `soft_delete_purge.go` — 返回 `purged_messages_count`、`purged_tools_count`、`retention_days`
- `think_extract.go` — 返回 `scanned_messages_count`、`extracted_messages_count`
- `blocked_hit_sync.go` — 返回 `synced_hits_count`

### 5. DTO

`internal/dto/cron.go` — `CronCallAuditItem` 加字段：

```go
type CronCallAuditItem struct {
    // ... existing fields ...
    Metadata map[string]any `json:"metadata"`
}
```

### 6. 前端类型

`web/src/lib/types.ts` — `CronCallAuditItem` 加：

```typescript
interface CronCallAuditItem {
  // ... existing fields ...
  metadata: Record<string, number>;
}
```

### 7. 前端页面

`web/src/app/(dashboard)/audit/cron/page.tsx` — 表格新增 "Metadata" 列：

- 各任务用不同模板渲染 label：`deduped_sessions_count` → "删除会话"、`scanned_messages_count` → "扫描" 等
- 格式：`标签: 数值 | 标签: 数值`
- 空 metadata（skip/panic 或无数据）显示 "-"

标签中文映射：

| 字段 | 标签 |
|------|------|
| `checked_sessions_count` | 检查会话 |
| `deduped_sessions_count` | 删除会话 |
| `purged_messages_count` | 删除消息 |
| `purged_tools_count` | 删除工具 |
| `retention_days` | 保留天数 |
| `scanned_messages_count` | 扫描消息 |
| `extracted_messages_count` | 提取消息 |
| `synced_hits_count` | 同步命中 |

## 前端效果示例

| Cron 名称 | ... | Metadata |
|-----------|-----|----------|
| ThinkExtractCron | ... | 扫描消息: 500 \| 提取消息: 120 |
| SessionDeduplicateCron | ... | 检查会话: 1500 \| 删除会话: 3 |
| SoftDeletePurgeCron | ... | 删除消息: 42 \| 删除工具: 8 \| 保留天数: 30 |
| BlockedHitSyncCron | ... | 同步命中: 156 |
