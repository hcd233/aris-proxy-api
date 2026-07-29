package trace

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/command"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/application/trace/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TestE2E_ClaudeTraceFlow 验证 claude 上报 → 入库 → 投影全链路。
func TestE2E_ClaudeTraceFlow(t *testing.T) {
	t.Parallel()
	repo := NewFakeRepo()
	report := command.NewReportTraceEventHandler(repo)
	conversationQuery := query.NewListTraceConversationHandler(repo, newFakeAPIKeyRepo(map[uint][]string{7: {"k"}}))
	ctx := context.Background()

	seq := int64(0)
	hook := func(event, turnID, payload string) {
		t.Helper()
		seq++
		_, err := report.Handle(ctx, port.ReportTraceEventCommand{
			SessionID: "claude-e2e", Agent: constant.TraceAgentClaude, UserID: 7, APIKeyName: "k",
			Records: []port.ReportTraceRecord{{
				Source: constant.TraceRecordSourceHook, RecordType: constant.TraceRecordTypeHookEvent,
				HookEventName: event, Event: event, TurnID: turnID,
				ClientSequence: seq, DedupKey: "hook:claude-e2e:" + event + turnID,
				Payload: []byte(payload),
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	transcript := func(recordType, event, payload string) {
		t.Helper()
		seq++
		_, err := report.Handle(ctx, port.ReportTraceEventCommand{
			SessionID: "claude-e2e", Agent: constant.TraceAgentClaude, UserID: 7, APIKeyName: "k",
			Records: []port.ReportTraceRecord{{
				Source: constant.TraceRecordSourceRollout, RecordType: recordType, Event: event,
				ClientSequence: seq, DedupKey: "rollout:claude-e2e:" + event + payload[:16],
				Payload: []byte(payload),
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	hook("SessionStart", "", `{"hook_event_name":"SessionStart","session_id":"claude-e2e","model":"claude-sonnet-5","source":"startup"}`)
	hook("UserPromptSubmit", "p1", `{"hook_event_name":"UserPromptSubmit","prompt_id":"p1","prompt":"列一下文件"}`)
	transcript(constant.TraceClaudeRecordUser, constant.TraceClaudeEventUserPrompt, `{"type":"user","uuid":"u1","promptId":"p1","message":{"role":"user","content":"列一下文件"}}`)
	transcript(constant.TraceClaudeRecordAssistant, constant.TraceClaudeEventAssistantMessage, `{"type":"assistant","uuid":"a1","message":{"role":"assistant","content":[{"type":"text","text":"好的"},{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`)
	transcript(constant.TraceClaudeRecordUser, constant.TraceClaudeEventToolResult, `{"type":"user","uuid":"u2","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file.go"}]}}`)
	hook("Stop", "p1", `{"hook_event_name":"Stop","prompt_id":"p1","last_assistant_message":"完成"}`)

	tr, err := repo.FindBySessionID(ctx, "claude-e2e")
	if err != nil || tr == nil {
		t.Fatalf("trace missing: %v", err)
	}
	if tr.Agent != constant.TraceAgentClaude || tr.Status != constant.TraceStatusActive {
		t.Fatalf("unexpected trace: %+v", tr)
	}

	view, err := conversationQuery.Handle(ctx, port.ListTraceConversationQuery{UserID: 7, TraceID: tr.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Turns) != 1 || view.Turns[0].TurnID != "u1" {
		t.Fatalf("unexpected turns: %+v", view.Turns)
	}
	var tool *port.TraceConversationItemView
	for _, item := range view.Turns[0].Items {
		if item.Kind == constant.TraceConversationKindToolCall {
			tool = item
		}
	}
	if tool == nil || tool.Output != "file.go" {
		t.Fatalf("tool call projection broken: %+v", tool)
	}

	hook(constant.TraceEventSessionEnd, "", `{"hook_event_name":"SessionEnd","session_id":"claude-e2e","reason":"other"}`)
	tr, _ = repo.FindBySessionID(ctx, "claude-e2e")
	if tr.Status != constant.TraceStatusDone {
		t.Fatalf("SessionEnd must mark trace done, got %s", tr.Status)
	}
}
