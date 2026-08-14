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

// extractOpenAIResponseText 提取 OpenAI Response API (/responses) 请求中的用户可控文本。
//
// 覆盖：Instructions 系统指令、Input 字符串、InputItem 的消息内容（Text/Parts）、
// FileSearchCall 查询、函数参数 JSON（Arguments）、自定义工具输入（Input）、
// 代码解释器代码（Code）与输出文本（Output）。
func extractOpenAIResponseText(req *dto.OpenAICreateResponseRequest) string {
	var buf strings.Builder

	if req.Body.Instructions != nil {
		buf.WriteString(*req.Body.Instructions)
	}
	if req.Body.Input != nil {
		if req.Body.Input.Text != "" {
			buf.WriteString(req.Body.Input.Text)
		}
		for _, item := range req.Body.Input.Items {
			extractResponseInputItemText(&buf, item)
		}
	}
	return buf.String()
}

func extractResponseInputItemText(buf *strings.Builder, item *dto.ResponseInputItem) {
	if item == nil {
		return
	}
	extractResponseItemContent(buf, item.Content)
	for _, q := range item.Queries {
		buf.WriteString(q)
	}
	if item.Arguments != nil {
		buf.WriteString(*item.Arguments)
	}
	if item.Input != nil {
		buf.WriteString(*item.Input)
	}
	if item.Code != nil {
		buf.WriteString(*item.Code)
	}
	extractResponseItemOutput(buf, item.Output)
}

// extractResponseItemContent 提取 ResponseInputItem 的消息内容文本（字符串或 parts 数组）。
func extractResponseItemContent(buf *strings.Builder, content *dto.ResponseInputMessageContent) {
	if content == nil {
		return
	}
	if content.Text != "" {
		buf.WriteString(content.Text)
	}
	for _, part := range content.Parts {
		if part != nil && part.Text != nil {
			buf.WriteString(*part.Text)
		}
	}
}

// extractResponseItemOutput 提取 ResponseInputItem 的输出文本（纯字符串或函数输出 content 列表）。
func extractResponseItemOutput(buf *strings.Builder, output *dto.ResponseInputItemOutput) {
	if output == nil {
		return
	}
	if output.Text != "" {
		buf.WriteString(output.Text)
	}
	if output.FunctionOutput == nil {
		return
	}
	if output.FunctionOutput.Text != "" {
		buf.WriteString(output.FunctionOutput.Text)
	}
	for _, part := range output.FunctionOutput.Parts {
		if part != nil && part.Text != nil {
			buf.WriteString(*part.Text)
		}
	}
}

func extractAnthropicMessageText(req *dto.AnthropicCreateMessageRequest) string {
	var buf strings.Builder
	// Anthropic system prompt 是顶层字段（不在 messages 内），需单独提取扫描
	if req.Body.System != nil {
		if req.Body.System.Text != "" {
			buf.WriteString(req.Body.System.Text)
		}
		for _, block := range req.Body.System.Blocks {
			if block.Text != nil {
				buf.WriteString(*block.Text)
			}
		}
	}
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
	if u.triggerChecker == nil {
		return nil
	}
	content := extractOpenAIChatText(req)
	return u.triggerChecker.Check(content)
}

func (u *openAIUseCase) checkResponseContent(req *dto.OpenAICreateResponseRequest) []uint {
	if u.triggerChecker == nil {
		return nil
	}
	content := extractOpenAIResponseText(req)
	return u.triggerChecker.Check(content)
}

func (u *anthropicUseCase) checkContent(req *dto.AnthropicCreateMessageRequest) []uint {
	if u.triggerChecker == nil {
		return nil
	}
	content := extractAnthropicMessageText(req)
	return u.triggerChecker.Check(content)
}

func formatTriggerWords(words []string) string {
	if len(words) == 0 {
		return ""
	}
	quoted := lo.Map(words, func(w string, _ int) string { return "`" + w + "`" })
	return strings.Join(quoted, constant.TriggerWordSeparator)
}
