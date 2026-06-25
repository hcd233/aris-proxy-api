package constant

const (
	LockKeyTemplateMiddleware = "%s:%s:%v"
	JWTUserCacheKeyTemplate   = "jwt:user:%d"
	TokenBucketKeyTemplate    = "tb:%s:%s:%v"
	ScannerBanKeyTemplate     = "scanner:ban:%s"
	ScannerStrikeKeyTemplate  = "scanner:strike:%s"
	ShareKeyTemplate          = "share:%s"
	UserSharesKeyTemplate     = "user_shares:%d"
	// CronLockKeyTemplate cron 任务互斥锁的 Redis key 模板（%s = CronModule*）
	CronLockKeyTemplate      = "cron:lock:%s"
	SessionSharesKeyTemplate = "session_shares:%d"

	// SessionMetaKeyTemplate 缓存 session 元数据（含 messageIDs/toolIDs，仅内部使用）
	SessionMetaKeyTemplate = "session:meta:%d"
	// MessageKeyTemplate 缓存单条 message 详情（不可变，TTL 内永远有效）
	MessageKeyTemplate = "message:%d"
	// ToolKeyTemplate 缓存单条 tool 详情（不可变，TTL 内永远有效）
	ToolKeyTemplate = "tool:%d"

	// RuntimeMetricsInstancesKey 运行时指标-实例注册表（ZSET：member=instanceID, score=最后flush的unix秒）
	RuntimeMetricsInstancesKey = "metrics:runtime:instances"
	// RuntimeMetricsDataKeyTemplate 运行时指标-单实例快照时序（ZSET：member=快照payload, score=快照unix秒），%s = instanceID
	RuntimeMetricsDataKeyTemplate = "metrics:runtime:data:%s"
)

const RedisZRangePositiveInfinity = "+inf"
