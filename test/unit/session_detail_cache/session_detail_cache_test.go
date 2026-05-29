// Package session_detail_cache_test 测试 sessionport.SessionDetailCache 的载荷类型
//
// 由于 go-redis 无官方 mock，Redis 实际通信由 E2E 覆盖。
// 本单元测试聚焦"载荷类型的序列化往返一致性"。
package session_detail_cache_test

import (
	"testing"
	"time"

	"github.com/bytedance/sonic"

	sessionport "github.com/hcd233/aris-proxy-api/internal/application/session/port"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

func TestSessionMetaCacheRecord_RoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	original := &sessionport.SessionMetaCacheRecord{
		ID:         42,
		APIKeyName: "user-key-1",
		CreatedAt:  now,
		UpdatedAt:  now.Add(time.Minute),
		Metadata:   map[string]string{"source": "openai"},
		MessageIDs: []uint{1, 2, 3, 5, 8},
		ToolIDs:    []uint{10, 20},
	}

	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded sessionport.SessionMetaCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", decoded.ID, original.ID)
	}
	if decoded.APIKeyName != original.APIKeyName {
		t.Errorf("APIKeyName mismatch")
	}
	if !decoded.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch")
	}
	if len(decoded.MessageIDs) != len(original.MessageIDs) {
		t.Fatalf("MessageIDs length mismatch")
	}
	for i, id := range original.MessageIDs {
		if decoded.MessageIDs[i] != id {
			t.Errorf("MessageIDs[%d] mismatch", i)
		}
	}
	if decoded.Metadata["source"] != "openai" {
		t.Errorf("Metadata not preserved")
	}
}

func TestSessionMetaCacheRecord_EmptyIDs(t *testing.T) {
	original := &sessionport.SessionMetaCacheRecord{
		ID:         1,
		APIKeyName: "k",
		MessageIDs: []uint{},
		ToolIDs:    []uint{},
	}
	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded sessionport.SessionMetaCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.MessageIDs) != 0 {
		t.Errorf("expected empty MessageIDs")
	}
}

func TestMessageCacheRecord_RoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 29, 11, 0, 0, 0, time.UTC)
	original := &sessionport.MessageCacheRecord{
		ID:    100,
		Model: "gpt-4",
		Message: &vo.UnifiedMessage{
			Role:    enum.RoleUser,
			Content: &vo.UnifiedContent{Text: "hello"},
		},
		CreatedAt: now,
	}

	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded sessionport.MessageCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != 100 || decoded.Model != "gpt-4" {
		t.Errorf("scalar fields mismatch")
	}
	if decoded.Message == nil {
		t.Fatalf("Message is nil after round trip")
	}
	if decoded.Message.Role != enum.RoleUser {
		t.Errorf("Role mismatch")
	}
	if decoded.Message.Content == nil || decoded.Message.Content.Text != "hello" {
		t.Errorf("Content text mismatch: got %+v", decoded.Message.Content)
	}
}

func TestToolCacheRecord_RoundTrip(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	original := &sessionport.ToolCacheRecord{
		ID: 7,
		Tool: &vo.UnifiedTool{
			Name:        "search",
			Description: "do a web search",
		},
		CreatedAt: now,
	}

	payload, err := sonic.MarshalString(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded sessionport.ToolCacheRecord
	if err := sonic.UnmarshalString(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != 7 {
		t.Errorf("ID mismatch")
	}
	if decoded.Tool == nil || decoded.Tool.Name != "search" {
		t.Errorf("Tool.Name not preserved")
	}
	if decoded.Tool.Description != "do a web search" {
		t.Errorf("Tool.Description not preserved")
	}
}
