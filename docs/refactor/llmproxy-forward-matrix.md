# LLMProxy 转发路径差异矩阵（DDD 重构 Step 1 产出）

> 本文档由 code-explorer subagent 深度扫描 `internal/service/{openai,anthropic}.go`、
> `internal/proxy/*.go`、`internal/converter/*.go`、`internal/dto/*.go`、`internal/util/*.go`
> 以及 `test/**` 后产出，作为 DDD 重构中 **6 步 Pipeline 抽象** 与 **严格字节级兼容** 的权威依据。
>
> Step 3（Pipeline 实现）/ Step 4（域填充）/ Step 5（handler 切换）/ Step 6（清理）的任何迁移
> 动作都必须以本文件的条目为回归校验基线。严禁抽象过度泛化导致特例丢失。

---

## 1. 10 条转发路径差异矩阵

| # | 入站协议 | 源函数（`internal/service/`） | 流式 | 上游协议 | Request 适配 | Response 适配 | Client Writer | 首 Token 判定 | 特殊兼容处理 |
|---|---|---|---|---|---|---|---|---|---|
| 1 | OpenAI `/chat/completions` | `openAIService.forwardNativeStream` | ✅ | OpenAI | `ReplaceModelInBody` + `max_tokens→max_completion_tokens` | 透传 chunk，`chunk.Model = req.Body.Model` | `bufio.Writer` `data: %s\n\n` | `chunk.Choices[0].Delta.Content != ""` | 手写 `data: [DONE]\n\n` |
| 2 | OpenAI `/chat/completions` | `openAIService.forwardNativeNonStream` | ❌ | OpenAI | 同 #1 | 透传 JSON，`completion.Model = req.Body.Model` | `JSONResponseWriter.WriteJSON` | 无（记 `totalMs`） | - |
| 3 | OpenAI `/chat/completions` | `openAIService.forwardViaAnthropicStream` | ✅ | Anthropic | `AnthropicProtocolConverter.FromOpenAIRequest` + `anthropicReq.Model = ep.Model` | `ToOpenAISSEResponse(event, req.Body.Model, chunkID)` 跨协议 | 同 #1 | 跨协议 chunk 的 `Delta.Content != ""` | 手写 `[DONE]`；`GenerateOpenAIChunkID()` 共享 chunkID |
| 4 | OpenAI `/chat/completions` | `openAIService.forwardViaAnthropicNonStream` | ❌ | Anthropic | 同 #3 | `ToOpenAIResponse(anthropicMsg)` + `completion.Model = req.Body.Model` | `JSONResponseWriter.WriteJSON` | - | - |
| 5 | OpenAI `/v1/responses` | `openAIService.forwardResponseStream` | ✅ | OpenAI（原生 Response API） | `ReplaceModelInBody` | 透传 `event: %s\ndata: %s\n\n` + `ReplaceModelInSSEData(data, req.Body.Model)`；拦截 terminal 事件解析 `ResponseStreamTerminalEvent` | `bufio.Writer`（带 event 行） | `util.IsResponseAPIDeltaEvent(event)` ≡ `strings.HasSuffix(event, ".delta")` | terminal 由 proxy 原样下发，**不补 `[DONE]`**；`SetErrorFromResponseStatus` 注入 in-band 失败 |
| 6 | OpenAI `/v1/responses` | `openAIService.forwardResponseNonStream` | ❌ | OpenAI（原生 Response API） | 同 #5 | 原样 `respBody` + `ReplaceModelInBody(respBody, req.Body.Model)`，**直写 BodyWriter**（不走 `WriteJSON` 避免二次 marshal） | `HumaCtx.BodyWriter().Write` | - | 旁路 `sonic.Unmarshal(respBody, &rsp)` 供 audit/存储；解析失败仅 warn，不影响响应 |
| 7 | OpenAI `/v1/responses` | `openAIService.forwardResponseAnthropicStream` | ✅ | Anthropic | `AnthropicProtocolConverter.FromResponseAPIRequest`（含 `reasoning→thinking`、`instructions→system`、`text.format→OutputConfig`） | **复用** `ToOpenAISSEResponse`——客户端收到的是 `/chat/completions` chunk 形态（**非** Response API SSE） | 同 #1 | 跨协议 chunk 的 `Delta.Content != ""` | 手写 `[DONE]`；存储走 `storeFromAnthropicMsgForResponse`（instructions+input 前置 + Anthropic blocks 内联映射） |
| 8 | OpenAI `/v1/responses` | `openAIService.forwardResponseAnthropicNonStream` | ❌ | Anthropic | 同 #7 | `ToOpenAIResponse(anthropicMsg)` → `/chat/completions` JSON 形态 | `JSONResponseWriter.WriteJSON` | - | 返回体是 ChatCompletion 结构，**非** Response 对象 |
| 9 | Anthropic `/v1/messages` | `anthropicService.forwardNativeStream` | ✅ | Anthropic | `ReplaceModelInBody` | 逐事件透传 `event: %s\n` + `data: %s\n\n` + `ReplaceModelInSSEData(event.Data, exposedModel)` | `bufio.Writer`（两行 Fprintf） | `event.Event == AnthropicSSEEventTypeContentBlockDelta` | `util.WriteAnthropicMessageStop(w)`（`event: message_stop\ndata: {"type":"message_stop"}\n\n`） |
| 10 | Anthropic `/v1/messages` | `anthropicService.forwardNativeNonStream` | ❌ | Anthropic | 同 #9 | 透传，`anthropicMsg.Model = exposedModel` | `JSONResponseWriter.WriteJSON` | - | - |
| 11 | Anthropic `/v1/messages` | `anthropicService.forwardViaOpenAIStream` | ✅ | OpenAI | `OpenAIProtocolConverter.FromAnthropicRequest` + `openAIReq.Model = ep.Model` | `ToAnthropicSSEResponse(chunk, isFirst, exposedModel, tracker)`；共享 `SSEContentBlockTracker` | 同 #9 | **`event.Event == AnthropicSSEEventTypeContentBlockStart`**（⚠️ 与 #9 不一致！） | `WriteAnthropicMessageStop`；流结束后合并 chunks 再 `ToAnthropicResponse` 以存储 |
| 12 | Anthropic `/v1/messages` | `anthropicService.forwardViaOpenAINonStream` | ❌ | OpenAI | 同 #11 | `ToAnthropicResponse(completion)` + `anthropicMsg.Model = exposedModel` | `JSONResponseWriter.WriteJSON` | - | - |

**端点路由规则**（`service.findEndpoint`）：
- OpenAI 入站（`/chat/completions`、`/v1/responses`）：优先 `ProviderOpenAI`，fallback `ProviderAnthropic`
- Anthropic 入站（`/v1/messages`）：优先 `ProviderAnthropic`，fallback `ProviderOpenAI`

**说明**：当前仓库 **不存在** Response API → OpenAI 上游的跨协议路径（仅透传原生 OpenAI Response API），路径 #5/#6 走原生 OpenAI 上游。

---

## 2. 首 Token 判定的 4 种口径

| 路径 | 判定条件（源代码精确） | 文件:行 |
|---|---|---|
| #1 | `len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != ""` | `service/openai.go:160` |
| #3 | 遍历转换后每个 chunk：`len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil && chunk.Choices[0].Delta.Content != ""` | `service/openai.go:274` |
| #5 | `util.IsResponseAPIDeltaEvent(event)` ≡ `strings.HasSuffix(event, ".delta")` | `service/openai.go:499` / `util/openai.go:241` |
| #7 | 同 #3（输出是 OpenAI chat chunk） | `service/openai.go:614` |
| #9 | `event.Event == enum.AnthropicSSEEventTypeContentBlockDelta`（**仅 delta**） | `service/anthropic.go:144` |
| #11 | `event.Event == enum.AnthropicSSEEventTypeContentBlockStart`（**不是 delta！**） | `service/anthropic.go:251` |

**`streamDurationMs` 计算规则一致**：`if !firstTokenTime.IsZero() { streamDurationMs = time.Since(firstTokenTime).Milliseconds() }`
- 流中从未命中首 token 时，两者均为 0
- 非流式路径 `FirstTokenLatencyMs = totalMs`（从 `startTime` 到 `Forward*` 返回），`StreamDurationMs = 0`

**抽象约束**：Pipeline 的 `FirstTokenDetector` 值对象必须支持这 4 种策略，且 #9 vs #11 在同一入站协议下策略不同——取决于上游协议。

---

## 3. 请求体预处理清单（兼容补丁）

| 补丁 | 触发路径 | 触发条件 | 源位置 |
|---|---|---|---|
| **`max_tokens → max_completion_tokens`** | 仅 #1、#2 | `req.Body.MaxTokens != nil` | `service/openai.go:139` |
| `ReplaceModelInBody(body, ep.Model)` | #1、#2、#5、#6、#9、#10 | marshal 后强行覆盖 JSON `model` 字段 | `proxy/anthropic.go:212` |
| `ReplaceModelInSSEData(event.Data, modelName)` | #5（`req.Body.Model`）、#9（`exposedModel`） | 覆盖每条 SSE data 的 `model` 及嵌套 `message.model` | `proxy/anthropic.go:228` |
| `anthropicReq.Model = ep.Model` | #3、#4、#7、#8 | 跨协议转换后赋值上游 model | `service/openai.go:250,477` |
| `openAIReq.Model = ep.Model` | #11、#12 | 同上 | `service/anthropic.go:225` |
| Chunk model 回写 exposed | #1/#2/#4/#8/#10/#12 | 响应落客户端前覆盖回 alias | 多处 |
| Anthropic→OpenAI 转换 `MaxTokens` | #11/#12 | `lo.ToPtr(req.MaxTokens)` | `converter/openai.go:37` |
| OpenAI→Anthropic `resolveMaxTokens` | #3/#4 | 优先 `MaxCompletionTokens`，回退 `MaxTokens` | `converter/anthropic.go:142` |
| Response API `max_output_tokens → MaxTokens` | #7/#8 | `req.MaxOutputTokens != nil` | `converter/anthropic.go:631` |
| Response API `reasoning.effort → thinking.type` | #7/#8 | low/medium/high/xhigh/minimal/none 映射（xhigh→high，默认 medium） | `converter/anthropic.go:640` |
| Response API `text.format` 过滤 | #7/#8 | 仅 `json_object`/`json_schema` 生效，其他 drop | `converter/anthropic.go:710` |
| `developer` 角色 → `system`（Response API） | #7/#8 | `resolveResponseAPIRole` | `converter/anthropic.go:790` |
| OpenAI→Anthropic 丢弃 `redacted_thinking` | #3/#4 | `convertOpenAIMessageToAnthropicContent` 不处理 | `converter/openai.go:343` |
| 无参数工具裁剪 | #11/#12 | `isEmptyObjectSchema` → `parameters=nil`（OpenAI 要求） | `converter/openai.go:443` |
| JSON Schema 清除 `$schema` | 所有跨协议工具 | `normalizeOpenAISchema` 浅拷贝清空 `SchemaURI` | `converter/openai.go:476` |

**抽象约束**：`RequestAdapter` 接口必须接受 **可组合的 `PostAdaptHook`**（如 max_tokens 改写），不能硬编码到单个 Adapter 内部。

---

## 4. 上游响应错误处理

### 4.1 错误类型语义（`util.ExtractUpstreamStatusAndError`）

| err 类型 | 语义 | 返回 `(statusCode, message)` |
|---|---|---|
| `nil` | 成功 | `(200, "")` |
| `*model.UpstreamError{StatusCode, Body}` | 上游 HTTP 非 200 | `(StatusCode, "upstream returned status N: <body>")` |
| `*model.UpstreamConnectionError{Cause}` | 网络层失败 | `(-1, "upstream connection error: ...")` |
| 其他 error | 未知（DTO 转换/ctx 取消等） | `(0, err.Error())` |

**回归保护**：`connection(-1) ≠ unknown(0)` 是显式断言（`test/upstream_error/`）。

### 4.2 非流式错误写入（#2/#4/#6/#8/#10/#12）

`util.WriteUpstreamError(log, writer, err, fallbackBody)`：
- `errors.As(err, &UpstreamError)` → 透传上游 status + body（`Content-Type: application/json`）
- 否则 → `fiber.StatusBadGateway` + `fallbackBody`

**两种 fallbackBody（预序列化为 `[]byte` 常量）**：
```json
// OpenAI (openAIInternalErrorBody, service/openai.go:31)
{"error":{"message":"Internal server error","type":"server_error","code":"internal_error"}}

// Anthropic (anthropicInternalErrorBody, service/anthropic.go:28)
{"type":"error","error":{"type":"api_error","message":"Internal server error"}}
```

### 4.3 流式错误：**悄悄丢**（关键特例！）

所有流式路径 `err != nil` 时：
- **不** 写任何 SSE error 事件
- **不** 补 `[DONE]` / `message_stop`（`if err == nil` 才补）
- 直接退出 handler（fasthttp StreamWriter 结束连接）
- 已 flush 的 chunk 保留（客户端看到截断流）
- **依旧** Submit 审计任务
- **不** 调用 `WriteUpstreamError`（header 已发送 200，无法再改 status）

**抽象约束**：Pipeline 的 `ClientWriter` **不需要** `WriteError(sseError)` 能力（现有行为就是断流）。如果抽象出"error emitter"会破坏现有行为。

---

## 5. 消息存储（Conversation）差异

### 5.1 存储函数与格式选择

| 服务函数 | 路径 | 输入形态 | 落盘格式 |
|---|---|---|---|
| `openAIService.storeFromCompletion` | #1/#2/#4 | `*OpenAIChatCompletion` | OpenAI Messages → Unified |
| `openAIService.storeFromAnthropicMsg` | #3 | `*AnthropicMessage` | **先 `ToOpenAIResponse` 转回 OpenAI 再 `FromOpenAIMessage`** |
| `openAIService.storeFromAnthropicMsgForResponse` | #7 | `*AnthropicMessage` + ResponseAPI 请求 | **`buildResponseRequestUnifiedMessages` + `anthropicResponseContentToUnified`**（不经过 `FromAnthropicResponse`）|
| `openAIService.storeFromResponseRsp` | #5/#6 | `*OpenAICreateResponseRsp` | `buildResponseRequestUnifiedMessages` + `FromResponseAPIOutputItems` |
| `anthropicService.storeFromAnthropicMsg` → `storeAnthropicMessages` | #9/#10/#11/#12 | `*AnthropicMessage` | **保持 Anthropic 格式**：`FromAnthropicMessage` / `FromAnthropicResponse` |

### 5.2 Model 字段方向性（高危混淆点）

| 字段 | 取值来源 |
|---|---|
| `MessageStoreTask.Model` | **upstream model**（`ep.Model` = DB 中的真实模型名，如 `gpt-4o-2024-08-06`） |
| `ModelCallAuditTask.Model` | **exposed model**（客户端请求的 alias / `req.Body.Model`） |

**抽象约束**：Pipeline 的 `Observer.OnCompleted` 必须同时携带这两个模型字段，不能只传一个。

### 5.3 `From*` 转换函数清单（所有迁入 `domain/llmproxy/converter/` 的入口）

| 函数 | 签名 | 错误类型 |
|---|---|---|
| `FromOpenAIMessage(*OpenAIChatCompletionMessageParam)` | → `(*UnifiedMessage, error)` | `ierr.ErrDTOConvert` |
| `FromAnthropicMessage(*AnthropicMessageParam)` | → `(*UnifiedMessage, error)` | `ierr.ErrDTOConvert/ErrDTOMarshal` |
| `FromAnthropicResponse(*AnthropicMessage)` | → `(*UnifiedMessage, error)` | 同上 |
| `FromResponseAPIInputItems([]*ResponseInputItem)` | → `([]*UnifiedMessage, error)` | `ierr.ErrDTOConvert` |
| `FromResponseAPIOutputItems([]*ResponseInputItem)` | → `([]*UnifiedMessage, error)` | 合并 reasoning 到下一个 assistant |
| `FromOpenAITool(*OpenAIChatCompletionTool)` | → `*UnifiedTool` | 无错误 |
| `FromAnthropicTool(*AnthropicTool)` | → `*UnifiedTool` | 无错误 |
| `FromResponseAPITool(*ResponseTool)` | → `*UnifiedTool`（非 function/custom 返回 nil） | 无错误 |

### 5.4 跨协议路径的存储格式（入站协议决定）

| 路径 | 请求侧来源 | 响应侧来源 | 落盘格式 |
|---|---|---|---|
| #1/#2 | `FromOpenAIMessage(req.Body.Messages)` | `FromOpenAIMessage(completion.Choices[0].Message)` | OpenAI→Unified |
| #3/#4 | **仍 `FromOpenAIMessage(req.Body.Messages)`** | 上游 `anthropicMsg` 先 `ToOpenAIResponse`，再 `FromOpenAIMessage` | **OpenAI→Unified**（响应做了回流再抽） |
| #5/#6 | `buildResponseRequestUnifiedMessages`：手工构造 `instructions→system` + `FromResponseAPIInputItems(Input.Items)` 或 `RoleUser{Text}` | `FromResponseAPIOutputItems(rsp.Output)` | Response API→Unified |
| #7/#8 | 同 #5/#6 | **`anthropicResponseContentToUnified`**（服务层内联实现，不走 `FromAnthropicResponse`）；tool_use input marshal 失败 → **整体放弃落盘** | Response API 请求 + Anthropic blocks →Unified |
| #9/#10 | `FromAnthropicMessage(req.Body.Messages)` | `FromAnthropicResponse(assistantMsg)` | Anthropic→Unified |
| #11/#12 | 同 #9/#10 | 上游 `completion` 先 `ToAnthropicResponse`，再 `FromAnthropicResponse` | **Anthropic→Unified**（响应做了回流再抽） |

**核心结论**：存储格式由**入站协议**决定，而非上游协议。跨协议路径会先把上游响应转回入站协议形态再抽取。Response API 是例外——`#7` 直接从 Anthropic blocks 内联映射到 Unified，**不经过 `FromAnthropicResponse`**。

### 5.5 `MessageStoreTask` 字段完整对照

```go
type MessageStoreTask struct {
    Ctx          context.Context   // util.CopyContextValues(ctx)
    APIKeyName   string            // util.CtxValueString(ctx, CtxKeyUserName)
    Model        string            // upstreamModel (ep.Model)
    Messages     []*UnifiedMessage
    Tools        []*UnifiedTool
    InputTokens  int               // usage.PromptTokens / InputTokens
    OutputTokens int               // usage.CompletionTokens / OutputTokens
    Metadata     map[string]string // OpenAI: req.Body.Metadata 整 map；Anthropic: util.ExtractAnthropicMetadata（仅 UserID→user_id）
}
```

---

## 6. 审计（ModelCallAudit）差异

### 6.1 字段取值规则（一致性契约）

| 字段 | 取值来源 |
|---|---|
| `Ctx` | `util.CopyContextValues(ctx)` |
| `ModelID` | `endpoint.ID`（DB 主键） |
| `Model` | **exposedModel**（`req.Body.Model`） |
| `UpstreamProvider` | `endpoint.Provider`（DB 中实际命中） |
| `APIProvider` | **入站协议硬编码**：OpenAI 服务全 `ProviderOpenAI`，Anthropic 服务全 `ProviderAnthropic`（#11/#12 上游是 OpenAI，但 APIProvider 仍是 Anthropic） |
| `FirstTokenLatencyMs` | 流式：首 token 触发时计算；非流式：`totalMs` |
| `StreamDurationMs` | 仅流式；`time.Since(firstTokenTime)`；非流式 0 |
| Token 字段 | 由 `SetTokensFrom{OpenAI,Anthropic,Response}Usage` 三件套写入 |
| `UpstreamStatusCode` | **非流式成功**：硬编码 `fiber.StatusOK`；**其它**（含流式成功）：`ExtractUpstreamStatusAndError(err)` |
| `ErrorMessage` | `ExtractUpstreamStatusAndError(err)`；#5/#6 额外 `SetErrorFromResponseStatus(rsp)` |

### 6.2 `SetTokensFrom*` 三件套

| 方法 | 源字段 → Audit 字段 | Null 处理 |
|---|---|---|
| `SetTokensFromOpenAIUsage(*OpenAICompletionUsage)` | `PromptTokens→InputTokens`；`CompletionTokens→OutputTokens` | nil 直接返回 |
| `SetTokensFromAnthropicUsage(*AnthropicMessage)` | `Usage.{InputTokens,OutputTokens,CacheCreationInputTokens,CacheReadInputTokens}` | msg/Usage nil 返回 |
| `SetTokensFromResponseUsage(*OpenAICreateResponseRsp)` | `Usage.{InputTokens,OutputTokens}` + `InputTokensDetails.CachedTokens→CacheReadInputTokens` | rsp/Usage nil 返回 |

### 6.3 `SetErrorFromResponseStatus`（仅 #5/#6）

只在 **原生 Response API 路径** 调用（#7/#8 上游是 Anthropic 没有 Response 对象 status）。规则：
- `t.ErrorMessage != ""` → **不覆盖**（传输层错误优先）
- `status == failed` + `rsp.Error != nil` → `"response.failed: <error.message>"`
- `status == failed` 无 error payload → `"response.failed"`
- `status == incomplete` + `IncompleteDetails != nil` → `"response.incomplete: <reason>"`
- `status == incomplete` 无 details → `"response.incomplete"`

### 6.4 失败路径的 Submit 逻辑

- **所有** 路径（成功/失败、流式/非流式）都 Submit 审计，无"流式失败跳过"分支
- #11 有**三层审计 submit**（流错误/转换失败/成功三分支），每次状态码设置不同
- 非流式失败：`WriteUpstreamError` 之后 Submit audit，然后 `return`（不 submit store）

---

## 7. SSE 终止帧与 terminal marker

| 协议 | 终止标记 | 由谁写 | 触发条件 |
|---|---|---|---|
| OpenAI Chat `[DONE]` | `data: [DONE]\n\n` | **Service** 手写 `fmt.Fprintf` | 仅 `err == nil` |
| Anthropic `message_stop` | `event: message_stop\ndata: {"type":"message_stop"}\n\n`（`constant.AnthropicMessageStopSSEFrame`） | `util.WriteAnthropicMessageStop(w)` | 仅 `err == nil`；#9 和 #11 共用 |
| Response API terminal | **不由网关补发**——上游的 `response.completed`/`failed`/`incomplete` 原样透传 | Proxy 透传 | - |

**`[DONE]` 触发路径**：#1、#3、#7（#7 输出 Chat chunk 形态所以也补 `[DONE]`）。
**`message_stop` 触发路径**：#9、#11。
**路径 #5 不写任何结束帧**（terminal 事件由上游发）。

### 跨协议补帧特例

| 方向 | 补帧规则 |
|---|---|
| Anthropic → OpenAI chunk（#3/#7） | Service 补 `[DONE]`；上游 `message_stop` 在 `ToOpenAISSEResponse` 映射为 `(nil, nil)` 丢弃 |
| OpenAI → Anthropic event（#11） | Service 补 `message_stop`；转换器首 chunk `isFirst=true` 生成 `message_start`；`SSEContentBlockTracker` 保证 `content_block_start` 每 index 只发一次；`finish_reason` 触发 `message_delta` |
| #11 特例 | **不显式生成 `content_block_stop` 事件**（已知协议不完整但被测试覆盖） |

---

## 8. 测试字节级断言点（不能改变的字节序/字段顺序）

| 测试目录 | 核心断言 |
|---|---|
| `test/anthropic_sse/` | `event: message_stop\ndata: {"type":"message_stop"}\n\n` 完全固定；禁止退化为 `data: {}\n` |
| `test/converter/` | `fixtures/cases.json` + `fixtures/response_cases.json` 字段级 DeepEqual（Model/MaxTokens/System.Text/Role/Blocks/Source.Type/finish_reason 映射/roundtrip） |
| `test/openai_response_handler/` | Codex `client_metadata` 字段不得触发 422 |
| `test/openai_response_dto/` | `TestOpenAICreateResponseReq_RoundTripAll`：所有 fixture `jsonEqual(Marshal(Unmarshal(body)), body)` 语义相等 |
| `test/unified_response/` | instructions+input+output 合并后的 UnifiedMessage 列表长度/字段；reasoning 合并到 assistant.ReasoningContent；terminal event 三种解析 |
| `test/message_checksum/` | `ComputeMessageChecksum` key 顺序无关；ToolCall.ID 忽略；ToolCallID 不忽略；schema-aware default 移除 |
| `test/tool_checksum/` | `ComputeToolChecksum` 稳定性、100 次 deterministic |
| `test/model_call_audit/` | 全字段 round-trip 等价 |
| `test/upstream_error/` | `connection(-1)` ≠ `unknown(0)` 显式断言 |

### 不得改变的字节/字段顺序

1. **Anthropic SSE 每帧顺序**：`event: X\ndata: Y\n\n`（两次 `Fprintf`，event 行必须在 data 行之前）
2. **`event: message_stop\ndata: {"type":"message_stop"}\n\n`** 字节完全固定
3. **OpenAI Chat SSE 无 event 行**，仅 `data: <json>\n\n` + 结束 `data: [DONE]\n\n`
4. **Response API SSE**：`event: <type>\ndata: <json>\n\n`，terminal 为 `response.completed/failed/incomplete`，**无** `[DONE]`
5. `ReplaceModelInBody/SSEData` 会通过 `map[string]any` round-trip **打乱 JSON key 顺序**——这是现有行为，迁移时必须保留（不能改为结构化替换）
6. `ToOpenAISSEResponse` 每个转换后 chunk 调用 `time.Now().Unix()`——测试不校验 `created` 字段相等
7. `message_start` payload 空数组而非 null：`{"type":"message","id":...,"role":"assistant","model":...,"content":[],"usage":{}}`
8. `content_block_delta` 的 `delta.type` 四种：`text_delta`/`thinking_delta`/`input_json_delta`/`signature_delta`；未知类型 `ConcatAnthropicSSEEvents` 返回 `ierr.ErrSSEUnknownEvent`

---

## 9. 关键特例清单（抽象 Pipeline 最易遗漏）

> 这些是本次 DDD 重构**最大的正确性风险点**。任何迁移都必须在对应测试通过后才能合入。

1. **首 Token 判定在 #11 用 `content_block_start` 而非 `content_block_delta`**——与 #9 不一致。`FirstTokenDetector` 值对象必须按上游协议区分。
2. **路径 #7 客户端协议混乱**：入站 Response API `/v1/responses`，但流式响应是 OpenAI Chat chunk 形态。非流式 #8 返回 OpenAI Chat JSON。
3. **路径 #6 不走 `WriteJSON`**：为避免 re-marshal 丢字段，直接 `BodyWriter().Write(replaced)`。`ClientWriter` 必须有"直写 bytes"能力。
4. **路径 #3/#4 落盘格式 = 入站 OpenAI**（响应做了回流再抽），而 #7 的落盘是"请求 Response API input + 响应 Anthropic blocks 内联转换"，**不经过 `FromAnthropicResponse`**。#7 专门写了 `anthropicResponseContentToUnified`（`service/openai.go:768`）。
5. **`MessageStoreTask.Model = upstream model`，`ModelCallAuditTask.Model = exposed model`**——方向相反，极易混淆。
6. **流式错误悄悄丢**：header 已发 200，错误直接 hang 断流；客户端靠连接断开发现。Pipeline **不能** 抽象出 SSE error emitter。
7. **`util.WriteUpstreamError` 只在非流式使用**。
8. **`SetErrorFromResponseStatus` 只在 #5/#6 用**（原生 Response API）。
9. **`util.ExtractAnthropicMetadata` 只抽 `user_id`**；OpenAI 路径整 map 直传。跨协议 metadata 丢失属于已知限制。
10. **`ForwardCreateResponse`（#6）返回 raw bytes** 而非 DTO。Service 二次 `sonic.Unmarshal` 做 audit/store（失败仅 warn，不阻响应）。
11. **`ConcatAnthropicSSEEvents` 对未知事件返回 `ErrSSEUnknownEvent`**，导致 `collectedEvents==0` → `(nil, nil)`。
12. **`ConcatChatCompletionChunks` 空 chunks 返回空 struct（非 nil）**，proxy 层在 `len(collectedChunks)==0` 时返回 `(nil, nil)`——两处对"空流"语义不一致。
13. **`ForwardCreateResponseStream` 返回 `error` 而无合并响应对象**——与其它 `Forward*Stream` 签名不一致；Service 靠拦截 terminal event 自己维护 `finalResponse`。
14. **`SSEContentBlockTracker.startedTextBlocks` 负数 key**（`thinkingKey := -(choice.Index + 1)`）——与正数 text key 共用 map 但用符号区分。
15. **`ReplaceModelInBody/SSEData` 是 `map[string]any` round-trip**——sonic 不保证 map key 顺序稳定。当前生产 OK 因为无客户端依赖原始字节顺序。
16. **路径 #11 有 3 层 audit submit**（流错误 / 转换失败 / 成功），每一支状态码设置不同。
17. **`OpenAICreateResponseReq.Input` 两态**：`.Text`（string）vs `.Items`（数组），优先 Items。`buildResponseRequestUnifiedMessages` 分两条路径。
18. **Anthropic 流式 `OutputTokens` 在 `message_delta` 覆盖**，`InputTokens/CacheCreation/CacheRead` 来自 `message_start`——跨事件拼装。
19. **`UpstreamError.Body` 原样透传**：若上游返回非 JSON，客户端会收到非 JSON 但 header 是 `application/json`——当前容忍。
20. **`endpointFields` 常量**：`["id","model","api_key","base_url","provider"]`；Anthropic `CountTokens` 用 `["model","api_key","base_url"]`（不做审计）。

---

## 10. Pipeline 抽象边界映射（Step 3 参考）

基于以上矩阵，6 步 Pipeline 接口的变点集中在：

| Pipeline 步骤 | 变点来源 | 主要实现类（Step 3 待建） |
|---|---|---|
| `EndpointResolver` | endpoint 查询 + provider 回退（`findEndpoint`） | `domain/llmproxy/service/resolver.go` |
| `RequestAdapter[In,Up]` | 路径 #1-#12 的 Request 适配差异 + `PostAdaptHook`（如 max_tokens 改写） | `application/llmproxy/adapter/openai_identity.go` / `openai_to_anthropic.go` / `anthropic_to_openai.go` / `response_api_to_anthropic.go` 等 5+ 种 |
| `UpstreamTransport` | 原 `proxy/{openai,anthropic}.go`，拆 Stream/Unary | `domain/llmproxy/transport/openai.go` / `anthropic.go` |
| `ResponseAdapter[Up,Out]` + `FirstTokenDetector` | 首 token 4 种口径 + 响应适配 5+ 种 | `application/llmproxy/adapter/*_stream.go` |
| `ClientWriter[Out]` | Chat SSE / Chat JSON / Anthropic SSE / Anthropic JSON / Raw bytes（#6） | `application/llmproxy/writer/` 5 种 |
| `Observer` | audit 字段构造（Model 方向性）+ message store（入站协议决定格式）+ 度量 | `application/llmproxy/observer/{metrics,audit,conversation}.go` |

**装配约束**：每条路径装配代码 ≤ 20 行，通过 `sync.Once` / `init()` 缓存单例。

---

## 变更记录

- 2026-04-22：基于 code-explorer 深度扫描初始产出，作为 DDD 全量重构的权威差异矩阵。后续迁移步骤（Step 2-6）必须以此为回归基线。
