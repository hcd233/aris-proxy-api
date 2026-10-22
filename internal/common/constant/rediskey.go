package constant

const (
	LockKeyTemplateMiddleware = "%s:%s:%v"
	JWTUserCacheKeyTemplate   = "jwt:user:%d"
	// APIKeyAuthKeyTemplate API Key 鉴权缓存（%s = 密钥 SHA-256 十六进制摘要）：
	// 用摘要而非明文做 key，避免 keyspace / 监控面板泄露可用密钥。
	APIKeyAuthKeyTemplate    = "auth:apikey:%s"
	TokenBucketKeyTemplate   = "tb:%s:%s:%v"
	ScannerBanKeyTemplate    = "scanner:ban:%s"
	ScannerStrikeKeyTemplate = "scanner:strike:%s"
	ShareKeyTemplate         = "share:%s"
	UserSharesKeyTemplate    = "user_shares:%d"
	// CronLockKeyTemplate cron 任务互斥锁的 Redis key 模板（%s = CronModule*）
	CronLockKeyTemplate      = "cron:lock:%s"
	SessionSharesKeyTemplate = "session_shares:%d"

	// SessionMetaKeyTemplate 缓存 session 元数据（含 messageIDs/toolIDs，仅内部使用）
	//
	// 版本后缀 v2（2026-09-27）：payload 新增 apiKeyId 字段承载归属判定。
	// 不 bump 版本会让上线后命中的旧 payload 反序列化出 apiKeyId=0，
	// 进而把所有会话判成无权访问；缓存命中绕过 DB 故不会自愈。
	// 结论：SessionMetaCacheRecord 的字段变更必须同步 bump 此处版本号。
	SessionMetaKeyTemplate = "session:meta:v2:%d"
	// MessageKeyTemplate 缓存单条 message 详情（不可变，TTL 内永远有效）
	MessageKeyTemplate = "message:%d"
	// ToolKeyTemplate 缓存单条 tool 详情（不可变，TTL 内永远有效）
	ToolKeyTemplate = "tool:%d"

	// ReadCacheAliasKeyTemplate 读缓存-用户模型别名列表（%d = userID）
	ReadCacheAliasKeyTemplate = "cache:rd:alias:%d"
	// ReadCacheDetailKeyTemplate 读缓存-用户启用模型详情列表（%d = userID）
	ReadCacheDetailKeyTemplate = "cache:rd:detail:%d"
	// ReadCacheModelListKeyTemplate 读缓存-Web 平铺模型列表（%s = scope 片段, %s = 查询签名摘要）
	ReadCacheModelListKeyTemplate = "cache:rd:modellist:%s:%s"
	// ReadCacheKeywordIDsKeyTemplate 读缓存-关键词命中用户消息 ID 集合（%s = 归一化 keyword 的 SHA-256 摘要）
	ReadCacheKeywordIDsKeyTemplate = "cache:rd:kwids:%s"
	// ReadCacheKeyScanPattern 读缓存命名空间扫描模式（InvalidateAll 用）
	ReadCacheKeyScanPattern = "cache:rd:*"

	// RuntimeMetricsInstancesKey 运行时指标-实例注册表（ZSET：member=instanceID, score=最后flush的unix秒）
	RuntimeMetricsInstancesKey = "metrics:runtime:instances"
	// RuntimeMetricsDataKeyTemplate 运行时指标-单实例快照时序（ZSET：member=快照payload, score=快照unix秒），%s = instanceID
	RuntimeMetricsDataKeyTemplate = "metrics:runtime:data:%s"

	// RedisOAuthStateKeyPrefix Web 登录 OAuth2 state 的 Redis key 前缀
	//	（订阅账号的 oauth:flow: / oauth:access: / oauth:refresh: 见下方 OAuth*KeyTemplate）
	RedisOAuthStateKeyPrefix = "oauth:state:"

	// OAuthFlowKeyTemplate 订阅账号授权流临时记录（%s = state；一次性 Take，防重放）
	OAuthFlowKeyTemplate = "oauth:flow:%s"
	// OAuthAccessKeyTemplate 订阅账号 access token 缓存（%d = endpointID）
	OAuthAccessKeyTemplate = "oauth:access:%d"
	// OAuthRefreshLockKeyTemplate 订阅账号刷新互斥锁（%d = endpointID）
	OAuthRefreshLockKeyTemplate = "oauth:refresh:%d"
)

const RedisZRangePositiveInfinity = "+inf"
