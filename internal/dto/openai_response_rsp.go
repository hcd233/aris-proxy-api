package dto

// ==================== Response API Response DTOs ====================
//
// References:
//   - docs/openai/create_response.md "Returns" section (response object).
//   - docs/openai/create_response.md "Streaming" section for the
//     `response.completed` terminal event shape.
//
// The response `output` is a heterogenous array of items that overlaps
// heavily with the request `input` union (Message / FunctionCall / Reasoning
// / ComputerCall / WebSearchCall / ...). To avoid duplicating ~30 subtypes
// we reuse ResponseInputItem here — it already models every documented
// item type via its flat `type`-dispatched layout.
//
//	@author centonhuang
//	@update 2026-04-18 15:00:00

// OpenAICreateResponseRsp Response API 顶层响应体
//
// Only fields needed by the gateway (message extraction, audit & storage)
// are modeled; the full documented shape is large but we can extend this
// struct as new needs arise. Unknown fields are dropped on unmarshal, which
// is what we want for storage — we only persist what we understand.
type OpenAICreateResponseRsp struct {
	ID                 string               `json:"id,omitempty" doc:"响应唯一 ID"`
	Object             string               `json:"object,omitempty" doc:"对象类型，固定 response"`
	CreatedAt          int64                `json:"created_at,omitempty" doc:"创建时间(Unix 秒)"`
	Status             string               `json:"status,omitempty" doc:"生命周期状态"`
	Model              string               `json:"model,omitempty" doc:"模型 ID"`
	Output             []*ResponseInputItem `json:"output,omitempty" doc:"输出项数组(message/reasoning/function_call/...)"`
	Usage              *ResponseUsage       `json:"usage,omitempty" doc:"token 使用量"`
	Error              *ResponseErrorBody   `json:"error,omitempty" doc:"上游错误(失败时填充)"`
	IncompleteDetails  *ResponseIncomplete  `json:"incomplete_details,omitempty" doc:"未完成原因"`
	PreviousResponseID *string              `json:"previous_response_id,omitempty" doc:"前置响应 ID"`
}

// ResponseUsage Response API 用量
type ResponseUsage struct {
	InputTokens           int                         `json:"input_tokens" doc:"输入 token 数"`
	InputTokensDetails    *ResponseInputTokensDetail  `json:"input_tokens_details,omitempty" doc:"输入 token 明细"`
	OutputTokens          int                         `json:"output_tokens" doc:"输出 token 数"`
	OutputTokensDetails   *ResponseOutputTokensDetail `json:"output_tokens_details,omitempty" doc:"输出 token 明细"`
	TotalTokens           int                         `json:"total_tokens" doc:"总 token 数"`
	PromptCacheHitTokens  *int                        `json:"prompt_cache_hit_tokens,omitempty" doc:"缓存命中的token数"`
	PromptCacheMissTokens *int                        `json:"prompt_cache_miss_tokens,omitempty" doc:"缓存未命中的token数"`
}

// ResponseInputTokensDetail Response API 输入 token 明细
type ResponseInputTokensDetail struct {
	CachedTokens int `json:"cached_tokens" doc:"命中缓存的 token 数"`
}

// ResponseOutputTokensDetail Response API 输出 token 明细
type ResponseOutputTokensDetail struct {
	ReasoningTokens int `json:"reasoning_tokens" doc:"推理 token 数"`
}

// ResponseErrorBody Response API 错误对象
type ResponseErrorBody struct {
	Code    string `json:"code,omitempty" doc:"错误代码"`
	Message string `json:"message,omitempty" doc:"错误消息"`
}

// ResponseIncomplete Response API 未完成原因
type ResponseIncomplete struct {
	Reason string `json:"reason,omitempty" doc:"未完成原因: max_output_tokens/content_filter"`
}

// ==================== Streaming Events ====================
//
// Only the terminal `response.completed` event carries the full final
// response object and usage. Intermediate delta/done events are forwarded
// byte-for-byte to the client but are not needed for storage.
//
// Other documented terminal-error events (response.failed /
// response.incomplete) share the same envelope.

// ResponseStreamTerminalEvent 终态 SSE 事件 payload（response.completed/failed/incomplete）
type ResponseStreamTerminalEvent struct {
	Type     string                   `json:"type" doc:"事件类型"`
	Response *OpenAICreateResponseRsp `json:"response,omitempty" doc:"最终响应对象"`
}
