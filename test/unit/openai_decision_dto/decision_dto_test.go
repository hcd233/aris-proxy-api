package openai_decision_dto

import (
	"reflect"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TestOpenAICreateDecisionRequest_DTOFollowsHumaBodyConvention 防回归：请求 DTO 必须按 huma 的
// "外层 Req + Body 包装" 模式定义，否则 POST body 反序列化会被 huma 忽略（线上事故模式）。
func TestOpenAICreateDecisionRequest_DTOFollowsHumaBodyConvention(t *testing.T) {
	t.Parallel()

	reqType := reflect.TypeOf(dto.OpenAICreateDecisionRequest{})
	bodyField, ok := reqType.FieldByName("Body")
	if !ok {
		t.Fatal("OpenAICreateDecisionRequest must have a Body field for huma JSON body binding")
	}
	if got := bodyField.Tag.Get("json"); got != "body" {
		t.Errorf(`OpenAICreateDecisionRequest.Body json tag = %q, want "body"`, got)
	}
	if _, exists := reqType.FieldByName("Model"); exists {
		t.Error("OpenAICreateDecisionRequest must NOT have top-level Model field; it belongs in OpenAICreateDecisionReq")
	}

	bodyType := reflect.TypeOf(dto.OpenAICreateDecisionReq{})
	for _, name := range []string{"Model", "Input", "Questions", "SafetyIdentifier"} {
		if _, ok := bodyType.FieldByName(name); !ok {
			t.Errorf("OpenAICreateDecisionReq must have %s field", name)
		}
	}
}

func TestDecisionInput_StringOrMessages(t *testing.T) {
	t.Parallel()

	var textInput dto.DecisionInput
	if err := sonic.UnmarshalString(`"hello"`, &textInput); err != nil {
		t.Fatalf("unmarshal string input: %v", err)
	}
	if textInput.Text == nil || *textInput.Text != "hello" {
		t.Fatalf("text input = %+v, want hello", textInput)
	}

	var msgInput dto.DecisionInput
	raw := `[{"role":"user","content":[{"type":"input_text","text":"a"},{"type":"input_image","image_url":"https://x/y.png","detail":"high"}]}]`
	if err := sonic.UnmarshalString(raw, &msgInput); err != nil {
		t.Fatalf("unmarshal messages input: %v", err)
	}
	if len(msgInput.Messages) != 1 || len(msgInput.Messages[0].Content.Parts) != 2 {
		t.Fatalf("messages input = %+v, want 1 message with 2 parts", msgInput)
	}
	if got := msgInput.Messages[0].Content.Parts[1].ImageURL; got == nil || *got != "https://x/y.png" {
		t.Fatalf("image url = %v, want https://x/y.png", got)
	}
}

func TestDecisionInput_MarshalRoundTrip(t *testing.T) {
	t.Parallel()

	text := "hi"
	out, err := sonic.Marshal(dto.DecisionInput{Text: &text})
	if err != nil {
		t.Fatalf("marshal text input: %v", err)
	}
	if string(out) != `"hi"` {
		t.Fatalf("marshal text input = %s, want \"hi\"", out)
	}

	nilOut, err := sonic.Marshal(dto.DecisionInput{})
	if err != nil {
		t.Fatalf("marshal empty input: %v", err)
	}
	if string(nilOut) != "null" {
		t.Fatalf("marshal empty input = %s, want null", nilOut)
	}
}

func TestDecisionChoiceValue_BoolIsNotString(t *testing.T) {
	t.Parallel()

	var boolValue dto.DecisionChoiceValue
	if err := sonic.UnmarshalString("true", &boolValue); err != nil {
		t.Fatalf("unmarshal bool: %v", err)
	}
	if boolValue.BooleanValue == nil || !*boolValue.BooleanValue {
		t.Fatalf("bool value = %+v, want true", boolValue)
	}
	if boolValue.StringValue != nil {
		t.Fatal("bool value must not populate StringValue")
	}

	var strValue dto.DecisionChoiceValue
	if err := sonic.UnmarshalString(`"true"`, &strValue); err != nil {
		t.Fatalf("unmarshal string: %v", err)
	}
	if strValue.StringValue == nil || *strValue.StringValue != "true" {
		t.Fatalf("string value = %+v, want \"true\"", strValue)
	}
	if strValue.BooleanValue != nil {
		t.Fatal("string value must not populate BooleanValue")
	}

	out, err := sonic.Marshal(dto.DecisionChoiceValue{BooleanValue: boolValue.BooleanValue})
	if err != nil {
		t.Fatalf("marshal bool: %v", err)
	}
	if string(out) != "true" {
		t.Fatalf("marshal bool = %s, want true", out)
	}
}

func TestDecisionQuestion_ThreeTypes(t *testing.T) {
	t.Parallel()

	raw := `[
		{"type":"predicate","instructions":"is it damaged?","name":"damaged"},
		{"type":"choice","instructions":"pick","choices":[{"value":"a","description":"A"},{"value":true}]},
		{"type":"score","instructions":"rate","levels":[{"label":"low"},{"label":"high","description":"very"}]}
	]`
	var questions []*dto.DecisionQuestion
	if err := sonic.UnmarshalString(raw, &questions); err != nil {
		t.Fatalf("unmarshal questions: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("questions = %d, want 3", len(questions))
	}
	if len(questions[1].Choices) != 2 || questions[1].Choices[1].Value.BooleanValue == nil {
		t.Fatalf("choice bool not parsed: %+v", questions[1].Choices)
	}
	if len(questions[2].Levels) != 2 || questions[2].Levels[1].Description == nil {
		t.Fatalf("score levels not parsed: %+v", questions[2].Levels)
	}
}

func TestOpenAIDecisionRsp_UsageDetails(t *testing.T) {
	t.Parallel()

	raw := `{"answers":[{"type":"predicate","name":"d","probability":0.9}],
		"usage":{"input_tokens":42,"output_tokens":7,"total_tokens":49,
		"input_tokens_details":{"cached_tokens":12,"cache_write_tokens":5},
		"output_tokens_details":{"reasoning_tokens":3}}}`
	var rsp dto.OpenAIDecisionRsp
	if err := sonic.UnmarshalString(raw, &rsp); err != nil {
		t.Fatalf("unmarshal rsp: %v", err)
	}
	if rsp.Usage == nil || rsp.Usage.InputTokensDetails == nil {
		t.Fatalf("usage = %+v, want details", rsp.Usage)
	}
	if rsp.Usage.InputTokensDetails.CachedTokens != 12 || rsp.Usage.InputTokensDetails.CacheWriteTokens != 5 {
		t.Fatalf("input details = %+v", rsp.Usage.InputTokensDetails)
	}
	if got := rsp.Usage.InputOutputTokens(); got != 49 {
		t.Fatalf("InputOutputTokens() = %d, want 49", got)
	}
	if string(rsp.Answers) == "" {
		t.Fatal("answers must be preserved raw")
	}
}
