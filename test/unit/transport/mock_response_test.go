package transport_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// TestBuildResponseStream_Events 验证 Responses 流式事件序列：
// created → output_item.added → N×output_text.delta → output_item.done → completed。
// 其中 output_item.done 与 completed 承载完整 item/response，是存储链路的内容来源。
func TestBuildResponseStream_Events(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefghijklmno")} // 15 字符 → 4 片
	events := transport.BuildResponseStream(spec, "mock-response", "resp-1")

	if len(events) != 8 { // created + added + 4 delta + output_item.done + completed
		t.Fatalf("want 8 events, got %d", len(events))
	}
	if events[0].Event != enum.ResponseStreamEventCreated {
		t.Fatalf("first event want created, got %s", events[0].Event)
	}
	if events[len(events)-1].Event != enum.ResponseStreamEventCompleted {
		t.Fatalf("last event want completed, got %s", events[len(events)-1].Event)
	}
	deltaCount, itemDone := 0, false
	for _, e := range events {
		switch e.Event {
		case enum.ResponseStreamEventOutputTextDelta:
			deltaCount++
		case enum.ResponseStreamEventOutputItemDone:
			itemDone = true
		}
	}
	if deltaCount != 4 {
		t.Fatalf("want 4 delta events, got %d", deltaCount)
	}
	if !itemDone {
		t.Fatal("missing output_item.done event (required by storage path)")
	}
	for i, e := range events {
		if len(e.Data) == 0 {
			t.Fatalf("event %d (%s) has empty payload", i, e.Event)
		}
	}
}

// TestBuildResponse_NonStream 验证非流式响应：status=completed、output 非空、usage 有值。
func TestBuildResponse_NonStream(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 8 字符 → 2 token
	rsp := transport.BuildResponse(spec, "mock-response")
	if rsp.Status != enum.ResponseStatusCompleted {
		t.Fatalf("status want completed, got %s", rsp.Status)
	}
	if rsp.Model != "mock-response" || len(rsp.Output) == 0 {
		t.Fatalf("response shape mismatch: %+v", rsp)
	}
	if rsp.Usage == nil || rsp.Usage.OutputTokens != 2 {
		t.Fatalf("usage mismatch: %+v", rsp.Usage)
	}
	if rsp.Usage.TotalTokens != rsp.Usage.InputTokens+rsp.Usage.OutputTokens {
		t.Fatal("total_tokens must equal input+output")
	}
}
