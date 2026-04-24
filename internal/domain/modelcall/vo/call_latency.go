// Package vo ModelCall 域值对象
package vo

import "time"

// CallLatency 模型调用延迟值对象
//
// FirstToken：首 token 延迟（非流式 = 总延迟）
// Stream：流式传输持续时间（从首 token 到流结束；非流式 = 0）
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type CallLatency struct {
	FirstToken time.Duration
	Stream     time.Duration
}

// FirstTokenMs 返回首 token 延迟毫秒
//
//	@receiver l CallLatency
//	@return int64
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (l CallLatency) FirstTokenMs() int64 { return l.FirstToken.Milliseconds() }

// StreamMs 返回流式持续时间毫秒
//
//	@receiver l CallLatency
//	@return int64
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (l CallLatency) StreamMs() int64 { return l.Stream.Milliseconds() }
