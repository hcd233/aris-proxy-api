// Package port defines application-layer ports for llmproxy use cases.
package port

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// ProxyError 是 application 层向 adapter 透传的协议错误载体。
//
// 携带上游或业务错误的 HTTP 状态语义、可透传 header、协议错误 body 和底层 cause；
// 由 adapter 映射为入口协议对应的 HTTP JSON 错误响应。
// application 不构造 Huma response，也不设置 HTTP status/header。
type ProxyError struct {
	// StatusCode 是 adapter 应写入的 HTTP 状态码。
	StatusCode int
	// Headers 是允许透传给客户端的上游/协议 header。
	Headers map[string]string
	// Body 是协议层已经确定的错误 JSON 字节，可直接写入 HTTP body。
	Body []byte
	// Cause 是底层错误，支持 errors.Is/As 穿透。
	Cause error
	// Protocol 指示入口协议，决定 adapter 使用哪种 error envelope。
	Protocol enum.ProtocolKind
}

// Error 实现 error 接口，便于 application 与 adapter 通过 errors.As 处理。
func (e *ProxyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf(constant.ProxyErrorWithCauseTemplate, e.StatusCode, e.Cause.Error())
	}
	return fmt.Sprintf(constant.ProxyErrorTemplate, e.StatusCode)
}

// Unwrap 暴露 Cause 以支持 errors.Is/As 穿透到底层错误。
func (e *ProxyError) Unwrap() error {
	return e.Cause
}

// StreamResult 是 application 层向 adapter 返回的流式代理结果。
//
// Open 由 adapter 在真实开始写流时调用一次，建立上游连接并返回 Stream；
// Open 失败时 adapter 必须在写出 SSE headers 之前将其映射为 HTTP JSON 错误，
// 不得写成 200 SSE。
type StreamResult struct {
	// Protocol 指示入口协议，决定 SSE framing 与事件格式。
	Protocol enum.ProtocolKind
	// Headers 是允许透传给客户端的 header（如 X-Accel-Buffering: no）。
	Headers map[string]string
	// Open 建立上游流并返回 Stream；adapter 在写出 SSE headers 之后调用。
	Open func(ctx context.Context) (Stream, error)
}

// Stream 是 application 定义的流消费接口。
//
// adapter 在 Open 成功后调用 Read 消费事件，并在 Read 完成（成功或失败）后
// 调用 Close 释放上游资源。Close 必须可重入安全。
type Stream interface {
	// Read 消费上游流，将事件写入 sink。返回 nil 表示流自然结束。
	Read(ctx context.Context, sink EventSink) error
	// Close 释放上游资源。adapter 在 Read 完成后必须调用一次。
	Close() error
}

// EventSink 是 application 定义的协议事件输出接口。
//
// adapter 提供具体实现，将 WriteEvent 转换为 SSE event/data 帧写入 bufio.Writer。
// application 通过此接口输出协议事件，不接触 HTTP writer 或 Huma context。
type EventSink interface {
	// WriteEvent 写入一个 SSE 事件。event 为空表示 data-only 帧。
	WriteEvent(event string, data []byte) error
}
