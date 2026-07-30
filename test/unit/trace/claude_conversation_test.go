package trace

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func claudeRecords() []*trace.TraceEvent {
	return []*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "UserPromptSubmit", TurnID: "p1", Payload: []byte(`{"hook_event_name":"UserPromptSubmit","prompt_id":"p1","prompt":"列一下文件"}`)},
		{ID: 2, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventUserPrompt, Payload: []byte(`{"type":"user","uuid":"u1","promptId":"p1","message":{"role":"user","content":"列一下文件"}}`)},
		{ID: 3, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordAssistant, Event: constant.TraceClaudeEventAssistantMessage, Payload: []byte(`{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"thinking","thinking":"看一下"},{"type":"text","text":"我来列一下"},{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`)},
		{ID: 4, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventToolResult, CallID: "toolu_1", Payload: []byte(`{"type":"user","uuid":"u2","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file.go"}]}}`)},
		{ID: 5, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordAssistant, Event: constant.TraceClaudeEventAssistantMessage, Payload: []byte(`{"type":"assistant","uuid":"a2","message":{"role":"assistant","content":[{"type":"text","text":"目录下只有 file.go"}]}}`)},
		{ID: 6, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "Stop", TurnID: "p1", Payload: []byte(`{"hook_event_name":"Stop","prompt_id":"p1","last_assistant_message":"目录下只有 file.go"}`)},
		{ID: 7, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordUser, Event: constant.TraceClaudeEventUserPrompt, Payload: []byte(`{"type":"user","uuid":"u3","promptId":"p2","isSidechain":false,"message":{"role":"user","content":"第二个问题"}}`)},
		{ID: 8, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceClaudeRecordAssistant, Event: constant.TraceClaudeEventAssistantMessage, Payload: []byte(`{"type":"assistant","uuid":"a3","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"子代理输出"}]}}`)},
		{ID: 9, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "PreToolUse", TurnID: "p1", Payload: []byte(`{"hook_event_name":"PreToolUse","prompt_id":"p1","agent_id":"sub-1","tool_use_id":"toolu_sub","tool_name":"Bash","tool_input":{"command":"pwd"}}`)},
	}
}

func TestBuildClaudeConversation_TurnsMessagesAndTools(t *testing.T) {
	t.Parallel()
	conversation, err := trace.BuildConversationFor(constant.TraceAgentClaude, claudeRecords())
	if err != nil {
		t.Fatal(err)
	}
	if len(conversation.Turns) != 2 {
		t.Fatalf("expected 2 turns, got %d: %+v", len(conversation.Turns), conversation.Turns)
	}
	turn := conversation.Turns[0]
	if turn.TurnID != "u1" {
		t.Fatalf("turn key must be transcript user uuid, got %q", turn.TurnID)
	}
	var userMsgs, assistantMsgs, toolCalls int
	var tool *trace.ConversationItem
	for _, item := range turn.Items {
		switch {
		case item.Kind == constant.TraceConversationKindMessage && item.Role == constant.TraceConversationRoleUser:
			userMsgs++
		case item.Kind == constant.TraceConversationKindMessage && item.Role == constant.TraceConversationRoleAssistant:
			assistantMsgs++
		case item.Kind == constant.TraceConversationKindToolCall:
			toolCalls++
			tool = item
		}
		if item.Content == "看一下" {
			t.Fatal("thinking block must be skipped")
		}
	}
	if userMsgs != 1 || assistantMsgs != 2 || toolCalls != 1 {
		t.Fatalf("user=%d assistant=%d tools=%d, want 1/2/1", userMsgs, assistantMsgs, toolCalls)
	}
	if tool == nil || tool.CallID != "toolu_1" || tool.ToolName != "Bash" {
		t.Fatalf("unexpected tool call: %+v", tool)
	}
	if tool.Output != "file.go" {
		t.Fatalf("tool output = %q, want file.go", tool.Output)
	}
	if tool.Arguments != `{"command":"ls"}` {
		t.Fatalf("tool arguments = %q", tool.Arguments)
	}
}

func TestBuildClaudeConversation_SkipsSidechainAndSubagentHooks(t *testing.T) {
	t.Parallel()
	conversation, err := trace.BuildConversationFor(constant.TraceAgentClaude, claudeRecords())
	if err != nil {
		t.Fatal(err)
	}
	for _, turn := range conversation.Turns {
		for _, item := range turn.Items {
			if item.Content == "子代理输出" || item.CallID == "toolu_sub" {
				t.Fatalf("subagent content must not enter projection: %+v", item)
			}
		}
	}
	if conversation.Turns[1].TurnID != "u3" {
		t.Fatalf("second turn key = %q, want u3", conversation.Turns[1].TurnID)
	}
}

func TestBuildClaudeConversation_HookFallbackWithoutTranscript(t *testing.T) {
	t.Parallel()
	records := []*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "UserPromptSubmit", TurnID: "p9", Payload: []byte(`{"hook_event_name":"UserPromptSubmit","prompt_id":"p9","prompt":"hi"}`)},
		{ID: 2, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "PreToolUse", TurnID: "p9", Payload: []byte(`{"hook_event_name":"PreToolUse","prompt_id":"p9","tool_name":"Bash","tool_use_id":"toolu_9","tool_input":{"command":"ls"}}`)},
		{ID: 3, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "PostToolUse", TurnID: "p9", Payload: []byte(`{"hook_event_name":"PostToolUse","prompt_id":"p9","tool_use_id":"toolu_9","tool_response":{"stdout":"ok"}}`)},
	}
	conversation, err := trace.BuildConversationFor(constant.TraceAgentClaude, records)
	if err != nil {
		t.Fatal(err)
	}
	if len(conversation.Turns) != 1 || len(conversation.Turns[0].Items) != 2 {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}
	var tool *trace.ConversationItem
	for _, item := range conversation.Turns[0].Items {
		if item.Kind == constant.TraceConversationKindToolCall {
			tool = item
		}
	}
	if tool == nil || tool.Output == "" {
		t.Fatalf("hook tool output must be paired: %+v", tool)
	}
}

func TestBuildConversationFor_UnknownAgent(t *testing.T) {
	t.Parallel()
	if _, err := trace.BuildConversationFor("gemini", nil); err == nil {
		t.Fatal("unknown agent must return error")
	}
}
