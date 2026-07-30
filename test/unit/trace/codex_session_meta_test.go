package trace

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

// TestBuildConversation_ProjectsCodexSystemPromptAndTools 验证 codex session_meta
// 记录中的系统提示词（base_instructions）和工具定义（dynamic_tools）被投影进对话视图：
// 系统提示词作为 role=system 的 message item（交给 dedupeMessage 去重），
// 工具定义收集到 Conversation.Tools（按 namespace:name 去重、保留首次出现顺序）。
func TestBuildConversation_ProjectsCodexSystemPromptAndTools(t *testing.T) {
	t.Parallel()

	// session_meta 真实结构：payload.base_instructions 为 {"text": "..."}，
	// payload.dynamic_tools 为命名空间组数组，每组嵌套真实工具定义。
	sessionMetaPayload := `{"type":"session_meta","payload":{"id":"s1","base_instructions":{"text":"You are a coding agent running in the Codex CLI."},"dynamic_tools":[{"name":"codex_app","type":"namespace","description":"Tools provided by the Codex app.","tools":[{"name":"automation_update","type":"function","description":"Create or update automations.","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}},{"name":"read_file","type":"function","description":"Read a file.","parameters":{"type":"object"}}]}]}}`

	records := []*trace.TraceEvent{
		// 两条 session_meta（base_instructions 相同）→ 系统提示词去重为 1 条。
		{ID: 1, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeSessionMeta, Payload: []byte(sessionMetaPayload)},
		{ID: 2, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeSessionMeta, Payload: []byte(sessionMetaPayload)},
		{ID: 3, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg, Event: "user_message", TurnID: "t1", Payload: []byte(`{"type":"event_msg","payload":{"type":"user_message","turn_id":"t1","message":"hi"}}`)},
	}

	conversation, err := trace.BuildConversationFor(constant.TraceAgentCodex, records)
	if err != nil {
		t.Fatal(err)
	}

	// 工具定义：2 个工具，按首次出现顺序保留。
	if len(conversation.Tools) != 2 {
		t.Fatalf("expected 2 tool definitions, got %d: %+v", len(conversation.Tools), conversation.Tools)
	}
	first := conversation.Tools[0]
	if first.Namespace != "codex_app" || first.Name != "automation_update" ||
		first.Description != "Create or update automations." || first.Parameters == "" {
		t.Fatalf("first tool definition not projected: %+v", first)
	}
	second := conversation.Tools[1]
	if second.Namespace != "codex_app" || second.Name != "read_file" {
		t.Fatalf("second tool definition not projected: %+v", second)
	}

	// 系统提示词：投影为 role=system 的 message，去重后只剩 1 条。
	var systemItems []*trace.ConversationItem
	for _, turn := range conversation.Turns {
		for _, item := range turn.Items {
			if item.Role == constant.TraceConversationRoleSystem {
				systemItems = append(systemItems, item)
			}
		}
	}
	if len(systemItems) != 1 {
		t.Fatalf("expected 1 deduped system message, got %d: %+v", len(systemItems), systemItems)
	}
	if systemItems[0].Kind != constant.TraceConversationKindMessage ||
		systemItems[0].Content != "You are a coding agent running in the Codex CLI." {
		t.Fatalf("system prompt not projected as message: %+v", systemItems[0])
	}
}
