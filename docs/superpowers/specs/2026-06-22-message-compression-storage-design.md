# 消息级压缩信息存储与展示 Design

> **日期**: 2026-06-22
> **目标**: 在数据库中存储每条 tool output 消息的压缩前后内容和触发的压缩策略，并在 Web session 详情页展示 diff 视图。

---

## 1. 背景与现状

### 1.1 当前压缩管线

压缩发生在**序列化后的请求体**上，在转发上游 LLM 之前执行：

```
DTO → marshal → bytes → CompressBody(bytes, protocol) → parse to map[string]any → modify map → re-marshal → forward
```

- `CompressBody` 按 `upstreamProtocol` 选择 Locator
- 三个 Locator（OpenAIChat / AnthropicMessages / OpenAIResponses）各自解析 body JSON 为 `map[string]any`，定位 tool output，调用 `Dispatcher.Compress`
- 三个 Compressor（SmartCrusher / LogCompressor / SearchCompressor）按内容类型执行确定性压缩
- 返回 `CompressionStats`（聚合统计：bytesBefore / bytesAfter / itemsCompressed / strategiesUsed）

### 1.2 当前消息存储

- `Message` 表存储 `*vo.UnifiedMessage`（JSON 序列化），包含 role / content / tool_calls / tool_call_id 等
- `Session` 表存储 `MessageIDs []uint`，关联消息
- 消息存储是**异步的**（通过 `MessageStoreTask` 提交到协程池）
- 存储的消息是**原始未压缩**的——压缩只作用于转发上游的 body，不影响存储

### 1.3 当前审计记录

`ModelCallAudit` 表已有压缩字段（`CompressionEnabled` / `CompressedTokens` / `CompressionStrategies`），但这是**请求级**的聚合统计，不是 per-message 的。

### 1.4 当前 Web 展示

- Session 详情页通过 `ChatMessage` 组件按角色分发渲染
- Tool output 消息（role=tool 或 role=user + tool_call_id）**不独立渲染**，而是内联到 `ToolCallCard` 中
- `buildToolResultsByID` 构建 `tool_call_id → result text` 的映射，传给 `ToolCallCard` 显示 Output
- `ToolCallCard` 展示工具名、参数、输出（可折叠）

### 1.5 核心问题

1. 压缩结果无法关联到单条存储的消息——per-item 详情（before/after/strategy）未记录
2. 压缩在 `map[string]any` 上操作，不够类型安全
3. 存在不必要的 marshal → parse → modify → re-marshal 循环

---

## 2. 方案选择

### 方案 A（已选）：Per-item 压缩结果 + tool_call_id 关联 + 直接在 DTO 上压缩

**核心思路**：
1. 压缩管线从操作序列化 bytes 改为**直接在 typed DTO 上 in-place 修改**
2. 记录 per-item 压缩结果（含 `ToolCallID` + `Input` + `Output` + `Strategy`）
3. Store 方法将压缩结果按 `tool_call_id` 回填到 `UnifiedMessage` 的新字段 `RawContent` 和 `CompressionStrategy`
4. 压缩后内容（after）自然透传——DTO 已被修改，转换 UnifiedMessage 时自动包含

**选择理由**：
- 类型安全（结构体字段访问，无 `map["key"]` 模式）
- 高效（消除 marshal → parse → re-marshal 循环）
- 确定性关联（tool_call_id）
- `UnifiedMessage` 加字段即可，无需 DB 迁移

---

## 3. 架构设计

### 3.1 新流程

```
Forward 方法:
  1. compression.CompressOpenAIChat(req.Body.Messages, dispatcher, minBytes) → stats
     // DTO 被 in-place 修改：tool output content = 压缩后内容
  2. body := proxyutil.MarshalUpstreamBody(req.Body) → forward
  3. 收到响应后:
     u.storeOpenAIChatMessages(ctx, req, aiMsg, model, usage, stats.Items)

Store 方法:
  1. unifiedMessages = convertRequestMessages(req)
     // DTO 已含压缩后内容，UnifiedMessage.Content 自动是 after
  2. compression.ApplyResultsToMessages(unifiedMessages, stats.Items)
     // 按 ToolCallID 匹配，设置 RawContent(before) + CompressionStrategy
  3. SubmitMessageStoreTask(messages: unifiedMessages)
     // MessageStoreTask 无需改动

Web Session 详情页:
  1. API 返回 MessageItem.message 含 raw_content + compression_strategy
  2. buildToolResultsByID 提取压缩信息到 ToolResultInfo
  3. ToolCallCard 默认显示压缩后内容，底部显示策略 + "查看原始内容"
  4. 点击展开 compression-diff 组件，支持单栏/双栏 toggle
```

### 3.2 数据流图

```
┌─────────────┐     in-place modify      ┌─────────────┐
│  Request DTO │ ──────────────────────→ │ Modified DTO │
│ (OpenAI/     │   CompressOpenAIChat()  │ (compressed  │
│  Anthropic/  │   returns stats.Items   │  tool output) │
│  Responses)  │                         └──────┬──────┘
└─────────────┘                                │
                                               │ marshal
                                               ▼
                                        ┌─────────────┐
                                        │   Body bytes │ → forward upstream
                                        └─────────────┘
                                               │
                                               │ after response
                                               ▼
┌─────────────────────────────────────────────────────────┐
│  Store Method                                            │
│                                                          │
│  1. convertRequestMessages(req) → []*UnifiedMessage      │
│     (content = compressed after, from modified DTO)      │
│                                                          │
│  2. ApplyResultsToMessages(msgs, stats.Items)            │
│     (set RawContent = before, CompressionStrategy)       │
│                                                          │
│  3. SubmitMessageStoreTask(msgs)                         │
└─────────────────────────────────────────────────────────┘
                               │
                               ▼
                    ┌─────────────────┐
                    │  Message table   │
                    │  message JSON:   │
                    │  {               │
                    │    role: "tool", │
                    │    content: {    │
                    │      text: "..." │ ← after (compressed)
                    │    },            │
                    │    tool_call_id, │
                    │    raw_content:  │ ← before (original)
                    │      "...",      │
                    │    compression_  │
                    │      strategy:   │
                    │      "smart_..." │
                    │  }               │
                    └─────────────────┘
                               │
                               ▼
                    ┌─────────────────┐
                    │  API response    │
                    │  MessageItem     │
                    │  .message        │ ← *vo.UnifiedMessage
                    │  (auto includes  │   raw_content +
                    │   new fields)    │   compression_strategy
                    └─────────────────┘
                               │
                               ▼
                    ┌─────────────────┐
                    │  Web             │
                    │  ToolCallCard    │
                    │  + CompressionDiff│
                    └─────────────────┘
```

---

## 4. 数据模型变更

### 4.1 `vo.UnifiedMessage` 扩展

文件: `internal/common/vo/unified_message.go`

```go
type UnifiedMessage struct {
    Role             enum.Role          `json:"role" doc:"消息角色"`
    Content          *UnifiedContent    `json:"content,omitempty" doc:"消息内容"`
    ReasoningContent string             `json:"reasoning_content,omitempty" doc:"推理/思考内容"`
    Name             string             `json:"name,omitempty" doc:"参与者名称"`
    ToolCalls        []*UnifiedToolCall `json:"tool_calls,omitempty" doc:"工具调用列表"`
    ToolCallID       string             `json:"tool_call_id,omitempty" doc:"工具调用ID(工具结果消息)"`
    Refusal          string             `json:"refusal,omitempty" doc:"拒绝消息"`

    // NEW: 压缩相关，仅当 tool output 被压缩时设置
    RawContent          *string `json:"raw_content,omitempty" doc:"压缩前原始内容"`
    CompressionStrategy string  `json:"compression_strategy,omitempty" doc:"压缩策略"`
}
```

- `Content` 存压缩后内容（after）← 现有字段，存储时已是压缩后内容
- `RawContent` 存压缩前内容（before）← 新字段，`omitempty`，未压缩时为 nil
- `CompressionStrategy` 存策略名 ← 新字段，`omitempty`，未压缩时为空
- **无需 DB 迁移**：字段存储在现有的 `message` JSON 列中
- **向后兼容**：`omitempty` 确保旧数据不受影响

### 4.2 `ItemCompressionResult` 扩展

文件: `internal/application/llmproxy/compression/result.go`

```go
type ItemCompressionResult struct {
    ToolCallID  string  // NEW: 关联到存储消息的 tool_call_id
    Input       string  // NEW: 压缩前原始内容
    Output      string  // 压缩后内容（或跳过/失败时的原始内容）
    Strategy    string
    Applied     bool
    BytesBefore int
    BytesAfter  int
}
```

### 4.3 `CompressionStats` 扩展

文件: `internal/application/llmproxy/compression/result.go`

```go
type CompressionStats struct {
    BytesBefore     int
    BytesAfter      int
    ItemsCompressed int
    ItemsSkipped    int
    StrategiesUsed  []string
    Items           []ItemCompressionResult  // NEW: per-item 详情列表
}
```

`addItem` 方法：当 `Applied == true` 时将 result（含 `ToolCallID` + `Input`）追加到 `Items`。

### 4.4 校验和影响

`ComputeMessageChecksum` 序列化整个 `UnifiedMessage` 计算 hash。新增的 `RawContent` 和 `CompressionStrategy`（`omitempty`）参与计算：
- 相同压缩内容 + 相同原始内容 + 相同策略 → 相同 checksum → 正常去重
- 压缩 vs 未压缩 → 不同 checksum → 分别存储（正确行为）

---

## 5. 压缩管线变更

### 5.1 架构变更

**Before**:
```
DTO → marshal → bytes → CompressBody(bytes, protocol) → parse to map → modify → re-marshal → forward
```

**After**:
```
DTO → CompressDTO(DTO, protocol) → 修改 DTO in-place → marshal → forward
```

### 5.2 接口变更

**删除**：
- `ToolOutputLocator` 接口（`locator.go`）
- `SelectLocator` 函数（`locator.go`）
- `CompressBody` 函数（`locator.go`）

**新增**：三个协议特定的函数，接收 typed DTO，in-place 修改，返回 stats

```go
// locator_openai.go
func CompressOpenAIChat(messages []*dto.OpenAIChatCompletionMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats

// locator_anthropic.go
func CompressAnthropicMessages(messages []*dto.AnthropicMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats

// locator_responses.go
func CompressOpenAIResponses(items []*dto.ResponseInputItem, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats
```

### 5.3 各 Locator 逻辑

**OpenAIChatLocator** — 遍历 `messages`，找 `role=tool`，压缩 `Content.Text`：

```go
func CompressOpenAIChat(messages []*dto.OpenAIChatCompletionMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats {
    stats := CompressionStats{}
    for _, msg := range messages {
        if msg.Role != enum.RoleTool || msg.Content == nil {
            continue
        }
        content := msg.Content.Text
        if len(content) < minToolOutputBytes {
            stats.addItem(skippedResult(content))
            continue
        }
        result := dispatcher.Compress(content)
        result.ToolCallID = lo.FromPtr(msg.ToolCallID)
        result.Input = content
        stats.addItem(result)
        if result.Applied {
            msg.Content.Text = result.Output
            msg.Content.Parts = nil
        }
    }
    return stats
}
```

**AnthropicMessagesLocator** — 遍历 `messages[].Content.Blocks`，找 `type=tool_result`，压缩 `Content.Text`（或从 Blocks 提取合并文本）：

```go
func CompressAnthropicMessages(messages []*dto.AnthropicMessageParam, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats {
    stats := CompressionStats{}
    for _, msg := range messages {
        if msg.Content == nil { continue }
        for _, block := range msg.Content.Blocks {
            if block.Type != "tool_result" || block.Content == nil { continue }
            content := extractAnthropicToolResultText(block.Content)
            if len(content) < minToolOutputBytes {
                stats.addItem(skippedResult(content))
                continue
            }
            result := dispatcher.Compress(content)
            result.ToolCallID = lo.FromPtr(block.ToolUseID)
            result.Input = content
            stats.addItem(result)
            if result.Applied {
                block.Content.Text = result.Output
                block.Content.Blocks = nil
            }
        }
    }
    return stats
}
```

`extractAnthropicToolResultText`：若 `Content.Text` 非空则直接返回；若 `Content.Blocks` 非空则提取所有 text block 的 text 合并返回。

**OpenAIResponsesLocator** — 遍历 `items`，找 `type=function_call_output`，压缩 `Output.Text`：

```go
func CompressOpenAIResponses(items []*dto.ResponseInputItem, dispatcher *Dispatcher, minToolOutputBytes int) CompressionStats {
    stats := CompressionStats{}
    for _, item := range items {
        if lo.FromPtr(item.Type) != "function_call_output" || item.Output == nil { continue }
        output := item.Output.Text
        if len(output) < minToolOutputBytes {
            stats.addItem(skippedResult(output))
            continue
        }
        result := dispatcher.Compress(output)
        result.ToolCallID = lo.FromPtr(item.CallID)
        result.Input = output
        stats.addItem(result)
        if result.Applied {
            item.Output.Text = result.Output
            item.Output.FunctionOutput = nil
        }
    }
    return stats
}
```

### 5.4 依赖变更

- `compression` 包新增 `import internal/dto` — 无循环依赖（DTO 是叶子包，`compression` 已通过 `util` 间接依赖 `dto`）
- Compressors（SmartCrusher / LogCompressor / SearchCompressor / Passthrough）**不变**
- `Dispatcher` **不变**

### 5.5 兼容路由处理

7 条转发路径各自在 marshal 前调用对应协议的压缩函数。对于兼容路由（如 OpenAI Chat → Anthropic），转换后的 DTO 已是目标协议类型，直接调用对应压缩函数。

---

## 6. Usecase 调用变更

### 6.1 `compressBodyIfNeeded` 重构

**Before**:
```go
func (u *openAIUseCase) compressBodyIfNeeded(ctx context.Context, body []byte, upstreamProtocol enum.ProtocolType) ([]byte, *compression.CompressionStats) {
    if !config.CompressionEnabled || u.dispatcher == nil || len(body) < config.CompressionMinBodyBytes {
        return body, nil
    }
    newBody, stats := compression.CompressBody(body, upstreamProtocol, u.dispatcher, config.CompressionMinToolOutputBytes)
    // ...
    return newBody, stats
}
```

**After**: 压缩在 DTO 上操作，不再接收/返回 bytes。每个 forward 方法在 marshal 前直接调用对应协议的压缩函数。

```go
// 在 forwardChatNative 中：
var compStats compression.CompressionStats
if config.CompressionEnabled && u.dispatcher != nil {
    compStats = compression.CompressOpenAIChat(req.Body.Messages, u.dispatcher, config.CompressionMinToolOutputBytes)
}
body := proxyutil.MarshalUpstreamBody(req.Body)  // 序列化已修改的 DTO
// forward body...
// after response:
u.storeOpenAIChatMessages(ctx, req, aiMsg, upstreamModel, usage, compStats.Items)
```

### 6.2 7 条转发路径

| UseCase | 方法 | 压缩函数 | Store 方法 |
|---------|------|---------|-----------|
| OpenAI | `forwardChatNative` | `CompressOpenAIChat` | `storeOpenAIChatMessages` |
| OpenAI | `forwardChatViaAnthropic` | `CompressAnthropicMessages` | `storeOpenAIChatMessages` |
| OpenAI | `forwardResponseNative` | `CompressOpenAIResponses` | `storeOpenAIResponseMessages` |
| OpenAI | `forwardResponseViaChat` | `CompressOpenAIChat` | `storeOpenAIResponseMessages` |
| OpenAI | `forwardResponseViaAnthropic` | `CompressAnthropicMessages` | `storeOpenAIResponseMessages` |
| Anthropic | `forwardMessageNative` | `CompressAnthropicMessages` | `storeAnthropicMessages` |
| Anthropic | `forwardMessageViaChat` | `CompressOpenAIChat` | `storeAnthropicMessages` |

每条路径在 marshal 前调用压缩函数，在 store 时传入 `compStats.Items`。

### 6.3 Body size 检查

**决定**：移除 `CompressionMinBodyBytes` 检查，仅保留 `CompressionMinToolOutputBytes`（per-item 检查已在 Locator 中）。body-level 检查在 bytes 方案中有意义（避免小 body 的压缩开销），但在 DTO 方案中，压缩是 per-message 的，小 body 自然没有大的 tool output，不会触发压缩。同时移除 `config.go` 中对应的配置项和 `env/api.env` 中的环境变量。

---

## 7. 消息存储流程

### 7.1 Store 方法变更

每个 store 方法新增 `compResults []compression.ItemCompressionResult` 参数：

```go
// Before
func (u *openAIUseCase) storeOpenAIChatMessages(ctx, req, aiMsg, upstreamModel, usage)

// After
func (u *openAIUseCase) storeOpenAIChatMessages(ctx, req, aiMsg, upstreamModel, usage, compResults []compression.ItemCompressionResult)
```

### 7.2 `ApplyResultsToMessages` 函数

新增 `internal/application/llmproxy/compression/apply.go`：

```go
func ApplyResultsToMessages(messages []*vo.UnifiedMessage, results []ItemCompressionResult) {
    if len(results) == 0 { return }
    resultMap := lo.SliceToMap(results, func(r ItemCompressionResult) (string, ItemCompressionResult) {
        return r.ToolCallID, r
    })
    for _, msg := range messages {
        if msg.ToolCallID == "" { continue }
        if result, ok := resultMap[msg.ToolCallID]; ok && result.Applied {
            msg.RawContent = &result.Input
            msg.CompressionStrategy = result.Strategy
        }
    }
}
```

### 7.3 Store 方法内部流程

```go
func (u *openAIUseCase) storeOpenAIChatMessages(ctx, req, aiMsg, upstreamModel, usage, compResults) {
    unifiedMessages, unifiedTools, inputTokens, outputTokens, err := u.convertRequestMessages(ctx, req)
    if err != nil { return }

    // 回填压缩信息到 UnifiedMessage
    compression.ApplyResultsToMessages(unifiedMessages, compResults)

    u.taskSubmitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
        Messages: unifiedMessages,
        // ...
    })
}
```

### 7.4 `MessageStoreTask` — 不变

`MessageStoreTask.Messages` 已是 `[]*vo.UnifiedMessage`，压缩信息嵌入 VO 中自然透传。`runMessageStoreTask` 和 dedup 逻辑无需改动。

---

## 8. API 层 — 无额外 DTO 改动

`dto.MessageItem.Message` 已是 `*vo.UnifiedMessage`，新增字段自动透传：

```
DB (Message.Message) → MessageCacheRecord.Message → MessageView.Message → dto.MessageItem.Message → API JSON
```

所有中间结构都直接引用 `*vo.UnifiedMessage`，无需逐层添加字段。Redis 缓存兼容：`omitempty` 确保旧缓存数据不受影响。

---

## 9. Web 前端变更

### 9.1 `types.ts`

```typescript
export interface UnifiedMessage {
  role: string;
  content?: string | Array<Record<string, unknown>>;
  name?: string;
  reasoning_content?: string;
  refusal?: string;
  tool_call_id?: string;
  tool_calls?: UnifiedToolCall[];
  raw_content?: string;           // NEW
  compression_strategy?: string;  // NEW
}
```

### 9.2 `content-extract.ts` 扩展

```typescript
export interface ToolResultInfo {
  text: string;                   // 压缩后内容 (after)
  rawContent?: string;            // 压缩前内容 (before)
  compressionStrategy?: string;   // 压缩策略
}

// 返回类型变更: Record<string, string> → Record<string, ToolResultInfo>
export function buildToolResultsByID(messages: MessageItem[]): Record<string, ToolResultInfo>

// lookupToolResult 返回类型: string | undefined → ToolResultInfo | undefined
```

### 9.3 组件 prop 类型更新

| 组件 | prop 变更 |
|------|----------|
| `ChatMessage` | `toolResultsByID: Record<string, ToolResultInfo>` |
| `AssistantMessage` | `toolResultsByID: Record<string, ToolResultInfo>` |
| `ToolCallCard` | `result?: ToolResultInfo`（原 `result?: string`） |
| `SharePage` | 同步更新 `toolResultsByID` 类型 |

### 9.4 `ToolCallCard` 压缩展示

**默认状态**（折叠）：
- Output 区域显示压缩后内容（`result.text`）
- 当 `result.compressionStrategy` 非空时，底部显示策略 badge + "查看原始内容" 按钮

**展开状态**（点击按钮）：
- 显示 `<CompressionDiff>` 组件
- 传入 `before={result.rawContent}` `after={result.text}` `strategy={result.compressionStrategy}`

### 9.5 新增 `CompressionDiff` 组件

新建 `web/src/components/chat/compression-diff.tsx`：

```typescript
interface CompressionDiffProps {
  before: string;
  after: string;
  strategy: string;
}
```

- 使用 `diff` npm 包（`diffLines`）计算行级 diff
- 内部状态：`mode: "inline" | "split"`，默认 `"split"`
- **单栏模式**（inline）：增删行用 +/- 标记 + 绿/红背景色
- **双栏模式**（split）：左 before / 右 after，对齐显示
- Tailwind 样式，匹配项目设计系统
- 顶部 toggle 按钮切换模式

### 9.6 新增依赖

`web/package.json`：
```json
"dependencies": {
  "diff": "^7.0.0"
},
"devDependencies": {
  "@types/diff": "^7.0.0"
}
```

### 9.7 影响范围汇总

| 文件 | 改动 |
|------|------|
| `web/src/lib/types.ts` | `UnifiedMessage` 加 2 字段 |
| `web/src/components/chat/content-extract.ts` | 返回类型 + `ToolResultInfo` |
| `web/src/components/chat/chat-message.tsx` | prop 类型 |
| `web/src/components/chat/assistant-message.tsx` | prop 类型 + 传 `ToolResultInfo` |
| `web/src/components/chat/tool-call-card.tsx` | 压缩展示 + diff 触发 |
| `web/src/components/chat/compression-diff.tsx` | **新建** diff 组件 |
| `web/src/app/share/page.tsx` | prop 类型 |
| `web/package.json` | 加 `diff` + `@types/diff` |

---

## 10. 错误处理与安全措施

- **压缩永不阻塞请求**：compressor 返回 passthrough result，Locator 不返回 error
- **Store 回填失败安全**：`ApplyResultsToMessages` 按 tool_call_id 匹配，未匹配到的消息不受影响
- **缓存兼容**：`omitempty` 确保旧 Redis 缓存数据反序列化不报错
- **向后兼容**：未压缩的消息 `RawContent = nil`，`CompressionStrategy = ""`，行为与现在一致

---

## 11. 测试策略

### 11.1 API 侧单元测试

- `test/unit/compression/` 现有测试适配新的 DTO 输入（替换 bytes 输入）
- 新增 `ApplyResultsToMessages` 单元测试：验证 tool_call_id 匹配、未匹配跳过、Applied=false 跳过
- 新增 `ItemCompressionResult` 字段验证：ToolCallID 和 Input 正确设置

### 11.2 Web 侧验证

- `cd web && npm run lint && npm run build` 验证类型与导出
- 手动验证 session 详情页 ToolCallCard 的压缩展示和 diff 视图

### 11.3 E2E 测试

- `test/e2e/compression/` 现有 E2E 适配（如需要）
- 验证压缩后的消息在 session 详情页正确展示 before/after 内容

---

## 12. 文件变更清单

### API 侧

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/common/vo/unified_message.go` | 修改 | 加 `RawContent` + `CompressionStrategy` |
| `internal/application/llmproxy/compression/result.go` | 修改 | `ItemCompressionResult` 加 `ToolCallID` + `Input`；`CompressionStats` 加 `Items`；`addItem` 逻辑 |
| `internal/application/llmproxy/compression/locator.go` | 修改 | 删除 `ToolOutputLocator` 接口、`SelectLocator`、`CompressBody` |
| `internal/application/llmproxy/compression/locator_openai.go` | 修改 | 重写为 `CompressOpenAIChat(messages []*dto.OpenAIChatCompletionMessageParam, ...)` |
| `internal/application/llmproxy/compression/locator_anthropic.go` | 修改 | 重写为 `CompressAnthropicMessages(messages []*dto.AnthropicMessageParam, ...)` |
| `internal/application/llmproxy/compression/locator_responses.go` | 修改 | 重写为 `CompressOpenAIResponses(items []*dto.ResponseInputItem, ...)` |
| `internal/application/llmproxy/compression/apply.go` | **新建** | `ApplyResultsToMessages` 函数 |
| `internal/application/llmproxy/usecase/openai.go` | 修改 | 删除 `compressBodyIfNeeded`，压缩逻辑移入 forward 方法 |
| `internal/application/llmproxy/usecase/anthropic.go` | 修改 | 同上 |
| `internal/application/llmproxy/usecase/openai_chat.go` | 修改 | 7 条 forward 路径加压缩调用 + store 传参 |
| `internal/application/llmproxy/usecase/openai_response.go` | 修改 | 同上 |
| `internal/application/llmproxy/usecase/anthropic_message.go` | 修改 | 同上 |
| `internal/application/llmproxy/usecase/openai_store.go` | 修改 | store 方法加 `compResults` 参数 + `ApplyResultsToMessages` 调用 |
| `internal/application/llmproxy/usecase/anthropic_store.go` | 修改 | 同上 |
| `internal/config/config.go` | 修改 | 移除 `CompressionMinBodyBytes` 配置项 |
| `env/api.env` | 修改 | 移除 `COMPRESSION_MIN_BODY_BYTES` 环境变量 |
| `test/unit/compression/*_test.go` | 修改 | 适配 DTO 输入 |
| `test/unit/compression/apply_test.go` | **新建** | `ApplyResultsToMessages` 单测 |

### Web 侧

| 文件 | 操作 | 说明 |
|------|------|------|
| `web/src/lib/types.ts` | 修改 | `UnifiedMessage` 加 2 字段 |
| `web/src/components/chat/content-extract.ts` | 修改 | `ToolResultInfo` + 返回类型 |
| `web/src/components/chat/chat-message.tsx` | 修改 | prop 类型 |
| `web/src/components/chat/assistant-message.tsx` | 修改 | prop 类型 |
| `web/src/components/chat/tool-call-card.tsx` | 修改 | 压缩展示 + diff 触发 |
| `web/src/components/chat/compression-diff.tsx` | **新建** | diff 组件 |
| `web/src/app/share/page.tsx` | 修改 | prop 类型 |
| `web/package.json` | 修改 | 加 `diff` + `@types/diff` |
