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
