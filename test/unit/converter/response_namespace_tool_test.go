package converter

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/converter"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/dto/schema"
	"github.com/samber/lo"
)

// buildNamespaceTools 构建包含 namespace 工具的 Response API tools 列表，模拟 Codex 发送的 MCP 工具场景。
func buildNamespaceTools() []*dto.ResponseTool {
	return []*dto.ResponseTool{{
		Type: enum.ResponseToolTypeFunction,
		Function: &dto.ResponseToolFunction{
			Type: enum.ResponseToolTypeFunction,
			Name: "exec_command",
			Parameters: &schema.JSONSchemaProperty{
				JSONSchemaProperty: vo.JSONSchemaProperty{
					Type: lo.ToPtr(vo.JSONSchemaTypeValue{Single: enum.JSONSchemaTypeObject}),
				},
			},
			Strict: true,
		},
	}, {
		Type: enum.ResponseToolTypeNamespace,
		Namespace: &dto.ResponseToolNamespace{
			Type:        enum.ResponseToolTypeNamespace,
			Name:        "mcp__openaiDeveloperDocs",
			Description: "Tools in the mcp__openaiDeveloperDocs namespace.",
			Tools: []*dto.ResponseNamespaceTool{{
				Name:        "search_openai_docs",
				Type:        enum.ResponseToolTypeFunction,
				Description: lo.ToPtr("Search OpenAI docs"),
				Parameters: &schema.JSONSchemaProperty{
					JSONSchemaProperty: vo.JSONSchemaProperty{
						Type: lo.ToPtr(vo.JSONSchemaTypeValue{Single: enum.JSONSchemaTypeObject}),
					},
				},
				Strict: lo.ToPtr(true),
			}, {
				Name:        "fetch_openai_doc",
				Type:        enum.ResponseToolTypeFunction,
				Description: lo.ToPtr("Fetch a specific OpenAI doc"),
			}},
		},
	}, {
		Type: enum.ResponseToolTypeNamespace,
		Namespace: &dto.ResponseToolNamespace{
			Type:        enum.ResponseToolTypeNamespace,
			Name:        "mcp__computer_use",
			Description: "Computer use tools",
			Tools: []*dto.ResponseNamespaceTool{{
				Name:        "click",
				Type:        enum.ResponseToolTypeFunction,
				Description: lo.ToPtr("Click at coordinates"),
			}, {
				Name:        "type_text",
				Type:        enum.ResponseToolTypeCustom,
				Description: lo.ToPtr("Type text into the focused element"),
			}},
		},
	}}
}

// TestFromResponseRequest_NamespaceFlatten 验证 namespace 工具被铺平为独立的 function/custom 工具，
// 子工具名称使用 `{namespace}__{subToolName}` 格式。这是回归测试，修复前 namespace 被当作单个
// 无参数的 stub function 转发，子工具全部丢失。
func TestFromResponseRequest_NamespaceFlatten(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("deepseek-v4-flash"),
		Tools: buildNamespaceTools(),
		Input: &dto.ResponseInput{Text: "hello"},
	}

	chatReq, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	if len(chatReq.Tools) != 5 {
		t.Fatalf("expected 5 chat tools (1 top-level function + 2 mcp__openaiDeveloperDocs + 2 mcp__computer_use), got %d", len(chatReq.Tools))
	}

	// 第一个：顶层 function 工具，名称不变
	if chatReq.Tools[0].Function == nil || chatReq.Tools[0].Function.Name != "exec_command" {
		t.Errorf("chatTools[0] function name = %v, want exec_command", chatReq.Tools[0].Function)
	}

	// 第二个：mcp__openaiDeveloperDocs__search_openai_docs (function sub-tool)
	sep := constant.NamespaceToolSeparator
	expectedName1 := "mcp__openaiDeveloperDocs" + sep + "search_openai_docs"
	if chatReq.Tools[1].Function == nil || chatReq.Tools[1].Function.Name != expectedName1 {
		t.Errorf("chatTools[1] function name = %v, want %q", chatReq.Tools[1].Function, expectedName1)
	}
	if chatReq.Tools[1].Function.Description == nil || *chatReq.Tools[1].Function.Description != "Search OpenAI docs" {
		t.Errorf("chatTools[1] description = %v, want Search OpenAI docs", chatReq.Tools[1].Function.Description)
	}

	// 第三个：mcp__openaiDeveloperDocs__fetch_openai_doc (function sub-tool, no params)
	expectedName2 := "mcp__openaiDeveloperDocs" + sep + "fetch_openai_doc"
	if chatReq.Tools[2].Function == nil || chatReq.Tools[2].Function.Name != expectedName2 {
		t.Errorf("chatTools[2] function name = %v, want %q", chatReq.Tools[2].Function, expectedName2)
	}

	// 第四个：mcp__computer_use__click (function sub-tool)
	expectedName3 := "mcp__computer_use" + sep + "click"
	if chatReq.Tools[3].Function == nil || chatReq.Tools[3].Function.Name != expectedName3 {
		t.Errorf("chatTools[3] function name = %v, want %q", chatReq.Tools[3].Function, expectedName3)
	}

	// 第五个：mcp__computer_use__type_text (custom sub-tool → 统一转换为 function 以兼容上游)
	expectedName4 := "mcp__computer_use" + sep + "type_text"
	if chatReq.Tools[4].Function == nil || chatReq.Tools[4].Function.Name != expectedName4 {
		t.Errorf("chatTools[4] function name = %v, want %q", chatReq.Tools[4].Function, expectedName4)
	}
	if chatReq.Tools[4].Type != enum.ToolTypeFunction {
		t.Errorf("chatTools[4] type = %v, want %q", chatReq.Tools[4].Type, enum.ToolTypeFunction)
	}
	if chatReq.Tools[4].Function.Description == nil || *chatReq.Tools[4].Function.Description != "Type text into the focused element" {
		t.Errorf("chatTools[4] description = %v, want Type text into the focused element", chatReq.Tools[4].Function.Description)
	}
}

// TestBuildToolTypeMap_Namespace 验证 BuildToolTypeMap 将 namespace 子工具的铺平名称映射到子工具类型。
func TestBuildToolTypeMap_Namespace(t *testing.T) {
	t.Parallel()
	tools := buildNamespaceTools()
	m := converter.BuildToolTypeMap(tools)

	cases := []struct {
		flatName string
		wantType string
	}{
		{"exec_command", enum.ResponseToolTypeFunction},
		{"mcp__openaiDeveloperDocs__search_openai_docs", enum.ResponseToolTypeFunction},
		{"mcp__openaiDeveloperDocs__fetch_openai_doc", enum.ResponseToolTypeFunction},
		{"mcp__computer_use__click", enum.ResponseToolTypeFunction},
		{"mcp__computer_use__type_text", enum.ResponseToolTypeCustom},
	}
	for _, tc := range cases {
		got, ok := m[tc.flatName]
		if !ok {
			t.Errorf("toolTypeMap missing key %q", tc.flatName)
			continue
		}
		if got != tc.wantType {
			t.Errorf("toolTypeMap[%q] = %q, want %q", tc.flatName, got, tc.wantType)
		}
	}

	// namespace 名称本身不应出现在映射中（修复前会映射 namespaceName → "namespace"）
	if _, ok := m["mcp__openaiDeveloperDocs"]; ok {
		t.Error("toolTypeMap should not contain bare namespace name as key")
	}
	if _, ok := m["mcp__computer_use"]; ok {
		t.Error("toolTypeMap should not contain bare namespace name as key")
	}
}

// TestBuildNamespaceMap 验证 BuildNamespaceMap 构建铺平名称 → 命名空间名称的映射。
func TestBuildNamespaceMap(t *testing.T) {
	t.Parallel()
	tools := buildNamespaceTools()
	m := converter.BuildNamespaceMap(tools)

	cases := []struct {
		flatName    string
		wantNS      string
		shouldExist bool
	}{
		{"mcp__openaiDeveloperDocs__search_openai_docs", "mcp__openaiDeveloperDocs", true},
		{"mcp__openaiDeveloperDocs__fetch_openai_doc", "mcp__openaiDeveloperDocs", true},
		{"mcp__computer_use__click", "mcp__computer_use", true},
		{"mcp__computer_use__type_text", "mcp__computer_use", true},
		{"exec_command", "", false},
	}
	for _, tc := range cases {
		got, ok := m[tc.flatName]
		if ok != tc.shouldExist {
			t.Errorf("namespaceMap[%q] exists = %v, want %v", tc.flatName, ok, tc.shouldExist)
			continue
		}
		if tc.shouldExist && got != tc.wantNS {
			t.Errorf("namespaceMap[%q] = %q, want %q", tc.flatName, got, tc.wantNS)
		}
	}
}

// TestFromResponseRequest_NamespaceMapsSet 验证 FromResponseRequest 正确设置 toolTypeMap 和 namespaceMap。
func TestFromResponseRequest_NamespaceMapsSet(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("deepseek-v4-flash"),
		Tools: buildNamespaceTools(),
		Input: &dto.ResponseInput{Text: "hello"},
	}

	_, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	nsMap := conv.NamespaceMap()
	if len(nsMap) != 4 {
		t.Errorf("expected 4 entries in namespaceMap, got %d", len(nsMap))
	}

	ttMap := conv.ToolTypeMap()
	if len(ttMap) != 5 {
		t.Errorf("expected 5 entries in toolTypeMap, got %d", len(ttMap))
	}
}

// TestToResponseResponse_NamespaceToolCallSplit 验证响应方向：
// 上游 ChatCompletion 返回铺平名称的 function call 时，转换为 Response API 输出应拆分回
// name=subToolName, namespace=namespaceName。修复前 name=flattenedName, namespace=nil。
func TestToResponseResponse_NamespaceToolCallSplit(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"mcp__openaiDeveloperDocs__search_openai_docs": enum.ResponseToolTypeFunction,
		"mcp__computer_use__type_text":                 enum.ResponseToolTypeCustom,
	})
	conv.SetNamespaceMap(map[string]string{
		"mcp__openaiDeveloperDocs__search_openai_docs": "mcp__openaiDeveloperDocs",
		"mcp__computer_use__type_text":                 "mcp__computer_use",
	})

	callID := "call_abc123"
	completion := &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-1",
		Model: "deepseek-v4-flash",
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index: 0,
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					ID:   lo.ToPtr(callID),
					Type: enum.ToolTypeFunction,
					Function: &dto.OpenAIChatCompletionMessageFunctionToolCall{
						Name:      "mcp__openaiDeveloperDocs__search_openai_docs",
						Arguments: `{"query":"gpt-4"}`,
					},
				}},
			},
			FinishReason: enum.FinishReasonToolCalls,
		}},
	}

	rsp, err := conv.ToResponseResponse(completion)
	if err != nil {
		t.Fatalf("ToResponseResponse() error: %v", err)
	}
	if rsp == nil {
		t.Fatal("ToResponseResponse() returned nil")
	}

	var fcItem *dto.ResponseInputItem
	for _, item := range rsp.Output {
		if item != nil && item.Type != nil && *item.Type == enum.ResponseInputItemTypeFunctionCall {
			fcItem = item
			break
		}
	}
	if fcItem == nil {
		t.Fatalf("expected a function_call item, got %+v", rsp.Output)
	}

	if fcItem.Name == nil || *fcItem.Name != "search_openai_docs" {
		t.Errorf("Name = %v, want search_openai_docs", fcItem.Name)
	}
	if fcItem.Namespace == nil || *fcItem.Namespace != "mcp__openaiDeveloperDocs" {
		t.Errorf("Namespace = %v, want mcp__openaiDeveloperDocs", fcItem.Namespace)
	}
}

// TestToResponseResponse_NamespaceCustomToolCallSplit 验证 custom sub-tool 的响应方向拆分。
func TestToResponseResponse_NamespaceCustomToolCallSplit(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	conv.SetToolTypeMap(map[string]string{
		"mcp__computer_use__type_text": enum.ResponseToolTypeCustom,
	})
	conv.SetNamespaceMap(map[string]string{
		"mcp__computer_use__type_text": "mcp__computer_use",
	})

	callID := "call_xyz789"
	completion := &dto.OpenAIChatCompletion{
		ID:    "chatcmpl-2",
		Model: "deepseek-v4-flash",
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index: 0,
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role: enum.RoleAssistant,
				ToolCalls: []*dto.OpenAIChatCompletionMessageToolCall{{
					ID:   lo.ToPtr(callID),
					Type: enum.ToolTypeCustom,
					Custom: &dto.OpenAIChatCompletionMessageCustomToolCall{
						Name:  "mcp__computer_use__type_text",
						Input: "hello world",
					},
				}},
			},
			FinishReason: enum.FinishReasonToolCalls,
		}},
	}

	rsp, err := conv.ToResponseResponse(completion)
	if err != nil {
		t.Fatalf("ToResponseResponse() error: %v", err)
	}

	var ctcItem *dto.ResponseInputItem
	for _, item := range rsp.Output {
		if item != nil && item.Type != nil && *item.Type == enum.ResponseInputItemTypeCustomToolCall {
			ctcItem = item
			break
		}
	}
	if ctcItem == nil {
		t.Fatalf("expected a custom_tool_call item, got %+v", rsp.Output)
	}

	if ctcItem.Name == nil || *ctcItem.Name != "type_text" {
		t.Errorf("Name = %v, want type_text", ctcItem.Name)
	}
	if ctcItem.Namespace == nil || *ctcItem.Namespace != "mcp__computer_use" {
		t.Errorf("Namespace = %v, want mcp__computer_use", ctcItem.Namespace)
	}
}

// TestFromResponseRequest_NamespacedFunctionCallInput 验证请求方向：
// 当 input 包含带 namespace 的 function_call item 时，转换为 ChatCompletion 时
// 函数名应被铺平为 `{namespace}__{name}`。
func TestFromResponseRequest_NamespacedFunctionCallInput(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	fcType := enum.ResponseInputItemTypeFunctionCall
	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("deepseek-v4-flash"),
		Tools: buildNamespaceTools(),
		Input: &dto.ResponseInput{
			Items: []*dto.ResponseInputItem{{
				Type:      &fcType,
				CallID:    lo.ToPtr("call_prev1"),
				Name:      lo.ToPtr("search_openai_docs"),
				Namespace: lo.ToPtr("mcp__openaiDeveloperDocs"),
				Arguments: lo.ToPtr(`{"query":"gpt-4"}`),
			}, {
				Type:   lo.ToPtr(enum.ResponseInputItemTypeFunctionCallOutput),
				CallID: lo.ToPtr("call_prev1"),
				Output: &dto.ResponseInputItemOutput{
					Text: `{"results":[]}`,
				},
			}, {
				Role: lo.ToPtr(enum.RoleUser),
				Content: &dto.ResponseInputMessageContent{
					Text: "now search for gpt-5",
				},
			}},
		},
	}

	chatReq, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}

	var toolCallName string
	for _, msg := range chatReq.Messages {
		if msg.Role == enum.RoleAssistant && len(msg.ToolCalls) > 0 {
			toolCallName = msg.ToolCalls[0].Function.Name
			break
		}
	}
	expected := "mcp__openaiDeveloperDocs" + constant.NamespaceToolSeparator + "search_openai_docs"
	if toolCallName != expected {
		t.Errorf("tool call function name = %q, want %q", toolCallName, expected)
	}
}

// TestFromResponseRequest_NamespaceNoSubtools 验证空 namespace（无子工具）不产生任何 chat 工具。
func TestFromResponseRequest_NamespaceNoSubtools(t *testing.T) {
	t.Parallel()
	conv := &converter.ResponseProtocolConverter{}
	req := &dto.OpenAICreateResponseReq{
		Model: lo.ToPtr("test-model"),
		Tools: []*dto.ResponseTool{{
			Type: enum.ResponseToolTypeNamespace,
			Namespace: &dto.ResponseToolNamespace{
				Type:  enum.ResponseToolTypeNamespace,
				Name:  "empty_ns",
				Tools: []*dto.ResponseNamespaceTool{},
			},
		}},
		Input: &dto.ResponseInput{Text: "hi"},
	}

	chatReq, err := conv.FromResponseRequest(req)
	if err != nil {
		t.Fatalf("FromResponseRequest() error: %v", err)
	}
	if len(chatReq.Tools) != 0 {
		t.Errorf("expected 0 chat tools for empty namespace, got %d", len(chatReq.Tools))
	}
}
