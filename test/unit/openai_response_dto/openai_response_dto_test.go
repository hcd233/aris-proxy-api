// Package openai_response_dto verifies the OpenAI Response API request DTO
// accepts and round-trips every documented shape of
// docs/openai/create_response.md body parameters.
package openai_response_dto

import (
	"os"
	"reflect"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// responseDTOCase mirrors the structure of fixtures/cases.json
type responseDTOCase struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	RequestBody   sonic.NoCopyRawMessage `json:"request_body"`
	ExpectedModel string                 `json:"expected_model,omitempty"`
}

func loadCases(t *testing.T) []responseDTOCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []responseDTOCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func findCase(t *testing.T, cases []responseDTOCase, name string) responseDTOCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return responseDTOCase{}
}

// jsonEqual compares two raw JSON byte slices semantically (ignoring whitespace
// and key order) by re-decoding both into generic structures.
func jsonEqual(t *testing.T, got, want []byte) bool {
	t.Helper()
	var a, b any
	if err := sonic.Unmarshal(got, &a); err != nil {
		t.Fatalf("failed to decode got json: %v\nraw: %s", err, string(got))
	}
	if err := sonic.Unmarshal(want, &b); err != nil {
		t.Fatalf("failed to decode want json: %v\nraw: %s", err, string(want))
	}
	return reflect.DeepEqual(a, b)
}

// TestOpenAICreateResponseReq_RoundTripAll verifies that every fixture body
// unmarshals and re-marshals to the same semantic JSON through the typed DTO.
func TestOpenAICreateResponseReq_RoundTripAll(t *testing.T) {
	allCases := loadCases(t)
	for _, tc := range allCases {
		t.Run(tc.Name, func(t *testing.T) {
			var req dto.OpenAICreateResponseReq
			if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
				t.Fatalf("unmarshal failed: %v\nbody: %s", err, string(tc.RequestBody))
			}
			got, err := sonic.Marshal(&req)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			if !jsonEqual(t, got, tc.RequestBody) {
				t.Errorf("round trip mismatch:\n got:  %s\n want: %s", string(got), string(tc.RequestBody))
			}
		})
	}
}

// TestOpenAICreateResponseReq_ClientMetadata asserts that Codex Desktop's
// client_metadata field is modeled as a typed map and round-trips cleanly.
func TestOpenAICreateResponseReq_ClientMetadata(t *testing.T) {
	tc := findCase(t, loadCases(t), "create_response_codex_client_metadata")
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(req.ClientMetadata) != 1 {
		t.Fatalf("ClientMetadata len = %d, want 1", len(req.ClientMetadata))
	}
	got := req.ClientMetadata["x-codex-installation-id"]
	want := "22ea42b7-a329-4c89-b272-29ec63753c29"
	if got != want {
		t.Errorf("ClientMetadata[x-codex-installation-id] = %q, want %q", got, want)
	}
}

// TestOpenAICreateResponseReq_CodexFullBody asserts the key routing fields
// (model/stream/store/parallel_tool_calls/input/tools/tool_choice/reasoning/text/
// include/prompt_cache_key/instructions) parse correctly from the Codex body.
func TestOpenAICreateResponseReq_CodexFullBody(t *testing.T) {
	tc := findCase(t, loadCases(t), "create_response_codex_full")
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if lo.FromPtr(req.Model) != "gpt-5.4" {
		t.Errorf("Model = %q, want gpt-5.4", lo.FromPtr(req.Model))
	}
	if req.Stream == nil || *req.Stream != true {
		t.Errorf("Stream should be true")
	}
	if req.Store == nil || *req.Store != false {
		t.Errorf("Store should be false")
	}
	if req.ParallelToolCalls == nil || *req.ParallelToolCalls != true {
		t.Errorf("ParallelToolCalls should be true")
	}
	if req.Instructions == nil || *req.Instructions == "" {
		t.Errorf("Instructions should be populated")
	}
	if req.PromptCacheKey == nil || *req.PromptCacheKey == "" {
		t.Errorf("PromptCacheKey should be populated")
	}

	// Input array with 2 messages
	if req.Input == nil {
		t.Fatal("Input should be populated")
	}
	if len(req.Input.Items) != 2 {
		t.Fatalf("Input.Items len = %d, want 2", len(req.Input.Items))
	}
	if lo.FromPtr(req.Input.Items[0].Role) != "developer" || lo.FromPtr(req.Input.Items[0].Type) != "message" {
		t.Errorf("Input.Items[0] role/type mismatch: role=%q type=%q", lo.FromPtr(req.Input.Items[0].Role), lo.FromPtr(req.Input.Items[0].Type))
	}

	// Reasoning
	if req.Reasoning == nil || lo.FromPtr(req.Reasoning.Effort) != "medium" || lo.FromPtr(req.Reasoning.Summary) != "auto" {
		t.Errorf("Reasoning not correctly populated: %+v", req.Reasoning)
	}

	// Text.format.type=json_schema
	if req.Text == nil || req.Text.Format == nil || req.Text.Format.Type != "json_schema" {
		t.Errorf("Text.Format.Type = json_schema expected, got %+v", req.Text)
	}
	if lo.FromPtr(req.Text.Verbosity) != "low" {
		t.Errorf("Text.Verbosity = %q, want low", lo.FromPtr(req.Text.Verbosity))
	}

	// Tools: function
	if len(req.Tools) != 1 || req.Tools[0].Function == nil || req.Tools[0].Function.Name != "exec_command" {
		t.Errorf("Tools[0] function name mismatch: %+v", req.Tools)
	}

	// tool_choice == "auto"
	if req.ToolChoice == nil || req.ToolChoice.Mode != "auto" {
		t.Errorf("ToolChoice.Mode = %q, want auto", func() string {
			if req.ToolChoice == nil {
				return "<nil>"
			}
			return req.ToolChoice.Mode
		}())
	}

	// Include
	if len(req.Include) != 1 || req.Include[0] != "reasoning.encrypted_content" {
		t.Errorf("Include = %v, want [reasoning.encrypted_content]", req.Include)
	}
}

// TestOpenAICreateResponseReq_Minimal ensures minimal body with only model parses.
func TestOpenAICreateResponseReq_Minimal(t *testing.T) {
	tc := findCase(t, loadCases(t), "create_response_minimal")
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if lo.FromPtr(req.Model) != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", lo.FromPtr(req.Model))
	}
	if req.Stream != nil {
		t.Errorf("Stream should be nil")
	}
}

// TestOpenAICreateResponseReq_Scalars verifies typed scalar fields.
func TestOpenAICreateResponseReq_Scalars(t *testing.T) {
	tc := findCase(t, loadCases(t), "create_response_scalars")
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.MaxOutputTokens == nil || *req.MaxOutputTokens != 4096 {
		t.Errorf("MaxOutputTokens = %v, want 4096", req.MaxOutputTokens)
	}
	if req.MaxToolCalls == nil || *req.MaxToolCalls != 32 {
		t.Errorf("MaxToolCalls = %v, want 32", req.MaxToolCalls)
	}
	if req.Temperature == nil || *req.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", req.Temperature)
	}
	if req.TopLogprobs == nil || *req.TopLogprobs != 5 {
		t.Errorf("TopLogprobs = %v, want 5", req.TopLogprobs)
	}
	if req.Metadata["k1"] != "v1" {
		t.Errorf("Metadata[k1] = %q, want v1", req.Metadata["k1"])
	}
}

// TestOpenAICreateResponseReq_ToolChoiceVariants covers string/function/mcp forms.
func TestOpenAICreateResponseReq_ToolChoiceVariants(t *testing.T) {
	allCases := loadCases(t)

	t.Run("string", func(t *testing.T) {
		tc := findCase(t, allCases, "tool_choice_string")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.ToolChoice == nil || req.ToolChoice.Mode != "required" || req.ToolChoice.Object != nil {
			t.Errorf("expected Mode=required, got %+v", req.ToolChoice)
		}
	})

	t.Run("function", func(t *testing.T) {
		tc := findCase(t, allCases, "tool_choice_object_function")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.ToolChoice == nil || req.ToolChoice.Object == nil ||
			req.ToolChoice.Object.Type != "function" || lo.FromPtr(req.ToolChoice.Object.Name) != "get_weather" {
			t.Errorf("unexpected tool_choice function: %+v", req.ToolChoice)
		}
	})

	t.Run("mcp", func(t *testing.T) {
		tc := findCase(t, allCases, "tool_choice_object_mcp")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.ToolChoice == nil || req.ToolChoice.Object == nil ||
			req.ToolChoice.Object.Type != "mcp" ||
			lo.FromPtr(req.ToolChoice.Object.ServerLabel) != "deepwiki" ||
			lo.FromPtr(req.ToolChoice.Object.Name) != "search" {
			t.Errorf("unexpected tool_choice mcp: %+v", req.ToolChoice)
		}
	})
}

// TestOpenAICreateResponseReq_InputVariants covers input as string vs array.
func TestOpenAICreateResponseReq_InputVariants(t *testing.T) {
	allCases := loadCases(t)

	t.Run("string", func(t *testing.T) {
		tc := findCase(t, allCases, "input_string")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Input == nil || req.Input.Text != "hello" || req.Input.Items != nil {
			t.Errorf("expected Input.Text=hello, got %+v", req.Input)
		}
	})

	t.Run("array_with_string_content", func(t *testing.T) {
		tc := findCase(t, allCases, "input_array_message_string_content")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Input == nil || len(req.Input.Items) != 1 {
			t.Fatalf("expected 1 input item, got %+v", req.Input)
		}
		item := req.Input.Items[0]
		if lo.FromPtr(item.Role) != "user" || item.Content == nil || item.Content.Text != "hi there" {
			t.Errorf("unexpected item: %+v content=%+v", item, item.Content)
		}
	})
}

// TestOpenAICreateResponseReq_ConversationVariants covers conversation string/object.
func TestOpenAICreateResponseReq_ConversationVariants(t *testing.T) {
	allCases := loadCases(t)

	t.Run("string", func(t *testing.T) {
		tc := findCase(t, allCases, "conversation_string")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Conversation == nil || req.Conversation.ID != "conv_abc" || req.Conversation.Param != nil {
			t.Errorf("expected Conversation.ID=conv_abc, got %+v", req.Conversation)
		}
	})

	t.Run("object", func(t *testing.T) {
		tc := findCase(t, allCases, "conversation_object")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if req.Conversation == nil || req.Conversation.Param == nil || req.Conversation.Param.ID != "conv_xyz" {
			t.Errorf("expected Conversation.Param.ID=conv_xyz, got %+v", req.Conversation)
		}
	})
}

// TestOpenAICreateResponseReq_TextJSONSchema verifies text.format with nested JSONSchemaProperty.
func TestOpenAICreateResponseReq_TextJSONSchema(t *testing.T) {
	tc := findCase(t, loadCases(t), "text_json_schema")
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.Text == nil || req.Text.Format == nil {
		t.Fatal("Text.Format should be populated")
	}
	f := req.Text.Format
	if f.Type != "json_schema" || lo.FromPtr(f.Name) != "person" || f.Strict == nil || *f.Strict != true {
		t.Errorf("unexpected format: %+v", f)
	}
	if f.Schema == nil || !f.Schema.HasType("object") {
		t.Errorf("schema not populated: %+v", f.Schema)
	}
	if len(f.Schema.Required) != 1 || f.Schema.Required[0] != "name" {
		t.Errorf("schema.required = %v, want [name]", f.Schema.Required)
	}
}

// TestOpenAICreateResponseReq_ToolsFileSearch verifies file_search compound filter parses.
func TestOpenAICreateResponseReq_ToolsFileSearch(t *testing.T) {
	tc := findCase(t, loadCases(t), "tools_file_search")
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.Tools) != 1 || req.Tools[0].FileSearch == nil {
		t.Fatalf("expected 1 file_search tool, got %+v", req.Tools)
	}
	fs := req.Tools[0].FileSearch
	if fs.Filters == nil || fs.Filters.Type != "and" || len(fs.Filters.Filters) != 2 {
		t.Errorf("compound filter not parsed: %+v", fs.Filters)
	}
	if fs.Filters.Filters[0].Type != "eq" || lo.FromPtr(fs.Filters.Filters[0].Key) != "lang" {
		t.Errorf("first inner filter mismatch: %+v", fs.Filters.Filters[0])
	}
	if v := fs.Filters.Filters[0].Value; v == nil || v.StringValue == nil || *v.StringValue != "zh" {
		t.Errorf("first filter value mismatch: %+v", v)
	}
	if v := fs.Filters.Filters[1].Value; v == nil || v.NumberValue == nil || *v.NumberValue != 2024 {
		t.Errorf("second filter value mismatch: %+v", v)
	}
}

// TestOpenAICreateResponseReq_ToolsFunctionTypeArray verifies function tool
// JSON Schema accepts union `type` arrays such as ["string","null"].
func TestOpenAICreateResponseReq_ToolsFunctionTypeArray(t *testing.T) {
	tc := findCase(t, loadCases(t), "tools_function_schema_type_array")
	var req dto.OpenAICreateResponseReq
	if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function == nil || req.Tools[0].Function.Parameters == nil {
		t.Fatalf("expected function tool parameters, got %+v", req.Tools)
	}
	property := req.Tools[0].Function.Parameters.Properties["localEnvironmentConfigPath"]
	if property == nil || property.Type == nil {
		t.Fatalf("expected localEnvironmentConfigPath.type to be populated, got %+v", property)
	}
	if !property.Type.HasType("string") || !property.Type.HasType("null") {
		t.Errorf("schema type should contain string and null, got %+v", property.Type)
	}
}

// TestOpenAICreateResponseReq_ToolsMcp covers MCP allowed_tools array and filter variants.
func TestOpenAICreateResponseReq_ToolsMcp(t *testing.T) {
	allCases := loadCases(t)

	t.Run("allowed_tools_array", func(t *testing.T) {
		tc := findCase(t, allCases, "tools_mcp_with_allowed_array")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		mcp := req.Tools[0].Mcp
		if mcp == nil || mcp.AllowedTools == nil {
			t.Fatalf("expected mcp tool: %+v", req.Tools)
		}
		if len(mcp.AllowedTools.Names) != 2 || mcp.AllowedTools.Filter != nil {
			t.Errorf("expected array variant, got %+v", mcp.AllowedTools)
		}
	})

	t.Run("allowed_tools_filter", func(t *testing.T) {
		tc := findCase(t, allCases, "tools_mcp_with_allowed_filter")
		var req dto.OpenAICreateResponseReq
		if err := sonic.Unmarshal(tc.RequestBody, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		mcp := req.Tools[0].Mcp
		if mcp == nil || mcp.AllowedTools == nil || mcp.AllowedTools.Filter == nil {
			t.Fatalf("expected filter variant, got %+v", mcp)
		}
		if mcp.AllowedTools.Filter.ReadOnly == nil || *mcp.AllowedTools.Filter.ReadOnly != true {
			t.Errorf("ReadOnly should be true: %+v", mcp.AllowedTools.Filter)
		}
		if mcp.RequireApproval == nil || mcp.RequireApproval.Mode != "never" {
			t.Errorf("RequireApproval mode = never expected, got %+v", mcp.RequireApproval)
		}
	})
}
