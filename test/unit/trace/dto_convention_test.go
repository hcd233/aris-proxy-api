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
// 契约丢失字段定义、校验下沉到 usecase 层。业务字段 + RawPayload() 透传存储才是正确模式。
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
	// Body 必须是结构体指针，禁止回退到 []byte 透传
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

// TestReportTraceEventReqBody_RawPayloadPreservesDynamicFields 验证自定义 UnmarshalJSON
// 在解析业务字段的同时保留原始 stdin JSON，动态字段（prompt / tool_input 等）不丢失，
// 供 events.payload 完整透传存储。
func TestReportTraceEventReqBody_RawPayloadPreservesDynamicFields(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"hook_event_name":"UserPromptSubmit","session_id":"s1","prompt":"hello","tool_input":{"cmd":"ls"}}`)
	body := &dto.ReportTraceEventReqBody{}
	if err := sonic.Unmarshal(raw, body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.HookEventName != "UserPromptSubmit" || body.SessionID != "s1" {
		t.Fatalf("business fields not parsed: %+v", body)
	}
	got := string(body.RawPayload())
	if !strings.Contains(got, `"prompt":"hello"`) {
		t.Errorf("RawPayload lost dynamic field prompt: %s", got)
	}
	if !strings.Contains(got, `"tool_input":{"cmd":"ls"}`) {
		t.Errorf("RawPayload lost dynamic field tool_input: %s", got)
	}
}
