package dto

import "context"

// PingTask 健康检查任务
//
//	author centonhuang
//	update 2026-02-04 16:30:00
type PingTask struct {
	Ctx context.Context
}

// MessageStoreTask 消息存储任务
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type MessageStoreTask struct {
	Ctx          context.Context
	APIKeyName   string
	Model        string
	Messages     []*UnifiedMessage // 统一消息格式列表
	Tools        []*UnifiedTool    // 统一工具格式列表
	InputTokens  int               // 上游返回的输入token数
	OutputTokens int               // 上游返回的输出token数
	Metadata     map[string]string // 请求元数据
}

// SummarizeTask Session总结任务
//
//	@author centonhuang
//	@update 2026-03-26 10:00:00
type SummarizeTask struct {
	Ctx       context.Context
	SessionID uint
	Content   string
}

// ScoreTask Session评分任务
//
//	@author centonhuang
//	@update 2026-04-02 10:00:00
type ScoreTask struct {
	Ctx       context.Context
	SessionID uint
	Content   string
}

// ModelCallAuditTask 模型调用审计任务
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ModelCallAuditTask struct {
	Ctx                      context.Context
	ModelID                  uint
	Model                    string
	UpstreamProvider         string
	APIProvider              string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	FirstTokenLatencyMs      int64
	StreamDurationMs         int64
	UpstreamStatusCode       int
	ErrorMessage             string
}

// SetTokensFromOpenAIUsage 从 OpenAI Usage 设置 token 计数
//
//	@receiver t *ModelCallAuditTask
//	@param usage *OpenAICompletionUsage
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (t *ModelCallAuditTask) SetTokensFromOpenAIUsage(usage *OpenAICompletionUsage) {
	if usage == nil {
		return
	}
	t.InputTokens = usage.PromptTokens
	t.OutputTokens = usage.CompletionTokens
}

// SetTokensFromAnthropicUsage 从 Anthropic Message Usage 设置 token 计数
//
//	@receiver t *ModelCallAuditTask
//	@param msg *AnthropicMessage
//	@author centonhuang
//	@update 2026-04-09 10:00:00
func (t *ModelCallAuditTask) SetTokensFromAnthropicUsage(msg *AnthropicMessage) {
	if msg == nil || msg.Usage == nil {
		return
	}
	t.InputTokens = msg.Usage.InputTokens
	t.OutputTokens = msg.Usage.OutputTokens
	t.CacheCreationInputTokens = msg.Usage.CacheCreationInputTokens
	t.CacheReadInputTokens = msg.Usage.CacheReadInputTokens
}

// SetTokensFromResponseUsage 从 Response API 响应设置 token 计数
//
//	@receiver t *ModelCallAuditTask
//	@param rsp *OpenAICreateResponseRsp
//	@author centonhuang
//	@update 2026-04-18 15:00:00
func (t *ModelCallAuditTask) SetTokensFromResponseUsage(rsp *OpenAICreateResponseRsp) {
	if rsp == nil || rsp.Usage == nil {
		return
	}
	t.InputTokens = rsp.Usage.InputTokens
	t.OutputTokens = rsp.Usage.OutputTokens
	if rsp.Usage.InputTokensDetails != nil {
		t.CacheReadInputTokens = rsp.Usage.InputTokensDetails.CachedTokens
	}
}
