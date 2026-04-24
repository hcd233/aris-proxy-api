package vo

// CallStatus 模型调用状态值对象
//
// UpstreamStatusCode 语义（与原 util.ExtractUpstreamStatusAndError 一致）：
//
//   - 200   ：成功
//
//   - >0    ：上游返回的 HTTP 状态码
//
//   - -1    ：上游连接错误
//
//   - 0     ：其它未知错误
//
//     @author centonhuang
//     @update 2026-04-22 17:00:00
type CallStatus struct {
	UpstreamStatusCode int
	ErrorMessage       string
}

// StatusCodeSuccess HTTP 成功状态码（非流式成功路径硬编码）
const StatusCodeSuccess = 200

// StatusCodeConnectionError 连接层错误状态码
const StatusCodeConnectionError = -1

// StatusCodeUnknownError 未知错误状态码
const StatusCodeUnknownError = 0

// IsSuccess 判断是否成功（HTTP 200）
//
//	@receiver s CallStatus
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (s CallStatus) IsSuccess() bool {
	return s.UpstreamStatusCode == StatusCodeSuccess && s.ErrorMessage == ""
}
