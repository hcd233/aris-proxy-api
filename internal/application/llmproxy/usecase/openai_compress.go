package usecase

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func compressOpenAIMessages(p compression.Pipeline, messages []*dto.OpenAIChatCompletionMessageParam) (*compression.PipelineResult, int, int) {
	originalLen := 0
	compressMsgs := make([]compression.Message, 0, len(messages))
	msgIndex := make([]int, 0, len(messages))
	for i, msg := range messages {
		if msg == nil || msg.Role != "tool" {
			continue
		}
		contentText := openAIMessageText(msg)
		if contentText == "" {
			continue
		}
		originalLen += len(contentText)
		compressMsgs = append(compressMsgs, compression.Message{Role: "tool", Content: contentText})
		msgIndex = append(msgIndex, i)
	}

	if len(compressMsgs) == 0 {
		return &compression.PipelineResult{}, originalLen, 0
	}

	_, result := p.Compress(nil, compressMsgs)

	compressedLen := 0
	for j, idx := range msgIndex {
		if j >= len(compressMsgs) {
			break
		}
		compressedLen += len(compressMsgs[j].Content)
		messages[idx].Content = &dto.OpenAIMessageContent{Text: compressMsgs[j].Content}
	}

	return result, originalLen, compressedLen
}

func openAIMessageText(msg *dto.OpenAIChatCompletionMessageParam) string {
	if msg.Content == nil {
		return ""
	}
	if msg.Content.Text != "" {
		return msg.Content.Text
	}
	if len(msg.Content.Parts) > 0 {
		var strs []string
		for _, part := range msg.Content.Parts {
			if part.Type == "text" && part.Text != nil {
				strs = append(strs, *part.Text)
			}
		}
		return strings.Join(strs, "\n")
	}
	return ""
}
