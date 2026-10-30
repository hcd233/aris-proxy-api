package vo

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// CallStatus 模型调用状态值对象
//
// UpstreamStatusCode 语义（与原 apiutil.ExtractUpstreamStatusAndError 一致）：
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

// IsError 判断调用是否为失败调用。
//
// 非 200 一律算失败；200 时 errorMessage 也可能是非错误标注（capture 触发词命中
// 写入的 "capture context by trigger word" 前缀），此类标注不算失败。
//
//	@receiver s CallStatus
//	@return bool
//	@author centonhuang
//	@update 2026-10-29 21:00:00
func (s CallStatus) IsError() bool {
	if s.upstreamStatusCode != enum.CallStatusSuccess {
		return true
	}
	if s.errorMessage == "" {
		return false
	}
	return !strings.HasPrefix(s.errorMessage, constant.TriggerCaptureAuditRemark)
}

// IsSuccess 判断是否成功（HTTP 200 且非错误，触发词标注不算错误）
//
//	@receiver s CallStatus
//	@return bool
//	@author centonhuang
//	@update 2026-10-29 21:00:00
func (s CallStatus) IsSuccess() bool {
	return !s.IsError()
}
