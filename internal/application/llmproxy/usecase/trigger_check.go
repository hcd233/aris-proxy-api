package usecase

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// extractOpenAIChatContentText 提取单条 OpenAI Chat 消息内容的可控文本（Text + Parts）。
func extractOpenAIChatContentText(content *dto.OpenAIMessageContent) string {
	if content == nil {
		return ""
	}
	var buf strings.Builder
	if content.Text != "" {
		buf.WriteString(content.Text)
	}
	for _, part := range content.Parts {
		if part != nil && part.Text != nil {
			buf.WriteString(*part.Text)
		}
	}
	return buf.String()
}

func extractOpenAIChatText(req *dto.OpenAIChatCompletionRequest) string {
	var buf strings.Builder
	for _, msg := range req.Body.Messages {
		buf.WriteString(extractOpenAIChatContentText(msg.Content))
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

// extractDecisionText 提取 Decision API 请求中的全部用户可控文本。
//
// 覆盖：input（字符串或消息 content 的文本/parts）、questions[].instructions、
// choices[].description/value、levels[].label/description。
func extractDecisionText(req *dto.OpenAICreateDecisionRequest) string {
	var buf strings.Builder

	buf.WriteString(extractDecisionInputText(req.Body.Input))
	for _, q := range req.Body.Questions {
		if q == nil {
			continue
		}
		buf.WriteString(extractDecisionQuestionText(q))
	}

	return buf.String()
}

// extractDecisionInputText 提取 Decision input（字符串或消息 content）中的文本。
func extractDecisionInputText(input dto.DecisionInput) string {
	var buf strings.Builder

	if input.Text != nil {
		buf.WriteString(*input.Text)
	}
	for _, msg := range input.Messages {
		if msg == nil {
			continue
		}
		buf.WriteString(msg.Content.Text)
		for _, part := range msg.Content.Parts {
			if part != nil && part.Text != nil {
				buf.WriteString(*part.Text)
			}
		}
	}

	return buf.String()
}

// extractDecisionQuestionText 提取单个 question 的指令、选项与等级文本。
func extractDecisionQuestionText(q *dto.DecisionQuestion) string {
	var buf strings.Builder

	buf.WriteString(q.Instructions)
	for _, choice := range q.Choices {
		if choice == nil {
			continue
		}
		buf.WriteString(lo.FromPtr(choice.Description))
		buf.WriteString(lo.FromPtr(choice.Value.StringValue))
	}
	for _, level := range q.Levels {
		if level == nil {
			continue
		}
		buf.WriteString(level.Label)
		buf.WriteString(lo.FromPtr(level.Description))
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
			buf.WriteString(extractAnthropicContentText(msg.Content))
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

func (u *openAIUseCase) checkDecisionContent(req *dto.OpenAICreateDecisionRequest) []uint {
	if u.triggerChecker == nil {
		return nil
	}
	return u.triggerChecker.Check(extractDecisionText(req))
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
