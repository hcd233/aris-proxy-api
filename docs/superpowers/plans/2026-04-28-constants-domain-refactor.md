# 常量按领域重组 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `internal/common/constant/` 下按类型拆分的 8 个文件重组为按领域拆分的 19 个文件，消除 `string.go` 大杂烩。

**Architecture:** 所有文件同属 `package constant`，常量名和包名均不变，外部无需修改 import。先清理旧文件（移除迁移走的常量），再创建新领域文件，最后验证编译和测试。

**Tech Stack:** Go 1.25, `internal/common/constant`

---

### 关键注意事项

- 所有常量都在 `package constant` 下，Go 不允许同一包内重复声明常量名
- 操作顺序必须是：**先修改旧文件移除常量 → 再创建新文件**，反之会编译冲突
- 本 refactoring 不改变常量名和包名，外部所有引用自动有效
- 操作中包会短暂处于不可编译状态，最终一次性恢复

### Task 1: 清理旧文件 - 移除被迁移走的常量

**Files:**
- Modify: `internal/common/constant/string.go` — 保留通用常量，移除所有领域常量的 section
- Modify: `internal/common/constant/http.go` — 保留 HTTP 连接池常量
- Modify: `internal/common/constant/database.go` — 保留连接池常量和 COS 常量
- Modify: `internal/common/constant/agent.go` — 移除 session 配置常量
- Modify: `internal/common/constant/number.go` — 全部移除（待删除）
- Modify: `internal/common/constant/rate.go` — 全部移除（待删除）
- Modify: `internal/common/constant/time.go` — 全部移除（待删除）

- [ ] **Step: 读取所有旧文件当前内容**

```bash
cat internal/common/constant/string.go | wc -l
cat internal/common/constant/number.go | wc -l
cat internal/common/constant/rate.go | wc -l
cat internal/common/constant/time.go | wc -l
cat internal/common/constant/http.go | wc -l
cat internal/common/constant/database.go | wc -l
cat internal/common/constant/agent.go | wc -l
```

Expected: 确认各文件当前行数。

- [ ] **Step: 重写 `string.go`** — 删除所有领域 section（SSE、HTTP Header、Redis Key、API Key、上游 API 路径/版本、CORS、路由路径、存储路径、数据格式、响应审计、LLM 内部错误、第三方 API URL、OAuth2、用户默认值、限流、日志文件名、Cron、Database、OAuth Provider、安全、CLS 级别/字段、Guard），只保留通用工具性常量：

保留的常量列表：
- `ProjectName`
- `FormatDefault`, `FormatDecimal`, `FormatFloatCompact`
- `TruncateSuffixPrefix`, `TruncateSuffixPostfix`
- `NewlineString`, `NewlineCRLF`
- `ZeroString`, `OneString`, `NullJSONLiteral`
- `QuoteString`, `ColonMessageTemplate`
- `JSONSchemaTypeString`, `JSONSchemaTypeNumber`, `JSONSchemaTypeBoolean`, `JSONSchemaTypeArray`, `JSONSchemaTypeObject`
- `HostPortTemplate`, `PostgresDSNTemplate`, `DefaultFormatJSON`
- `DataURLTemplate`, `DataURLPrefix`, `DataURLBase64Separator`, `Base64SourceType`, `URLSourceType`
- `DigNameAccessSigner`, `DigNameRefreshSigner`
- `OpenAPISchemasPrefix`, `OpenAPIDocsPath`, `OpenAPISchemasPath`

```go
package constant

const (
	ProjectName = "aris-proxy-api"

	FormatDefault             = "%v"
	FormatDecimal             = "%d"
	FormatFloatCompact        = "%g"
	TruncateSuffixPrefix      = "...(truncated, total "
	TruncateSuffixPostfix     = " chars)"
	NewlineString             = "\n"
	NewlineCRLF               = "\r\n"
	ZeroString                = "0"
	OneString                 = "1"
	NullJSONLiteral           = "null"
	QuoteString               = "\""
	ColonMessageTemplate      = ": %s"
	JSONSchemaTypeString      = "string"
	JSONSchemaTypeNumber      = "number"
	JSONSchemaTypeBoolean     = "boolean"
	JSONSchemaTypeArray       = "array"
	JSONSchemaTypeObject      = "object"
	HostPortTemplate          = "%s:%s"
	PostgresDSNTemplate       = "host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Shanghai"
	DefaultFormatJSON         = "application/json"
	DataURLTemplate           = "data:%s;base64,%s"
	DataURLPrefix             = "data:"
	DataURLBase64Separator    = ";base64,"
	Base64SourceType          = "base64"
	URLSourceType             = "url"
	DigNameAccessSigner       = "accessSigner"
	DigNameRefreshSigner      = "refreshSigner"
	OpenAPISchemasPrefix      = "#/components/schemas/"
	OpenAPIDocsPath           = "/openapi"
	OpenAPISchemasPath        = "/schemas"

	ParseFloat64BitSize       = 64
	DecimalBase               = 10
	GoCommand                 = "go"
	GoVetCommand              = "vet"
	GoAllPackagesPattern      = "./..."
	StaticcheckCommand        = "staticcheck"
	StaticChecksFailedMessage = "static checks failed"
)
```

- [ ] **Step: 重写 `http.go`** — 加入来自 `string.go` 的 HTTP Header/Content-Type 常量和来自 `time.go` 的 HTTP 超时常量：

```go
package constant

import "time"

const (
	HTTPMaxIdleConns        = 100
	HTTPMaxIdleConnsPerHost = 20
	HTTPClientTimeout       = 5 * time.Minute
	HTTPDialTimeout         = 10 * time.Second
	HTTPKeepAlive           = 30 * time.Second
	HTTPTLSHandshakeTimeout = 10 * time.Second
	HTTPResponseHeaderTimeout = 30 * time.Second
	HTTPIdleConnTimeout     = 90 * time.Second

	HTTPHeaderContentType       = "Content-Type"
	HTTPHeaderAuthorization     = "Authorization"
	HTTPHeaderAPIKey            = "x-api-key"
	HTTPHeaderAnthropicVersion  = "anthropic-version"
	HTTPHeaderCacheControl      = "Cache-Control"
	HTTPHeaderConnection        = "Connection"
	HTTPHeaderTransferEncoding  = "Transfer-Encoding"
	HTTPHeaderXAccelBuffering   = "X-Accel-Buffering"
	HTTPHeaderUserAgent         = "User-Agent"
	HTTPHeaderLastModified      = "Last-Modified"
	HTTPHeaderETag              = "ETag"
	HTTPHeaderTraceID           = "X-Trace-Id"
	HTTPHeaderXRateLimitLimit   = "X-RateLimit-Limit"
	HTTPHeaderXRateLimitRemaining = "X-RateLimit-Remaining"
	HTTPHeaderRetryAfter        = "Retry-After"

	HTTPAuthBearerPrefix        = "Bearer "
	HTTPContentTypeJSON         = "application/json"
	HTTPContentTypeEventStream  = "text/event-stream"
	HTTPContentDispositionParam = "response-content-disposition"
	HTTPContentTypeParam        = "response-content-type"
	HTTPAttachmentFilenameTemplate = "attachment; filename=%q"
	HTTPCacheControlNoCache     = "no-cache"
	HTTPConnectionKeepAlive     = "keep-alive"
	HTTPTransferEncodingChunked = "chunked"
	HTTPHeaderDisabled          = "no"

	MIMETypeOctetStream = "application/octet-stream"

	CORSAllowOrigins     = "http://localhost:3000"
	CORSPreflightMaxAge  = 12 * time.Hour

	IdleTimeout          = 2 * time.Minute
	ShutdownTimeout      = 60 * time.Second
	FiberShutdownTimeout = 30 * time.Second
)
```

- [ ] **Step: 重写 `database.go`** — 加入来自 `string.go` 的 DB 字段名、查询条件、聚合根类型，来自 `time.go` 的 PostgresConnMaxLifetime：

```go
package constant

import "time"

const (
	PostgresMaxIdleConns = 10
	PostgresMaxOpenConns = 100
	PostgresConnMaxLifetime = 5 * time.Hour

	DBFieldID        = "id"
	DBFieldSummary   = "summary"
	DBFieldScoreVersion = "score_version"
	DBFieldMessageIDs = "message_ids"
	DBFieldToolIDs   = "tool_ids"
	DBFieldMessage   = "message"
	DBFieldCheckSum  = "check_sum"
	DBFieldUserID    = "user_id"
	DBFieldName      = "name"
	DBFieldPermission = "permission"
	DBFieldModel     = "model"
	DBFieldCreatedAt = "created_at"
	DBFieldTool      = "tool"
	DBFieldUpdatedAt = "updated_at"
	DBFieldDeletedAt = "deleted_at"

	DBConditionDeletedAtZero      = "deleted_at = 0"
	DBConditionDeletedAtNotZero   = "deleted_at != 0"
	DBConditionInTemplate         = "%s IN ?"

	AggregateTypeEndpoint       = "llmproxy.endpoint"
	AggregateTypeAPIKey         = "apikey.proxy_api_key"
	AggregateTypeUser           = "identity.user"
	AggregateTypeOAuthIdentity  = "oauth2.identity"
	AggregateTypeModelCallAudit = "modelcall.audit"
	AggregateTypeMessage        = "conversation.message"
	AggregateTypeTool           = "conversation.tool"
	AggregateTypeSession        = "session.session"
)
```

- [ ] **Step: 重写 `agent.go`** — 保留 Agent 名称/描述/指令，移除 session 配置（SummarizeMaxRetries、SummarizeMaxTokens、ScoreMaxRetries、ScoreMaxTokens、ScoreVersion）：

```go
package constant

const (
	SessionSummarizerAgentName        = "SessionSummarizer"
	SessionSummarizerAgentDescription = "Summarize the session content into a concise summary."
	SessionSummarizerAgentInstruction = `# 角色定义
你是一个专业的对话总结助手。你的唯一任务是将对话内容转化为简洁的中文摘要。

## 任务描述
分析提供的对话内容，提取核心主题，生成一段简短的总结。

## 输出规范
- **语言**: 必须且只能使用简体中文
- **长度**: 严格控制在 5-10 个中文字符
- **格式**: 纯文本，禁止添加任何标点符号、前缀或后缀
- **内容**: 准确捕捉对话的核心主题或目的

## 禁止事项
- 禁止使用英文、日文或其他任何非中文语言
- 禁止输出解释、分析过程或额外说明
- 禁止使用引号、括号或其他标点符号包裹输出
- 禁止输出"总结:"、"摘要:"等前缀

## 示例输入
用户: 你好，请问怎么学习Go语言？
助手: 建议从官方文档开始，然后实践项目...

## 示例输出
Go语言学习方法

## 执行指令
直接输出总结内容，不要有任何其他内容。`

	SessionScorerAgentName        = "SessionScorer"
	SessionScorerAgentDescription = "Evaluate the quality of a conversation session across three dimensions: coherence, depth, and value."
	SessionScorerAgentInstruction = `# 角色定义
你是一个专业的对话质量评估专家。你的任务是分析对话内容，从三个维度对对话质量进行客观评分。

## 评分维度

### 1. 连贯性 (Coherence)
评估对话的逻辑连贯程度：
- 对话是否围绕明确主题展开
- 上下文是否保持一致
- 是否存在明显的逻辑跳跃或主题漂移
- 用户和AI之间的交流是否相互呼应

评分标准：
- 1-3分：对话主题混乱，上下文不连贯
- 4-6分：对话基本连贯，偶有主题偏移
- 7-8分：对话逻辑清晰，上下文保持较好
- 9-10分：对话非常连贯，逻辑严密，上下文完全一致

### 2. 深度 (Depth)
评估对话的思考深度和信息量：
- 对话涉及的问题复杂程度
- 是否包含深入的分析和推理
- 是否探讨了多个层面的内容
- AI回复是否有深度见解

评分标准：
- 1-3分：对话浅显，仅涉及表面信息
- 4-6分：对话有一定深度，涉及一些分析
- 7-8分：对话深入，包含复杂推理和分析
- 9-10分：对话非常有深度，探讨了问题的多个层面

### 3. 价值 (Value)
评估对话的实际价值和问题解决程度：
- 是否解决了用户的问题或需求
- 对话是否产生了实用信息
- 用户是否获得了有价值的帮助
- 对话结果的完整性

评分标准：
- 1-3分：对话价值低，未解决用户问题
- 4-6分：对话有一定价值，部分解决问题
- 7-8分：对话价值较高，有效解决了用户问题
- 9-10分：对话非常有价值，完美解决用户需求

## 输出格式
你必须严格按照以下JSON格式输出评分结果，不要添加任何其他内容：

{
  "coherence": <1-10的整数>,
  "depth": <1-10的整数>,
  "value": <1-10的整数>
}

## 示例输出
{"coherence":8,"depth":7,"value":9}

## 执行指令
直接输出JSON格式的评分结果，不要有任何解释、前缀或后缀。`
)
```

- [ ] **Step: 重写 `number.go` 为空（待删除）** — 文件体只保留 package 声明：

```go
package constant
```

- [ ] **Step: 重写 `rate.go` 为空（待删除）**：

```go
package constant
```

- [ ] **Step: 重写 `time.go` 为空（待删除）**：

```go
package constant
```

### Task 2: 创建新领域常量文件

**Files:** Create 11 new files.

- [ ] **Step: 创建 `route.go`**

```go
package constant

const (
	RoutePathRoot                          = "/"
	RoutePathHealth                        = "/health"
	RoutePathSSEHealth                     = "/ssehealth"
	RoutePathTokenRefresh                  = "/refresh"
	RoutePathUserCurrent                   = "/current"
	RoutePathSessionList                   = "/list"
	RoutePathOAuthLogin                    = "/login"
	RoutePathOAuthCallback                 = "/callback"
	RoutePathModels                        = "/models"
	RoutePathAnthropicMessages             = "/messages"
	RoutePathAnthropicMessagesCountTokens  = "/messages/count_tokens"
	RoutePathOpenAIChatCompletions         = "/chat/completions"
	RoutePathAPIKeyByID                    = "/{id}"
	RoutePathFavicon                       = "/favicon.ico"
	RoutePathRobots                        = "/robots.txt"
	RoutePathAppleTouchIcon                = "/apple-touch-icon.png"
	RoutePathAppleTouchIconPrecomposed     = "/apple-touch-icon-precomposed.png"
	RoutePathWellKnownSecurity             = "/.well-known/security.txt"
)
```

- [ ] **Step: 创建 `sse.go`**

```go
package constant

import "time"

const (
	SSEHeartbeatCount = 30
	HeartbeatInterval = 1 * time.Second

	SSEDataPrefix  = "data: "
	SSEDoneSignal  = "[DONE]"
	SSEEventPrefix = "event: "

	AnthropicMessageStopSSEFrame = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"

	SSEDataFrameTemplate       = "data: %s\n\n"
	SSEEventFrameTemplate      = "event: %s\ndata: %s\n\n"
	SSEEventLineTemplate       = "event: %s\n"
	SSEDataLineTemplate        = "data: %s\n\n"
	SSEOpenAIUpstreamErrorFrame = "data: {\"error\":{\"message\":\"upstream returned status %d\",\"type\":\"server_error\",\"code\":\"upstream_error\"}}\n\n"
	SSEOpenAIInternalErrorFrame = "data: {\"error\":{\"message\":\"internal server error\",\"type\":\"server_error\",\"code\":\"internal_error\"}}\n\n"
)
```

- [ ] **Step: 创建 `upstream.go`**

```go
package constant

const (
	UpstreamPathOpenAIChatCompletions  = "/chat/completions"
	UpstreamPathOpenAIResponses        = "/responses"
	UpstreamPathAnthropicMessages      = "/messages"
	UpstreamPathAnthropicCountTokens   = "/messages/count_tokens"

	AnthropicAPIVersion = "2023-06-01"

	AnthropicMessageIDTemplate = "msg_%s"
	OpenAIChunkIDTemplate      = "chatcmpl-%s"

	OpenAIInvalidRequestErrorType = "invalid_request_error"
	OpenAIModelNotFoundCode       = "model_not_found"
	OpenAIModelNotFoundMessageTemplate = "The model `%s` does not exist"
	OpenAIInternalErrorShortMessage    = "Internal error"
	OpenAIInternalErrorMessage         = "Internal server error"
	OpenAIInternalErrorType            = "server_error"
	OpenAIInternalErrorCode            = "internal_error"

	AnthropicNotFoundErrorType          = "not_found_error"
	AnthropicModelNotFoundMessageTemplate = "model: %s"
	AnthropicInternalErrorMessage       = "Internal server error"
	AnthropicInternalErrorType          = "api_error"
	AnthropicInternalErrorBodyType      = "error"

	UpstreamErrorType              = "upstream_error"
	UpstreamStatusMessageTemplate  = "Upstream returned status %d"

	CallStatusSuccess        = 200
	CallStatusConnectionError = -1
	CallStatusUnknownError   = 0

	ResponseFailedAuditReason            = "response.failed"
	ResponseFailedAuditReasonTemplate     = "response.failed: %s"
	ResponseIncompleteAuditReason         = "response.incomplete"
	ResponseIncompleteAuditReasonTemplate = "response.incomplete: %s"
)
```

- [ ] **Step: 创建 `apikey.go`**

```go
package constant

import "time"

const (
	APIKeyPrefix     = "sk-aris-"
	APIKeyCharset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	APIKeyRandomLength = 24
	APIKeyMaxCount   = 5

	PeriodManageAPIKey = 1 * time.Minute
	LimitManageAPIKey  = 20
)
```

- [ ] **Step: 创建 `oauth.go`**

```go
package constant

import "time"

const (
	OAuthProviderGithub = "github"
	OAuthProviderGoogle = "google"

	GithubUserURL      = "https://api.github.com/user"
	GithubUserEmailURL = "https://api.github.com/user/emails"
	GoogleUserInfoURL  = "https://www.googleapis.com/oauth2/v2/userinfo"

	DefaultUserNamePrefix = "ArisUser"

	OAuthStateBytes = 32

	PeriodOAuth2Callback = 5 * time.Second
	LimitOAuth2Callback  = 16

	OAuthStateManagerTTL       = 10 * time.Minute
	OAuthStateCleanupInterval  = 5 * time.Minute
)
```

- [ ] **Step: 创建 `rediskey.go`**

```go
package constant

const (
	LockKeyTemplateMiddleware = "%s:%s:%v"
	JWTUserCacheKeyTemplate   = "jwt:user:%d"
	TokenBucketKeyTemplate    = "tb:%s:%s:%v"
	ScannerBanKeyTemplate     = "scanner:ban:%s"
	ScannerStrikeKeyTemplate  = "scanner:strike:%s"
)
```

- [ ] **Step: 创建 `session.go`**

```go
package constant

const (
	SummarizeMaxRetries = 3
	SummarizeMaxTokens  = 20

	ScoreMaxRetries = 3
	ScoreMaxTokens  = 200
	ScoreVersion    = "v1.0.0"

	EmptySessionSummary = "空会话"

	CronModuleSessionSummarize    = "SessionSummarizeCron"
	CronModuleSessionScore        = "SessionScoreCron"
	CronModuleSessionDeduplicate  = "SessionDeduplicateCron"

	CronSpecSessionSummarize    = "0 2 * * *"
	CronSpecSessionScore        = "0 3 * * *"
	CronSpecSessionDeduplicate  = "0 1 * * *"
)
```

- [ ] **Step: 创建 `cron.go`**

```go
package constant

const (
	CronDefaultModule  = "Cron"
	CronInvalidKey     = "invalid_key"
	CronModuleSoftDeletePurge = "SoftDeletePurgeCron"
	CronSpecSoftDeletePurge   = "0 4 * * 0"
)
```

- [ ] **Step: 创建 `ratelimit.go`**

```go
package constant

import "time"

const (
	PeriodCallProxyLLM = 1 * time.Second
	LimitCallProxyLLM  = 100

	PeriodRefreshToken = 1 * time.Minute
	LimitRefreshToken  = 10

	RateLimitKeyByIP = "ip"
)
```

- [ ] **Step: 创建 `guard.go`**

```go
package constant

import "time"

const (
	GuardStrikeThreshold = 5
	GuardStrikeWindow    = 1 * time.Minute
	GuardBanDuration     = 1 * time.Hour
)
```

- [ ] **Step: 创建 `log.go`**

```go
package constant

import "time"

const (
	LogMiddlewareSamplingInterval = 5 * time.Minute
	LogFieldValueMaxLength = 512

	LogInfoMaxSizeMB    = 100
	LogInfoMaxBackups   = 3
	LogInfoMaxAgeDays   = 7
	LogErrMaxSizeMB     = 500
	LogErrMaxBackups    = 3
	LogErrMaxAgeDays    = 30

	LogInfoFileName   = "aris-proxy-api.log"
	LogErrFileName    = "aris-proxy-api-error.log"
	LogPanicFileName  = "aris-proxy-api-panic.log"

	CLSLevelDebug = "DEBUG"
	CLSLevelInfo  = "INFO"
	CLSLevelWarn  = "WARN"
	CLSLevelError = "ERROR"
	CLSFieldMessage   = "message"
	CLSFieldLevel     = "level"
	CLSFieldTimestamp = "timestamp"
	CLSFieldCaller    = "caller"
	CLSFieldStack     = "stack"

	CLSProducerCloseTimeoutMs = 10000
)
```

- [ ] **Step: 创建 `objectstorage.go`**

```go
package constant

import "time"

const (
	CosListObjectsMaxKeys = 1000
	ObjectStorageDirTemplate = "user-%d/%s"
	ObjectStorageNotConfiguredMessage = "no object storage configured"
	ObjectStorageUnsupportedMessage   = "unsupported storage type"
	COSBucketURLTemplate = "https://%s-%s.cos.%s.myqcloud.com"
	PresignObjectExpire  = 5 * time.Minute
)
```

- [ ] **Step: 创建 `security.go`**

```go
package constant

const (
	MaskSecretMinLength    = 8
	MaskSecretPlaceholder  = "***"
	MaskSecretTemplate     = "%s***%s"
	ByteMax                = 256
)
```

- [ ] **Step: 创建 `conversation.go`**

```go
package constant

const (
	MessageFormatRole          = "Role: %s"
	MessageFormatName          = "Name: %s"
	MessageFormatContent       = "Content: %s"
	MessageFormatContentText   = "Content[text]: %s"
	MessageFormatContentImage  = "Content[image]: %s"
	MessageFormatContentAudio  = "Content[audio]: %s"
	MessageFormatContentFile   = "Content[file]: %s"
	MessageFormatContentRefusal = "Content[refusal]: %s"
	MessageFormatReasoning     = "Reasoning: %s"
	MessageFormatToolCall      = "ToolCall: %s(%s)"
	MessageFormatToolCallID    = "ToolCallID: %s"
	MessageFormatRefusal       = "Refusal: %s"
	MessageContentSeparator    = " | "
)
```

### Task 3: 清理废弃文件 & 验证编译

- [ ] **Step: 删除空文件**

```bash
rm internal/common/constant/number.go
rm internal/common/constant/rate.go
rm internal/common/constant/time.go
```

- [ ] **Step: 验证编译**

```bash
go vet ./internal/common/constant/...
```

Expected: 无编译错误（空输出）。

- [ ] **Step: 临时保留 `ctx.go` 不动**

已确认 `ctx.go` 不参与迁移，保持不变。

### Task 4: 全量验证

- [ ] **Step: 运行全量测试**

```bash
go test -count=1 ./internal/common/constant/...
```

Expected: PASS.

- [ ] **Step: 运行全量测试**

```bash
go test -count=1 ./...
```

Expected: 全部 PASS.

- [ ] **Step: 运行自定义 lint**

```bash
make lint
```

Expected: 无错误.
