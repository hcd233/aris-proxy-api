package audit_dto

import (
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestListAuditLogsRsp_EmptyLogs(t *testing.T) {
	t.Parallel()
	rsp := &dto.ListAuditLogsRsp{
		Logs:     nil,
		PageInfo: &model.PageInfo{Page: 1, PageSize: 20, Total: 0},
	}
	data, err := sonic.Marshal(rsp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var obj map[string]any
	if err := sonic.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	pi, ok := obj["pageInfo"]
	if !ok {
		t.Fatal("pageInfo field missing")
	}
	_ = pi
}

func TestAuditLogItem_JSONTags(t *testing.T) {
	t.Parallel()
	createdAt := time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)
	item := dto.AuditLogItem{
		ID:                  1,
		CreatedAt:           createdAt,
		Model:               "gpt-4o",
		UpstreamProtocol:    "openai-chat-completion",
		APIProtocol:         "openai-chat-completion",
		Endpoint:            "openai-ep-1",
		InputTokens:         100,
		OutputTokens:        50,
		FirstTokenLatencyMs: 200,
		StreamDurationMs:    1500,
		UpstreamStatusCode:  200,
		TraceID:             "abc123",
		APIKeyName:          "my-key",
		UserName:            "alice",
		UserEmail:           "alice@example.com",
	}
	data, err := sonic.Marshal(item)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var obj map[string]any
	if err := sonic.Unmarshal(data, &obj); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if v, ok := obj["traceId"]; !ok {
		t.Errorf("traceId field missing")
	} else if v != "abc123" {
		t.Errorf("traceId = %v, want abc123", v)
	}
	if v, ok := obj["model"]; !ok {
		t.Errorf("model field missing")
	} else if v != "gpt-4o" {
		t.Errorf("model = %v, want gpt-4o", v)
	}
	if v, ok := obj["inputTokens"]; !ok {
		t.Errorf("inputTokens field missing")
	} else {
		n, ok := v.(float64)
		if !ok || int(n) != 100 {
			t.Errorf("inputTokens = %v, want 100", v)
		}
	}
	if v, ok := obj["apiKeyName"]; !ok {
		t.Errorf("apiKeyName field missing")
	} else if v != "my-key" {
		t.Errorf("apiKeyName = %v, want my-key", v)
	}
	if v, ok := obj["userName"]; !ok {
		t.Errorf("userName field missing")
	} else if v != "alice" {
		t.Errorf("userName = %v, want alice", v)
	}
	if v, ok := obj["userEmail"]; !ok {
		t.Errorf("userEmail field missing")
	} else if v != "alice@example.com" {
		t.Errorf("userEmail = %v, want alice@example.com", v)
	}
}
