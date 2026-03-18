package dto

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
)

// ==================== Anthropic Common DTOs ====================

// CacheControl Anthropic 缓存控制
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type CacheControl struct {
	Type string `json:"type" doc:"缓存类型: ephemeral"`
}

// ==================== Anthropic Context Management DTOs ====================

// AnthropicContextManagementEdit Anthropic 上下文管理编辑项
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicContextManagementEdit struct {
	Type string `json:"type" doc:"编辑类型"`
	Keep string `json:"keep,omitempty" doc:"保留策略"`
}

// AnthropicContextManagement Anthropic 上下文管理配置
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicContextManagement struct {
	Edits []*AnthropicContextManagementEdit `json:"edits,omitempty" doc:"上下文编辑操作列表"`
}

// ==================== Anthropic Tool DTOs ====================

// AnthropicTool Anthropic 工具定义（联合结构体，覆盖所有工具类型）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicTool struct {
	// 通用字段
	Type         string        `json:"type,omitempty" doc:"工具类型: custom/bash_20250124/text_editor_20250124/text_editor_20250429/text_editor_20250728/computer_20250124/code_execution_20250522/code_execution_20250825/web_search_20250305 等"`
	Name         string        `json:"name,omitempty" doc:"工具名称"`
	CacheControl *CacheControl `json:"cache_control,omitempty" doc:"缓存控制"`
	DeferLoading *bool         `json:"defer_loading,omitempty" doc:"延迟加载"`
	Strict       *bool         `json:"strict,omitempty" doc:"严格模式"`

	// 自定义工具字段 (type=custom 或 type 为空)
	Description   string              `json:"description,omitempty" doc:"工具描述"`
	InputSchema   *JSONSchemaProperty `json:"input_schema,omitempty" doc:"输入JSON Schema"`
	InputExamples []map[string]string `json:"input_examples,omitempty" doc:"输入示例"`

	// 计算机使用工具字段 (type=computer_20250124)
	DisplayWidthPx  *int `json:"display_width_px,omitempty" doc:"显示宽度(像素)"`
	DisplayHeightPx *int `json:"display_height_px,omitempty" doc:"显示高度(像素)"`
	DisplayNumber   *int `json:"display_number,omitempty" doc:"显示编号"`

	// 文本编辑器工具字段 (type=text_editor_20250728)
	MaxCharacters *int `json:"max_characters,omitempty" doc:"查看文件时最大字符数"`
}

// ==================== Anthropic Message Param DTOs ====================

// AnthropicMessageContent Anthropic 消息内容（字符串或 ContentBlock 数组的联合类型）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicMessageContent struct {
	Text   string                   `json:"-"`
	Blocks []*AnthropicContentBlock `json:"-"`
}

// UnmarshalJSON 自定义反序列化：区分字符串和数组
func (c *AnthropicMessageContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	return sonic.Unmarshal(data, &c.Blocks)
}

// MarshalJSON 自定义序列化：Blocks 优先，否则输出字符串
func (c AnthropicMessageContent) MarshalJSON() ([]byte, error) {
	if len(c.Blocks) > 0 {
		return sonic.Marshal(c.Blocks)
	}
	return sonic.Marshal(c.Text)
}

// Schema 实现 huma.SchemaProvider 接口，告诉 Huma 此类型接受字符串或 ContentBlock 数组
func (c AnthropicMessageContent) Schema(r huma.Registry) *huma.Schema {
	contentBlockSchema := r.Schema(reflect.TypeFor[AnthropicContentBlock](), true, "AnthropicContentBlock")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "array", Items: contentBlockSchema},
		},
	}
}

// AnthropicToolResultContent tool_result 的嵌套内容（字符串或 ContentBlock 数组的联合类型）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicToolResultContent struct {
	Text   string                   `json:"-"`
	Blocks []*AnthropicContentBlock `json:"-"`
}

// UnmarshalJSON 自定义反序列化：区分字符串和数组
func (c *AnthropicToolResultContent) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.Text = s
		return nil
	}
	return sonic.Unmarshal(data, &c.Blocks)
}

// MarshalJSON 自定义序列化：Blocks 优先，否则输出字符串
func (c AnthropicToolResultContent) MarshalJSON() ([]byte, error) {
	if len(c.Blocks) > 0 {
		return sonic.Marshal(c.Blocks)
	}
	return sonic.Marshal(c.Text)
}

// Schema 实现 huma.SchemaProvider 接口，告诉 Huma 此类型接受字符串或 ContentBlock 数组
func (c AnthropicToolResultContent) Schema(r huma.Registry) *huma.Schema {
	contentBlockSchema := r.Schema(reflect.TypeFor[AnthropicContentBlock](), true, "AnthropicContentBlock")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			{Type: "array", Items: contentBlockSchema},
		},
	}
}

// AnthropicImageSource Anthropic 图片来源（Base64 或 URL）
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicImageSource struct {
	Type      string `json:"type" doc:"来源类型: base64/url"`
	MediaType string `json:"media_type,omitempty" doc:"媒体类型(type=base64时): image/jpeg, image/png, image/gif, image/webp"`
	Data      string `json:"data,omitempty" doc:"Base64编码的图片数据(type=base64时)"`
	URL       string `json:"url,omitempty" doc:"图片URL(type=url时)"`
}

// AnthropicMessageParam Anthropic 消息参数
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicMessageParam struct {
	Role    string                   `json:"role" doc:"消息角色: user 或 assistant"`
	Content *AnthropicMessageContent `json:"content" doc:"消息内容(字符串或ContentBlock数组)"`
}

// AnthropicContentBlock Anthropic 内容块基础结构
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
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
	ID    string         `json:"id,omitempty" doc:"工具调用ID(type=tool_use)"`
	Name  string         `json:"name,omitempty" doc:"工具名称(type=tool_use)"`
	Input map[string]any `json:"input,omitempty" doc:"工具输入(type=tool_use)"`
	// ToolResultBlock 字段
	ToolUseID string                      `json:"tool_use_id,omitempty" doc:"关联的工具调用ID(type=tool_result)"`
	IsError   *bool                       `json:"is_error,omitempty" doc:"是否为错误结果(type=tool_result)"`
	Content   *AnthropicToolResultContent `json:"content,omitempty" doc:"工具结果内容(type=tool_result)"`
	// ImageBlock 字段
	Source *AnthropicImageSource `json:"source,omitempty" doc:"图片来源(type=image)"`
	// 通用字段
	CacheControl *CacheControl `json:"cache_control,omitempty" doc:"缓存控制"`
}

// ==================== Anthropic Output Config DTOs ====================

// AnthropicJSONOutputFormat Anthropic JSON输出格式配置
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicJSONOutputFormat struct {
	Type   string              `json:"type" doc:"格式类型: json_schema"`
	Schema *JSONSchemaProperty `json:"schema,omitempty" doc:"JSON Schema对象"`
}

// AnthropicOutputConfig Anthropic 输出配置
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicOutputConfig struct {
	Effort string                     `json:"effort,omitempty" doc:"努力级别: low/medium/high/max"`
	Format *AnthropicJSONOutputFormat `json:"format,omitempty" doc:"输出格式配置"`
}

// ==================== Anthropic Thinking Config DTOs ====================

// AnthropicThinkingConfig Anthropic 思考配置
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicThinkingConfig struct {
	Type         string `json:"type" doc:"思考类型: enabled/disabled"`
	BudgetTokens *int   `json:"budget_tokens,omitempty" doc:"思考预算token数"`
}

// ==================== Anthropic Tool Choice DTOs ====================

// AnthropicToolChoice Anthropic 工具选择配置
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicToolChoice struct {
	Type                   string `json:"type" doc:"工具选择类型: auto/any/tool/none"`
	Name                   string `json:"name,omitempty" doc:"指定工具名称(type=tool时必填)"`
	DisableParallelToolUse *bool  `json:"disable_parallel_tool_use,omitempty" doc:"禁用并行工具调用"`
}

// ==================== Anthropic Metadata DTOs ====================

// AnthropicMetadata Anthropic 请求元数据
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicMetadata struct {
	UserID string `json:"user_id,omitempty" doc:"用户标识符"`
}

// ==================== Anthropic Create Message Request DTOs ====================

// AnthropicCreateMessageReq Anthropic Create Message 请求体
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type AnthropicCreateMessageReq struct {
	MaxTokens         int                         `json:"max_tokens" doc:"最大生成 token 数"`
	Messages          []*AnthropicMessageParam    `json:"messages" doc:"消息列表"`
	Model             string                      `json:"model" doc:"模型ID"`
	Stream            *bool                       `json:"stream,omitempty" doc:"是否流式"`
	System            *AnthropicMessageContent    `json:"system,omitempty" doc:"系统提示(字符串或TextBlockParam数组)"`
	Temperature       *float64                    `json:"temperature,omitempty" doc:"采样温度(0-1)"`
	TopK              *int                        `json:"top_k,omitempty" doc:"Top-K采样"`
	TopP              *float64                    `json:"top_p,omitempty" doc:"核采样概率"`
	StopSequences     []string                    `json:"stop_sequences,omitempty" doc:"停止序列"`
	Tools             []*AnthropicTool            `json:"tools,omitempty" doc:"工具定义列表"`
	ToolChoice        *AnthropicToolChoice        `json:"tool_choice,omitempty" doc:"工具选择配置"`
	Thinking          *AnthropicThinkingConfig    `json:"thinking,omitempty" doc:"思考配置"`
	Metadata          *AnthropicMetadata          `json:"metadata,omitempty" doc:"元数据"`
	ServiceTier       string                      `json:"service_tier,omitempty" doc:"服务层级"`
	OutputConfig      *AnthropicOutputConfig      `json:"output_config,omitempty" doc:"输出配置(输出格式、努力级别等)"`
	CacheControl      *CacheControl               `json:"cache_control,omitempty" doc:"顶层缓存控制"`
	Container         string                      `json:"container,omitempty" doc:"容器标识符"`
	InferenceGeo      string                      `json:"inference_geo,omitempty" doc:"推理地理区域"`
	ContextManagement *AnthropicContextManagement `json:"context_management,omitempty" doc:"上下文管理配置"`
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
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []*AnthropicContentBlock `json:"content"`
	Model        string                   `json:"model"`
	StopReason   *string                  `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence"`
	Usage        *AnthropicUsage          `json:"usage"`
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
