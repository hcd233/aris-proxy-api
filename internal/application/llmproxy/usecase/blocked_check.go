package usecase

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
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

func (u *openAIUseCase) checkContent(req *dto.OpenAIChatCompletionRequest) []uint {
	if u.blockedChecker == nil {
		return nil
	}
	content := extractOpenAIChatText(req)
	return u.blockedChecker.Check(content)
}

func (u *anthropicUseCase) checkContent(req *dto.AnthropicCreateMessageRequest) []uint {
	if u.blockedChecker == nil {
		return nil
	}
	content := extractAnthropicMessageText(req)
	return u.blockedChecker.Check(content)
}

func formatBlockedWords(words []string) string {
	if len(words) == 0 {
		return ""
	}
	quoted := lo.Map(words, func(w string, _ int) string { return "`" + w + "`" })
	return strings.Join(quoted, constant.BlockedWordSeparator)
}
