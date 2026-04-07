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

	// ScannerBanKeyPrefix 路由扫描封禁 Redis key 前缀
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	ScannerBanKeyPrefix = "scanner:ban:"

	// ScannerStrikeKeyPrefix 路由扫描违规计数 Redis key 前缀
	//	@author centonhuang
	//	@update 2026-04-07 10:00:00
	ScannerStrikeKeyPrefix = "scanner:strike:"

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
)
