package transport_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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

// TestBuildTokensCount 验证 count_tokens 响应按同口径估算。
func TestBuildTokensCount(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 8 字符 → 2 token
	if got := transport.BuildTokensCount(spec).InputTokens; got != 2 {
		t.Fatalf("input_tokens want 2, got %d", got)
	}
}
