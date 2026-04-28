package constant

const (
	ProjectName = "aris-proxy-api"

	FormatDefault          = "%v"
	FormatDecimal          = "%d"
	FormatFloatCompact     = "%g"
	TruncateSuffixPrefix   = "...(truncated, total "
	TruncateSuffixPostfix  = " chars)"
	NewlineString          = "\n"
	NewlineCRLF            = "\r\n"
	ZeroString             = "0"
	OneString              = "1"
	NullJSONLiteral        = "null"
	QuoteString            = "\""
	ColonMessageTemplate   = ": %s"
	JSONSchemaTypeString   = "string"
	JSONSchemaTypeNumber   = "number"
	JSONSchemaTypeBoolean  = "boolean"
	JSONSchemaTypeArray    = "array"
	JSONSchemaTypeObject   = "object"
	HostPortTemplate       = "%s:%s"
	PostgresDSNTemplate    = "host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Shanghai"
	DefaultFormatJSON      = "application/json"
	DataURLTemplate        = "data:%s;base64,%s"
	DataURLPrefix          = "data:"
	DataURLBase64Separator = ";base64,"
	Base64SourceType       = "base64"
	URLSourceType          = "url"
	DigNameAccessSigner    = "accessSigner"
	DigNameRefreshSigner   = "refreshSigner"
	OpenAPISchemasPrefix   = "#/components/schemas/"
	OpenAPIDocsPath        = "/openapi"
	OpenAPISchemasPath     = "/schemas"

	ParseFloat64BitSize       = 64
	DecimalBase               = 10
	GoCommand                 = "go"
	GoVetCommand              = "vet"
	GoAllPackagesPattern      = "./..."
	StaticcheckCommand        = "staticcheck"
	StaticChecksFailedMessage = "static checks failed"

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
	OpenAICompletionObject      = "chat.completion"
	OpenAICompletionChunkObject = "chat.completion.chunk"

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

	FieldNameID = "id"

	MCPApprovalAlways = "always"
	MCPApprovalNever  = "never"

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
)
