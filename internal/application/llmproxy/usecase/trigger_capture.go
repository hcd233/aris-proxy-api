package usecase

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	proxyutil "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	convvo "github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// ==================== Capture 触发词短路 ====================
//
// capture 命中且触发词位于「最后一条用户提问消息」时：保存该消息之前的
// 历史对话（无 assistant 回复），不请求上游，返回固定回复。判定与保存逻辑
// 按协议拆分到本文件，入口（anthropic.go / openai.go）只调用 intercept*。

// captureHit 描述一次生效的 capture 命中。
type captureHit struct {
	words   []string // 生效的触发词（用于审计 remark）
	hasHist bool     // 触发消息之前是否存在历史消息
	lastIdx int      // 触发消息在请求 messages 中的索引（历史切片上界）
}

// findLastIndex 倒序查找第一个满足条件的索引（lo 未提供 FindLastIndexOf）。
func findLastIndex[T any](slice []T, match func(T) bool) int {
	for i, item := range slices.Backward(slice) {
		if match(item) {
			return i
		}
	}
	return -1
}

// isAnthropicToolResult 判定 Anthropic tool 结果回传消息（role=user 但内容为
// tool_result 块）：它不是用户提问，capture 判定需跳过——对齐 OpenAI Chat 版
// 排除 tool_call_id 用户消息的口径，保证两协议命中位置判定一致。
func isAnthropicToolResult(msg *dto.AnthropicMessageParam) bool {
	if msg == nil || msg.Role != enum.RoleUser || msg.Content == nil {
		return false
	}
	return lo.ContainsBy(msg.Content.Blocks, func(block *dto.AnthropicContentBlock) bool {
		return block != nil && block.Type == enum.AnthropicContentBlockTypeToolResult
	})
}

// captureHitOnLastQuestion 判定 capture 词是否出现在最后一条用户提问消息中。
// 返回 nil 表示 capture 未生效（词只出现在历史/system 中），请求照常转发。
func (u *anthropicUseCase) captureHitOnLastQuestion(matched []uint, req *dto.AnthropicCreateMessageRequest) *captureHit {
	captureIDs := u.triggerChecker.CaptureIDs(matched)
	if len(captureIDs) == 0 {
		return nil
	}
	idx := findLastIndex(req.Body.Messages, func(msg *dto.AnthropicMessageParam) bool {
		return msg != nil && msg.Role == enum.RoleUser && !isAnthropicToolResult(msg)
	})
	if idx < 0 {
		return nil
	}
	lastQ := req.Body.Messages[idx]
	if lastQ == nil || lastQ.Content == nil {
		return nil
	}
	hits := lo.Intersect(u.triggerChecker.Check(extractAnthropicContentText(lastQ.Content)), captureIDs)
	if len(hits) == 0 {
		return nil
	}
	return &captureHit{
		words:   u.triggerChecker.MatchedWords(hits),
		hasHist: idx > 0,
		lastIdx: idx,
	}
}

// extractAnthropicContentText 提取单条消息内容的可控文本（text + blocks 的 text/thinking）。
func extractAnthropicContentText(content *dto.AnthropicMessageContent) string {
	var buf strings.Builder
	if content.Text != "" {
		buf.WriteString(content.Text)
	}
	for _, block := range content.Blocks {
		if block == nil {
			continue
		}
		if block.Text != nil {
			buf.WriteString(*block.Text)
		}
		if block.Thinking != nil {
			buf.WriteString(*block.Thinking)
		}
	}
	return buf.String()
}

// captureHitOnLastChatQuestion OpenAI Chat 版触发位置判定。
func (u *openAIUseCase) captureHitOnLastChatQuestion(matched []uint, req *dto.OpenAIChatCompletionRequest) *captureHit {
	captureIDs := u.triggerChecker.CaptureIDs(matched)
	if len(captureIDs) == 0 {
		return nil
	}
	idx := findLastIndex(req.Body.Messages, func(msg *dto.OpenAIChatCompletionMessageParam) bool {
		return msg != nil && msg.Role == enum.RoleUser && lo.FromPtr(msg.ToolCallID) == ""
	})
	if idx < 0 {
		return nil
	}
	lastQ := req.Body.Messages[idx]
	hits := lo.Intersect(u.triggerChecker.Check(extractOpenAIChatContentText(lastQ.Content)), captureIDs)
	if len(hits) == 0 {
		return nil
	}
	return &captureHit{
		words:   u.triggerChecker.MatchedWords(hits),
		hasHist: idx > 0,
		lastIdx: idx,
	}
}

// captureHitOnLastResponseQuestion OpenAI Response 版触发位置判定。
// input 为字符串时整个 input 即用户提问（历史仅剩 Instructions）；为 items 时
// 取最后一条 role=user 的 message item。
func (u *openAIUseCase) captureHitOnLastResponseQuestion(matched []uint, req *dto.OpenAICreateResponseRequest) *captureHit {
	captureIDs := u.triggerChecker.CaptureIDs(matched)
	if len(captureIDs) == 0 {
		return nil
	}
	if req.Body.Input == nil {
		return nil
	}
	hasInstructions := req.Body.Instructions != nil && *req.Body.Instructions != ""

	// 字符串 input：触发消息即该字符串，历史 = Instructions（若非空）
	if len(req.Body.Input.Items) == 0 {
		if req.Body.Input.Text == "" {
			return nil
		}
		hits := lo.Intersect(u.triggerChecker.Check(req.Body.Input.Text), captureIDs)
		if len(hits) == 0 {
			return nil
		}
		return &captureHit{words: u.triggerChecker.MatchedWords(hits), hasHist: hasInstructions, lastIdx: 0}
	}

	// items input：倒序找最后一条 role=user 的 message item
	idx := findLastIndex(req.Body.Input.Items, func(item *dto.ResponseInputItem) bool {
		return item != nil && lo.FromPtr(item.Role) == enum.RoleUser && item.Content != nil
	})
	if idx < 0 {
		return nil
	}
	lastQ := req.Body.Input.Items[idx]
	var buf strings.Builder
	extractResponseItemContent(&buf, lastQ.Content)
	hits := lo.Intersect(u.triggerChecker.Check(buf.String()), captureIDs)
	if len(hits) == 0 {
		return nil
	}
	return &captureHit{
		words:   u.triggerChecker.MatchedWords(hits),
		hasHist: idx > 0 || hasInstructions,
		lastIdx: idx,
	}
}

// captureReplyText 按是否有历史返回固定文案。
func captureReplyText(hasHist bool) string {
	if hasHist {
		return constant.TriggerCaptureSavedReply
	}
	return constant.TriggerCaptureEmptyReply
}

// submitCaptureAudit 提交 capture 短路的审计任务（tokens 全 0，无上游调用）。
func submitCaptureAudit(ctx context.Context, submitter TaskSubmitter, m *aggregate.Model, endpoint string, upstreamProtocol, apiProtocol enum.ProtocolType, words []string) {
	auditTask := &dto.ModelCallAuditTask{
		Ctx:              util.CopyContextValues(ctx),
		ModelID:          m.ModelID(),
		Endpoint:         endpoint,
		UpstreamProtocol: upstreamProtocol,
		APIProtocol:      apiProtocol,
		ErrorMessage:     fmt.Sprintf(constant.TriggerCaptureAuditTemplate, formatTriggerWords(words)),
	}
	_ = submitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
}

// submitCaptureStore 提交历史上下文存储任务（仅历史消息，无 assistant 回复）。
func submitCaptureStore(ctx context.Context, submitter TaskSubmitter, modelID string, messages []*convvo.UnifiedMessage, tools []*convvo.UnifiedTool, metadata map[string]string) {
	if len(messages) == 0 {
		return
	}
	if err := submitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:        util.CopyContextValues(ctx),
		APIKeyName: util.CtxValueString(ctx, constant.CtxKeyAPIKeyName),
		ModelID:    modelID,
		Messages:   messages,
		Tools:      tools,
		Metadata:   metadata,
	}); err != nil {
		logger.WithCtx(ctx).Error("[TriggerCapture] Failed to submit capture store task", zap.Error(err))
	}
}

// anthropicRouteUpstreamProtocol 按 compatRoute 得到 anthropic 入口的审计用上游协议。
func anthropicRouteUpstreamProtocol(compatRoute enum.CompatRoute) enum.ProtocolType {
	switch compatRoute {
	case enum.CompatRouteViaOpenAIChat:
		return enum.ProtocolOpenAIChatCompletion
	default:
		return enum.ProtocolAnthropicMessage
	}
}

// openAIChatRouteUpstreamProtocol 按 compatRoute 得到 OpenAI Chat 入口的审计用上游协议。
func openAIChatRouteUpstreamProtocol(compatRoute enum.CompatRoute) enum.ProtocolType {
	switch compatRoute {
	case enum.CompatRouteViaAnthropicMessage:
		return enum.ProtocolAnthropicMessage
	default:
		return enum.ProtocolOpenAIChatCompletion
	}
}

// openAIResponseRouteUpstreamProtocol 按 compatRoute 得到 OpenAI Response 入口的审计用上游协议。
func openAIResponseRouteUpstreamProtocol(compatRoute enum.CompatRoute) enum.ProtocolType {
	switch compatRoute {
	case enum.CompatRouteViaAnthropicMessage:
		return enum.ProtocolAnthropicMessage
	case enum.CompatRouteViaOpenAIChat:
		return enum.ProtocolOpenAIChatCompletion
	default:
		return enum.ProtocolOpenAIResponse
	}
}

// interceptAnthropicCapture anthropic 入口的 capture 短路。
// 返回非 nil Result 表示已短路；nil 表示未生效，继续正常转发。
func (u *anthropicUseCase) interceptAnthropicCapture(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstreamProtocol enum.ProtocolType, matched []uint, stream bool) port.Result {
	hit := u.captureHitOnLastQuestion(matched, req)
	if hit == nil {
		return nil
	}
	submitCaptureAudit(ctx, u.taskSubmitter, m, ep.Name(), upstreamProtocol, enum.ProtocolAnthropicMessage, hit.words)
	u.storeAnthropicHistory(ctx, req, m, hit.lastIdx)
	return proxyutil.NewAnthropicCaptureReply(captureReplyText(hit.hasHist), req.Body.Model, stream)
}

// storeAnthropicHistory 保存 Anthropic 请求中指定上界之前的历史上下文（无 assistant 回复）。
func (u *anthropicUseCase) storeAnthropicHistory(ctx context.Context, req *dto.AnthropicCreateMessageRequest, m *aggregate.Model, lastIdx int) {
	if lastIdx <= 0 {
		return
	}
	hist := req.Body.Messages[:lastIdx]
	unified := make([]*convvo.UnifiedMessage, 0, len(hist))
	for _, msg := range hist {
		um, err := dto.FromAnthropicMessage(msg)
		if err != nil {
			logger.WithCtx(ctx).Warn("[TriggerCapture] Skip message conversion in history", zap.Error(err))
			continue
		}
		unified = append(unified, um)
	}
	tools := lo.Map(req.Body.Tools, func(tool *dto.AnthropicTool, _ int) *convvo.UnifiedTool {
		return dto.FromAnthropicTool(tool)
	})
	submitCaptureStore(ctx, u.taskSubmitter, m.ModelID(), unified, tools, proxyutil.ExtractAnthropicMetadata(req.Body.Metadata))
}

// omitAndCaptureMessageHit 判定「同时命中 omit 与 capture 词且 capture 未短路」的旁路保存。
// capture 词未落在最后一条用户提问中（captureHitOnLastQuestion 未命中）时，仍保存
// 最后一条用户提问之前的历史，不短路、照常转发——omit 与 capture 两个逻辑都执行。
func (u *anthropicUseCase) omitAndCaptureMessageHit(matched []uint, req *dto.AnthropicCreateMessageRequest) *captureHit {
	if len(u.triggerChecker.OmitIDs(matched)) == 0 || len(u.triggerChecker.CaptureIDs(matched)) == 0 {
		return nil
	}
	idx := findLastIndex(req.Body.Messages, func(msg *dto.AnthropicMessageParam) bool {
		return msg != nil && msg.Role == enum.RoleUser && !isAnthropicToolResult(msg)
	})
	if idx < 0 {
		return nil
	}
	return &captureHit{
		words:   u.triggerChecker.MatchedWords(u.triggerChecker.CaptureIDs(matched)),
		lastIdx: idx,
	}
}

// interceptChatCapture OpenAI Chat 入口的 capture 短路。
func (u *openAIUseCase) interceptChatCapture(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstreamProtocol enum.ProtocolType, matched []uint, stream bool) port.Result {
	hit := u.captureHitOnLastChatQuestion(matched, req)
	if hit == nil {
		return nil
	}
	submitCaptureAudit(ctx, u.taskSubmitter, m, ep.Name(), upstreamProtocol, enum.ProtocolOpenAIChatCompletion, hit.words)
	u.storeOpenAIChatHistory(ctx, req, m, hit.lastIdx)
	return proxyutil.NewOpenAIChatCaptureReply(captureReplyText(hit.hasHist), req.Body.Model, stream)
}

// storeOpenAIChatHistory 保存 OpenAI Chat 请求中指定上界之前的历史上下文（无 assistant 回复）。
func (u *openAIUseCase) storeOpenAIChatHistory(ctx context.Context, req *dto.OpenAIChatCompletionRequest, m *aggregate.Model, lastIdx int) {
	if lastIdx <= 0 {
		return
	}
	hist := req.Body.Messages[:lastIdx]
	unified := make([]*convvo.UnifiedMessage, 0, len(hist))
	for _, msg := range hist {
		um, err := dto.FromOpenAIMessage(msg)
		if err != nil {
			logger.WithCtx(ctx).Warn("[TriggerCapture] Skip message conversion in history", zap.Error(err))
			continue
		}
		unified = append(unified, um)
	}
	tools := lo.Map(req.Body.Tools, func(tool dto.OpenAIChatCompletionTool, _ int) *convvo.UnifiedTool {
		return dto.FromOpenAITool(&tool)
	})
	submitCaptureStore(ctx, u.taskSubmitter, m.ModelID(), unified, tools, req.Body.Metadata)
}

// omitAndCaptureChatHit OpenAI Chat 版「同时命中 omit 与 capture 词且 capture 未短路」的旁路保存判定。
func (u *openAIUseCase) omitAndCaptureChatHit(matched []uint, req *dto.OpenAIChatCompletionRequest) *captureHit {
	if len(u.triggerChecker.OmitIDs(matched)) == 0 || len(u.triggerChecker.CaptureIDs(matched)) == 0 {
		return nil
	}
	idx := findLastIndex(req.Body.Messages, func(msg *dto.OpenAIChatCompletionMessageParam) bool {
		return msg != nil && msg.Role == enum.RoleUser && lo.FromPtr(msg.ToolCallID) == ""
	})
	if idx < 0 {
		return nil
	}
	return &captureHit{
		words:   u.triggerChecker.MatchedWords(u.triggerChecker.CaptureIDs(matched)),
		lastIdx: idx,
	}
}

// captureResponseHistory 组装 Response 入口的历史上下文（Instructions + 触发消息前的 input items）。
// 转换失败的消息跳过并记录 Warn，不阻断保存其余历史。
func captureResponseHistory(ctx context.Context, req *dto.OpenAICreateResponseRequest, lastIdx int) []*convvo.UnifiedMessage {
	var unified []*convvo.UnifiedMessage
	if req.Body.Instructions != nil && *req.Body.Instructions != "" {
		unified = append(unified, &convvo.UnifiedMessage{
			Role:    enum.RoleSystem,
			Content: &convvo.UnifiedContent{Text: *req.Body.Instructions},
		})
	}
	if req.Body.Input == nil || len(req.Body.Input.Items) == 0 || lastIdx <= 0 {
		return unified
	}
	ums, err := dto.FromResponseAPIInputItems(req.Body.Input.Items[:lastIdx])
	if err != nil {
		logger.WithCtx(ctx).Warn("[TriggerCapture] Skip input items conversion in history", zap.Error(err))
		return unified
	}
	return append(unified, ums...)
}

// omitAndCaptureResponseHit OpenAI Response 版「同时命中 omit 与 capture 词且 capture 未短路」的旁路保存判定。
func (u *openAIUseCase) omitAndCaptureResponseHit(matched []uint, req *dto.OpenAICreateResponseRequest) *captureHit {
	if len(u.triggerChecker.OmitIDs(matched)) == 0 || len(u.triggerChecker.CaptureIDs(matched)) == 0 {
		return nil
	}
	if req.Body.Input == nil {
		return nil
	}

	// 字符串 input：历史仅剩 Instructions（若有），由 captureResponseHistory 组装
	if len(req.Body.Input.Items) == 0 {
		return &captureHit{
			words:   u.triggerChecker.MatchedWords(u.triggerChecker.CaptureIDs(matched)),
			lastIdx: 0,
		}
	}

	idx := findLastIndex(req.Body.Input.Items, func(item *dto.ResponseInputItem) bool {
		return item != nil && lo.FromPtr(item.Role) == enum.RoleUser && item.Content != nil
	})
	if idx < 0 {
		return nil
	}
	return &captureHit{
		words:   u.triggerChecker.MatchedWords(u.triggerChecker.CaptureIDs(matched)),
		lastIdx: idx,
	}
}

// interceptResponseCapture OpenAI Response 入口的 capture 短路。
func (u *openAIUseCase) interceptResponseCapture(ctx context.Context, req *dto.OpenAICreateResponseRequest, m *aggregate.Model, ep *aggregate.Endpoint, upstreamProtocol enum.ProtocolType, matched []uint, stream bool) port.Result {
	hit := u.captureHitOnLastResponseQuestion(matched, req)
	if hit == nil {
		return nil
	}
	submitCaptureAudit(ctx, u.taskSubmitter, m, ep.Name(), upstreamProtocol, enum.ProtocolOpenAIResponse, hit.words)

	if hit.hasHist {
		unified := captureResponseHistory(ctx, req, hit.lastIdx)
		tools := dto.FromResponseAPITools(req.Body.Tools)
		submitCaptureStore(ctx, u.taskSubmitter, m.ModelID(), unified, tools, req.Body.Metadata)
	}
	return proxyutil.NewOpenAIResponseCaptureReply(captureReplyText(hit.hasHist), lo.FromPtr(req.Body.Model), stream)
}
