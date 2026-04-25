// Package vo ModelCall 域值对象
package vo

import "time"

// CallLatency 模型调用延迟值对象
//
// FirstToken：首 token 延迟（非流式 = 总延迟）
// Stream：流式传输持续时间（从首 token 到流结束；非流式 = 0）
//
//	@author centonhuang
//	@update 2026-04-26 10:00:00
type CallLatency struct {
	firstToken time.Duration
	stream     time.Duration
}

// NewCallLatency 构造延迟值对象
//
//	@param firstToken time.Duration
//	@param stream time.Duration
//	@return CallLatency
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewCallLatency(firstToken, stream time.Duration) CallLatency {
	return CallLatency{firstToken: firstToken, stream: stream}
}

// FirstToken 返回首 token 延迟
//
//	@receiver l CallLatency
//	@return time.Duration
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (l CallLatency) FirstToken() time.Duration { return l.firstToken }

// Stream 返回流式持续时间
//
//	@receiver l CallLatency
//	@return time.Duration
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (l CallLatency) Stream() time.Duration { return l.stream }

// FirstTokenMs 返回首 token 延迟毫秒
//
//	@receiver l CallLatency
//	@return int64
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (l CallLatency) FirstTokenMs() int64 { return l.firstToken.Milliseconds() }

// StreamMs 返回流式持续时间毫秒
//
//	@receiver l CallLatency
//	@return int64
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func (l CallLatency) StreamMs() int64 { return l.stream.Milliseconds() }
