package constant

// ==================== SSE ====================

const (
	// SSEHeartbeatCount SSE 健康检查心跳发送次数
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	SSEHeartbeatCount = 30
)

// ==================== 安全 / 加密 ====================

const (
	// OAuthStateBytes OAuth2 state 随机字节数（256-bit）
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	OAuthStateBytes = 32

	// APIKeyRandomLength API Key 随机字符串长度
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	APIKeyRandomLength = 24

	// APIKeyMaxCount 单用户最大 API Key 数量
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	APIKeyMaxCount = 5
)

// ==================== 浮点解析 ====================

const (
	// ParseFloat64BitSize strconv.ParseFloat 64位精度参数
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	ParseFloat64BitSize = 64
)

// ==================== 字节范围 ====================

const (
	// ByteMax byte 类型的取值范围上限（256），用于 rejection sampling 避免字节分布偏差
	//	@author centonhuang
	//	@update 2026-04-14 00:00:00
	ByteMax = 256
)

// ==================== 日志字段 ====================

const (
	// LogFieldValueMaxLength 日志字段值最大长度，超出部分截断
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogFieldValueMaxLength = 512
)

// ==================== 日志轮转 ====================

const (
	// LogInfoMaxSizeMB 通用日志单文件最大体积（MB）
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogInfoMaxSizeMB = 100

	// LogInfoMaxBackups 通用日志最大保留文件数
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogInfoMaxBackups = 3

	// LogInfoMaxAgeDays 通用日志最大保留天数
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogInfoMaxAgeDays = 7

	// LogErrMaxSizeMB 错误/panic 日志单文件最大体积（MB）
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogErrMaxSizeMB = 500

	// LogErrMaxBackups 错误/panic 日志最大保留文件数
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogErrMaxBackups = 3

	// LogErrMaxAgeDays 错误/panic 日志最大保留天数
	//	@author centonhuang
	//	@update 2026-04-09 15:00:00
	LogErrMaxAgeDays = 30

	// CLSProducerCloseTimeoutMs CLS Producer 优雅关闭超时时间（毫秒）
	//	@author centonhuang
	//	@update 2026-04-25 10:00:00
	CLSProducerCloseTimeoutMs = 10000
)
