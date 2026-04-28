package dto

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

// ==================== Conversion: OpenAI Response API -> Unified ====================
//
// Response API uses a radically different input/output shape than
// /chat/completions: a flat array of typed `items` that can be messages,
// reasoning segments, function calls, shell/computer calls, etc.
//
// For storage we only persist the items that map cleanly onto UnifiedMessage
// (the existing cross-provider format used by /chat/completions):
//
//   - message          -> role/content (+ refusal)
//   - function_call    -> assistant tool_calls
//   - function_call_output / custom_tool_call_output
//                      -> tool-role message with tool_call_id
//   - reasoning        -> reasoning_content on assistant (joined text)
//
// Less-common items (computer_call, web_search_call, shell_call, mcp_*,
// image_generation_call, code_interpreter_call, apply_patch_call, ...)
// are not storable via the unified schema today; skip them silently so we
// never break a store path because of a new item type upstream.
//
//	@author centonhuang
//	@update 2026-04-18 15:00:00

// FromResponseAPIInputItems 将 Response API 请求 input 数组转换为 UnifiedMessage 列表
//
// Top-level `instructions` should be prepended by the caller as a system
// message — this function only handles the `input` array.
//
//	@param items []*ResponseInputItem
//	@return []*UnifiedMessage
//	@return error
func FromResponseAPIInputItems(items []*ResponseInputItem) ([]*vo.UnifiedMessage, error) {
	var msgs []*vo.UnifiedMessage
	for i, item := range items {
		if item == nil {
			continue
		}
		um, err := fromResponseAPIItem(item)
		if err != nil {
			return nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert response input item[%d]", i)
		}
		if um != nil {
			msgs = append(msgs, um)
		}
	}
	return msgs, nil
}

// FromResponseAPIOutputItems 将 Response API 响应 output 数组转换为 UnifiedMessage 列表
//
//	@param items []*ResponseInputItem
//	@return []*UnifiedMessage
//	@return error
func FromResponseAPIOutputItems(items []*ResponseInputItem) ([]*vo.UnifiedMessage, error) {
	var msgs []*vo.UnifiedMessage
	var pendingReasoning strings.Builder
	for i, item := range items {
		if item == nil {
			continue
		}
		switch item.Type {
		case enum.ResponseInputItemTypeReasoning:
			// Reasoning 项单独处理：不生成独立 UnifiedMessage，而是挂到下一个 assistant message 上
			if text := collectReasoningText(item); text != "" {
				if pendingReasoning.Len() > 0 {
					pendingReasoning.WriteString("\n")
				}
				pendingReasoning.WriteString(text)
			}
			continue
		}

		um, err := fromResponseAPIItem(item)
		if err != nil {
			return nil, ierr.Wrapf(ierr.ErrDTOConvert, err, "convert response output item[%d]", i)
		}
		if um == nil {
			continue
		}
		if um.Role == enum.RoleAssistant && pendingReasoning.Len() > 0 {
			um.ReasoningContent = pendingReasoning.String()
			pendingReasoning.Reset()
		}
		msgs = append(msgs, um)
	}

	// 推理内容未挂接到任何 assistant（例如响应只有 reasoning 项）：单独落一条
	if pendingReasoning.Len() > 0 {
		msgs = append(msgs, &vo.UnifiedMessage{
			Role:             enum.RoleAssistant,
			ReasoningContent: pendingReasoning.String(),
		})
	}
	return msgs, nil
}

// fromResponseAPIItem 转换单个 Response API item，无法映射时返回 (nil, nil)
func fromResponseAPIItem(item *ResponseInputItem) (*vo.UnifiedMessage, error) {
	switch item.Type {
	case "", enum.ResponseInputItemTypeMessage:
		return fromResponseAPIMessage(item)
	case enum.ResponseInputItemTypeFunctionCall, enum.ResponseInputItemTypeCustomToolCall:
		return fromResponseAPIFunctionCall(item), nil
	case enum.ResponseInputItemTypeFunctionCallOutput, enum.ResponseInputItemTypeCustomToolCallOutput:
		return fromResponseAPIFunctionCallOutput(item), nil
	case enum.ResponseInputItemTypeReasoning:
		return fromResponseAPIReasoning(item), nil
	default:
		// 未支持的 item 类型（mcp_call、shell_call 等）在统一消息格式中无对应表达，
		// 直接跳过；调用方继续处理其他 item。
		return nil, nil
	}
}

// fromResponseAPIMessage 转换 message 类型 item（EasyInputMessage / Message / OutputMessage 共用）
func fromResponseAPIMessage(item *ResponseInputItem) (*vo.UnifiedMessage, error) {
	role := resolveRole(item.Role)
	um := &vo.UnifiedMessage{Role: role}
	if item.Content == nil {
		return um, nil
	}

	// content 是字符串形态
	if len(item.Content.Parts) == 0 {
		um.Content = &vo.UnifiedContent{Text: item.Content.Text}
		return um, nil
	}

	parts := make([]*vo.UnifiedContentPart, 0, len(item.Content.Parts))
	var refusal string
	for i, p := range item.Content.Parts {
		if p == nil {
			continue
		}
		switch p.Type {
		case enum.ResponseContentTypeInputText, enum.ResponseContentTypeOutputText:
			parts = append(parts, &vo.UnifiedContentPart{
				Type: enum.ContentPartTypeText,
				Text: lo.FromPtr(p.Text),
			})
		case enum.ResponseContentTypeRefusal:
			// refusal 映射到 UnifiedMessage.Refusal（与 /chat/completions 行为保持一致）
			text := lo.FromPtr(p.Refusal)
			if refusal == "" {
				refusal = text
			} else {
				refusal = refusal + "\n" + text
			}
		case enum.ResponseContentTypeInputImage:
			parts = append(parts, &vo.UnifiedContentPart{
				Type:        enum.ContentPartTypeImageURL,
				ImageURL:    lo.FromPtr(p.ImageURL),
				ImageDetail: lo.FromPtr(p.Detail),
			})
		case enum.ResponseContentTypeInputFile:
			parts = append(parts, &vo.UnifiedContentPart{
				Type:     enum.ContentPartTypeFile,
				FileData: lo.FromPtr(p.FileData),
				FileID:   lo.FromPtr(p.FileID),
				Filename: lo.FromPtr(p.Filename),
			})
		case enum.ResponseContentTypeSummaryText, enum.ResponseContentTypeReasoningText:
			// Reasoning 相关块挂到 ReasoningContent，避免污染 Content
			text := lo.FromPtr(p.Text)
			if um.ReasoningContent == "" {
				um.ReasoningContent = text
			} else {
				um.ReasoningContent = um.ReasoningContent + "\n" + text
			}
		default:
			return nil, ierr.Newf(ierr.ErrDTOConvert, "unsupported response content type: %q at part[%d]", p.Type, i)
		}
	}
	if len(parts) > 0 {
		um.Content = &vo.UnifiedContent{Parts: parts}
	}

	return um, nil
}

func fromResponseAPIFunctionCall(item *ResponseInputItem) *vo.UnifiedMessage {
	args := item.Arguments
	if args == "" {
		args = item.Input
	}
	return &vo.UnifiedMessage{
		Role: enum.RoleAssistant,
		ToolCalls: []*vo.UnifiedToolCall{{
			ID:        item.CallID,
			Name:      item.Name,
			Arguments: args,
		}},
	}
}

// fromResponseAPIFunctionCallOutput 将 function_call_output / custom_tool_call_output 转为 tool 角色消息
func fromResponseAPIFunctionCallOutput(item *ResponseInputItem) *vo.UnifiedMessage {
	um := &vo.UnifiedMessage{
		Role:       enum.RoleTool,
		ToolCallID: item.CallID,
	}
	if item.Output == nil {
		return um
	}
	out := item.Output
	switch {
	case out.FunctionOutput != nil:
		if len(out.FunctionOutput.Parts) > 0 {
			parts := make([]*vo.UnifiedContentPart, 0, len(out.FunctionOutput.Parts))
			for _, p := range out.FunctionOutput.Parts {
				if p == nil {
					continue
				}
				if p.Type == enum.ResponseContentTypeInputText || p.Type == enum.ResponseContentTypeOutputText {
					parts = append(parts, &vo.UnifiedContentPart{Type: enum.ContentPartTypeText, Text: lo.FromPtr(p.Text)})
				}
			}
			if len(parts) > 0 {
				um.Content = &vo.UnifiedContent{Parts: parts}
				return um
			}
		}
		um.Content = &vo.UnifiedContent{Text: out.FunctionOutput.Text}
	case out.Text != "":
		um.Content = &vo.UnifiedContent{Text: out.Text}
	}
	return um
}

// fromResponseAPIReasoning 将 reasoning item 转为独立 assistant 消息
func fromResponseAPIReasoning(item *ResponseInputItem) *vo.UnifiedMessage {
	return &vo.UnifiedMessage{
		Role:             enum.RoleAssistant,
		ReasoningContent: collectReasoningText(item),
	}
}

// collectReasoningText 合并 reasoning item 的 summary / content 文本
func collectReasoningText(item *ResponseInputItem) string {
	var parts []string
	for _, s := range item.Summary {
		if s != nil && s.Text != "" {
			parts = append(parts, s.Text)
		}
	}
	for _, c := range item.ReasoningContent {
		if c != nil && c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// resolveRole 将 Response API 角色字符串解析为 enum.Role，未知值按 user 处理
func resolveRole(role string) enum.Role {
	switch role {
	case string(enum.RoleAssistant):
		return enum.RoleAssistant
	case string(enum.RoleSystem):
		return enum.RoleSystem
	case string(enum.RoleDeveloper):
		return enum.RoleDeveloper
	case string(enum.RoleTool):
		return enum.RoleTool
	case string(enum.RoleUser), "":
		return enum.RoleUser
	default:
		return enum.Role(role)
	}
}

// ==================== Tools ====================

// FromResponseAPITool 将 Response API tools 元素转换为 UnifiedTool
//
// 只有可通过函数签名表达的工具 (function / custom) 能映射到统一工具格式，
// 其他类型（file_search/web_search/mcp/...）返回 nil，调用方应跳过。
//
//	@param tool *ResponseTool
//	@return *UnifiedTool
func FromResponseAPITool(tool *ResponseTool) *vo.UnifiedTool {
	if tool == nil {
		return nil
	}
	switch {
	case tool.Function != nil:
		return &vo.UnifiedTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  &tool.Function.Parameters.JSONSchemaProperty,
		}
	case tool.Custom != nil:
		return &vo.UnifiedTool{
			Name:        tool.Custom.Name,
			Description: tool.Custom.Description,
		}
	default:
		return nil
	}
}
