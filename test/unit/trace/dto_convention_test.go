package trace

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	traceschema "github.com/hcd233/aris-proxy-api/internal/dto/schema"
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
	if hookField.Tag.Get("json") != "hook_event_name,omitempty" {
		t.Errorf(`HookEventName json tag = %q, want "hook_event_name,omitempty"`, hookField.Tag.Get("json"))
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

// TestTraceQueryReqs_UseIDQueryParameter keeps trace GET endpoints consistent
// with the rest of the management API and the frontend client.
func TestTraceQueryReqs_UseIDQueryParameter(t *testing.T) {
	t.Parallel()

	for _, req := range []struct {
		name   string
		typeOf reflect.Type
	}{
		{name: "get trace", typeOf: reflect.TypeOf(dto.GetTraceReq{})},
		{name: "list trace events", typeOf: reflect.TypeOf(dto.ListTraceEventsReq{})},
	} {
		t.Run(req.name, func(t *testing.T) {
			t.Parallel()

			field, ok := req.typeOf.FieldByName("TraceID")
			if !ok {
				t.Fatal("trace request must have TraceID field")
			}
			if got := field.Tag.Get("query"); got != "id" {
				t.Errorf(`TraceID query tag = %q, want "id"`, got)
			}
		})
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
		ToolInput:     traceschema.RawJSON(`{"command":"ls"}`),
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

func TestReportTraceEventReq_PreservesUnknownRawRecordFields(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("./fixtures/raw_records.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var body dto.ReportTraceEventReqBody
	if err := sonic.Unmarshal(data, &body); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(body.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(body.Records))
	}

	var hookPayload map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(body.Records[0].Payload, &hookPayload); err != nil {
		t.Fatalf("unmarshal hook payload: %v", err)
	}
	if _, ok := hookPayload["future_field"]; !ok {
		t.Fatal("hook unknown field was dropped")
	}

	var rolloutEnvelope map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(body.Records[1].Payload, &rolloutEnvelope); err != nil {
		t.Fatalf("unmarshal rollout payload: %v", err)
	}
	var rolloutPayload map[string]sonic.NoCopyRawMessage
	if err := sonic.Unmarshal(rolloutEnvelope["payload"], &rolloutPayload); err != nil {
		t.Fatalf("unmarshal rollout payload body: %v", err)
	}
	if string(rolloutPayload["future_field"]) != `"preserved"` {
		t.Fatalf("rollout unknown field changed: %s", rolloutPayload["future_field"])
	}
}
