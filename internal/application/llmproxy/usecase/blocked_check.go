package usecase

import (
	"context"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

func extractOpenAIChatText(req *dto.OpenAIChatCompletionRequest) string {
	var buf strings.Builder
	for _, msg := range req.Body.Messages {
		if msg.Content != nil {
			if msg.Content.Text != "" {
				buf.WriteString(msg.Content.Text)
			}
			for _, part := range msg.Content.Parts {
				if part.Text != nil {
					buf.WriteString(*part.Text)
				}
			}
		}
		if msg.ReasoningContent != nil {
			buf.WriteString(*msg.ReasoningContent)
		}
	}
	return buf.String()
}

func extractAnthropicMessageText(req *dto.AnthropicCreateMessageRequest) string {
	var buf strings.Builder
	for _, msg := range req.Body.Messages {
		if msg.Content != nil {
			if msg.Content.Text != "" {
				buf.WriteString(msg.Content.Text)
			}
			for _, block := range msg.Content.Blocks {
				if block.Text != nil {
					buf.WriteString(*block.Text)
				}
				if block.Thinking != nil {
					buf.WriteString(*block.Thinking)
				}
			}
		}
	}
	return buf.String()
}

func (u *openAIUseCase) checkContent(ctx context.Context, req *dto.OpenAIChatCompletionRequest) error {
	if u.blockedChecker == nil {
		return nil
	}
	content := extractOpenAIChatText(req)
	matched := u.blockedChecker.Check(content)
	if len(matched) > 0 {
		auditTask := &dto.ModelCallAuditTask{
			Ctx:          util.CopyContextValues(ctx),
			ErrorMessage: constant.BlockedAuditRemark,
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
		return ierr.New(ierr.ErrContentBlocked, "ContentBlocked")
	}
	return nil
}

func (u *anthropicUseCase) checkContent(ctx context.Context, req *dto.AnthropicCreateMessageRequest) error {
	if u.blockedChecker == nil {
		return nil
	}
	content := extractAnthropicMessageText(req)
	matched := u.blockedChecker.Check(content)
	if len(matched) > 0 {
		auditTask := &dto.ModelCallAuditTask{
			Ctx:          util.CopyContextValues(ctx),
			ErrorMessage: constant.BlockedAuditRemark,
		}
		_ = u.taskSubmitter.SubmitModelCallAuditTask(auditTask) //nolint:errcheck // best-effort audit
		return ierr.New(ierr.ErrContentBlocked, "ContentBlocked")
	}
	return nil
}
