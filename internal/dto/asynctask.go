package dto

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/samber/lo"
)

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
	Messages     []*vo.UnifiedMessage // 统一消息格式列表
	Tools        []*vo.UnifiedTool    // 统一工具格式列表
	InputTokens  int                  // 上游返回的输入token数
	OutputTokens int                  // 上游返回的输出token数
	Metadata     map[string]string    // 请求元数据
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
//	@update 2026-04-29 10:00:00
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
//	@update 2026-04-29 10:00:00
func (t *ModelCallAuditTask) SetTokensFromOpenAIUsage(usage *OpenAICompletionUsage) {
	if usage == nil {
		return
	}
	t.InputTokens = usage.PromptTokens
	t.OutputTokens = usage.CompletionTokens
	t.CacheReadInputTokens = lo.FromPtr(usage.PromptCacheHitTokens)
}

// SetTokensFromAnthropicUsage 从 Anthropic Message Usage 设置 token 计数
//
//	@receiver t *ModelCallAuditTask
//	@param msg *AnthropicMessage
//	@author centonhuang
//	@update 2026-04-29 10:00:00
func (t *ModelCallAuditTask) SetTokensFromAnthropicUsage(msg *AnthropicMessage) {
	if msg == nil || msg.Usage == nil {
		return
	}
	t.InputTokens = msg.Usage.InputTokens
	t.OutputTokens = msg.Usage.OutputTokens
	t.CacheCreationInputTokens = lo.FromPtr(msg.Usage.CacheCreationInputTokens)
	t.CacheReadInputTokens = lo.FromPtr(msg.Usage.CacheReadInputTokens)
	if msg.Usage.PromptCacheHitTokens != nil {
		t.CacheReadInputTokens = lo.FromPtr(msg.Usage.PromptCacheHitTokens)
	}
}

// SetTokensFromResponseUsage 从 Response API 响应设置 token 计数
//
//	@receiver t *ModelCallAuditTask
//	@param rsp *OpenAICreateResponseRsp
//	@author centonhuang
//	@update 2026-04-29 15:00:00
func (t *ModelCallAuditTask) SetTokensFromResponseUsage(rsp *OpenAICreateResponseRsp) {
	if rsp == nil || rsp.Usage == nil {
		return
	}
	t.InputTokens = rsp.Usage.InputTokens
	t.OutputTokens = rsp.Usage.OutputTokens
	if rsp.Usage.InputTokensDetails != nil {
		t.CacheReadInputTokens = rsp.Usage.InputTokensDetails.CachedTokens
	}
	if rsp.Usage.PromptCacheHitTokens != nil {
		t.CacheReadInputTokens = lo.FromPtr(rsp.Usage.PromptCacheHitTokens)
	}
}

// SetErrorFromResponseStatus 将 Response API 终态中的 in-band 失败/未完成原因
// 注入到审计任务 ErrorMessage。
//
// 场景：上游 HTTP 200 正常返回，但 Response 对象 status=failed 或
// status=incomplete，此时 ExtractUpstreamStatusAndError 只能看到 HTTP
// 层，拿到的是成功；网关需要从响应对象本身抽取失败/未完成原因（error.message
// 或 incomplete_details.reason），审计仪表盘才能区分"业务失败"和"成功"。
// 若 t.ErrorMessage 已非空（传输层已经报错），则不覆盖。
//
//	@receiver t *ModelCallAuditTask
//	@param rsp *OpenAICreateResponseRsp
//	@author centonhuang
//	@update 2026-04-18 17:00:00
func (t *ModelCallAuditTask) SetErrorFromResponseStatus(rsp *OpenAICreateResponseRsp) {
	if rsp == nil || t.ErrorMessage != "" {
		return
	}
	switch rsp.Status {
	case enum.ResponseStatusFailed:
		if rsp.Error != nil && rsp.Error.Message != "" {
			t.ErrorMessage = fmt.Sprintf(constant.ResponseFailedAuditReasonTemplate, rsp.Error.Message)
			return
		}
		t.ErrorMessage = constant.ResponseFailedAuditReason
	case enum.ResponseStatusIncomplete:
		if rsp.IncompleteDetails != nil && rsp.IncompleteDetails.Reason != "" {
			t.ErrorMessage = fmt.Sprintf(constant.ResponseIncompleteAuditReasonTemplate, rsp.IncompleteDetails.Reason)
			return
		}
		t.ErrorMessage = constant.ResponseIncompleteAuditReason
	}
}
