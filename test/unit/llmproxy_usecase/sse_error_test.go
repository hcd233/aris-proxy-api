package llmproxy_usecase

import (
	"context"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// TestWriteUpstreamSSEError_CanceledWritesShuttingDownFrame 验证优雅退出
// soft deadline 广播导致的 context.Canceled 会写出 server_shutting_down 帧，
// 客户端可识别为服务发布并自动重试。
func TestWriteUpstreamSSEError_CanceledWritesShuttingDownFrame(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	proxyutil.WriteUpstreamSSEError(context.Background(), sink, context.Canceled, enum.ProtocolKindOpenAI)

	if len(sink.events) != 1 {
		t.Fatalf("captured %d events, want 1", len(sink.events))
	}
	if got := string(sink.events[0].data); got != constant.SSEOpenAIShuttingDownData {
		t.Fatalf("event data = %q, want %q", got, constant.SSEOpenAIShuttingDownData)
	}
}

// TestWriteUpstreamSSEError_UnwrappedCanceled 验证包装为 UpstreamConnectionError
// 的 context.Canceled 也能被识别（Unwrap 穿透），覆盖真实读循环错误路径。
func TestWriteUpstreamSSEError_UnwrappedCanceled(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	proxyutil.WriteUpstreamSSEError(context.Background(), sink, &model.UpstreamConnectionError{Cause: context.Canceled}, enum.ProtocolKindOpenAI)

	if len(sink.events) != 1 {
		t.Fatalf("captured %d events, want 1", len(sink.events))
	}
	if got := string(sink.events[0].data); got != constant.SSEOpenAIShuttingDownData {
		t.Fatalf("event data = %q, want %q", got, constant.SSEOpenAIShuttingDownData)
	}
}

// TestWriteUpstreamSSEError_AnthropicShuttingDownFrame 验证 Anthropic 协议
// 走 event: error 帧（而非 OpenAI 的 data-only 帧），形态符合 Anthropic SSE
// 错误事件规范，Claude Code 等客户端可解析并重试。
func TestWriteUpstreamSSEError_AnthropicShuttingDownFrame(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	proxyutil.WriteUpstreamSSEError(context.Background(), sink, context.Canceled, enum.ProtocolKindAnthropic)

	if len(sink.events) != 1 {
		t.Fatalf("captured %d events, want 1", len(sink.events))
	}
	if got := sink.events[0].event; got != enum.AnthropicSSEEventTypeError {
		t.Fatalf("event name = %q, want %q", got, enum.AnthropicSSEEventTypeError)
	}
	if got := string(sink.events[0].data); got != constant.SSEAnthropicShuttingDownData {
		t.Fatalf("event data = %q, want %q", got, constant.SSEAnthropicShuttingDownData)
	}
}
