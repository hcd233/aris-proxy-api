# API DTO 与上游 DTO 分离 + 协议互转设计

## 背景

当前项目中 OpenAI 和 Anthropic 的协议定义（请求/响应 DTO）混在 `internal/dto/openai.go` 和 `internal/dto/anthropic.go` 中，且没有跨协议转换能力。现有的 `UnifiedMessage` / `UnifiedTool` 仅用于 session 存储，不支持反向转换。

模型端点配置已从 YAML 迁移到数据库（`ModelEndpoint` 表，通过 `dao.ModelEndpointDAO` 查询），`internal/proxy/proxy.go` 已删除。

本设计将 API DTO 与上游 DTO 分离，建立通用 DTO 层和独立 converter 包，实现 OpenAI 协议和 Anthropic 协议的完整互转，支持跨协议路由（客户端发 OpenAI 格式，上游仅有 Anthropic 端点时自动转换转发）。

## 目标

1. **DTO 分层**：按目录分离 OpenAI / Anthropic / Common DTO
2. **协议互转**：OpenAI ↔ Common ↔ Anthropic 双向转换
3. **流式转换**：SSE 流式响应的协议转换
4. **尽量映射**：不对称字段尽量映射到目标协议类似字段，无法映射的降级丢弃并记录 warning 日志
5. **全面覆盖**：尽可能全面覆盖可映射字段

## 目录结构

```
internal/
  dto/
    openai/                    # OpenAI 协议 wire DTO
      request.go               # ChatCompletionReq, ChatCompletionMessageParam, MessageContent, ChatCompletionContentPart, StopSequence, ChatCompletionToolChoiceParam
      response.go              # ChatCompletion, ChatCompletionChunk, ChatCompletionChoice, CompletionUsage, Logprobs
      tool.go                  # ChatCompletionTool, FunctionDefinition, CustomToolDefinition, ChatCompletionToolChoice
      union_types.go           # VoiceParam, Schema 实现（Huma 接口）
      error.go                 # OpenAIError, OpenAIErrorResponse
      list_models.go           # ListModelsRsp, OpenAIModel
    anthropic/                 # Anthropic 协议 wire DTO
      request.go               # CreateMessageReq, MessageParam, MessageContent
      response.go              # Message, Usage, SSE payloads (SSEMessageStart, SSEContentBlockDelta...)
      content_block.go         # ContentBlock, ContentSource, ToolResultContent
      tool.go                  # Tool, ToolChoice
      error.go                 # Error, ErrorResponse
      list_models.go           # ListModelsRsp, ModelInfo
      count_tokens.go          # CountTokensReq, TokensCount
      common.go                # CacheControl, CitationsConfig, Metadata, ThinkingConfig, OutputConfig, ContextManagement
    common/                    # 通用 DTO（跨协议共用）
      request.go               # ChatRequest
      response.go              # ChatResponse
      message.go               # Message, Content, ContentPart
      tool.go                  # Tool, ToolCall, ToolChoice
      stream.go                # StreamChunk, StreamEventType
      usage.go                 # Usage
      thinking.go              # ThinkingConfig, OutputConfig
    base.go                    # HTTPResponse, SSEResponse（保持不变）
    common_legacy.go           # CommonRsp, EmptyReq（保持不变）
    asynctask.go               # MessageStoreTask 等（保持不变）
    session.go                 # 保持不变
    user.go                    # 保持不变
    oauth2.go                  # 保持不变
    ping.go                    # 保持不变
    json_schema.go             # JSONSchemaProperty（被 common 和各协议共用，字段使用 sonic.NoCopyRawMessage，Schema 字段名改为 SchemaURI）

  converter/                   # 协议转换器
    openai.go                  # OpenAI ↔ Common 转换（请求+响应+消息+工具）
    anthropic.go               # Anthropic ↔ Common 转换（请求+响应+消息+工具）
    stream_openai.go           # OpenAI SSE 流式读写器
    stream_anthropic.go        # Anthropic SSE 流式读写器
    mapping.go                 # 枚举映射表（FinishReason, Role, ToolChoice 等）
```

## 通用 DTO 定义

### ChatRequest（通用请求）

```go
type ChatRequest struct {
    Model            string            `json:"model"`
    Messages         []*Message        `json:"messages"`
    MaxTokens        *int              `json:"max_tokens,omitempty"`
    Temperature      *float64          `json:"temperature,omitempty"`
    TopP             *float64          `json:"top_p,omitempty"`
    TopK             *int              `json:"top_k,omitempty"`
    Stop             []string          `json:"stop,omitempty"`
    Stream           *bool             `json:"stream,omitempty"`
    Tools            []*Tool           `json:"tools,omitempty"`
    ToolChoice       *ToolChoice       `json:"tool_choice,omitempty"`
    Metadata         map[string]string `json:"metadata,omitempty"`
    ResponseFormat   *ResponseFormat   `json:"response_format,omitempty"`
    Thinking         *ThinkingConfig   `json:"thinking,omitempty"`

    // OpenAI 特有 → 通用（Anthropic 无对应时降级丢弃）
    FrequencyPenalty *float64          `json:"frequency_penalty,omitempty"`
    PresencePenalty  *float64          `json:"presence_penalty,omitempty"`
    Seed             *int              `json:"seed,omitempty"`
    N                *int              `json:"n,omitempty"`
    Logprobs         *bool             `json:"logprobs,omitempty"`
    TopLogprobs      *int              `json:"top_logprobs,omitempty"`
    LogitBias        map[string]int    `json:"logit_bias,omitempty"`
    User             string            `json:"user,omitempty"`
    ParallelToolCalls *bool            `json:"parallel_tool_calls,omitempty"`

    // Anthropic 特有 → 通用（OpenAI 无对应时降级丢弃）
    StopSequences    []string          `json:"stop_sequences,omitempty"` // 等价于 Stop
    CacheControl     *CacheControl     `json:"cache_control,omitempty"`
    ServiceTier      string            `json:"service_tier,omitempty"`
    OutputConfig     *OutputConfig     `json:"output_config,omitempty"`
}
```

### Message（通用消息）

```go
type Message struct {
    Role             string        `json:"role"`
    Content          *Content      `json:"content,omitempty"`
    Name             string        `json:"name,omitempty"`
    ToolCalls        []*ToolCall   `json:"tool_calls,omitempty"`
    ToolCallID       string        `json:"tool_call_id,omitempty"`
    ReasoningContent string        `json:"reasoning_content,omitempty"`
    Refusal          string        `json:"refusal,omitempty"`
}

type Content struct {
    Text  string           `json:"-"`
    Parts []*ContentPart   `json:"-"`
}
```

### Tool / ToolCall / ToolChoice

```go
type Tool struct {
    Name        string              `json:"name"`
    Description string              `json:"description,omitempty"`
    Parameters  *JSONSchemaProperty `json:"parameters,omitempty"`
}

type ToolCall struct {
    ID        string `json:"id,omitempty"`
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}

type ToolChoice struct {
    Mode  string // auto / none / required
    Name  string // 指定工具名称（required + name）
    DisableParallelToolUse *bool
}
```

### ChatResponse（通用响应）

```go
type ChatResponse struct {
    ID           string      `json:"id"`
    Model        string      `json:"model"`
    Messages     []*Message  `json:"messages"`
    StopReason   string      `json:"stop_reason"`
    Usage        *Usage      `json:"usage,omitempty"`
}

type Usage struct {
    InputTokens  int `json:"input_tokens"`
    OutputTokens int `json:"output_tokens"`
}
```

### StreamChunk（通用流式块）

```go
type StreamEventType = string
const (
    StreamEventContent    StreamEventType = "content"
    StreamEventToolCall   StreamEventType = "tool_call"
    StreamEventStop       StreamEventType = "stop"
    StreamEventUsage      StreamEventType = "usage"
    StreamEventError      StreamEventType = "error"
)

type StreamChunk struct {
    Type       StreamEventType `json:"type"`
    Content    string          `json:"content,omitempty"`
    ToolCall   *ToolCall       `json:"tool_call,omitempty"`
    StopReason string          `json:"stop_reason,omitempty"`
    Usage      *Usage          `json:"usage,omitempty"`
    Delta      string          `json:"delta,omitempty"` // thinking delta
}
```

## 字段映射规则

### 共享字段（直接映射）

| Common | OpenAI | Anthropic |
|--------|--------|-----------|
| Model | model | model |
| Messages | messages | messages |
| MaxTokens | max_completion_tokens | max_tokens |
| Temperature | temperature | temperature |
| TopP | top_p | top_p |
| Stop | stop | stop_sequences |
| Stream | stream | stream |
| Tools | tools | tools |
| ToolChoice | tool_choice | tool_choice |
| Metadata | metadata | metadata.user_id |

### Role 映射

| OpenAI | Common | Anthropic |
|--------|--------|-----------|
| developer | system | — (合并到 system 字段) |
| system | system | — (合并到 system 字段) |
| user | user | user |
| assistant | assistant | assistant |
| tool | tool | — (转为 tool_result content block) |

**规则**: OpenAI 的 `developer` / `system` 消息 → 转为 Anthropic 的顶层 `system` 字段。
Anthropic 的 `system` 字段 → 转为 OpenAI 的 `system` role 消息。

### FinishReason 映射

| OpenAI | Common | Anthropic |
|--------|--------|-----------|
| stop | stop | end_turn |
| length | length | max_tokens |
| tool_calls | tool_calls | tool_use |
| content_filter | content_filter | — |

### ToolChoice 映射

| OpenAI | Common | Anthropic |
|--------|--------|-----------|
| "none" | none | {"type":"none"} |
| "auto" | auto | {"type":"auto"} |
| "required" | required | {"type":"any"} |
| {"function":{"name":"X"}} | name:X | {"type":"tool","name":"X"} |

### ContentPart 映射

| OpenAI | Common | Anthropic |
|--------|--------|-----------|
| type=text | text | type=text |
| type=image_url | image_url | type=image + source{type=url,url} |
| type=input_audio | input_audio | — (降级丢弃) |
| type=file | file | type=document + source |
| type=refusal | refusal | — (降级丢弃) |

### 不对称字段处理

| 字段 | 方向 | 处理 |
|------|------|------|
| thinking | Anthropic → OpenAI | 映射到 reasoning_effort: low/medium/high |
| reasoning_effort | OpenAI → Anthropic | 映射到 thinking.type=enabled + budget_tokens 估算 |
| cache_control | Anthropic → OpenAI | 映射到 prompt_cache_key |
| prompt_cache_key | OpenAI → Anthropic | 映射到 cache_control.type=ephemeral |
| top_k | Anthropic → OpenAI | 降级丢弃 + warning 日志 |
| logprobs / top_logprobs | OpenAI → Anthropic | 降级丢弃 + warning 日志 |
| seed | OpenAI → Anthropic | 降级丢弃 + warning 日志 |
| n | OpenAI → Anthropic | 降级丢弃 + warning 日志（Anthropic 只能生成 1 个） |
| frequency_penalty / presence_penalty | OpenAI → Anthropic | 降级丢弃 + warning 日志 |
| logit_bias | OpenAI → Anthropic | 降级丢弃 + warning 日志 |
| audio / modalities | OpenAI → Anthropic | 降级丢弃 + warning 日志 |
| prediction | OpenAI → Anthropic | 降级丢弃 + warning 日志 |
| context_management | Anthropic → OpenAI | 降级丢弃 + warning 日志 |
| container / inference_geo | Anthropic → OpenAI | 降级丢弃 + warning 日志 |

## Converter 接口设计

### openai.go

```go
// 请求转换
func OpenAIReqToCommon(req *openai.ChatCompletionReq) (*common.ChatRequest, error)
func CommonReqToOpenAI(req *common.ChatRequest) (*openai.ChatCompletionReq, error)

// 响应转换
func OpenAIRspToCommon(rsp *openai.ChatCompletion) (*common.ChatResponse, error)
func CommonRspToOpenAI(rsp *common.ChatResponse) (*openai.ChatCompletion, error)

// 消息转换
func OpenAIMessageToCommon(msg *openai.ChatCompletionMessageParam) (*common.Message, error)
func CommonMessageToOpenAI(msg *common.Message) (*openai.ChatCompletionMessageParam, error)

// 工具转换
func OpenAIToolToCommon(tool *openai.ChatCompletionTool) *common.Tool
func CommonToolToOpenAI(tool *common.Tool) *openai.ChatCompletionTool
```

### anthropic.go

```go
// 请求转换
func AnthropicReqToCommon(req *anthropic.CreateMessageReq) (*common.ChatRequest, error)
func CommonReqToAnthropic(req *common.ChatRequest) (*anthropic.CreateMessageReq, error)

// 响应转换
func AnthropicRspToCommon(rsp *anthropic.Message) (*common.ChatResponse, error)
func CommonRspToAnthropic(rsp *common.ChatResponse) (*anthropic.Message, error)

// 消息转换
func AnthropicMessageToCommon(msg *anthropic.MessageParam) (*common.Message, error)
func CommonMessageToAnthropic(msg *common.Message) (*anthropic.MessageParam, error)

// System 字段转换
func OpenAISystemToAnthropicSystem(messages []*common.Message) (*anthropic.MessageContent, []*common.Message)
func AnthropicSystemToOpenAIMessages(system *anthropic.MessageContent) []*common.Message
```

### stream_openai.go / stream_anthropic.go

```go
// OpenAI 流式
type OpenAIStreamReader struct { ... }
func NewOpenAIStreamReader(reader io.Reader) *OpenAIStreamReader
func (r *OpenAIStreamReader) Next() (*common.StreamChunk, error)

type OpenAIStreamWriter struct { ... }
func NewOpenAIStreamWriter(w io.Writer) *OpenAIStreamWriter
func (w *OpenAIStreamWriter) Write(chunk *common.StreamChunk) error

// Anthropic 流式
type AnthropicStreamReader struct { ... }
func NewAnthropicStreamReader(reader io.Reader) *AnthropicStreamReader
func (r *AnthropicStreamReader) Next() (*common.StreamChunk, error)

type AnthropicStreamWriter struct { ... }
func NewAnthropicStreamWriter(w io.Writer) *AnthropicStreamWriter
func (w *AnthropicStreamWriter) Write(chunk *common.StreamChunk) error
```

## Service 层改造

模型端点配置已从 YAML (`proxy.GetLLMProxyConfig()`) 迁移到数据库 (`dao.ModelEndpointDAO`)。
`ModelEndpoint` 表结构：`(ID, Alias, Model, Provider, APIKey, BaseURL)`，通过 `(Alias, Provider)` 唯一索引查询。

改造后的请求流程：

```
客户端 (OpenAI) → handler → OpenAIReqToCommon() → common.ChatRequest
  → service 通过 dao.ModelEndpointDAO 查询模型端点，确定目标上游协议
  → 目标是 OpenAI: CommonReqToOpenAI() → 转发
  → 目标是 Anthropic: CommonReqToAnthropic() → 转发

客户端 (Anthropic) → handler → AnthropicReqToCommon() → common.ChatRequest
  → 同上路由逻辑
```

Service 层的核心改造：

```go
// 在 service 层新增统一路由逻辑
func routeRequest(ctx context.Context, req *common.ChatRequest, clientProtocol enum.ProviderType) (*huma.StreamResponse, error) {
    db := database.GetDBInstance(ctx)
    modelEndpointDAO := dao.GetModelEndpointDAO()

    // 优先查找客户端同协议端点
    endpoint, err := modelEndpointDAO.Get(db, &dbmodel.ModelEndpoint{
        Alias: req.Model, Provider: clientProtocol,
    }, []string{"model", "api_key", "base_url"})

    if err == nil {
        // 同协议：直接转发
        return forwardSameProtocol(ctx, req, clientProtocol, endpoint)
    }

    // 尝试其他协议端点
    for _, provider := range []enum.ProviderType{enum.ProviderOpenAI, enum.ProviderAnthropic} {
        if provider == clientProtocol { continue }
        endpoint, err := modelEndpointDAO.Get(db, &dbmodel.ModelEndpoint{
            Alias: req.Model, Provider: provider,
        }, []string{"model", "api_key", "base_url"})
        if err == nil {
            // 跨协议：转换到目标协议转发
            return forwardCrossProtocol(ctx, req, clientProtocol, provider, endpoint)
        }
    }

    return sendModelNotFoundError(req.Model, clientProtocol)
}
```

## 与现有 UnifiedMessage 的关系

现有的 `UnifiedMessage` / `UnifiedTool`（在 `internal/dto/unified_message.go` 和 `unified_tool.go`）专用于 session 存储。改造后：

- **存储场景**: `converter` 提供 `CommonMessageToUnified(msg *common.Message) *dto.UnifiedMessage` 转换函数，将通用消息转为存储格式
- **保持 `FromOpenAIMessage` / `FromAnthropicMessage`**: 这些函数保留但可以改为委托给 converter，减少重复代码
- **MessageStoreTask**: 保持使用 `UnifiedMessage` / `UnifiedTool`，但 service 层从 `common.ChatRequest` 转换而来

## 实施步骤

1. **创建 `internal/dto/common/` 包**: 定义通用 DTO 结构体（使用 `sonic.NoCopyRawMessage` 替代 `json.RawMessage`）
2. **创建 `internal/dto/openai/` 包**: 从现有 `openai.go` 拆分
3. **创建 `internal/dto/anthropic/` 包**: 从现有 `anthropic.go` 拆分
4. **创建 `internal/converter/` 包**: 实现 OpenAI 和 Anthropic 的双向转换
5. **实现流式转换器**: StreamReader / StreamWriter
6. **改造 service 层**: 引入通用路由逻辑，通过 `dao.ModelEndpointDAO` 查询端点，支持跨协议转发
7. **适配 unified_message**: 增加 Common → Unified 的转换函数
8. **更新所有 import 引用**: 确保 handler、router 等层的引用正确
9. **编写测试**: 转换器的单元测试
