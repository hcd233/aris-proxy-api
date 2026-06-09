# SoftDeletePurgeCron 逻辑重设计

- 日期: 2026-06-09
- 作者: centonhuang
- 状态: 已批准

## 背景

当前 `SoftDeletePurgeCron` 逻辑简单地硬删除所有已软删除的 Message、Session、Tool 记录。这会导致一个问题：当 Session 被软删除时，它引用的 Message 和 Tool 也会被硬删除，即使其他未删除的 Session 仍在使用它们。

## 设计目标

1. 从被软删除的 Sessions 中提取 message_ids 和 tool_ids 并去重
2. 扫描所有未被删除的 Sessions，过滤出未被使用的 message_ids 和 tool_ids
3. 硬删除被软删除的 Session，以及未被其他 Session 使用的 Message 和 Tool

## 数据模型

```
Session {
    MessageIDs []uint  // 关联的 Message ID 列表
    ToolIDs    []uint  // 关联的 Tool ID 列表
}

Message {
    ID uint
}

Tool {
    ID uint
}
```

## 方案选择

### 方案 A：批量收集 + 内存过滤（推荐）

**逻辑流程：**
1. 查询所有被软删除的 Session
2. 提取所有 message_ids 和 tool_ids 并去重
3. 查询所有未删除的 Session，收集所有引用的 message_ids 和 tool_ids
4. 计算差集：未被引用的 = 软删除session引用的 - 未删除session引用的
5. 批量硬删除这些 message/tool
6. 硬删除被软删除的 session

**优点：**
- 逻辑清晰，易于理解和维护
- 减少数据库查询次数
- 使用 Go 的 `lo` 库进行集合操作，代码简洁

**缺点：**
- 需要将所有 ID 加载到内存
- 对于大量数据可能有内存压力

### 方案 B：SQL 子查询

**逻辑流程：**
1. 使用 NOT EXISTS 子查询直接删除未被引用的 message/tool
2. 硬删除被软删除的 session

**优点：**
- 数据库层面完成，减少应用层内存使用
- 性能可能更好

**缺点：**
- SQL 语句复杂，难以维护
- 调试困难

### 选择：方案 A

对于当前数据规模，方案 A 的内存开销可以接受，且代码可读性和可维护性更好。

## 详细设计

### 核心逻辑

```go
func (c *SoftDeletePurgeCron) purge(ctx context.Context) {
    log := logger.WithCtx(ctx)
    db := c.db.WithContext(ctx)

    // 1. 查询所有被软删除的 session
    softDeletedSessions, err := c.sessionDAO.FindSoftDeleted(db)
    if err != nil {
        log.Error("[SoftDeletePurgeCron] Failed to find soft deleted sessions", zap.Error(err))
        return
    }

    if len(softDeletedSessions) == 0 {
        log.Info("[SoftDeletePurgeCron] No soft deleted sessions found")
        return
    }

    // 2. 从被软删除的 session 中提取 message_ids 和 tool_ids 并去重
    candidateMessageIDs := make([]uint, 0)
    candidateToolIDs := make([]uint, 0)
    for _, session := range softDeletedSessions {
        candidateMessageIDs = append(candidateMessageIDs, session.MessageIDs...)
        candidateToolIDs = append(candidateToolIDs, session.ToolIDs...)
    }
    candidateMessageIDs = lo.Uniq(candidateMessageIDs)
    candidateToolIDs = lo.Uniq(candidateToolIDs)

    // 3. 查询所有未删除的 session，收集引用的 message_ids 和 tool_ids
    activeSessions, err := c.sessionDAO.FindAllActive(db)
    if err != nil {
        log.Error("[SoftDeletePurgeCron] Failed to find active sessions", zap.Error(err))
        return
    }

    usedMessageIDs := make([]uint, 0)
    usedToolIDs := make([]uint, 0)
    for _, session := range activeSessions {
        usedMessageIDs = append(usedMessageIDs, session.MessageIDs...)
        usedToolIDs = append(usedToolIDs, session.ToolIDs...)
    }
    usedMessageIDs = lo.Uniq(usedMessageIDs)
    usedToolIDs = lo.Uniq(usedToolIDs)

    // 4. 计算差集：未被引用的 = 候选 - 已使用
    orphanMessageIDs := lo.Difference(candidateMessageIDs, usedMessageIDs)
    orphanToolIDs := lo.Difference(candidateToolIDs, usedToolIDs)

    // 5. 批量硬删除未被引用的 message 和 tool
    var msgCount, toolCount int64
    if len(orphanMessageIDs) > 0 {
        msgCount, err = c.messageDAO.HardDeleteByIDs(db, orphanMessageIDs)
        if err != nil {
            log.Error("[SoftDeletePurgeCron] Failed to purge messages", zap.Error(err))
            return
        }
    }

    if len(orphanToolIDs) > 0 {
        toolCount, err = c.toolDAO.HardDeleteByIDs(db, orphanToolIDs)
        if err != nil {
            log.Error("[SoftDeletePurgeCron] Failed to purge tools", zap.Error(err))
            return
        }
    }

    // 6. 硬删除被软删除的 session
    sessionCount, err := c.sessionDAO.HardDeleteSoftDeleted(db)
    if err != nil {
        log.Error("[SoftDeletePurgeCron] Failed to purge sessions", zap.Error(err))
        return
    }

    log.Info("[SoftDeletePurgeCron] Purge completed",
        zap.Int64("sessionsDeleted", sessionCount),
        zap.Int64("messagesDeleted", msgCount),
        zap.Int64("toolsDeleted", toolCount))
}
```

### DAO 层新增方法

需要在 DAO 层新增以下方法：

1. `FindSoftDeleted(db *gorm.DB) ([]*ModelT, error)` - 查询所有被软删除的记录
2. `FindAllActive(db *gorm.DB) ([]*ModelT, error)` - 查询所有未删除的记录
3. `HardDeleteByIDs(db *gorm.DB, ids []uint) (int64, error)` - 根据 ID 列表硬删除

这些方法可以放在 `baseDAO` 中作为通用方法。

### 数据库常量

已有常量（`internal/common/constant/database.go`）：
- `DBConditionDeletedAtZero` = "deleted_at = 0"
- `DBConditionDeletedAtNotZero` = "deleted_at != 0"

无需新增常量。

## 边界情况处理

1. **没有被软删除的 session**：直接返回，不做任何操作
2. **没有未删除的 session**：所有被软删除 session 引用的 message/tool 都是孤儿，应该删除
3. **message/tool 本身被软删除**：如果它没有被任何未删除 session 引用，应该硬删除
4. **并发安全**：通过分布式锁保证同一时间只有一个实例执行

## 测试策略

1. **单元测试**：测试 `purge` 方法的各种场景
2. **边界测试**：
   - 没有被软删除的 session
   - 没有未删除的 session
   - message/tool 被多个 session 引用
   - message/tool 只被软删除的 session 引用

## 影响范围

- `internal/cron/soft_delete_purge.go` - 主要修改
- `internal/infrastructure/database/dao/base.go` - 新增通用方法
- `internal/infrastructure/database/dao/session.go` - 可能需要新增方法
- `test/unit/cron/` - 更新测试
