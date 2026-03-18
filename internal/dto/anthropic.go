package dto

import "encoding/json"

// ==================== Anthropic Tool DTOs ====================

// AnthropicToolInputSchema Anthropic 工具输入 Schema
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicToolInputSchema struct {
	Type       string                         `json:"type" doc:"Schema类型，通常为object"`
	Properties map[string]*JSONSchemaProperty `json:"properties,omitempty" doc:"属性定义"`
	Required   []string                       `json:"required,omitempty" doc:"必填字段"`
}

// AnthropicTool Anthropic 工具定义（联合结构体，覆盖所有工具类型）
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicTool struct {
	// 通用字段
	Type         string `json:"type,omitempty" doc:"工具类型: custom/bash_20250124/text_editor_20250124/text_editor_20250429/text_editor_20250728/computer_20250124/code_execution_20250522/code_execution_20250825/web_search_20250305 等"`
	Name         string `json:"name,omitempty" doc:"工具名称"`
	CacheControl any    `json:"cache_control,omitempty" doc:"缓存控制"`
	DeferLoading *bool  `json:"defer_loading,omitempty" doc:"延迟加载"`
	Strict       *bool  `json:"strict,omitempty" doc:"严格模式"`

	// 自定义工具字段 (type=custom 或 type 为空)
	Description   string                    `json:"description,omitempty" doc:"工具描述"`
	InputSchema   *AnthropicToolInputSchema `json:"input_schema,omitempty" doc:"输入JSON Schema"`
	InputExamples []map[string]any          `json:"input_examples,omitempty" doc:"输入示例"`

	// 计算机使用工具字段 (type=computer_20250124)
	DisplayWidthPx  *int `json:"display_width_px,omitempty" doc:"显示宽度(像素)"`
	DisplayHeightPx *int `json:"display_height_px,omitempty" doc:"显示高度(像素)"`
	DisplayNumber   *int `json:"display_number,omitempty" doc:"显示编号"`

	// 文本编辑器工具字段 (type=text_editor_20250728)
	MaxCharacters *int `json:"max_characters,omitempty" doc:"查看文件时最大字符数"`
}

// ==================== Anthropic Message Param DTOs ====================

// AnthropicMessageParam Anthropic 消息参数
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicMessageParam struct {
	Role    string `json:"role" doc:"消息角色: user 或 assistant"`
	Content any    `json:"content" doc:"消息内容(字符串或ContentBlock数组)"`
}

// AnthropicContentBlock Anthropic 内容块基础结构
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicContentBlock struct {
	Type string `json:"type" doc:"内容块类型"`
	// TextBlock 字段
	Text string `json:"text,omitempty" doc:"文本内容(type=text)"`
	// ThinkingBlock 字段
	Thinking  string `json:"thinking,omitempty" doc:"思考内容(type=thinking)"`
	Signature string `json:"signature,omitempty" doc:"思考签名(type=thinking)"`
	// RedactedThinkingBlock 字段
	Data string `json:"data,omitempty" doc:"编辑后的思考数据(type=redacted_thinking)"`
	// ToolUseBlock 字段
	ID    string `json:"id,omitempty" doc:"工具调用ID(type=tool_use)"`
	Name  string `json:"name,omitempty" doc:"工具名称(type=tool_use)"`
	Input any    `json:"input,omitempty" doc:"工具输入(type=tool_use)"`
	// ToolResultBlock 字段
	ToolUseID string `json:"tool_use_id,omitempty" doc:"关联的工具调用ID(type=tool_result)"`
	IsError   *bool  `json:"is_error,omitempty" doc:"是否为错误结果(type=tool_result)"`
	// ImageBlock 字段
	Source any `json:"source,omitempty" doc:"图片来源(type=image)"`
	// 通用字段
	CacheControl any `json:"cache_control,omitempty" doc:"缓存控制"`
}

// ==================== Anthropic Create Message Request DTOs ====================

// AnthropicCreateMessageReq Anthropic Create Message 请求体
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicCreateMessageReq struct {
	MaxTokens     int                      `json:"max_tokens" doc:"最大生成 token 数"`
	Messages      []*AnthropicMessageParam `json:"messages" doc:"消息列表"`
	Model         string                   `json:"model" doc:"模型ID"`
	Stream        *bool                    `json:"stream,omitempty" doc:"是否流式"`
	System        any                      `json:"system,omitempty" doc:"系统提示(字符串或TextBlockParam数组)"`
	Temperature   *float64                 `json:"temperature,omitempty" doc:"采样温度(0-1)"`
	TopK          *int                     `json:"top_k,omitempty" doc:"Top-K采样"`
	TopP          *float64                 `json:"top_p,omitempty" doc:"核采样概率"`
	StopSequences []string                 `json:"stop_sequences,omitempty" doc:"停止序列"`
	Tools         []*AnthropicTool         `json:"tools,omitempty" doc:"工具定义列表"`
	ToolChoice    any                      `json:"tool_choice,omitempty" doc:"工具选择(auto/any/tool/none)"`
	Thinking      any                      `json:"thinking,omitempty" doc:"思考配置"`
	Metadata      any                      `json:"metadata,omitempty" doc:"元数据"`
	ServiceTier   string                   `json:"service_tier,omitempty" doc:"服务层级"`
}

// AnthropicCreateMessageRequest Anthropic Create Message 请求包装（Huma格式）
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicCreateMessageRequest struct {
	Body *AnthropicCreateMessageReq `json:"body" doc:"请求体"`
}

// ==================== Anthropic Create Message Response DTOs ====================

// AnthropicMessage Anthropic Message 响应
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicMessage struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Role         string            `json:"role"`
	Content      []json.RawMessage `json:"content"`
	Model        string            `json:"model"`
	StopReason   *string           `json:"stop_reason"`
	StopSequence *string           `json:"stop_sequence"`
	Usage        *AnthropicUsage   `json:"usage"`
}

// AnthropicUsage Anthropic Token 用量统计
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ==================== Anthropic List Models DTOs ====================

// AnthropicModelInfo Anthropic 模型信息
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicModelInfo struct {
	ID          string `json:"id" doc:"模型ID"`
	CreatedAt   string `json:"created_at" doc:"创建时间(RFC3339)"`
	DisplayName string `json:"display_name" doc:"模型显示名称"`
	Type        string `json:"type" doc:"对象类型: model"`
}

// AnthropicListModelsRsp Anthropic 模型列表响应
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicListModelsRsp struct {
	Data    []*AnthropicModelInfo `json:"data" doc:"模型列表"`
	HasMore bool                  `json:"has_more" doc:"是否有更多"`
	FirstID string                `json:"first_id,omitempty" doc:"第一个模型ID"`
	LastID  string                `json:"last_id,omitempty" doc:"最后一个模型ID"`
}

// ==================== Anthropic Error DTOs ====================

// AnthropicError Anthropic 错误信息
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicError struct {
	Type    string `json:"type" doc:"错误类型"`
	Message string `json:"message" doc:"错误消息"`
}

// AnthropicErrorResponse Anthropic 错误响应包装
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicErrorResponse struct {
	Type  string          `json:"type" doc:"对象类型: error"`
	Error *AnthropicError `json:"error" doc:"错误信息"`
}
