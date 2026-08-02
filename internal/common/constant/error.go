package constant

// 业务错误码（与 internal/common/ierr/sentinels.go 中各哨兵的 bizError code 保持一致）。
//
// 分段规则：
//   - 10000-10099: 通用业务错误
//   - 10001-10099 分段与 internal/common/ierr/sentinels.go 一一对应
//
// 新增长度说明：model.Error.StatusCode() 与 apiutil.statusToBizCode 依赖本组常量做
// 业务码 ↔ HTTP 状态码双向映射，修改任何业务码时必须同步两处。

const (
	// BizErrorCodeInternal 内部错误（兜底）
	BizErrorCodeInternal = 10000
	// BizErrorCodeUnauthorized 未授权
	BizErrorCodeUnauthorized = 10001
	// BizErrorCodeNoPermission 没有权限
	BizErrorCodeNoPermission = 10002
	// BizErrorCodeDataNotExists 数据不存在
	BizErrorCodeDataNotExists = 10003
	// BizErrorCodeDataExists 数据已存在
	BizErrorCodeDataExists = 10004
	// BizErrorCodeTooManyRequests 请求过于频繁
	BizErrorCodeTooManyRequests = 10005
	// BizErrorCodeBadRequest 请求参数错误
	BizErrorCodeBadRequest = 10006
	// BizErrorCodeInsufficientQuota 配额不足
	BizErrorCodeInsufficientQuota = 10007
	// BizErrorCodeQuotaExceeded 配额超限
	BizErrorCodeQuotaExceeded = 10008
	// BizErrorCodeResourceLocked 资源锁定
	BizErrorCodeResourceLocked = 10009
	// BizErrorCodeContentBlocked 内容违反策略
	BizErrorCodeContentBlocked = 10010

	// BizErrorDetailSep 框架错误 message 与字段错误细节之间的分隔符
	BizErrorDetailSep = ": "
	// BizErrorDetailJoinSep 多个字段错误细节之间的分隔符
	BizErrorDetailJoinSep = "; "
)
