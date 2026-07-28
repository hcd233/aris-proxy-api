package trace

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

func TestBuildConversation_PrefersRolloutAndPairsToolOutput(t *testing.T) {
	t.Parallel()

	records := []*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg, Event: "user_message", TurnID: "t1", Payload: []byte(`{"type":"event_msg","payload":{"type":"user_message","turn_id":"t1","message":"检查当前目录"}}`)},
		{ID: 2, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "UserPromptSubmit", TurnID: "t1", Payload: []byte(`{"hook_event_name":"UserPromptSubmit","prompt":"检查当前目录"}`)},
		{ID: 3, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeResponseItem, Event: "function_call", TurnID: "t1", CallID: "call-1", Payload: []byte(`{"type":"response_item","payload":{"type":"function_call","turn_id":"t1","name":"exec_command","call_id":"call-1","arguments":"{\"cmd\":\"pwd\"}"}}`)},
		{ID: 4, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeResponseItem, Event: "function_call_output", TurnID: "t1", CallID: "call-1", Payload: []byte(`{"type":"response_item","payload":{"type":"function_call_output","turn_id":"t1","call_id":"call-1","output":"/work"}}`)},
		{ID: 5, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeEventMsg, Event: "agent_message", TurnID: "t1", Payload: []byte(`{"type":"event_msg","payload":{"type":"agent_message","turn_id":"t1","message":"当前目录是 /work"}}`)},
	}

	conversation := trace.BuildConversation(records)
	if len(conversation.Turns) != 1 || len(conversation.Turns[0].Items) != 3 {
		t.Fatalf("unexpected conversation: %+v", conversation)
	}
	tool := conversation.Turns[0].Items[1]
	if tool.Kind != constant.TraceConversationKindToolCall || tool.CallID != "call-1" || tool.Output != "/work" {
		t.Fatalf("tool call/output not paired: %+v", tool)
	}
}

func TestBuildConversation_ReplacesEarlierHookWithRollout(t *testing.T) {
	t.Parallel()

	conversation := trace.BuildConversation([]*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, Event: constant.TraceEventUserPromptSubmit, TurnID: "t1", Payload: []byte(`{"prompt":"hello"}`)},
		{ID: 2, Source: constant.TraceRecordSourceRollout, Event: constant.TraceConversationEventUserMessage, TurnID: "t1", Payload: []byte(`{"type":"event_msg","payload":{"type":"user_message","turn_id":"t1","message":"hello"}}`)},
	})
	item := conversation.Turns[0].Items[0]
	if len(conversation.Turns[0].Items) != 1 || item.Source != constant.TraceRecordSourceRollout ||
		len(item.RecordIDs) != 1 || item.RecordIDs[0] != 2 {
		t.Fatalf("rollout did not replace Hook fallback: %+v", conversation.Turns[0].Items)
	}
}

func TestBuildConversation_SkipsRecordsWithoutConversationItems(t *testing.T) {
	t.Parallel()

	conversation := trace.BuildConversation([]*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventSessionStart, Payload: []byte(`{"hook_event_name":"SessionStart","session_id":"s1"}`)},
		{ID: 2, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeSessionMeta, Event: "session_meta", Payload: []byte(`{"type":"session_meta","payload":{"id":"s1"}}`)},
		{ID: 3, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventUserPromptSubmit, TurnID: "t1", Payload: []byte(`{"prompt":"hello"}`)},
		{ID: 4, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventStop, TurnID: "t1", Payload: []byte(`{"last_assistant_message":"world"}`)},
	})
	if len(conversation.Turns) != 1 || conversation.Turns[0].TurnID != "t1" || len(conversation.Turns[0].Items) != 2 {
		t.Fatalf("records without conversation items must not create turns: %+v", conversation)
	}
}

func TestBuildConversation_ProjectsHookToolCalls(t *testing.T) {
	t.Parallel()

	// 镜像 trace 34 的 legacy shell 上报格式：无顶层 callId，tool_use_id 仅在 payload 内。
	conversation := trace.BuildConversation([]*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventPreToolUse, TurnID: "t1", Payload: []byte(`{"hook_event_name":"PreToolUse","turn_id":"t1","tool_name":"Bash","tool_use_id":"call-1","tool_input":{"command":"ls"}}`)},
		{ID: 2, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventPreToolUse, TurnID: "t1", CallID: "call-2", Payload: []byte(`{"hook_event_name":"PreToolUse","turn_id":"t1","tool_name":"Read","tool_use_id":"call-2","tool_input":{"file_path":"/tmp/a"}}`)},
		{ID: 3, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventPostToolUse, TurnID: "t1", Payload: []byte(`{"hook_event_name":"PostToolUse","turn_id":"t1","tool_name":"Bash","tool_use_id":"call-1","tool_response":"ok"}`)},
		{ID: 4, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventPostToolUse, TurnID: "t1", CallID: "call-2", Payload: []byte(`{"hook_event_name":"PostToolUse","turn_id":"t1","tool_name":"Read","tool_use_id":"call-2","tool_response":"file content"}`)},
	})
	if len(conversation.Turns) != 1 || len(conversation.Turns[0].Items) != 2 {
		t.Fatalf("expected 2 tool calls, got %+v", conversation)
	}
	first := conversation.Turns[0].Items[0]
	if first.Kind != constant.TraceConversationKindToolCall || first.ToolName != "Bash" || first.CallID != "call-1" ||
		first.Arguments != `{"command":"ls"}` || first.Output != "ok" ||
		len(first.RecordIDs) != 2 || first.RecordIDs[0] != 1 || first.RecordIDs[1] != 3 {
		t.Fatalf("hook tool call not projected/paired: %+v", first)
	}
	second := conversation.Turns[0].Items[1]
	if second.ToolName != "Read" || second.CallID != "call-2" || second.Output != "file content" {
		t.Fatalf("record-level callId fallback failed: %+v", second)
	}
}

func TestBuildConversation_UpgradesHookToolCallWithRollout(t *testing.T) {
	t.Parallel()

	conversation := trace.BuildConversation([]*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: constant.TraceEventPreToolUse, TurnID: "t1", Payload: []byte(`{"hook_event_name":"PreToolUse","turn_id":"t1","tool_name":"Bash","tool_use_id":"call-1","tool_input":{"command":"ls"}}`)},
		{ID: 2, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeResponseItem, Event: constant.TraceConversationEventFunctionCall, TurnID: "t1", CallID: "call-1", Payload: []byte(`{"type":"response_item","payload":{"type":"function_call","turn_id":"t1","name":"exec_command","call_id":"call-1","arguments":"{\"cmd\":\"ls\"}"}}`)},
		{ID: 3, Source: constant.TraceRecordSourceRollout, RecordType: constant.TraceRecordTypeResponseItem, Event: constant.TraceConversationEventFunctionOutput, TurnID: "t1", CallID: "call-1", Payload: []byte(`{"type":"response_item","payload":{"type":"function_call_output","turn_id":"t1","call_id":"call-1","output":"ok"}}`)},
	})
	if len(conversation.Turns) != 1 || len(conversation.Turns[0].Items) != 1 {
		t.Fatalf("duplicate tool call not deduped: %+v", conversation)
	}
	item := conversation.Turns[0].Items[0]
	if item.Source != constant.TraceRecordSourceRollout || item.ToolName != "exec_command" || item.Output != "ok" ||
		len(item.RecordIDs) != 3 {
		t.Fatalf("hook tool call not upgraded by rollout: %+v", item)
	}
}

func TestBuildConversation_UsesHookFallback(t *testing.T) {
	t.Parallel()

	conversation := trace.BuildConversation([]*trace.TraceEvent{
		{ID: 1, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "UserPromptSubmit", TurnID: "t1", Payload: []byte(`{"prompt":"hello"}`)},
		{ID: 2, Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent, Event: "Stop", TurnID: "t1", Payload: []byte(`{"last_assistant_message":"world"}`)},
	})
	if len(conversation.Turns) != 1 || len(conversation.Turns[0].Items) != 2 {
		t.Fatalf("expected Hook fallback items, got %+v", conversation)
	}
	if conversation.Turns[0].Items[0].Content != "hello" || conversation.Turns[0].Items[1].Content != "world" {
		t.Fatalf("unexpected fallback content: %+v", conversation.Turns[0].Items)
	}
}
