package llmproxy_usecase

import (
	"context"
	"testing"

	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// TestWriteUpstreamSSEError_CanceledWritesShuttingDownFrame 验证优雅退出
// soft deadline 广播导致的 context.Canceled 会写出 server_shutting_down 帧，
// 客户端可识别为服务发布并自动重试。
func TestWriteUpstreamSSEError_CanceledWritesShuttingDownFrame(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	proxyutil.WriteUpstreamSSEError(context.Background(), sink, context.Canceled)

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
	proxyutil.WriteUpstreamSSEError(context.Background(), sink, &model.UpstreamConnectionError{Cause: context.Canceled})

	if len(sink.events) != 1 {
		t.Fatalf("captured %d events, want 1", len(sink.events))
	}
	if got := string(sink.events[0].data); got != constant.SSEOpenAIShuttingDownData {
		t.Fatalf("event data = %q, want %q", got, constant.SSEOpenAIShuttingDownData)
	}
}
