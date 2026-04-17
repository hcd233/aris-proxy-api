package dto

import (
	"reflect"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
)

// ==================== Response API Request DTOs ====================
//
// 参考 docs/openai/create_response.md（Body Parameters 第 13-4316 行）
//
// 所有字段按文档精确建模，联合类型通过专用结构体 + 自定义 Marshal/Unmarshal + Schema 实现。
//
//	@author centonhuang
//	@update 2026-04-17 17:00:00

// ==================== reasoning 字段 ====================

// Response API reasoning effort 常量
// Response API reasoning summary/generate_summary 常量
// ResponseReasoningConfig reasoning 配置对象
type ResponseReasoningConfig struct {
	Effort          string `json:"effort,omitempty" doc:"推理强度: none/minimal/low/medium/high/xhigh"`
	GenerateSummary string `json:"generate_summary,omitempty" doc:"[Deprecated] 摘要策略: auto/concise/detailed"`
	Summary         string `json:"summary,omitempty" doc:"摘要策略: auto/concise/detailed"`
}

// ==================== text 字段 ====================

// Response API text format type 常量
// Response API text verbosity 常量
// ResponseTextConfig text 配置对象
type ResponseTextConfig struct {
	Format    *ResponseTextFormat `json:"format,omitempty" doc:"输出格式"`
	Verbosity string              `json:"verbosity,omitempty" doc:"详细程度: low/medium/high"`
}

// ResponseTextFormat text.format 联合（ResponseFormatText / ResponseFormatTextJSONSchemaConfig / ResponseFormatJSONObject）
type ResponseTextFormat struct {
	Type string `json:"type" doc:"text/json_schema/json_object"`

	// json_schema 专用
	Name        string              `json:"name,omitempty" doc:"格式名称(json_schema)"`
	Schema      *JSONSchemaProperty `json:"schema,omitempty" doc:"JSON Schema 描述"`
	Description string              `json:"description,omitempty" doc:"格式描述"`
	Strict      *bool               `json:"strict,omitempty" doc:"严格模式"`
}

// ==================== prompt 字段 ====================

// ResponsePrompt prompt 模板引用
type ResponsePrompt struct {
	ID        string                             `json:"id" doc:"prompt 模板 ID"`
	Variables map[string]*ResponsePromptVariable `json:"variables,omitempty" doc:"模板变量值"`
	Version   string                             `json:"version,omitempty" doc:"模板版本"`
}

// ResponsePromptVariable prompt.variables 值（string | ResponseInputText | ResponseInputImage | ResponseInputFile）
// 通过 ResponseInputContent 承载复杂值
type ResponsePromptVariable struct {
	StringValue *string               `json:"-"`
	Content     *ResponseInputContent `json:"-"`
}

// UnmarshalJSON 字符串或对象
func (p *ResponsePromptVariable) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		p.StringValue = &s
		return nil
	}
	p.Content = &ResponseInputContent{}
	return sonic.Unmarshal(data, p.Content)
}

// MarshalJSON 按分支输出
func (p ResponsePromptVariable) MarshalJSON() ([]byte, error) {
	if p.Content != nil {
		return sonic.Marshal(p.Content)
	}
	if p.StringValue != nil {
		return sonic.Marshal(*p.StringValue)
	}
	return []byte("null"), nil
}

// Schema 字符串或内容块对象
func (ResponsePromptVariable) Schema(reg huma.Registry) *huma.Schema {
	contentSchema := reg.Schema(reflect.TypeFor[ResponseInputContent](), true, "ResponseInputContent")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			contentSchema,
		},
	}
}

// ==================== stream_options 字段 ====================

// ResponseStreamOptions stream_options 配置
type ResponseStreamOptions struct {
	IncludeObfuscation *bool `json:"include_obfuscation,omitempty" doc:"是否启用流式混淆"`
}

// ==================== conversation 字段 ====================

// ResponseConversationParam conversation 对象
type ResponseConversationParam struct {
	ID    string                     `json:"-"`
	Param *ResponseConversationValue `json:"-"`
}

// ResponseConversationValue conversation 对象形态 { id }
type ResponseConversationValue struct {
	ID string `json:"id" doc:"会话 ID"`
}

// UnmarshalJSON 字符串或对象
func (c *ResponseConversationParam) UnmarshalJSON(data []byte) error {
	var s string
	if err := sonic.Unmarshal(data, &s); err == nil {
		c.ID = s
		return nil
	}
	c.Param = &ResponseConversationValue{}
	return sonic.Unmarshal(data, c.Param)
}

// MarshalJSON 按分支输出
func (c ResponseConversationParam) MarshalJSON() ([]byte, error) {
	if c.Param != nil {
		return sonic.Marshal(c.Param)
	}
	return sonic.Marshal(c.ID)
}

// Schema 字符串或 { id }
func (ResponseConversationParam) Schema(reg huma.Registry) *huma.Schema {
	valueSchema := reg.Schema(reflect.TypeFor[ResponseConversationValue](), true, "ResponseConversationValue")
	return &huma.Schema{
		OneOf: []*huma.Schema{
			{Type: "string"},
			valueSchema,
		},
	}
}

// ==================== context_management 字段 ====================

// Response API context_management entry type 常量
// ResponseContextManagementEntry 上下文管理配置项
type ResponseContextManagementEntry struct {
	Type             string   `json:"type" doc:"固定 compaction"`
	CompactThreshold *float64 `json:"compact_threshold,omitempty" doc:"触发压缩的 token 阈值"`
}

// ==================== include 字段 ====================
//
// include 列表中的单项字面量常量复用 enum.ResponseInclude*。

// ==================== 顶层请求 ====================

// OpenAICreateResponseReq Response API 请求体（按文档精确建模）
type OpenAICreateResponseReq struct {
	// ---------- 布尔/标量 ----------
	Background           *bool             `json:"background,omitempty" doc:"是否后台运行"`
	Instructions         *string           `json:"instructions,omitempty" doc:"系统指令"`
	MaxOutputTokens      *int64            `json:"max_output_tokens,omitempty" doc:"最大输出 token 数"`
	MaxToolCalls         *int64            `json:"max_tool_calls,omitempty" doc:"最大工具调用数"`
	Metadata             map[string]string `json:"metadata,omitempty" doc:"元数据"`
	Model                string            `json:"model,omitempty" doc:"模型 ID"`
	ParallelToolCalls    *bool             `json:"parallel_tool_calls,omitempty" doc:"是否并行工具调用"`
	PreviousResponseID   *string           `json:"previous_response_id,omitempty" doc:"前置响应 ID"`
	PromptCacheKey       *string           `json:"prompt_cache_key,omitempty" doc:"提示缓存键"`
	PromptCacheRetention *string           `json:"prompt_cache_retention,omitempty" doc:"提示缓存保留策略: in-memory/24h"`
	SafetyIdentifier     *string           `json:"safety_identifier,omitempty" doc:"安全标识符"`
	ServiceTier          *string           `json:"service_tier,omitempty" doc:"服务层级: auto/default/flex/scale/priority"`
	Store                *bool             `json:"store,omitempty" doc:"是否存储响应"`
	Stream               *bool             `json:"stream,omitempty" doc:"是否流式响应"`
	Temperature          *float64          `json:"temperature,omitempty" doc:"采样温度"`
	TopLogprobs          *int              `json:"top_logprobs,omitempty" doc:"返回 top logprobs 数量"`
	TopP                 *float64          `json:"top_p,omitempty" doc:"核采样概率质量"`
	Truncation           *string           `json:"truncation,omitempty" doc:"截断策略: auto/disabled"`
	User                 *string           `json:"user,omitempty" doc:"用户标识符（已被 safety_identifier 替代）"`

	// ---------- include 列表 ----------
	Include []string `json:"include,omitempty" doc:"需要包含的附加输出字段"`

	// ---------- 复杂对象 ----------
	Input             *ResponseInput                    `json:"input,omitempty" doc:"输入内容(字符串或消息数组)"`
	ContextManagement []*ResponseContextManagementEntry `json:"context_management,omitempty" doc:"上下文管理配置"`
	Conversation      *ResponseConversationParam        `json:"conversation,omitempty" doc:"关联会话(字符串 ID 或对象)"`
	Prompt            *ResponsePrompt                   `json:"prompt,omitempty" doc:"提示模板"`
	Reasoning         *ResponseReasoningConfig          `json:"reasoning,omitempty" doc:"推理配置"`
	StreamOptions     *ResponseStreamOptions            `json:"stream_options,omitempty" doc:"流式选项"`
	Text              *ResponseTextConfig               `json:"text,omitempty" doc:"文本格式配置"`
	ToolChoice        *ResponseToolChoiceParam          `json:"tool_choice,omitempty" doc:"工具选择策略"`
	Tools             []*ResponseTool                   `json:"tools,omitempty" doc:"工具列表"`
}

// OpenAICreateResponseRequest Response API 请求包装（Huma body 绑定）
type OpenAICreateResponseRequest struct {
	Body *OpenAICreateResponseReq `json:"body" doc:"请求体"`
}
