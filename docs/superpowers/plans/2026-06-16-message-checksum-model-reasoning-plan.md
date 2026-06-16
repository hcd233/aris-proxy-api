# Message Checksum 纳入 Model 与 ReasoningContent 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 `ComputeMessageChecksum`，使上游 model 参与 checksum，ReasoningContent 与 Content 在空值场景下语义等价，并更新相关测试。

**Architecture:** 修改 `ComputeMessageChecksum` 签名新增 `model string` 参数；内部定义 `messageChecksumWire` 做稳定序列化；规范化 Content/ReasoningContent（Content 为空时把 ReasoningContent 移入 Content）；保持 ToolCall schema-aware 规范化不变。

**Tech Stack:** Go 1.25.1, bytedance/sonic, samber/lo, sha256

---

## 文件清单

- 修改：`internal/common/vo/checksum.go`
- 修改：`internal/infrastructure/pool/store_pool.go`
- 修改：`test/unit/message_checksum/checksum_test.go`

---

## Task 0: 创建隔离 worktree（如尚未创建）

**Files:**
- 工作目录：`.worktrees/message-checksum-model-reasoning-2026-06-16`

- [ ] **Step 1: 使用 using-git-worktrees skill 创建/切换到隔离 worktree**

调用 `superpowers:using-git-worktrees`，目标分支名：`bugfix/message-checksum-model-reasoning-2026-06-16`。

---

## Task 1: 修改 ComputeMessageChecksum 实现

**Files:**
- 修改：`internal/common/vo/checksum.go`

- [ ] **Step 1: 新增 messageChecksumWire 结构体**

在 `internal/common/vo/checksum.go` 中，`ComputeMessageChecksum` 函数之前新增：

```go
// messageChecksumWire 用于稳定 message checksum 序列化的内部结构体
//
//	@author centonhuang
//	@update 2026-06-16 10:00:00
type messageChecksumWire struct {
	Model            string             `json:"model"`
	Role             enum.Role          `json:"role"`
	Content          *UnifiedContent    `json:"content,omitempty"`
	ReasoningContent string             `json:"reasoning_content,omitempty"`
	Name             string             `json:"name,omitempty"`
	ToolCalls        []*UnifiedToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string             `json:"tool_call_id,omitempty"`
	Refusal          string             `json:"refusal,omitempty"`
}
```

- [ ] **Step 2: 修改 ComputeMessageChecksum 签名与实现**

将原函数：

```go
func ComputeMessageChecksum(msg *UnifiedMessage, toolSchemas ToolSchemaMap) string
```

改为：

```go
func ComputeMessageChecksum(msg *UnifiedMessage, model string, toolSchemas ToolSchemaMap) string
```

函数体替换为：

```go
func ComputeMessageChecksum(msg *UnifiedMessage, model string, toolSchemas ToolSchemaMap) string {
	normalized := *msg

	// 规范化 Content / ReasoningContent：
	// 当 Content 为空且 ReasoningContent 非空时，将 ReasoningContent 视为 Content。
	if isUnifiedContentEmpty(normalized.Content) && normalized.ReasoningContent != "" {
		normalized.Content = &UnifiedContent{Text: normalized.ReasoningContent}
		normalized.ReasoningContent = ""
	}

	if len(normalized.ToolCalls) > 0 {
		cleanedCalls := make([]*UnifiedToolCall, len(normalized.ToolCalls))
		for i, tc := range normalized.ToolCalls {
			var schema *JSONSchemaProperty
			if toolSchemas != nil {
				schema = toolSchemas[tc.Name]
			}
			cleanedCalls[i] = &UnifiedToolCall{
				Name:      tc.Name,
				Arguments: normalizeArgumentsWithSchema(tc.Arguments, schema),
			}
		}
		normalized.ToolCalls = cleanedCalls
	}

	wire := messageChecksumWire{
		Model:            model,
		Role:             normalized.Role,
		Content:          normalized.Content,
		ReasoningContent: normalized.ReasoningContent,
		Name:             normalized.Name,
		ToolCalls:        normalized.ToolCalls,
		ToolCallID:       normalized.ToolCallID,
		Refusal:          normalized.Refusal,
	}

	hash := sha256.Sum256(lo.Must1(encoder.Encode(wire, encoder.SortMapKeys)))
	return hex.EncodeToString(hash[:])
}
```

- [ ] **Step 3: 新增 isUnifiedContentEmpty 辅助函数**

在 `normalizeArgumentsWithSchema` 之前新增：

```go
// isUnifiedContentEmpty 判断 UnifiedContent 是否为空
//
//	@param content *UnifiedContent
//	@return bool
//	@author centonhuang
//	@update 2026-06-16 10:00:00
func isUnifiedContentEmpty(content *UnifiedContent) bool {
	if content == nil {
		return true
	}
	return content.Text == "" && len(content.Parts) == 0
}
```

- [ ] **Step 4: 编译检查**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/common/vo/
```

Expected: 无编译错误。

- [ ] **Step 5: Commit**

```bash
git add internal/common/vo/checksum.go
git commit -m "feat(vo): include model and reasoning_content in message checksum"
```

---

## Task 2: 更新调用方 store_pool.go

**Files:**
- 修改：`internal/infrastructure/pool/store_pool.go:55`

- [ ] **Step 1: 修改 ComputeMessageChecksum 调用**

将 `internal/infrastructure/pool/store_pool.go` 第 55 行：

```go
CheckSum: vo.ComputeMessageChecksum(m, toolSchemas),
```

改为：

```go
CheckSum: vo.ComputeMessageChecksum(m, task.Model, toolSchemas),
```

- [ ] **Step 2: 编译检查**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./internal/infrastructure/pool/
```

Expected: 无编译错误。

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/pool/store_pool.go
git commit -m "chore(pool): pass model to ComputeMessageChecksum"
```

---

## Task 3: 更新单元测试

**Files:**
- 修改：`test/unit/message_checksum/checksum_test.go`

- [ ] **Step 1: 全文件搜索并替换所有 ComputeMessageChecksum 调用签名**

将文件中所有：

```go
vo.ComputeMessageChecksum(msg, nil)
vo.ComputeMessageChecksum(msgA, nil)
vo.ComputeMessageChecksum(msgB, nil)
vo.ComputeMessageChecksum(msg, schemas)
vo.ComputeMessageChecksum(msgA, schemas)
vo.ComputeMessageChecksum(msgB, schemas)
vo.ComputeMessageChecksum(withReasoning, nil)
vo.ComputeMessageChecksum(withoutReasoning, nil)
vo.ComputeMessageChecksum(msgA, nil)
vo.ComputeMessageChecksum(msgB, nil)
```

统一改为传空 model 字符串以兼容现有断言：

```go
vo.ComputeMessageChecksum(msg, "", nil)
vo.ComputeMessageChecksum(msgA, "", nil)
vo.ComputeMessageChecksum(msgB, "", nil)
vo.ComputeMessageChecksum(msg, "", schemas)
vo.ComputeMessageChecksum(msgA, "", schemas)
vo.ComputeMessageChecksum(msgB, "", schemas)
vo.ComputeMessageChecksum(withReasoning, "", nil)
vo.ComputeMessageChecksum(withoutReasoning, "", nil)
vo.ComputeMessageChecksum(msgA, "", nil)
vo.ComputeMessageChecksum(msgB, "", nil)
```

- [ ] **Step 2: 重写 ReasoningContent 相关测试**

找到 `TestComputeMessageChecksum_ReasoningContentIgnored`（原假设 reasoning 被忽略），替换为 `TestComputeMessageChecksum_ReasoningContentSwap`：

```go
func TestComputeMessageChecksum_ReasoningContentSwap(t *testing.T) {
	msgA := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: ""},
		ReasoningContent: "a",
	}
	msgB := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: "a"},
		ReasoningContent: "",
	}

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA (rc=a, c=empty): %s", checksumA)
	t.Logf("checksumB (rc=empty, c=a): %s", checksumB)

	if checksumA != checksumB {
		t.Errorf("ComputeMessageChecksum should swap reasoning_content into empty content: got %s and %s", checksumA, checksumB)
	}
}
```

- [ ] **Step 3: 新增 BothNonEmpty 测试**

在 `TestComputeMessageChecksum_ReasoningContentSwap` 后新增：

```go
func TestComputeMessageChecksum_ReasoningContentBothNonEmpty(t *testing.T) {
	msgA := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: "a"},
		ReasoningContent: "b",
	}
	msgB := &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		Content:          &vo.UnifiedContent{Text: "b"},
		ReasoningContent: "a",
	}

	checksumA := vo.ComputeMessageChecksum(msgA, "", nil)
	checksumB := vo.ComputeMessageChecksum(msgB, "", nil)

	t.Logf("checksumA (rc=b, c=a): %s", checksumA)
	t.Logf("checksumB (rc=a, c=b): %s", checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum should produce different checksums when both content and reasoning_content are non-empty: both got %s", checksumA)
	}
}
```

- [ ] **Step 4: 新增 Model 参与测试**

在文件末尾新增：

```go
func TestComputeMessageChecksum_ModelIncluded(t *testing.T) {
	msg := &vo.UnifiedMessage{
		Role:    enum.RoleAssistant,
		Content: &vo.UnifiedContent{Text: "hello"},
	}

	checksumA := vo.ComputeMessageChecksum(msg, "gpt-4", nil)
	checksumB := vo.ComputeMessageChecksum(msg, "claude-3", nil)

	t.Logf("checksumA (model=gpt-4): %s", checksumA)
	t.Logf("checksumB (model=claude-3): %s", checksumB)

	if checksumA == checksumB {
		t.Errorf("ComputeMessageChecksum should include model: expected different checksums, both got %s", checksumA)
	}
}
```

- [ ] **Step 5: 运行 message_checksum 单测**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api && go test -v -count=1 ./test/unit/message_checksum/
```

Expected: 全部 PASS。

- [ ] **Step 6: Commit**

```bash
git add test/unit/message_checksum/checksum_test.go
git commit -m "test(vo): update message checksum tests for model and reasoning_content"
```

---

## Task 4: 回归验证

**Files:**
- 全局编译与测试

- [ ] **Step 1: 全局编译检查**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api && go build ./...
```

Expected: 无编译错误。

- [ ] **Step 2: 运行可能受影响的单元测试**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api && go test -count=1 ./test/unit/message_checksum/ ./test/unit/llmproxy_usecase/ ./test/unit/tool_checksum/
```

Expected: 全部 PASS。

- [ ] **Step 3: 运行 lint**

运行：

```bash
cd /Users/centonhuang/Desktop/code/aris-proxy-api && make lint
```

Expected: 全部通过。

- [ ] **Step 4: Commit（如无额外改动则跳过）**

---

## Self-Review Checklist

1. **Spec coverage:**
   - Model 参与 checksum → Task 1 Step 2、Task 3 Step 4。
   - ReasoningContent 空值兼容 → Task 1 Step 2、Task 3 Step 2。
   - Both non-empty 保持区分 → Task 3 Step 3。
   - ToolCall 规范化不变 → Task 1 Step 2 保留原有逻辑。
   - 调用方更新 → Task 2。
   - 测试更新 → Task 3。

2. **Placeholder scan:** 无 TBD/TODO/"implement later"/"fill in details"。

3. **Type consistency:**
   - `ComputeMessageChecksum` 签名统一为 `(msg *UnifiedMessage, model string, toolSchemas ToolSchemaMap) string`。
   - `messageChecksumWire` 字段与 `UnifiedMessage` 字段类型一致。
