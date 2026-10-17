package transport_test

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// TestBuildMessageStream_Events 验证 Anthropic 流式事件序列：
// message_start → content_block_start → N×content_block_delta → content_block_stop →
// message_delta → message_stop（均为聚合器已知类型，未知类型会报错）。
func TestBuildMessageStream_Events(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefghijklmno")} // 15 字符 → 4 片
	events := transport.BuildMessageStream(spec, "mock-messages", "msg-1")

	if len(events) != 9 { // start + block_start + 4 delta + block_stop + msg_delta + msg_stop
		t.Fatalf("want 9 events, got %d", len(events))
	}
	if events[0].Event != enum.AnthropicSSEEventTypeMessageStart {
		t.Fatalf("first event want message_start, got %s", events[0].Event)
	}
	if events[len(events)-1].Event != enum.AnthropicSSEEventTypeMessageStop {
		t.Fatalf("last event want message_stop, got %s", events[len(events)-1].Event)
	}
	deltaCount := 0
	for _, e := range events {
		if e.Event == enum.AnthropicSSEEventTypeContentBlockDelta {
			deltaCount++
		}
		if len(e.Data) == 0 {
			t.Fatalf("event %s has empty payload", e.Event)
		}
	}
	if deltaCount != 4 {
		t.Fatalf("want 4 content_block_delta events, got %d", deltaCount)
	}
}

// TestBuildMessageStream_ContentBlockStartEmpty 验证 content_block_start 只声明块存在、
// text 为空串，正文全部走 content_block_delta——start 带全文会让 native 客户端按
// 「start 初值 + delta 追加」累加出两倍正文。
func TestBuildMessageStream_ContentBlockStartEmpty(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 8 字符 → 2 片
	events := transport.BuildMessageStream(spec, "mock-messages", "msg-1")

	var start *dto.AnthropicSSEContentBlockStart
	var textBuilder strings.Builder
	for _, e := range events {
		switch e.Event {
		case enum.AnthropicSSEEventTypeContentBlockStart:
			var payload dto.AnthropicSSEContentBlockStart
			if err := sonic.Unmarshal(e.Data, &payload); err != nil {
				t.Fatalf("unmarshal content_block_start: %v", err)
			}
			start = &payload
		case enum.AnthropicSSEEventTypeContentBlockDelta:
			var payload dto.AnthropicSSEContentBlockDelta
			if err := sonic.Unmarshal(e.Data, &payload); err != nil {
				t.Fatalf("unmarshal delta: %v", err)
			}
			textBuilder.WriteString(payload.Delta.Text)
		}
	}
	if start == nil || start.ContentBlock == nil || start.ContentBlock.Text == nil {
		t.Fatal("content_block_start missing content_block.text")
	}
	if *start.ContentBlock.Text != "" {
		t.Fatalf("content_block_start text must be empty, got %q", *start.ContentBlock.Text)
	}
	if got := textBuilder.String(); got != string(spec.Chars) {
		t.Fatalf("delta text concat = %q, want %q", got, spec.Chars)
	}
}

// TestBuildMessage_NonStream 验证非流式响应：单 text block、stop_reason=end_turn、usage 有值。
func TestBuildMessage_NonStream(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 8 字符 → 2 token
	msg := transport.BuildMessage(spec, "mock-messages")
	if msg.Model != "mock-messages" || len(msg.Content) != 1 {
		t.Fatalf("message shape mismatch: %+v", msg)
	}
	if msg.Content[0].Text == nil || *msg.Content[0].Text != "abcdefgh" {
		t.Fatalf("content text mismatch: %+v", msg.Content[0])
	}
	if msg.StopReason == nil || *msg.StopReason != enum.AnthropicStopReasonEndTurn {
		t.Fatal("stop_reason must be end_turn")
	}
	if msg.Usage == nil || msg.Usage.OutputTokens != 2 {
		t.Fatalf("usage mismatch: %+v", msg.Usage)
	}
}

// TestBuildTokensCount 验证 count_tokens 返回请求体估算值（与 /messages 的
// usage.input_tokens 同源，同一 body 计数稳定）。
func TestBuildTokensCount(t *testing.T) {
	t.Parallel()
	if got := transport.BuildTokensCount(7).InputTokens; got != 7 {
		t.Fatalf("input_tokens want 7, got %d", got)
	}
}
