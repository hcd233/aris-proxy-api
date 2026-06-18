# 消息去重 reasoning_content 兼容设计

## 设计概述

当前 `ComputeMessageChecksum` 将 `UnifiedMessage` 全部字段（含 `ReasoningContent`）纳入 SHA256 计算，
导致内容相同但推理过程不同的两条消息被分配不同的 checksum，无法在存储层去重。

本设计从 checksum 计算、存储升级、数据迁移三个维度解决此问题。

## 问题描述

1. **checksum 含推理内容**：`reasoning_content` 参与哈希计算，同一段文本内容因模型每次推理过程不同而 checksum 不同
2. **消息存储去重失效**：`deduplicateAndStoreMessages` 按 `check_sum` 列查找已存记录，不同的 checksum 意味着重复存储
3. **会话去重失效**：session 层面的 `SessionDeduplicateCron` 依赖 message ID 数组的连续子序列判断，重复消息 ID 的差异使算法认为非冗余（*注：该 cron 因 K8s 重启频繁实际从未执行过，但问题本身仍需解决*）
4. **优先保留推理内容**：当内容相同的一份消息有/无 `reasoning_content` 时，应保留有推理内容的版本

## 设计目标

1. 内容相同的消息（同 `role` + `content`）产生相同的 checksum，忽略 `reasoning_content` 差异
2. 存储时自动检测缺失 `reasoning_content` 的记录并补充
3. 提供向后兼容的数据迁移，修复存量数据的 checksum
4. 合并存量重复消息，更新会话引用

## 设计方案

### 1. checksum 计算变更

**位置**：`internal/common/vo/checksum.go`

将 `ComputeMessageChecksum` 中的 `ReasoningContent` 字段清空后再序列化：

```go
func ComputeMessageChecksum(msg *UnifiedMessage, toolSchemas ToolSchemaMap) string {
    normalized := *msg
    normalized.ReasoningContent = ""
    // 其余不变（ToolCalls ID 清理 + Argument normalize + SHA256）
}
```

影响范围：
- `store_pool.go:53` — 新消息写入时自动使用 content-only checksum
- 旧消息（reasoning 不为空）的 `check_sum` 需通过迁移修正

### 2. 存储时补充 reasoning_content

**位置**：`internal/infrastructure/pool/store_pool.go`

`runMessageStoreTask` 在 `deduplicateAndStoreMessages` 返回后，对新输入中有 `reasoning_content` 的消息做补充检查：

```
1. 收集所有输入消息 m 满足 m.ReasoningContent != ""
2. 对以上消息取返回的 ID 列表
3. 批量 SELECT ... WHERE id IN (...) AND message->>'reasoning_content' IS NULL
4. 对命中记录逐一 UPDATE message 列（补充 reasoning_content）+ updated_at
```

只查询"缺失 reasoning"的记录，保证 `O(K)` 条 SQL 而不是 `O(N)`。

### 3. 数据迁移

**位置**：`internal/infrastructure/database/postgresql.go` → `AutoMigrate` 末尾

#### Phase 1：刷新 checksum

逐批处理有 `reasoning_content` 的消息，重算为 content-only checksum：

```
SELECT id, message FROM messages
WHERE message->>'reasoning_content' IS NOT NULL
ORDER BY id LIMIT 1000
→ ComputeMessageChecksum(msg, nil)
→ UPDATE messages SET check_sum = ? WHERE id = ?
```

#### Phase 2：合并重复消息

**背景**：Phase 1 后，部分 checksum 出现多条记录（之前因 reasoning 不同而分开存储的同一内容消息）。

处理步骤：

```
1. 按 check_sum GROUP BY，找出 count > 1 的分组
   - 每组的 ids 按 [(has_reasoning ? 0 : 1), id DESC] 权重排序
   - ids[0] = 最优保留记录，ids[1:] = 冗余记录

2. 对每条冗余记录 oldID：
   a. 会话引用替换（message_ids + questions 两列）
   b. 删除 messages 记录

会话替换 SQL：
  UPDATE sessions SET message_ids = (
    SELECT COALESCE(jsonb_agg(
      CASE WHEN value = $1::jsonb THEN $2::jsonb ELSE value END
    ), '[]'::jsonb)
    FROM jsonb_array_elements(COALESCE(message_ids::jsonb, '[]'::jsonb)) AS t(value)
  )
  WHERE message_ids::jsonb @> $1::jsonb

  UPDATE sessions SET questions = (
    SELECT COALESCE(jsonb_agg(
      CASE WHEN value = $1::jsonb THEN $2::jsonb ELSE value END
    ), '[]'::jsonb)
    FROM jsonb_array_elements(COALESCE(questions::jsonb, '[]'::jsonb)) AS t(value)
  )
  WHERE questions IS NOT NULL AND questions::jsonb @> $1::jsonb
```

#### 迁移前提条件

- 幂等：可重复执行，第二次运行时无匹配
- 可中断续跑：分批 LIMIT/OFFSET，中间提交
- 耗时预估：O(冗余记录数) 次 UPDATE，典型场景百万级消息在数分钟内完成

### 4. 不受影响的功能

- **工具去重**（`tool` 表）：与 `reasoning_content` 无关
- **会话层去重 cron**（`SessionDeduplicateCron`）：本修复不直接启动它，但 Phase 2 合并后 message_ids 不再含冗余 ID，算法前提更干净
- **`message_repository.go` `BatchSaveDedup`**：当前为死代码，无需修改

## 数据流

```
写入前: msg = {role: assistant, content: "Hello", reasoning_content: "thinking..."}

旧流程:
  ComputeMessageChecksum → SHA256("assistant" + "Hello" + "thinking..." + ...)
  → check_sum = A_old
  → 匹配不到已有记录 → INSERT

新流程:
  ComputeMessageChecksum → SHA256("assistant" + "Hello" + "" + ...)
  → check_sum = A_new（与无 reasoning 的版相同）
  → 匹配到已有记录 → 复用 ID
  → 检测到已有记录缺失 reasoning_content → UPDATE message 列补充
```

## 错误处理

| 场景 | 行为 |
|------|------|
| 迁移中断 | 幂等重入，已处理的 check_sum 不再变更 |
| Phase 2 GROUP BY 后无冗余 | 跳过，无影响 |
| session 的 questions 为 NULL | `COALESCE(questions::jsonb, '[]'::jsonb)` 容错 |
| jsonb 替换无匹配 | WHERE 条件 `@>` 过滤，不会全表扫+写 |

## 测试策略

1. **单元测试**：`checksum_test.go` 新增用例，验证 `reasoning_content` 被忽略
2. **单元测试**：验证有/无 `reasoning_content` 的两条消息产生相同 checksum
3. **集成测试**：迁移脚本在测试数据库上执行，验证 Phase 2 合并后 message_ids 正确
