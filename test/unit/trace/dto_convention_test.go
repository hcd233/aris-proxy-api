package trace

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TestReportTraceEventReq_DTOFollowsHumaBodyConvention 防回归：上报接口的请求体必须是
// 实打实的结构体（*ReportTraceEventReqBody），而不是 []byte 透传。[]byte 会让 OpenAPI
// 契约丢失字段定义、校验下沉到 usecase 层。
func TestReportTraceEventReq_DTOFollowsHumaBodyConvention(t *testing.T) {
	t.Parallel()
	reqType := reflect.TypeOf(dto.ReportTraceEventReq{})
	bodyField, ok := reqType.FieldByName("Body")
	if !ok {
		t.Fatal("ReportTraceEventReq must have a Body field for huma JSON body binding")
	}
	if bodyField.Tag.Get("json") != "body" {
		t.Errorf(`ReportTraceEventReq.Body json tag = %q, want "body"`, bodyField.Tag.Get("json"))
	}
	if bodyField.Type.Kind() != reflect.Pointer || bodyField.Type.Elem().Name() != "ReportTraceEventReqBody" {
		t.Fatalf("ReportTraceEventReq.Body must be *ReportTraceEventReqBody, got %s", bodyField.Type)
	}

	bodyType := reflect.TypeOf(dto.ReportTraceEventReqBody{})
	hookField, ok := bodyType.FieldByName("HookEventName")
	if !ok {
		t.Fatal("ReportTraceEventReqBody must have HookEventName field")
	}
	if hookField.Tag.Get("json") != "hook_event_name" {
		t.Errorf(`HookEventName json tag = %q, want "hook_event_name"`, hookField.Tag.Get("json"))
	}
	if hookField.Tag.Get("required") != "true" {
		t.Errorf(`HookEventName required tag = %q, want "true"`, hookField.Tag.Get("required"))
	}

	sessionField, ok := bodyType.FieldByName("SessionID")
	if !ok {
		t.Fatal("ReportTraceEventReqBody must have SessionID field")
	}
	if sessionField.Tag.Get("json") != "session_id" {
		t.Errorf(`SessionID json tag = %q, want "session_id"`, sessionField.Tag.Get("json"))
	}
	if sessionField.Tag.Get("required") != "true" {
		t.Errorf(`SessionID required tag = %q, want "true"`, sessionField.Tag.Get("required"))
	}
}

// TestReportTraceEventReqBody_HasNoByteFields 防回归：DTO 不得出现任何 []byte 字段
// （含 json:"-" 的隐藏透传字段）。任意 JSON 必须用 sonic.NoCopyRawMessage 建模。
func TestReportTraceEventReqBody_HasNoByteFields(t *testing.T) {
	t.Parallel()
	bodyType := reflect.TypeOf(dto.ReportTraceEventReqBody{})
	byteType := reflect.TypeOf([]byte(nil))
	for i := 0; i < bodyType.NumField(); i++ {
		f := bodyType.Field(i)
		// 只拦匿名 []byte；sonic.NoCopyRawMessage 是 named type，允许用于任意 JSON
		if f.Type == byteType {
			t.Errorf("ReportTraceEventReqBody.%s is []byte — DTO must use concrete types, not []byte passthrough", f.Name)
		}
	}
}

// TestReportTraceEventReqBody_MarshalPreservesDynamicFields 验证结构体序列化后任意 JSON
// 字段（tool_input 等）完整保留，供 events.payload 透传存储。
func TestReportTraceEventReqBody_MarshalPreservesDynamicFields(t *testing.T) {
	t.Parallel()
	body := &dto.ReportTraceEventReqBody{
		HookEventName: "PreToolUse",
		SessionID:     "s1",
		ToolName:      "Bash",
		ToolInput:     sonic.NoCopyRawMessage(`{"command":"ls"}`),
	}
	raw, err := sonic.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(raw)
	if !strings.Contains(got, `"hook_event_name":"PreToolUse"`) {
		t.Errorf("marshal lost hook_event_name: %s", got)
	}
	if !strings.Contains(got, `"tool_input":{"command":"ls"}`) {
		t.Errorf("marshal lost tool_input: %s", got)
	}
}
