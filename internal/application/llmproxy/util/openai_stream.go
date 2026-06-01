package proxyutil

import (
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// HasNonEmptyDelta 检查 chunk 是否包含有实质内容的 delta（文本、推理内容或工具调用）。
//
//	@param chunk *dto.OpenAIChatCompletionChunk
//	@return bool
//	@author centonhuang
//	@update 2026-06-01 20:30:00
func HasNonEmptyDelta(chunk *dto.OpenAIChatCompletionChunk) bool {
	if chunk == nil || len(chunk.Choices) == 0 || chunk.Choices[0].Delta == nil {
		return false
	}
	d := chunk.Choices[0].Delta
	if d.Content != nil && *d.Content != "" {
		return true
	}
	if d.ReasoningContent != nil && *d.ReasoningContent != "" {
		return true
	}
	if len(d.ToolCalls) > 0 {
		return true
	}
	return false
}

// NormalizeOpenAIStreamToolCalls 规范化 OpenAI 流式工具调用增量。
//
//	@param chunk *dto.OpenAIChatCompletionChunk
//	@param toolCallIDs map[int]string
//	@author centonhuang
//	@update 2026-04-26 14:00:00
func NormalizeOpenAIStreamToolCalls(chunk *dto.OpenAIChatCompletionChunk, toolCallIDs map[int]string) {
	if chunk == nil {
		return
	}
	for _, choice := range chunk.Choices {
		if choice == nil || choice.Delta == nil {
			continue
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			if toolCall == nil {
				continue
			}
			if toolCall.Index == nil {
				toolCall.Index = &choice.Index
			}
			if lo.FromPtr(toolCall.ID) != "" {
				toolCallIDs[*toolCall.Index] = lo.FromPtr(toolCall.ID)
				continue
			}
			toolCall.ID = lo.ToPtr(toolCallIDs[*toolCall.Index])
		}
	}
}
