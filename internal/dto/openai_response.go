package dto

import "github.com/bytedance/sonic"

// ==================== Response API Request DTOs ====================

// OpenAICreateResponseReq Response API 请求体
//
// 作为代理网关，仅解析路由转发必需的字段（model、stream），其余字段原样透传到上游。
//
//	@author centonhuang
//	@update 2026-04-17 10:00:00
type OpenAICreateResponseReq struct {
	Model  string                 `json:"model" doc:"模型ID"`
	Stream *bool                  `json:"stream,omitempty" doc:"是否流式响应"`
	Input  sonic.NoCopyRawMessage `json:"input,omitempty" doc:"输入内容"`

	// 以下字段仅做 JSON 透传，不在 Go 侧使用
	Instructions         sonic.NoCopyRawMessage `json:"instructions,omitempty" doc:"系统指令"`
	Background           *bool                  `json:"background,omitempty" doc:"是否后台运行"`
	ContextManagement    sonic.NoCopyRawMessage `json:"context_management,omitempty" doc:"上下文管理"`
	Conversation         sonic.NoCopyRawMessage `json:"conversation,omitempty" doc:"会话"`
	Include              sonic.NoCopyRawMessage `json:"include,omitempty" doc:"额外输出数据"`
	MaxOutputTokens      sonic.NoCopyRawMessage `json:"max_output_tokens,omitempty" doc:"最大输出token数"`
	Metadata             map[string]string      `json:"metadata,omitempty" doc:"元数据"`
	ParallelToolCalls    *bool                  `json:"parallel_tool_calls,omitempty" doc:"是否并行工具调用"`
	PreviousResponseID   string                 `json:"previous_response_id,omitempty" doc:"前置响应ID"`
	Reasoning            sonic.NoCopyRawMessage `json:"reasoning,omitempty" doc:"推理配置"`
	ServiceTier          string                 `json:"service_tier,omitempty" doc:"服务层级"`
	Store                *bool                  `json:"store,omitempty" doc:"是否存储"`
	Temperature          *float64               `json:"temperature,omitempty" doc:"采样温度"`
	Text                 sonic.NoCopyRawMessage `json:"text,omitempty" doc:"文本格式配置"`
	ToolChoice           sonic.NoCopyRawMessage `json:"tool_choice,omitempty" doc:"工具选择"`
	Tools                sonic.NoCopyRawMessage `json:"tools,omitempty" doc:"工具列表"`
	TopLogprobs          *int                   `json:"top_logprobs,omitempty" doc:"返回top logprobs数量"`
	TopP                 *float64               `json:"top_p,omitempty" doc:"核采样概率质量"`
	Truncation           string                 `json:"truncation,omitempty" doc:"截断策略"`
	User                 string                 `json:"user,omitempty" doc:"用户标识符"`
	PromptCacheRetention string                 `json:"prompt_cache_retention,omitempty" doc:"提示缓存保留策略"`
}

// OpenAICreateResponseRequest Response API 请求包装
//
//	@author centonhuang
//	@update 2026-04-17 10:00:00
type OpenAICreateResponseRequest struct {
	Body *OpenAICreateResponseReq `json:"body" doc:"请求体"`
}
