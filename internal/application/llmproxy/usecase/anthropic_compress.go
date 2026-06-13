package usecase

import (
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func compressAnthropicMessages(p compression.Pipeline, messages []*dto.AnthropicMessageParam) (*compression.PipelineResult, int, int) {
	originalLen := 0
	compressMsgs := make([]compression.Message, 0, len(messages))
	msgIndex := make([]int, 0, len(messages))
	for i, msg := range messages {
		if msg == nil || msg.Role != "user" || msg.Content == nil {
			continue
		}
		contentText := anthropicMessageText(msg)
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
		messages[idx].Content = &dto.AnthropicMessageContent{
			Text: compressMsgs[j].Content,
		}
	}

	return result, originalLen, compressedLen
}

func anthropicMessageText(msg *dto.AnthropicMessageParam) string {
	if msg.Content == nil {
		return ""
	}
	if msg.Content.Text != "" {
		return msg.Content.Text
	}
	var strs []string
	for _, block := range msg.Content.Blocks {
		if block != nil && block.Text != nil {
			strs = append(strs, *block.Text)
		}
	}
	return strings.Join(strs, "\n")
}
