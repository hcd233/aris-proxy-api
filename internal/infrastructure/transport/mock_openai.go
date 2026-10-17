package transport

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ChatStreamLen 流式 chunk 总数：N 个内容片 + 1 个收尾片（finish_reason + usage）。
func ChatStreamLen(spec MockSpec) int { return spec.ChunkCount() + 1 }

// BuildChatChunkAt 按需构造第 i 个 chunk（i ∈ [0, ChatStreamLen)）：
// 内容片带 delta.content（首片带 role），收尾片带 finish_reason 与 usage。
//
// created 由调用方统一传入，保证同一响应所有 chunk 的时间戳一致；逐片按需构造
// 而非预生成切片，避免每流持有全部 chunk（16K 字符上限 = 4096 片）。
func BuildChatChunkAt(spec MockSpec, model, chunkID string, created int64, i int) *dto.OpenAIChatCompletionChunk {
	chunk := &dto.OpenAIChatCompletionChunk{
		ID:      chunkID,
		Created: created,
		Model:   model,
		Object:  enum.CompletionObjectChatCompletionChunk,
	}
	if i >= spec.ChunkCount() {
		stop := enum.FinishReasonStop
		chunk.Choices = []*dto.OpenAIChatCompletionChunkChoice{{Index: 0, FinishReason: &stop}}
		chunk.Usage = BuildChatUsage(spec)
		return chunk
	}
	piece := spec.ChunkAt(i)
	delta := &dto.OpenAIChatCompletionChunkDelta{Content: &piece}
	if i == 0 {
		delta.Role = enum.RoleAssistant
	}
	chunk.Choices = []*dto.OpenAIChatCompletionChunkChoice{{Index: 0, Delta: delta}}
	return chunk
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

// BuildChatUsage 按 1 token ≈ 4 字符口径估算 usage；输入维由调用方按请求 body 填入 Spec.InputTokens。
func BuildChatUsage(spec MockSpec) *dto.OpenAICompletionUsage {
	out := spec.Tokens()
	return &dto.OpenAICompletionUsage{
		PromptTokens:     spec.InputTokens,
		CompletionTokens: out,
		TotalTokens:      spec.InputTokens + out,
	}
}
