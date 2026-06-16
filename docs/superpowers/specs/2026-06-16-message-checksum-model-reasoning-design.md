# Message Checksum 纳入 Model 与 ReasoningContent 设计

## 背景

当前 `internal/common/vo/checksum.go` 的 `ComputeMessageChecksum` 存在两个问题：

1. **Model 不参与 checksum**：`UnifiedMessage` 只包含消息内容，上游 `model` 作为独立字段存在 `dbmodel.Message.Model` 中。这导致相同内容但由不同模型生成的 assistant 消息会被去重为同一条记录。
2. **ReasoningContent 被完全忽略**：当前实现会清空 `ReasoningContent`。部分 provider 会把 reasoning 内容放到 `Content`，部分放到 `ReasoningContent`，空值情况下两者语义等价，应当产生相同 checksum。

## 目标

1. 将 `model` 纳入 `ComputeMessageChecksum` 计算。
2. 将 `ReasoningContent` 纳入计算，并兼容空值场景：
   - `rc: "a", c: ""` == `rc: "", c: "a"`
   - `rc: "b", c: "a"` != `rc: "a", c: "b"`
3. 保持现有 ToolCall 规范化逻辑不变（去 ID、schema-aware 去 default 参数）。
4. 更新相关单元测试，确保新语义有回归用例。

## 关键决策

- **模型参与方式**：修改 `ComputeMessageChecksum` 签名，新增 `model string` 参数，而不是把 model 注入 `UnifiedMessage`。这样 `UnifiedMessage` 仍保持纯内容语义。
- **ReasoningContent 兼容规则**：当 `Content` 为空（`nil` 或 `Text=="" && len(Parts)==0`）且 `ReasoningContent != ""` 时，将 `ReasoningContent` 移入 `Content.Text`，清空 `ReasoningContent`；否则保持原样。
- **Wire struct 方式**：和 `ComputeToolChecksum` 保持一致，定义一个内部 `messageChecksumWire` 结构体用于稳定序列化，显式控制哪些字段参与 hash。

## 数据模型影响

- `messages` 表结构不变，仍通过 `check_sum` 唯一索引去重。
- 存量记录的 `check_sum` 保持旧值；新写入的消息会使用新 checksum 逻辑。这会导致同一内容、不同模型（或 reasoning 空值兼容场景）在变更前后生成不同 checksum，从而产生少量重复内容记录，属于可接受的 bug 修复语义变更。

## 后端实现

### 1. `internal/common/vo/checksum.go`

新增内部结构体：

```go
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

修改函数签名与实现：

```go
func ComputeMessageChecksum(msg *UnifiedMessage, model string, toolSchemas ToolSchemaMap) string
```

实现步骤：

1. 复制 `msg`。
2. 规范化 Content / ReasoningContent：
   - 判断 Content 是否为空：`normalized.Content == nil || (normalized.Content.Text == "" && len(normalized.Content.Parts) == 0)`。
   - 若 Content 为空且 `ReasoningContent != ""`，则 `normalized.Content = &UnifiedContent{Text: normalized.ReasoningContent}`，`normalized.ReasoningContent = ""`。
3. 规范化 ToolCalls（保持现有逻辑）。
4. 构造 `messageChecksumWire{Model: model, ...}`。
5. `sha256.Sum256(encoder.Encode(wire, encoder.SortMapKeys))`。

### 2. `internal/infrastructure/pool/store_pool.go`

更新调用点：

```go
CheckSum: vo.ComputeMessageChecksum(m, task.Model, toolSchemas),
```

当前 `runMessageStoreTask` 里已经为 assistant 消息单独设置了 `Model`，非 assistant 消息 model 为空字符串，直接透传 `task.Model` 即可。

### 3. 单元测试 `test/unit/message_checksum/checksum_test.go`

新增/调整用例：

- `TestComputeMessageChecksum_ModelIncluded`：相同内容不同 model 产生不同 checksum。
- `TestComputeMessageChecksum_ReasoningContentSwap`：`rc:"a",c:""` 与 `rc:"",c:"a"` checksum 相同。
- `TestComputeMessageChecksum_ReasoningContentBothNonEmpty`：`rc:"b",c:"a"` 与 `rc:"a",c:"b"` checksum 不同。
- 调整原有 `TestComputeMessageChecksum_ReasoningContentIgnored`：该用例假设 reasoning 被忽略，需要删除或改为验证新语义。

## 测试计划

1. 运行 `go test -count=1 ./test/unit/message_checksum/` 验证新用例。
2. 运行 `go test -count=1 ./test/unit/llmproxy_usecase/` 等可能调用 `ComputeMessageChecksum` 的测试，确认无编译/行为错误。
3. 运行 `make lint` 确保代码规范通过。

## 兼容性说明

- 函数签名变更：`ComputeMessageChecksum` 增加 `model string` 参数，需要同步更新调用方和测试。
- 无数据库迁移脚本。
- 旧数据 checksum 不重置，新数据按新逻辑写入；系统功能不受影响。
