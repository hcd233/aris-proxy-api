package constant

const (
	LockKeyTemplateMiddleware = "%s:%s:%v"
	JWTUserCacheKeyTemplate   = "jwt:user:%d"
	TokenBucketKeyTemplate    = "tb:%s:%s:%v"
	ScannerBanKeyTemplate     = "scanner:ban:%s"
	ScannerStrikeKeyTemplate  = "scanner:strike:%s"
	ShareKeyTemplate          = "share:%s"
	UserSharesKeyTemplate     = "user_shares:%d"
	SessionSharesKeyTemplate  = "session_shares:%d"

	// SessionMetaKeyTemplate 缓存 session 元数据（含 messageIDs/toolIDs，仅内部使用）
	SessionMetaKeyTemplate = "session:meta:%d"
	// MessageKeyTemplate 缓存单条 message 详情（不可变，TTL 内永远有效）
	MessageKeyTemplate = "message:%d"
	// ToolKeyTemplate 缓存单条 tool 详情（不可变，TTL 内永远有效）
	ToolKeyTemplate = "tool:%d"
)
