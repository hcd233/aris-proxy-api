package usecase

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	commonvo "github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// storeDecisionSession 把一次 Decision 调用落成一条会话：user=input、assistant=answers 原始 JSON 文本。
//
// Decision 的 input 是单一逻辑载荷（非多轮对话），故多个 input 消息合并为一条 user 消息；
// answers 以原始 JSON 文本存储，信息无损且不发明渲染格式。
func (u *openAIUseCase) storeDecisionSession(ctx context.Context, req *dto.OpenAICreateDecisionRequest, rsp *dto.OpenAIDecisionRsp, modelID string) {
	if util.CtxValueBool(ctx, constant.CtxKeySkipStore) {
		return
	}
	log := logger.WithCtx(ctx)

	var inputTokens, outputTokens int
	if rsp.Usage != nil {
		inputTokens = rsp.Usage.InputTokens
		outputTokens = rsp.Usage.OutputTokens
	}

	if err := u.taskSubmitter.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:        util.CopyContextValues(ctx),
		APIKeyName: util.CtxValueString(ctx, constant.CtxKeyAPIKeyName),
		APIKeyID:   util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
		ModelID:    modelID,
		Messages: []*commonvo.UnifiedMessage{
			buildDecisionInputMessage(req.Body.Input),
			{
				Role:    enum.RoleAssistant,
				Content: &commonvo.UnifiedContent{Text: string(rsp.Answers)},
			},
		},
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}); err != nil {
		log.Error("[OpenAIUseCase] Failed to submit decision message store task", zap.Error(err))
	}
}

// buildDecisionInputMessage 把 Decision input 合并为一条 user 统一消息。
func buildDecisionInputMessage(input dto.DecisionInput) *commonvo.UnifiedMessage {
	if input.Text != nil {
		return &commonvo.UnifiedMessage{Role: enum.RoleUser, Content: &commonvo.UnifiedContent{Text: *input.Text}}
	}

	parts := make([]*commonvo.UnifiedContentPart, 0, len(input.Messages))
	for _, msg := range input.Messages {
		if msg == nil {
			continue
		}
		if msg.Content.Text != "" {
			parts = append(parts, &commonvo.UnifiedContentPart{Type: enum.ContentPartTypeText, Text: msg.Content.Text})
		}
		for _, part := range msg.Content.Parts {
			if part == nil {
				continue
			}
			// Decision 与 Response API 共用内容块类型，归一为统一词汇
			// （input_text/input_image → text/image_url），与 dto.FromResponseAPIMessage 口径一致。
			// input_file 不在支持范围（Decision 不支持文件），故不映射。
			switch part.Type {
			case enum.ResponseContentTypeInputText:
				parts = append(parts, &commonvo.UnifiedContentPart{
					Type: enum.ContentPartTypeText,
					Text: lo.FromPtr(part.Text),
				})
			case enum.ResponseContentTypeInputImage:
				parts = append(parts, &commonvo.UnifiedContentPart{
					Type:        enum.ContentPartTypeImageURL,
					ImageURL:    lo.FromPtr(part.ImageURL),
					ImageDetail: lo.FromPtr(part.Detail),
				})
			}
		}
	}
	return &commonvo.UnifiedMessage{Role: enum.RoleUser, Content: &commonvo.UnifiedContent{Parts: parts}}
}
