# 消息去重 reasoning_content 兼容 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消息内容（role + content）相同但 `reasoning_content` 不同的两条消息在存储层共享同一条记录，优先保留有推理内容的版本，并对存量数据做迁移修复。

**Architecture:** `ComputeMessageChecksum` 清除 `ReasoningContent` 使 checksum 统一为 content-only；`store_pool.go` 在去重后批量检查缺失 `reasoning_content` 的存量记录并补充；`AutoMigrate` 末尾追加三阶段数据迁移（刷新 checksum → 合并重复消息 → 清理）。

**Tech Stack:** Go 1.25.1 / GORM / PostgreSQL

**Spec:** `docs/superpowers/specs/2026-06-13-message-dedup-reasoning-design.md`

---

## File Structure

- Modify: `internal/common/vo/checksum.go` — `ComputeMessageChecksum` 清除 `ReasoningContent`
- Modify: `test/unit/message_checksum/checksum_test.go` — 新增 reasoning_content 被忽略的测试
- Modify: `internal/infrastructure/pool/store_pool.go` — 补充 reasoning_content 升级逻辑
- Modify: `internal/infrastructure/database/postgresql.go` — 追加三阶段数据迁移

---

## Task 0: 创建 worktree 与开发分支

**Files:**
- 工作区切换到 `.worktrees/feature-message-dedup-reasoning-2026-06-13`

- [ ] **Step 1：创建 worktree 并切换分支**

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api
git worktree add -b feature/message-dedup-reasoning-2026-06-13 \
  .worktrees/feature-message-dedup-reasoning-2026-06-13 master
```

---

## Task 1: 修改 ComputeMessageChecksum 清除 ReasoningContent

**Files:**
- Modify: `internal/common/vo/checksum.go:37-57`
- Test: `test/unit/message_checksum/checksum_test.go`

- [ ] **Step 1: 写 failing test**

```go
// reasoning_content should NOT affect checksum
func TestComputeMessageChecksum_ReasoningContentIgnored(t *testing.T) {
	t.Parallel()

	withReasoning := &vo.UnifiedMessage{
		Role:             "assistant",
		Content:          &vo.UnifiedContent{Text: "Hello, world!"},
		ReasoningContent: "step 1: think...",
	}
	withoutReasoning := &vo.UnifiedMessage{
		Role:    "assistant",
		Content: &vo.UnifiedContent{Text: "Hello, world!"},
	}

	cs1 := vo.ComputeMessageChecksum(withReasoning, nil)
	cs2 := vo.ComputeMessageChecksum(withoutReasoning, nil)

	t.Logf("with reasoning: %s, without: %s", cs1, cs2)

	if cs1 != cs2 {
		t.Errorf("ComputeMessageChecksum should ignore reasoning_content: got %s and %s", cs1, cs2)
	}
}

// Different content should still produce different checksums
func TestComputeMessageChecksum_DifferentContentStillDiffers(t *testing.T) {
	t.Parallel()

	msgA := &vo.UnifiedMessage{
		Role:             "assistant",
		Content:          &vo.UnifiedContent{Text: "Hello"},
		ReasoningContent: "thinking A",
	}
	msgB := &vo.UnifiedMessage{
		Role:             "assistant",
		Content:          &vo.UnifiedContent{Text: "World"},
		ReasoningContent: "thinking B",
	}

	csA := vo.ComputeMessageChecksum(msgA, nil)
	csB := vo.ComputeMessageChecksum(msgB, nil)

	if csA == csB {
		t.Errorf("ComputeMessageChecksum should produce different checksums for different content: both got %s", csA)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test -v -count=1 -run TestComputeMessageChecksum_ReasoningContentIgnored ./test/unit/message_checksum/`
Expected: FAIL (旧逻辑会产出不同的 checksum)

- [ ] **Step 3: 修改 ComputeMessageChecksum**

在 `checksum.go:38` 的 `normalized := *msg` 之后追加一行：

```go
func ComputeMessageChecksum(msg *UnifiedMessage, toolSchemas ToolSchemaMap) string {
	normalized := *msg
	normalized.ReasoningContent = ""
	// 其余不变...
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test -v -count=1 -run 'TestComputeMessageChecksum_ReasoningContent|TestComputeMessageChecksum_Different' ./test/unit/message_checksum/`
Expected: 全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/common/vo/checksum.go test/unit/message_checksum/checksum_test.go
git commit -m "fix: clear ReasoningContent in ComputeMessageChecksum for content-only dedup"
```

---

## Task 2: 存储时补充 reasoning_content

**Files:**
- Modify: `internal/infrastructure/pool/store_pool.go`
- Depends on: `internal/common/vo` (已导入), `dbmodel`, `time`

- [ ] **Step 1: 在 store_pool.go 中追加升级逻辑**

在 `runMessageStoreTask` 的 `deduplicateAndStoreMessages` 调用之后、tool 去重之前插入 reasoning_content 补充逻辑：

```go
err := db.Transaction(func(tx *gorm.DB) error {
	messageIDs, err := pm.deduplicateAndStoreMessages(tx, messages)
	if err != nil {
		log.Error("[StorePool] Failed to store messages", zap.Error(err))
		return err
	}

	// ── 补充 reasoning_content ──
	// 找出输入中有 reasoning_content 的消息 ID 列表
	var needsUpgradeIDs []uint
	msgByID := make(map[uint]*vo.UnifiedMessage)
	for i, m := range messages {
		if m.Message.ReasoningContent != "" {
			needsUpgradeIDs = append(needsUpgradeIDs, messageIDs[i])
			msgByID[messageIDs[i]] = m.Message
		}
	}
	if len(needsUpgradeIDs) > 0 {
		// 查询存量记录中哪些缺失 reasoning_content
		var missing []*dbmodel.Message
		if err := tx.Model(&dbmodel.Message{}).
			Where("id IN ? AND (message::jsonb->>'reasoning_content' IS NULL OR message::jsonb->>'reasoning_content' = '')", needsUpgradeIDs).
			Select("id").
			Find(&missing).Error; err != nil {
			return err
		}
		for _, mr := range missing {
			if msg, ok := msgByID[mr.ID]; ok {
				if err := tx.Model(&dbmodel.Message{ID: mr.ID}).
					Select("message", "updated_at").
					Updates(map[string]any{
						"message":    msg,
						"updated_at": time.Now().UTC(),
					}).Error; err != nil {
					return err
				}
			}
		}
		if len(missing) > 0 {
			log.Info("[StorePool] Upgraded reasoning_content for existing messages",
				zap.Int("count", len(missing)))
		}
	}
	// ── END reasoning_content 补充 ──
```

需要补充 import：`"time"`（检查是否已有，无则加）

- [ ] **Step 2: 确认编译通过**

Run: `make build`
Expected: 编译成功

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/pool/store_pool.go
git commit -m "feat: upgrade reasoning_content on stored messages after dedup"
```

---

## Task 3: 三阶段数据迁移

**Files:**
- Modify: `internal/infrastructure/database/postgresql.go`
- New import: `internal/common/vo`

- [ ] **Step 1: 在 AutoMigrate 末尾追加迁移入口**

```go
func AutoMigrate(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	if err := db.AutoMigrate(model.Models...); err != nil {
		return err
	}
	return migrateMessageChecksums(db.WithContext(ctx))
}
```

- [ ] **Step 2: 实现 Phase 1 — 刷新 checksum**

```go
// migrateMessageChecksums 三阶段数据迁移：
// Phase 1: 刷新有 reasoning_content 的消息 check_sum 为 content-only
// Phase 2: 合并 checksum 重复的消息，更新会话引用
func migrateMessageChecksums(db *gorm.DB) error {
	log := logger.Logger()
	const batchSize = 1000

	// ── Phase 1: 刷新 checksum ──
	log.Info("[Database] Phase 1: refreshing message checksums for reasoning_content")
	offset := 0
	for {
		var messages []*dbmodel.Message
		if err := db.Where("message::jsonb->>'reasoning_content' IS NOT NULL").
			Select("id, message").
			Order("id").
			Limit(batchSize).
			Offset(offset).
			Find(&messages).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "phase 1: select messages with reasoning")
		}
		if len(messages) == 0 {
			break
		}
		for _, m := range messages {
			newCS := vo.ComputeMessageChecksum(m.Message, nil)
			if err := db.Model(&dbmodel.Message{ID: m.ID}).
				Where("check_sum != ?", newCS).
				Update("check_sum", newCS).Error; err != nil {
				return ierr.Wrap(ierr.ErrDBUpdate, err, "phase 1: update checksum")
			}
		}
		offset += len(messages)
		log.Info("[Database] Phase 1 progress", zap.Int("processed", offset))
	}
	log.Info("[Database] Phase 1 complete")
```

- [ ] **Step 3: 实现 Phase 2 — 合并重复消息**

```go
	// ── Phase 2: 合并重复消息 ──
	log.Info("[Database] Phase 2: merging duplicate messages by content checksum")

	phase2Offset := 0
	for {
		type dupRow struct {
			CheckSum string `gorm:"column:check_sum"`
			IDs      string `gorm:"column:ids"` // JSON array from json_agg
		}
		var groups []dupRow
		if err := db.Raw(`
			SELECT check_sum, json_agg(id ORDER BY
				CASE WHEN message::jsonb->>'reasoning_content' IS NOT NULL THEN 0 ELSE 1 END,
				id DESC
			) AS ids
			FROM messages
			WHERE check_sum != ''
			GROUP BY check_sum
			HAVING count(*) > 1
			LIMIT ? OFFSET ?
		`, batchSize, phase2Offset).Scan(&groups).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "phase 2: find duplicate groups")
		}
		if len(groups) == 0 {
			break
		}
		for _, g := range groups {
			var ids []uint
			if err := sonic.UnmarshalString(g.IDs, &ids); err != nil {
				return ierr.Wrap(ierr.ErrDBQuery, err, "phase 2: unmarshal ids")
			}
			if len(ids) < 2 {
				continue
			}
			keepID := ids[0]
			for _, oldID := range ids[1:] {
				// 替换 sessions.message_ids
				if err := db.Exec(`
					UPDATE sessions SET message_ids = (
						SELECT COALESCE(jsonb_agg(
							CASE WHEN value = ?::jsonb THEN ?::jsonb ELSE value END
						), '[]'::jsonb)
						FROM jsonb_array_elements(COALESCE(message_ids::jsonb, '[]'::jsonb)) AS t(value)
					)
					WHERE message_ids::jsonb @> ?::jsonb
				`, oldID, keepID, oldID).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBUpdate, err, "phase 2: update session message_ids")
				}
				// 替换 sessions.questions
				if err := db.Exec(`
					UPDATE sessions SET questions = (
						SELECT COALESCE(jsonb_agg(
							CASE WHEN value = ?::jsonb THEN ?::jsonb ELSE value END
						), '[]'::jsonb)
						FROM jsonb_array_elements(COALESCE(questions::jsonb, '[]'::jsonb)) AS t(value)
					)
					WHERE questions IS NOT NULL AND questions::jsonb @> ?::jsonb
				`, oldID, keepID, oldID).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBUpdate, err, "phase 2: update session questions")
				}
				// 删除冗余消息
				if err := db.Delete(&dbmodel.Message{ID: oldID}).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBDelete, err, "phase 2: delete redundant message")
				}
			}
		}
		phase2Offset += len(groups)
		log.Info("[Database] Phase 2 progress", zap.Int("groups_processed", phase2Offset))
	}
	log.Info("[Database] Phase 2 complete")

	return nil
}
```

- [ ] **Step 4: 补充 import**

在 `postgresql.go` 的 import 块中添加：

```go
"github.com/hcd233/aris-proxy-api/internal/common/vo"
"github.com/bytedance/sonic"
```

- [ ] **Step 5: 确认编译通过**

Run: `make build`
Expected: 编译成功

- [ ] **Step 6: Commit**

```bash
git add internal/infrastructure/database/postgresql.go
git commit -m "feat: add data migration for message content-only checksum and dedup"
```

---

## Task 4: 全量测试验证

**Files:**
- 无改动，仅运行测试

- [ ] **Step 1: 运行 lint**

```bash
make lint
```
Expected: 全部通过

- [ ] **Step 2: 运行全量测试**

```bash
make test
```
Expected: 全部通过（注意新测试 `TestComputeMessageChecksum_ReasoningContentIgnored` 和 `TestComputeMessageChecksum_DifferentContentStillDiffers` 包含在 `./test/unit/message_checksum/` 中）

- [ ] **Step 3: 聚焦验证 checksum 测试**

```bash
go test -v -count=1 -run 'TestComputeMessageChecksum_' ./test/unit/message_checksum/
```
Expected: 全部 PASS

- [ ] **Step 4: Commit**

```bash
git commit --allow-empty -m "chore: verify all tests pass"
```
