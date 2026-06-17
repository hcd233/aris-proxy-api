package constant

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
	ZeroString             = "0"
	OneString              = "1"
	NullJSONLiteral        = "null"
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
	FieldModelModelName                      = "model"
	FieldModelEndpointID                     = "endpoint_id"
	FieldModelEnabled                        = "enabled"

	// Router tag names
	TagAnthropic = "Anthropic"
	TagAPIKey    = "APIKey"
	TagAudit     = "Audit"
	TagCron      = "Cron"
	TagCronAudit = "CronAudit"
	TagEndpoint  = "Endpoint"
	TagHealth    = "Health"
	TagModel     = "Model"
	TagOpenAI    = "OpenAI"
	TagSession   = "Session"
	TagBlocked   = "Blocked"

	// Router sub-paths
	RoutePathList       = "/list"
	RoutePathOptionList = "/option/list"

	// Router path/query/field constants
	WhereIDEquals = "id = ?"

	// ── Session delete error messages ──
	SessionDeleteErrorFindFailed   = "failed to find session"
	SessionDeleteErrorNotFound     = "session not found"
	SessionDeleteErrorNoPermission = "no permission"
	SessionDeleteErrorDeleteFailed = "failed to delete"

	// ── Session option values ──
	SessionOptionScoreValueUnscored = "unscored"

	// ── Blocked Word constants ──
	BlockedContentBlockedErrorMessage = "ContentBlocked"
	BlockedContentBlockedErrorType    = "forbidden"
	BlockedContentBlockedErrorCode    = "content_blocked"
	BlockedAuditRemark                = "trigger blocked word"
	BlockedAuditRemarkTemplate        = "trigger blocked word: %s"
	BlockedWordSeparator              = ", "
	BlockedTableName                  = "blocked_words"
	BlockedHitKeyPrefix               = "blocked:hit:%d"
	BlockedHitKeyScanPattern          = "blocked:hit:*"
)
