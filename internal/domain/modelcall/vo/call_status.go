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
//     @update 2026-04-22 17:00:00
type CallStatus struct {
	UpstreamStatusCode int
	ErrorMessage       string
}

// IsSuccess 判断是否成功（HTTP 200）
//
//	@receiver s CallStatus
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (s CallStatus) IsSuccess() bool {
	return s.UpstreamStatusCode == constant.CallStatusSuccess && s.ErrorMessage == ""
}
