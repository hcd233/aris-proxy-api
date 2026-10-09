package dto

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
)

// MessageStoreTask 消息存储任务
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type MessageStoreTask struct {
	Ctx        context.Context
	APIKeyName string
	// APIKeyID 归属 API Key ID（鉴权唯一依据；名称仅用于展示）
	APIKeyID     uint
	ModelID      string
	Messages     []*vo.UnifiedMessage // 统一消息格式列表
	Tools        []*vo.UnifiedTool    // 统一工具格式列表
	InputTokens  int                  // 上游返回的输入token数
	OutputTokens int                  // 上游返回的输出token数
	Metadata     map[string]string    // 请求元数据
}

// ModelCallAuditTask 模型调用审计任务
//
//	@author centonhuang
//	@update 2026-04-29 10:00:00
type ModelCallAuditTask struct {
	Ctx                        context.Context
	ModelID                    string
	UpstreamProtocol           string
	APIProtocol                string
	Endpoint                   string
	InputTokens                int
	OutputTokens               int
	CacheCreationInputTokens   int
	CacheCreation1hInputTokens int
	CacheReadInputTokens       int
	FirstTokenLatencyMs        int64
	StreamDurationMs           int64
	UpstreamStatusCode         int
	ErrorMessage               string
	CostMicro                  *int64
	InputCostMicro             *int64
	OutputCostMicro            *int64
	CacheCreateCostMicro       *int64
	CacheReadCostMicro         *int64
	PricingCurrency            string
	CreatedAt                  time.Time
}

// SetTokensFromOpenAIUsage 从 OpenAI Usage 设置 token 计数。
//
// OpenAI 语义下 prompt_tokens 为含 cached_tokens 的输入总量（cached 是它的子集），
// 而审计与计费按「输入/输出/缓存创建/缓存读取」互斥四维口径，
// 故 InputTokens 落净输入（prompt_tokens − cached）。
//
//	@receiver t *ModelCallAuditTask
//	@param usage *OpenAICompletionUsage
//	@author centonhuang
//	@update 2026-10-08 10:00:00
func (t *ModelCallAuditTask) SetTokensFromOpenAIUsage(usage *OpenAICompletionUsage) {
	if usage == nil {
		return
	}
	t.InputTokens = usage.PromptTokens
	t.OutputTokens = usage.CompletionTokens
	switch {
	case usage.PromptCacheHitTokens != nil && *usage.PromptCacheHitTokens > 0:
		t.CacheReadInputTokens = *usage.PromptCacheHitTokens
	case usage.PromptTokensDetails != nil && usage.PromptTokensDetails.CachedTokens != nil && *usage.PromptTokensDetails.CachedTokens > 0:
		t.CacheReadInputTokens = *usage.PromptTokensDetails.CachedTokens
	}
	t.InputTokens = max(t.InputTokens-t.CacheReadInputTokens, 0)
}

// SetTokensFromAnthropicUsage 从 Anthropic Message Usage 设置 token 计数。
//
// Anthropic 官方语义 input_tokens 不含缓存两维，四维天然互斥、无需归一化；
// DeepSeek 风格 usage（prompt_cache_hit_tokens 与 prompt_cache_miss_tokens 成对出现、
// input_tokens 含命中部分）才需落净输入。
//
//	@receiver t *ModelCallAuditTask
//	@param msg *AnthropicMessage
//	@author centonhuang
//	@update 2026-10-08 10:00:00
func (t *ModelCallAuditTask) SetTokensFromAnthropicUsage(msg *AnthropicMessage) {
	if msg == nil || msg.Usage == nil {
		return
	}
	t.InputTokens = msg.Usage.InputTokens
	t.OutputTokens = msg.Usage.OutputTokens
	switch {
	case msg.Usage.CacheCreation != nil:
		// 新版 cache_creation 对象：总量 = 5m + 1h（对象优先于总量字段）
		t.CacheCreation1hInputTokens = lo.FromPtr(msg.Usage.CacheCreation.Ephemeral1hInputTokens)
		t.CacheCreationInputTokens = lo.FromPtr(msg.Usage.CacheCreation.Ephemeral5mInputTokens) + t.CacheCreation1hInputTokens
	default:
		// 旧版仅总量字段：全部归 5m 档（1h=0）
		t.CacheCreationInputTokens = lo.FromPtr(msg.Usage.CacheCreationInputTokens)
	}
	switch {
	case msg.Usage.PromptCacheHitTokens != nil && *msg.Usage.PromptCacheHitTokens > 0:
		t.CacheReadInputTokens = *msg.Usage.PromptCacheHitTokens
		if msg.Usage.PromptCacheMissTokens != nil {
			t.InputTokens = max(t.InputTokens-t.CacheReadInputTokens, 0)
		}
	case msg.Usage.CacheReadInputTokens != nil && *msg.Usage.CacheReadInputTokens > 0:
		t.CacheReadInputTokens = *msg.Usage.CacheReadInputTokens
	}
}

// SetTokensFromResponseUsage 从 Response API 响应设置 token 计数。
//
// 同 OpenAI Chat：input_tokens 含 cached_tokens，InputTokens 落净输入。
//
//	@receiver t *ModelCallAuditTask
//	@param rsp *OpenAICreateResponseRsp
//	@author centonhuang
//	@update 2026-10-08 10:00:00
func (t *ModelCallAuditTask) SetTokensFromResponseUsage(rsp *OpenAICreateResponseRsp) {
	if rsp == nil || rsp.Usage == nil {
		return
	}
	t.InputTokens = rsp.Usage.InputTokens
	t.OutputTokens = rsp.Usage.OutputTokens
	switch {
	case rsp.Usage.PromptCacheHitTokens != nil && *rsp.Usage.PromptCacheHitTokens > 0:
		t.CacheReadInputTokens = *rsp.Usage.PromptCacheHitTokens
	case rsp.Usage.InputTokensDetails != nil && rsp.Usage.InputTokensDetails.CachedTokens > 0:
		t.CacheReadInputTokens = rsp.Usage.InputTokensDetails.CachedTokens
	}
	t.InputTokens = max(t.InputTokens-t.CacheReadInputTokens, 0)
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

// DemoAccessAuditTask Demo 访问审计异步落库任务
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAuditTask struct {
	Ctx       context.Context
	Action    string // login / login_denied / module_access / module_denied
	Module    string // demo 模块名；login 类为空串
	Path      string // 请求路径
	IP        string // 客户端 IP
	UserAgent string // User-Agent
	Reason    string // 拒绝原因；成功时为空串
}
