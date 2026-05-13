package util

import (
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

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
