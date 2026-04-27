package vo

import "github.com/hcd233/aris-proxy-api/internal/common/constant"

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
//     @update 2026-04-26 10:00:00
type CallStatus struct {
	upstreamStatusCode int
	errorMessage       string
}

// NewCallStatus 构造调用状态值对象
//
//	@param upstreamStatusCode int
//	@param errorMessage string
//	@return CallStatus
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewCallStatus(upstreamStatusCode int, errorMessage string) CallStatus {
	return CallStatus{upstreamStatusCode: upstreamStatusCode, errorMessage: errorMessage}
}

// UpstreamStatusCode 返回上游状态码
//
//	@receiver s CallStatus
//	@return int
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (s CallStatus) UpstreamStatusCode() int { return s.upstreamStatusCode }

// ErrorMessage 返回错误信息
//
//	@receiver s CallStatus
//	@return string
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (s CallStatus) ErrorMessage() string { return s.errorMessage }

// IsSuccess 判断是否成功（HTTP 200）
//
//	@receiver s CallStatus
//	@return bool
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (s CallStatus) IsSuccess() bool {
	return s.upstreamStatusCode == constant.CallStatusSuccess && s.errorMessage == ""
}
