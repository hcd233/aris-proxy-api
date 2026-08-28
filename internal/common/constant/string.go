package constant

import "time"

const (
	ProjectName = "aris-proxy-api"

	// ── 字符串模板（含 Printf/格式化占位符）──
	FormatDefault        = "%v"
	FormatDecimal        = "%d"
	FormatFloatCompact   = "%g"
	ColonMessageTemplate = ": %s"
	HostPortTemplate     = "%s:%s"

	PostgresDSNTemplate = "host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Shanghai"
	DataURLTemplate     = "data:%s;base64,%s"

	TruncateSuffixPrefix  = "...(truncated, total "
	TruncateSuffixPostfix = " chars)"

	// ── 运行时字符串字面量（无格式化占位符）──
	NewlineCRLF            = "\r\n"
	DoubleNewline          = "\n\n"
	ZeroString             = "0"
	OneString              = "1"
	NullJSONLiteral        = "null"
	EmptyJSONObject        = "{}"
	QuoteString            = "\""
	DefaultFormatJSON      = "application/json"
	DataURLPrefix          = "data:"
	DataURLBase64Separator = ";base64,"
	// ── dig 容器命名 ──
	DigNameApplicationModule = "application"
	DigNameCronModule        = "cron"
	DigNameHandlerModule     = "handler"
	DigNameInfraModule       = "infrastructure"
	DigNameRepositoryModule  = "repository"

	// ── OpenAPI 路径 ──
	OpenAPISchemasPrefix = "#/components/schemas/"
	OpenAPIDocsPath      = "/openapi"
	OpenAPISchemasPath   = "/schemas"

	// ── 数值常量 / 构建命令 ──
	ParseFloat64BitSize       = 64
	DecimalBase               = 10
	GoCommand                 = "go"
	GoVetCommand              = "vet"
	GoAllPackagesPattern      = "./..."
	StaticcheckCommand        = "staticcheck"
	GolangciLintCommand       = "golangci-lint"
	GolangciLintRunCommand    = "run"
	StaticChecksFailedMessage = "static checks failed"
	GoEnvCommand              = "env"
	GoEnvKeyGOPATH            = "GOPATH"
	GobinEnvKey               = "GOBIN"
	GopathBinSubDir           = "bin"
	GopathBinFileMode         = 0o111

	// OpenAPI / Huma configuration
	OpenAPIVersion       = "3.1.0"
	APITitle             = "Aris API Tmpl"
	APIDescription       = "Aris API Tmpl is a RESTful API Template."
	APIVersion           = "1.0"
	ContactName          = "hcd233"
	ContactEmail         = "lvlvko233@qq.com"
	ContactURL           = "https://github.com/hcd233"
	LicenseName          = "Apache 2.0"
	LicenseURL           = "https://www.apache.org/licenses/LICENSE-2.0.html"
	SecuritySchemeJWT    = "jwtAuth"
	SecuritySchemeAPIKey = "apiKeyAuth"
	SecurityTypeAPIKey   = "apiKey"
	SecurityTypeHTTP     = "http"
	HeaderAuthorization  = "Authorization"
	SecurityInHeader     = "header"
	SecuritySchemeBearer = "bearer"
	JWTDescription       = "JWT Authentication，Please pass the JWT token in the Authorization header."
	APIKeyDescription    = "API Key Authentication, Please pass the API Key as Bearer token in the Authorization header."

	// OpenAI protocol object types
	// OpenAI list models response fields
	OpenAIListObject   = "list"
	OpenAIModelObject  = "model"
	OpenAIModelOwnedBy = "openai"

	// Anthropic protocol type fields
	AnthropicMessageType = "message"
	AnthropicModelType   = "model"

	// Ping status
	PingStatusOK = "ok"

	// Logger console encoder config
	LoggerConsoleSeparator = "  "

	// CORS middleware config
	CORSAllowMethods  = "GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS"
	CORSAllowHeaders  = "Origin,Content-Type,Accept,Authorization,X-Requested-With,X-Trace-Id"
	CORSExposeHeaders = "Content-Length"

	// Fallback JSON map key for parse errors
	FallbackJSONRawKey = "raw"

	FieldNameID    = "id"
	FieldNameModel = "model"

	GithubScopeUserEmail = "user:email"
	GithubScopeRepo      = "repo"
	GithubScopeReadOrg   = "read:org"

	GoogleScopeOpenID          = "openid"
	GoogleScopeProfile         = "profile"
	GoogleScopeEmail           = "email"
	GoogleScopeUserInfoProfile = "https://www.googleapis.com/auth/userinfo.profile"
	GoogleScopeUserInfoEmail   = "https://www.googleapis.com/auth/userinfo.email"

	UserNameBlacklistAdmin         = "admin"
	UserNameBlacklistRoot          = "root"
	UserNameBlacklistAdministrator = "administrator"
	UserNameBlacklistSuperuser     = "superuser"
	UserNameBlacklistMe            = "me"

	// Error message templates
	ErrorModelTemplate              = "code: %d, message: %s"
	UpstreamErrorTemplate           = "upstream returned status %d"
	UpstreamConnectionErrorTemplate = "upstream connection error: %v"
	UpstreamConnectionErrorMsg      = "upstream connection error"

	// Endpoint/Model DB field names
	FieldEndpointName                        = "name"
	FieldEndpointOpenaiBaseURL               = "openai_base_url"
	FieldEndpointAnthropicBaseURL            = "anthropic_base_url"
	FieldEndpointAPIKey                      = "api_key"
	FieldEndpointSupportOpenAIChatCompletion = "support_openai_chat_completion"
	FieldEndpointSupportOpenAIResponse       = "support_openai_response"
	FieldEndpointSupportAnthropicMessage     = "support_anthropic_message"
	FieldModelAlias                          = "alias"
	FieldModelUpstreamModel                  = "upstream_model"
	FieldModelEndpointID                     = "endpoint_id"
	FieldModelEnabled                        = "enabled"
	FieldModelContextLength                  = "context_length"
	FieldModelMaxOutputTokens                = "max_output_tokens"
	FieldModelCapabilities                   = "capabilities"

	// Router tag names
	TagAnthropic = "Anthropic"
	TagAPIKey    = "APIKey"
	TagAudit     = "Audit"
	TagCron      = "Cron"
	TagCronAudit = "CronAudit"
	TagClient    = "Client"
	TagDemo      = "Demo"
	TagEndpoint  = "Endpoint"
	TagHealth    = "Health"
	TagModel     = "Model"
	TagMonitor   = "Monitor"
	TagOpenAI    = "OpenAI"
	TagSession   = "Session"
	TagUpstream  = "Upstream"
	TagTrace     = "Trace"
	TagTrigger   = "Trigger"
	TagDataset   = "Dataset"
	TagUser      = "User"

	// Router sub-paths
	RoutePathList = "/list"

	// ClientModelsAPIPrefix / ClientModelsRoutePath 客户端模型分发接口的路径契约：
	// 服务端以 ClientModelsRoutePath 为组内路径注册，客户端以二者拼接后的
	// ClientModelsListPath 请求。任一改名只需改这里，避免两侧脱节（#165 曾因
	// 只改 SDK 路径、未改 group 前缀导致 /api/v1/client/model/list 而 404）。
	// 2026-08-27 路由分区治理：客户端路由归入 CLI 分区，前缀改为 /api/cli/v1。
	ClientModelsAPIPrefix = "/api/cli/v1"
	ClientModelsRoutePath = "/model/list"

	// Router path/query/field constants
	WhereIDEquals = "id = ?"

	// ── Session delete error messages ──
	SessionDeleteErrorFindFailed   = "failed to find session"
	SessionDeleteErrorNotFound     = "session not found"
	SessionDeleteErrorNoPermission = "no permission"
	SessionDeleteErrorDeleteFailed = "failed to delete"

	// ── Trace delete error messages ──
	TraceDeleteErrorFindFailed   = "failed to find trace"
	TraceDeleteErrorNotFound     = "trace not found"
	TraceDeleteErrorNoPermission = "no permission"
	TraceDeleteErrorDeleteFailed = "failed to delete"

	// ── Session option values ──
	SessionOptionScoreValueUnscored = "unscored"

	// ── Trigger Word constants ──
	TriggerContentFilterMessage   = "Content blocked by policy"
	TriggerAuditRemark            = "deny by trigger word"
	TriggerAuditRemarkTemplate    = "deny by trigger word: %s"
	TriggerCaptureAuditRemark     = "capture context by trigger word"
	TriggerCaptureAuditTemplate   = "capture context by trigger word: %s"
	TriggerCaptureSavedReply      = "Context saved."
	TriggerCaptureEmptyReply      = "No conversation history to save."
	TriggerWordSeparator          = ", "
	TriggerTableName              = "trigger_words"
	TriggerHitKeyPrefix           = "trigger:hit:%d"
	TriggerHitKeyScanPattern      = "trigger:hit:*"
	TriggerVersionKey             = "trigger:version"
	TriggerChangeChannel          = "trigger:changed"
	TriggerChangeMessage          = "changed"
	TriggerLowFreqRebuildInterval = 5 * time.Minute
	TriggerVersionPollInterval    = 2 * time.Second

	// ── Cron Pub/Sub ──
	CronReloadChannel = "cron:reload"

	// ── Demo account ──
	DemoConfigTableName   = "demo_configs"
	DemoSessionTableName  = "demo_sessions"
	DemoConfigSingletonID = 1
	// DemoSessionMaxPageSize demo 会话白名单列表单页上限（与 ListDemoSessionsReq pageSize maximum 一致）
	DemoSessionMaxPageSize = 100

	// ── Demo access audit ──
	DemoAccessAuditTableName = "demo_access_audits"
	// 拒绝原因
	DemoAccessReasonLoginDisabled = "login_disabled"
	DemoAccessReasonNoDemoUser    = "no_demo_user"
	DemoAccessReasonModuleClosed  = "module_closed"

	// ── Locale ──
	LocaleEmbedDir        = "locales"
	LocaleFileExt         = ".json"
	LocaleDefaultLocale   = "en"
	LocalePrimaryZH       = "zh"
	LocalePrimaryJA       = "ja"
	LocaleAcceptSeparator = "-_;"

	// ── Dataset export SSE ──
	ShareGPTFromSystem      = "system"
	ShareGPTFromUser        = "user"
	ShareGPTFromAssistant   = "assistant"
	ShareGPTFromFunction    = "function"
	ThinkTagClose           = "\n"
	DatasetExportEventStart = "start"
	DatasetExportEventData  = "data"
	DatasetExportEventDone  = "done"
	DatasetExportEventError = "error"

	ContentPartTypeText = "text"

	// ── Custom tool conversion ──
	CustomToolParamContent   = "content"
	CustomToolFormatDefault  = "text"
	CustomToolFormatLabelFmt = "\n\nExpected format (%s grammar):\n"

	// ── Dataset export progress ──
	DatasetExportNoMatchError = "no sessions match the filter"
	DatasetExportProgressFull = 100
)
