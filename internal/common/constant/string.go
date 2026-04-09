package constant

const (

	// ProjectName 项目名称
	//	@update 2026-03-31 10:00:00
	ProjectName = "aris-proxy-api"

	// ==================== Redis Key 模板 ====================

	// LockKeyTemplateMiddleware 中间件锁键模板
	//	@update 2025-11-11 17:23:31
	LockKeyTemplateMiddleware = "%s:%s:%v"

	// JWTUserCacheKeyTemplate JWT 用户信息缓存 Redis key 模板
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	JWTUserCacheKeyTemplate = "jwt:user:%d"

	// TokenBucketKeyTemplate 令牌桶限流 Redis key 模板
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	TokenBucketKeyTemplate = "tb:%s:%s:%v"

	// ScannerBanKeyTemplate 路由扫描封禁 Redis key 前缀
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	ScannerBanKeyTemplate = "scanner:ban:%s"

	// ScannerStrikeKeyTemplate 路由扫描违规计数 Redis key 前缀
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	ScannerStrikeKeyTemplate = "scanner:strike:%s"

	// ==================== ID 生成模板 ====================

	// AnthropicMessageIDTemplate Anthropic 风格消息 ID 模板
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	AnthropicMessageIDTemplate = "msg_%s"

	// OpenAIChunkIDTemplate OpenAI 风格 chunk ID 模板
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	OpenAIChunkIDTemplate = "chatcmpl-%s"

	// ==================== 存储路径模板 ====================

	// ObjectStorageDirTemplate 对象存储目录模板
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	ObjectStorageDirTemplate = "user-%d/%s"

	// ==================== 数据格式模板 ====================

	// DataURLTemplate Data URL 格式模板
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	DataURLTemplate = "data:%s;base64,%s"

	// ==================== SSE 协议常量 ====================

	// SSEDataPrefix OpenAI SSE 数据行前缀
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	SSEDataPrefix = "data: "

	// SSEDoneSignal OpenAI SSE 流式结束标记
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	SSEDoneSignal = "[DONE]"

	// ==================== 上游 API 版本 ====================

	// AnthropicAPIVersion Anthropic API 版本 header 值
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	AnthropicAPIVersion = "2023-06-01"

	// ==================== 第三方 API URL ====================

	// GithubUserURL GitHub 用户信息 API 地址
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	GithubUserURL = "https://api.github.com/user"

	// GithubUserEmailURL GitHub 用户邮箱 API 地址
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	GithubUserEmailURL = "https://api.github.com/user/emails"

	// ==================== OAuth2 ====================

	// OAuthStateHexString OAuth2 state hex 编码前缀说明（随机字节数 = 32）
	// 对应 make([]byte, 32) 生成 256-bit 随机数
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00

	// ==================== 用户默认值 ====================

	// DefaultUserNamePrefix 新 OAuth2 用户名不合法时的默认前缀
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	DefaultUserNamePrefix = "ArisUser"

	// EmptySessionSummary 空会话的默认摘要
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	EmptySessionSummary = "空会话"

	// ==================== 限流 ====================

	// RateLimitKeyByIP 按 IP 限流时的 keyValue 标识
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	RateLimitKeyByIP = "ip"

	// ==================== 日志文件名 ====================

	// LogInfoFileName 通用日志文件名
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogInfoFileName = "aris-proxy-api.log"

	// LogErrFileName 错误日志文件名
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogErrFileName = "aris-proxy-api-error.log"

	// LogPanicFileName panic 日志文件名
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogPanicFileName = "aris-proxy-api-panic.log"

	// ==================== Cron ====================

	// CronDefaultModule 未指定 module 名时 cron 日志的默认模块名
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	CronDefaultModule = "Cron"

	// CronInvalidKey cron key-value 解析失败时的兜底 key 名
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	CronInvalidKey = "invalid_key"

	// ==================== Database ====================

	// DBFieldUpdatedAt GORM updated_at column name
	//	@author centonhuang
	//	@update 2026-04-10 12:00:00
	DBFieldUpdatedAt = "updated_at"

	// MIMETypeOctetStream default binary Content-Type when extension is unknown
	//	@author centonhuang
	//	@update 2026-04-10 12:00:00
	MIMETypeOctetStream = "application/octet-stream"

	// ==================== 安全 ====================

	// MaskSecretPlaceholder 短密钥（<=8位）的掩码替换字符串
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	MaskSecretPlaceholder = "***"

	// ==================== API Key 常量 ====================

	// APIKeyPrefix API Key 前缀
	//	@author centonhuang
	//	@update 2026-04-09 10:00:00
	APIKeyPrefix = "sk-aris-"

	// APIKeyCharset API Key 字符集
	//	@author centonhuang
	//	@update 2026-04-09 10:00:00
	APIKeyCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// ==================== CORS ====================

	// CORSAllowOrigins CORS 允许的来源（开发环境前端地址）
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	CORSAllowOrigins = "http://localhost:3000"

	// ==================== HTTP 路由路径 ====================

	// RoutePathRoot 根路径
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathRoot = "/"

	// RoutePathHealth 健康检查
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathHealth = "/health"

	// RoutePathSSEHealth SSE 健康检查
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathSSEHealth = "/ssehealth"

	// RoutePathTokenRefresh 刷新 Token
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathTokenRefresh = "/refresh"

	// RoutePathUserCurrent 当前用户
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathUserCurrent = "/current"

	// RoutePathSessionList 会话列表
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathSessionList = "/list"

	// RoutePathOAuthLogin OAuth2 登录入口
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathOAuthLogin = "/login"

	// RoutePathOAuthCallback OAuth2 回调
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathOAuthCallback = "/callback"

	// RoutePathModels 模型列表（OpenAI / Anthropic）
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathModels = "/models"

	// RoutePathAnthropicMessages Anthropic messages
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathAnthropicMessages = "/messages"

	// RoutePathAnthropicMessagesCountTokens Anthropic count_tokens
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathAnthropicMessagesCountTokens = "/messages/count_tokens"

	// RoutePathOpenAIChatCompletions OpenAI chat completions
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathOpenAIChatCompletions = "/chat/completions"

	// RoutePathAPIKeyByID 按 ID 操作 API Key（路径参数）
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathAPIKeyByID = "/{id}"

	// ==================== Guard: 忽略路由扫描计分的常见探测路径 ====================

	// RoutePathFavicon 站点图标（浏览器默认请求）
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathFavicon = "/favicon.ico"

	// RoutePathRobots robots.txt
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathRobots = "/robots.txt"

	// RoutePathAppleTouchIcon Apple touch 图标
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathAppleTouchIcon = "/apple-touch-icon.png"

	// RoutePathAppleTouchIconPrecomposed Apple touch 预合成图标
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathAppleTouchIconPrecomposed = "/apple-touch-icon-precomposed.png"

	// RoutePathWellKnownSecurity security.txt（/.well-known）
	//	@author centonhuang
	//	@update 2026-04-10 10:00:00
	RoutePathWellKnownSecurity = "/.well-known/security.txt"
)
