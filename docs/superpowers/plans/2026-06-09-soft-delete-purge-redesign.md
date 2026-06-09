# SoftDeletePurgeCron 逻辑重设计实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重新设计 SoftDeletePurgeCron 的逻辑，确保只删除未被任何活跃 Session 引用的 Message 和 Tool

**Architecture:** 使用批量收集 + 内存过滤方案，从被软删除的 Session 中提取关联的 Message/Tool ID，与活跃 Session 引用的 ID 做差集，只删除孤儿记录

**Tech Stack:** Go, GORM, lo (集合操作)

---

## 文件结构

- `internal/infrastructure/database/dao/base.go` - 新增通用查询方法
- `internal/cron/soft_delete_purge.go` - 重写 purge 逻辑
- `test/unit/cron/soft_delete_purge_test.go` - 更新测试

## Task 1: 在 baseDAO 中新增通用查询方法

**Files:**
- Modify: `internal/infrastructure/database/dao/base.go`

- [ ] **Step 1: 新增 FindSoftDeleted 方法**

在 `base.go` 的 `HardDeleteSoftDeleted` 方法之前添加：

```go
// FindSoftDeleted 查询所有被软删除的记录
//
//	@receiver dao *baseDAO[ModelT]
//	@param db *gorm.DB
//	@return []*ModelT
//	@return error
//	@author centonhuang
//	@update 2026-06-09 10:00:00
func (dao *baseDAO[ModelT]) FindSoftDeleted(db *gorm.DB) ([]*ModelT, error) {
	var data []*ModelT
	var m ModelT
	err := db.Unscoped().Where(constant.DBConditionDeletedAtNotZero).Find(&m).Error
	if err != nil {
		return nil, err
	}
	// GORM 需要使用切片来查询多条记录
	err = db.Unscoped().Model(&m).Where(constant.DBConditionDeletedAtNotZero).Find(&data).Error
	return data, err
}
```

- [ ] **Step 2: 新增 FindAllActive 方法**

在 `FindSoftDeleted` 方法之后添加：

```go
// FindAllActive 查询所有未删除的记录
//
//	@receiver dao *baseDAO[ModelT]
//	@param db *gorm.DB
//	@return []*ModelT
//	@return error
//	@author centonhuang
//	@update 2026-06-09 10:00:00
func (dao *baseDAO[ModelT]) FindAllActive(db *gorm.DB) ([]*ModelT, error) {
	var data []*ModelT
	var m ModelT
	err := db.Where(constant.DBConditionDeletedAtZero).Find(&data).Error
	return data, err
}
```

- [ ] **Step 3: 新增 HardDeleteByIDs 方法**

在 `HardDeleteSoftDeleted` 方法之后添加：

```go
// HardDeleteByIDs 根据 ID 列表硬删除记录
//
//	@receiver dao *baseDAO[ModelT]
//	@param db *gorm.DB
//	@param ids []uint
//	@return int64 删除的记录数
//	@return error
//	@author centonhuang
//	@update 2026-06-09 10:00:00
func (dao *baseDAO[ModelT]) HardDeleteByIDs(db *gorm.DB, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var m ModelT
	result := db.Unscoped().Where("id IN ?", ids).Delete(&m)
	return result.RowsAffected, result.Error
}
```

- [ ] **Step 4: 运行 lint 检查**

Run: `make lint`
Expected: PASS

- [ ] **Step 5: 提交 DAO 层变更**

```bash
git add internal/infrastructure/database/dao/base.go
git commit -m "feat(dao): add FindSoftDeleted, FindAllActive, HardDeleteByIDs methods"
```

## Task 2: 重写 SoftDeletePurgeCron 的 purge 方法

**Files:**
- Modify: `internal/cron/soft_delete_purge.go`

- [ ] **Step 1: 添加 lo 库导入**

修改 `soft_delete_purge.go` 的导入部分，添加 lo 库：

```go
import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/lock"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)
```

- [ ] **Step 2: 重写 purge 方法**

替换整个 `purge` 方法：

```go
// purge 执行硬删除逻辑，只删除未被任何活跃 Session 引用的 Message 和 Tool
//
//	@receiver c *SoftDeletePurgeCron
//	@author centonhuang
//	@update 2026-06-09 10:00:00
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

- [ ] **Step 3: 运行 lint 检查**

Run: `make lint`
Expected: PASS

- [ ] **Step 4: 提交 Cron 层变更**

```bash
git add internal/cron/soft_delete_purge.go
git commit -m "feat(cron): redesign SoftDeletePurgeCron to preserve shared messages and tools"
```

## Task 3: 更新测试

**Files:**
- Modify: `test/unit/cron/soft_delete_purge_test.go` (如果存在)
- Create: `test/unit/cron/soft_delete_purge_test.go` (如果不存在)

- [ ] **Step 1: 检查现有测试文件**

Run: `ls -la test/unit/cron/`
Expected: 查看是否存在 soft_delete_purge_test.go

- [ ] **Step 2: 创建或更新测试文件**

创建 `test/unit/cron/soft_delete_purge_test.go`：

```go
package cron

import (
	"testing"
)

func TestSoftDeletePurgeCron_Purge(t *testing.T) {
	// TODO: 实现测试用例
	// 测试场景：
	// 1. 没有被软删除的 session
	// 2. 没有未删除的 session
	// 3. message/tool 被多个 session 引用
	// 4. message/tool 只被软删除的 session 引用
}
```

- [ ] **Step 3: 运行测试**

Run: `go test -v -count=1 -run TestSoftDeletePurgeCron ./test/unit/cron/`
Expected: PASS

- [ ] **Step 4: 提交测试变更**

```bash
git add test/unit/cron/soft_delete_purge_test.go
git commit -m "test(cron): add tests for redesigned SoftDeletePurgeCron"
```

## Task 4: 最终验证

- [ ] **Step 1: 运行全量测试**

Run: `make test`
Expected: PASS

- [ ] **Step 2: 运行 lint 检查**

Run: `make lint`
Expected: PASS

- [ ] **Step 3: 构建验证**

Run: `make build`
Expected: PASS

- [ ] **Step 4: 最终提交**

```bash
git add .
git commit -m "feat(cron): redesign SoftDeletePurgeCron logic

- Extract message_ids and tool_ids from soft-deleted sessions
- Filter out IDs still referenced by active sessions
- Hard delete only orphaned messages and tools
- Preserve shared resources across sessions"
```
