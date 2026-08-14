package llmproxy_usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// TestProxyError_PreservesFields 验证 ProxyError 能完整携带 adapter 构造 HTTP JSON 响应所需的全部信息：
// status、headers、body、cause 和入口协议。后续迁移路径中业务错误（model not found、content trigger）
// 与上游 Open 错误都通过此类型向 adapter 传递。
func TestProxyError_PreservesFields(t *testing.T) {
	t.Parallel()

	cause := ierr.New(ierr.ErrInternal, "upstream connection refused")
	err := &port.ProxyError{
		StatusCode: 429,
		Headers:    map[string]string{"X-Request-ID": "req-abc", "Retry-After": "30"},
		Body:       []byte(`{"error":{"message":"rate limited"}}`),
		Cause:      cause,
		Protocol:   enum.ProtocolKindOpenAI,
	}

	if err.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", err.StatusCode)
	}
	if got := err.Headers["X-Request-ID"]; got != "req-abc" {
		t.Errorf("Headers[X-Request-ID] = %q, want %q", got, "req-abc")
	}
	if got := err.Headers["Retry-After"]; got != "30" {
		t.Errorf("Headers[Retry-After] = %q, want %q", got, "30")
	}
	if string(err.Body) != `{"error":{"message":"rate limited"}}` {
		t.Errorf("Body = %q, want rate limited JSON body", string(err.Body))
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(err, cause) = false; Cause must be reachable via Unwrap")
	}
	if err.Protocol != enum.ProtocolKindOpenAI {
		t.Errorf("Protocol = %v, want ProtocolOpenAI", err.Protocol)
	}
	if err.Error() == "" {
		t.Errorf("Error() must return non-empty message to satisfy error interface")
	}
}

// TestStreamResult_OpenReturnsStream 验证 StreamResult.Open 调用一次即返回 Stream，
// 且不预先消费上游或写入任何 transport。adapter 在真实写流时调用 Open；
// Open 失败时返回 error，adapter 在写出 SSE headers 之前将其映射为 HTTP JSON 错误。
func TestStreamResult_OpenReturnsStream(t *testing.T) {
	t.Parallel()

	openCalls := 0
	result := &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Headers:  map[string]string{"X-Accel-Buffering": "no"},
		Open: func(ctx context.Context) (port.Stream, error) {
			openCalls++
			return &fakeStream{}, nil
		},
	}

	if result.Protocol != enum.ProtocolKindOpenAI {
		t.Errorf("Protocol = %v, want ProtocolOpenAI", result.Protocol)
	}
	if result.Headers["X-Accel-Buffering"] != "no" {
		t.Errorf("Headers[X-Accel-Buffering] = %q, want %q", result.Headers["X-Accel-Buffering"], "no")
	}

	s, err := result.Open(context.Background())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if s == nil {
		t.Fatal("Open returned nil stream")
	}
	if openCalls != 1 {
		t.Errorf("Open invoked %d times, want exactly 1", openCalls)
	}
}

// TestStream_Close_AfterReadSuccess 验证 adapter 在 Read 成功后调用 Close 释放上游资源。
// 这是 Open/Read 两阶段契约的一部分：Close 必须可重入安全且不依赖 Read 结果。
func TestStream_Close_AfterReadSuccess(t *testing.T) {
	t.Parallel()

	s := &fakeStream{}
	sink := &captureSink{}

	if err := s.Read(context.Background(), sink); err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
	if s.closeCalls != 1 {
		t.Errorf("Close invoked %d times, want 1", s.closeCalls)
	}
	if len(sink.events) != 1 {
		t.Errorf("sink captured %d events, want 1", len(sink.events))
	}
}

// TestStream_Close_AfterReadError 验证 adapter 在 Read 失败时仍能调用 Close 释放资源。
// 流式路径中 Read 中途错误（上游中断、解析失败）不应导致资源泄漏。
func TestStream_Close_AfterReadError(t *testing.T) {
	t.Parallel()

	readErr := ierr.New(ierr.ErrInternal, "upstream stream closed unexpectedly")
	s := &fakeStream{readErr: readErr}
	sink := &captureSink{}

	if err := s.Read(context.Background(), sink); err == nil {
		t.Fatal("Read expected to propagate readErr, got nil")
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
	if s.closeCalls != 1 {
		t.Errorf("Close invoked %d times, want 1", s.closeCalls)
	}
}

// TestEventSink_WriteEvent_Invocable 验证 EventSink 接口可被 application 实现，
// 且不依赖 HTTP writer/bufio/Huma context。adapter 提供具体实现把 WriteEvent 转换为
// SSE 帧写入 bufio.Writer。
func TestEventSink_WriteEvent_Invocable(t *testing.T) {
	t.Parallel()

	sink := &captureSink{}
	if err := sink.WriteEvent("message_delta", []byte(`{"type":"message_delta"}`)); err != nil {
		t.Fatalf("WriteEvent returned error: %v", err)
	}
	if err := sink.WriteEvent("", []byte(`data: [DONE]`)); err != nil {
		t.Fatalf("WriteEvent(empty event) returned error: %v", err)
	}
	if len(sink.events) != 2 {
		t.Fatalf("captured %d events, want 2", len(sink.events))
	}
	if sink.events[0].event != "message_delta" {
		t.Errorf("events[0].event = %q, want %q", sink.events[0].event, "message_delta")
	}
	if sink.events[1].event != "" {
		t.Errorf("events[1].event = %q, want empty (data-only frame)", sink.events[1].event)
	}
}

// TestPortResponseFileDoesNotImportHTTPTransport 验证 Task 2 新增的 response.go
// 不导入 Huma/Fiber/internal-api-util/bufio/net/http，确保 transport-neutral 结果类型
// 不反向引入 transport 依赖。
//
// 整个 port + usecase 包的全量依赖边界由 architecture_test.go 中的
// TestLLMProxyApplicationDoesNotImportHTTPTransport 在 Task 6 启用（迁移期间 t.Skip），
// 因为 handler.go 要等 Task 3 才会移除 huma 依赖。
func TestPortResponseFileDoesNotImportHTTPTransport(t *testing.T) {
	t.Parallel()

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate test file")
	}
	moduleRoot := filepath.Join(filepath.Dir(testFile), "..", "..", "..")
	responseFile := filepath.Join(moduleRoot, "internal", "application", "llmproxy", "port", "response.go")

	forbidden := []string{
		"github.com/danielgtaylor/huma/v2",
		"github.com/gofiber/fiber/v3",
		"internal/api/util",
		"bufio",
		"net/http",
	}

	content, err := os.ReadFile(responseFile)
	if err != nil {
		t.Fatalf("read %s: %v", responseFile, err)
	}

	for _, bad := range forbidden {
		if strings.Contains(string(content), `"`+bad+`"`) {
			rel, _ := filepath.Rel(moduleRoot, responseFile)
			t.Fatalf("%s: contains forbidden import %s", rel, bad)
		}
	}
}

// fakeStream 是测试用的 port.Stream 实现，记录 Close 调用次数和可选的 Read 错误。
type fakeStream struct {
	closeCalls int
	readErr    error
}

func (s *fakeStream) Read(_ context.Context, sink port.EventSink) error {
	if s.readErr != nil {
		return s.readErr
	}
	return sink.WriteEvent("data", []byte("hello"))
}

func (s *fakeStream) Close() error {
	s.closeCalls++
	return nil
}

// captureSink 是测试用的 port.EventSink 实现，收集所有 WriteEvent 调用。
type captureSink struct {
	events []capturedEvent
}

type capturedEvent struct {
	event string
	data  []byte
}

func (s *captureSink) WriteEvent(event string, data []byte) error {
	s.events = append(s.events, capturedEvent{event: event, data: append([]byte(nil), data...)})
	return nil
}
