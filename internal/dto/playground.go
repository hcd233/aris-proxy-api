package dto

// PlaygroundChatReq Playground 调试请求（请求体复用 OpenAI Chat 契约）
//
// APIKeyID 仅用于 OpenAPI 声明与校验：实际归属校验与 ctx 注入由
// middleware.PlaygroundAPIKeyMiddleware 完成（中间件先于 huma 参数绑定执行）。
//
//	@author centonhuang
//	@update 2026-10-09 10:00:00
type PlaygroundChatReq struct {
	APIKeyID uint                     `query:"apiKeyID" required:"true" minimum:"1" doc:"调用归属的 API Key ID（须属于当前用户；审计与限流按该 Key 计）"`
	Body     *OpenAIChatCompletionReq `json:"body" doc:"OpenAI Chat 请求体"`
}
