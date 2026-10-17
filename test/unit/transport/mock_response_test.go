package transport_test

import (
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/transport"
)

// collectResponseEvents 用按需 API 拼装完整事件序列（生产路径已改为逐事件按需生成，
// 拼装仅便于对整条流做断言）。
func collectResponseEvents(spec transport.MockSpec, model, responseID string) []transport.MockResponseEvent {
	out := make([]transport.MockResponseEvent, 0, transport.ResponseStreamLen(spec))
	for i := range transport.ResponseStreamLen(spec) {
		out = append(out, transport.BuildResponseEventAt(spec, model, responseID, "msg-test", i))
	}
	return out
}

// TestBuildResponseEventAt_Events 验证 Responses 流式事件序列：
// created → output_item.added → N×output_text.delta → output_item.done → completed。
// 其中 output_item.done 与 completed 承载完整 item/response，是存储链路的内容来源。
func TestBuildResponseEventAt_Events(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefghijklmno")} // 15 字符 → 4 片
	events := collectResponseEvents(spec, "mock-response", "resp-1")

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

// TestBuildResponseEventAt_PayloadShape 断言事件负载符合 Responses 协议形态：
// 每个事件带 type 判别字段（等于事件名）、output_item.added 的 item 为 in_progress +
// 空 content 数组且带 id、delta 带同一 item_id、done 的 item content 为 output_text 数组
// （而非字符串）。
func TestBuildResponseEventAt_PayloadShape(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 8 字符 → 2 片
	events := collectResponseEvents(spec, "mock-response", "resp-1")

	payloads := make([]map[string]any, 0, len(events))
	for _, e := range events {
		var payload map[string]any
		if err := sonic.Unmarshal(e.Data, &payload); err != nil {
			t.Fatalf("event %s payload not an object: %v", e.Event, err)
		}
		if got, _ := payload[constant.ResponseStreamFieldType].(string); got != e.Event {
			t.Fatalf("event %s payload type = %q, want %q", e.Event, got, e.Event)
		}
		payloads = append(payloads, payload)
	}

	addedItem, ok := payloads[1][constant.ResponseStreamFieldItem].(map[string]any)
	if !ok {
		t.Fatalf("output_item.added item not an object: %#v", payloads[1])
	}
	itemID, _ := addedItem[constant.ResponseStreamFieldID].(string)
	if itemID == "" {
		t.Fatal("output_item.added item must carry id")
	}
	if got := addedItem[constant.ResponseStreamFieldStatus]; got != enum.ResponseStatusInProgress {
		t.Fatalf("added item status = %v, want %s", got, enum.ResponseStatusInProgress)
	}
	if content, ok := addedItem[constant.ResponseStreamFieldContent].([]any); !ok || len(content) != 0 {
		t.Fatalf("added item content must be empty array, got %#v", addedItem[constant.ResponseStreamFieldContent])
	}

	for i, e := range events {
		if e.Event != enum.ResponseStreamEventOutputTextDelta {
			continue
		}
		if got, _ := payloads[i][constant.ResponseStreamFieldItemID].(string); got != itemID {
			t.Fatalf("delta item_id = %q, want %q", got, itemID)
		}
	}

	doneIdx := len(events) - 2
	var done dto.ResponseStreamOutputItemDoneEvent
	if err := sonic.Unmarshal(events[doneIdx].Data, &done); err != nil {
		t.Fatalf("unmarshal output_item.done: %v", err)
	}
	if done.Item == nil || done.Item.Content == nil || len(done.Item.Content.Parts) == 0 {
		t.Fatalf("done item must carry content parts: %+v", done.Item)
	}
	part := done.Item.Content.Parts[0]
	if part.Type != enum.ResponseContentTypeOutputText || part.Text == nil || *part.Text != string(spec.Chars) {
		t.Fatalf("done item part mismatch: %+v", part)
	}
}

// TestBuildResponseEventAt_ResponseIDConsistent 验证同一流内 created 与 completed
// 事件携带同一 response id（同一响应对象在整条流内必须同 ID）。
func TestBuildResponseEventAt_ResponseIDConsistent(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 2 片
	events := collectResponseEvents(spec, "mock-response", "resp-123")

	var createdID string
	var completed dto.ResponseStreamTerminalEvent
	for _, e := range events {
		switch e.Event {
		case enum.ResponseStreamEventCreated:
			var payload map[string]any
			if err := sonic.Unmarshal(e.Data, &payload); err != nil {
				t.Fatalf("unmarshal created: %v", err)
			}
			createdID, _ = payload[constant.ResponseStreamFieldID].(string)
		case enum.ResponseStreamEventCompleted:
			if err := sonic.Unmarshal(e.Data, &completed); err != nil {
				t.Fatalf("unmarshal completed: %v", err)
			}
		}
	}
	if createdID != "resp-123" {
		t.Fatalf("created id = %q, want resp-123", createdID)
	}
	if completed.Response == nil || completed.Response.ID != createdID {
		t.Fatalf("completed response = %+v, want id %q", completed.Response, createdID)
	}
}

// TestBuildResponse_NonStream 验证非流式响应：status=completed、output 非空、usage 有值。
func TestBuildResponse_NonStream(t *testing.T) {
	t.Parallel()
	spec := transport.MockSpec{Chars: []byte("abcdefgh")} // 8 字符 → 2 token
	rsp := transport.BuildResponse(spec, "mock-response", "resp-nonstream")
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
