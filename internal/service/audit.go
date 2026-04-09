package service

import (
	"context"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/enum"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// auditTokens 审计任务中的 token 计数信息
type auditTokens struct {
	Input         int
	Output        int
	CacheCreation int
	CacheRead     int
}

// auditTokensFromOpenAIUsage 从 OpenAI Usage 提取 token 计数
func auditTokensFromOpenAIUsage(usage *dto.OpenAICompletionUsage) auditTokens {
	if usage == nil {
		return auditTokens{}
	}
	return auditTokens{
		Input:  usage.PromptTokens,
		Output: usage.CompletionTokens,
	}
}

// auditTokensFromAnthropicUsage 从 Anthropic Message Usage 提取 token 计数
func auditTokensFromAnthropicUsage(msg *dto.AnthropicMessage) auditTokens {
	if msg == nil || msg.Usage == nil {
		return auditTokens{}
	}
	return auditTokens{
		Input:         msg.Usage.InputTokens,
		Output:        msg.Usage.OutputTokens,
		CacheCreation: msg.Usage.CacheCreationInputTokens,
		CacheRead:     msg.Usage.CacheReadInputTokens,
	}
}

// submitAuditTask 提交模型调用审计任务（service 层内部公共函数）
//
//	@param ctx context.Context
//	@param endpoint *dbmodel.ModelEndpoint
//	@param exposedModel string 对外暴露的模型别名
//	@param apiProvider enum.ProviderType  接口层协议（openai/anthropic）
//	@param tokens auditTokens  token 计数
//	@param firstTokenLatencyMs int64
//	@param streamDurationMs int64
//	@param err error  上游错误（nil 表示成功）
func submitAuditTask(ctx context.Context, endpoint *dbmodel.ModelEndpoint, exposedModel string, apiProvider enum.ProviderType, tokens auditTokens, firstTokenLatencyMs, streamDurationMs int64, err error) {
	statusCode, errorMessage := util.ExtractUpstreamStatusAndError(err)
	pool.GetPoolManager().SubmitModelCallAuditTask(&dto.ModelCallAuditTask{
		Ctx:                      util.CopyContextValues(ctx),
		APIKeyID:                 util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
		ModelID:                  endpoint.ID,
		Model:                    exposedModel,
		UpstreamProvider:         endpoint.Provider,
		APIProvider:              string(apiProvider),
		InputTokens:              tokens.Input,
		OutputTokens:             tokens.Output,
		CacheCreationInputTokens: tokens.CacheCreation,
		CacheReadInputTokens:     tokens.CacheRead,
		FirstTokenLatencyMs:      firstTokenLatencyMs,
		StreamDurationMs:         streamDurationMs,
		UserAgent:                util.CtxValueString(ctx, constant.CtxKeyClient),
		UpstreamStatusCode:       statusCode,
		ErrorMessage:             errorMessage,
		TraceID:                  util.CtxValueString(ctx, constant.CtxKeyTraceID),
	})
}
