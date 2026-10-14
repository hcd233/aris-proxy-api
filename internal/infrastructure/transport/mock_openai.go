package transport

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// BuildChatStream 生成 Chat Completions 流式 chunk 序列（不含时序等待）。
// 结构：首个 chunk 带 role，中间片带 content，末片带 finish_reason + usage。
func BuildChatStream(spec MockSpec, model, chunkID string) []*dto.OpenAIChatCompletionChunk {
	pieces := spec.Chunks()
	created := time.Now().Unix()
	out := make([]*dto.OpenAIChatCompletionChunk, 0, len(pieces)+1)

	for i, piece := range pieces {
		content := piece
		delta := &dto.OpenAIChatCompletionChunkDelta{Content: &content}
		if i == 0 {
			delta.Role = enum.RoleAssistant
		}
		out = append(out, &dto.OpenAIChatCompletionChunk{
			ID:      chunkID,
			Created: created,
			Model:   model,
			Object:  enum.CompletionObjectChatCompletionChunk,
			Choices: []*dto.OpenAIChatCompletionChunkChoice{{Index: 0, Delta: delta}},
		})
	}

	stop := enum.FinishReasonStop
	out = append(out, &dto.OpenAIChatCompletionChunk{
		ID:      chunkID,
		Created: created,
		Model:   model,
		Object:  enum.CompletionObjectChatCompletionChunk,
		Choices: []*dto.OpenAIChatCompletionChunkChoice{{Index: 0, FinishReason: &stop}},
		Usage:   BuildChatUsage(spec),
	})
	return out
}

// BuildChatCompletion 生成非流式 Chat Completions 响应。
func BuildChatCompletion(spec MockSpec, model string) *dto.OpenAIChatCompletion {
	content := string(spec.Chars)
	return &dto.OpenAIChatCompletion{
		ID:      fmt.Sprintf(constant.OpenAIChunkIDTemplate, strconv.FormatInt(time.Now().UnixNano(), 10)),
		Created: time.Now().Unix(),
		Model:   model,
		Object:  enum.CompletionObjectChatCompletion,
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index:        0,
			Message:      &dto.OpenAIChatCompletionMessageParam{Role: enum.RoleAssistant, Content: &dto.OpenAIMessageContent{Text: content}},
			FinishReason: enum.FinishReasonStop,
		}},
		Usage: BuildChatUsage(spec),
	}
}

// BuildChatUsage 按 1 token ≈ 4 字符口径估算 usage（input 计 0，由调用方按请求文本回填）。
func BuildChatUsage(spec MockSpec) *dto.OpenAICompletionUsage {
	out := spec.Tokens()
	return &dto.OpenAICompletionUsage{CompletionTokens: out, TotalTokens: out}
}
