package proxyutil

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// ==================== Capture 固定回复 ====================
//
// 触发词 capture 命中短路时不请求上游，按入口协议返回固定回复。
// stream=true 时以预生成事件序列经 presetStream 一次性写出，复用各协议
// 现有的 SSE 事件形态，保证 Claude Code / Codex 等流式客户端可正常解析。

func captureJSONHeaders() map[string]string {
	return map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON}
}

// ==================== Anthropic ====================

// NewAnthropicCaptureReply 构造 Anthropic 协议 capture 固定回复。
//
// stream=true 时输出完整 Anthropic SSE 事件序列（message_start →
// content_block_start → content_block_delta → content_block_stop →
// message_delta(end_turn) → message_stop）。
func NewAnthropicCaptureReply(reply, exposedModel string, stream bool) port.Result {
	msg := &dto.AnthropicMessage{
		ID:         fmt.Sprintf(constant.AnthropicMessageIDTemplate, uuid.NewString()),
		Type:       constant.AnthropicMessageType,
		Role:       enum.RoleAssistant,
		Model:      exposedModel,
		StopReason: lo.ToPtr(enum.AnthropicStopReasonEndTurn),
		Usage:      &dto.AnthropicUsage{},
		Content: []*dto.AnthropicContentBlock{{
			Type: enum.AnthropicContentBlockTypeText,
			Text: lo.ToPtr(reply),
		}},
	}
	if !stream {
		return &port.JSONResult{
			StatusCode: http.StatusOK,
			Headers:    captureJSONHeaders(),
			Body:       lo.Must1(sonic.Marshal(msg)),
			Protocol:   enum.ProtocolKindAnthropic,
		}
	}

	messageStart := lo.Must1(sonic.Marshal(&dto.AnthropicSSEMessageStart{
		Message: &dto.AnthropicMessage{
			ID:      msg.ID,
			Type:    msg.Type,
			Role:    msg.Role,
			Model:   msg.Model,
			Content: []*dto.AnthropicContentBlock{},
			Usage:   &dto.AnthropicUsage{},
		},
	}))
	blockStart := lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockStart{
		Index: 0,
		ContentBlock: &dto.AnthropicContentBlock{
			Type: enum.AnthropicContentBlockTypeText,
			Text: lo.ToPtr(""),
		},
	}))
	textDelta := lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
		Index: 0,
		Delta: dto.AnthropicSSEContentBlockDeltaPayload{
			Type: enum.AnthropicDeltaTypeTextDelta,
			Text: reply,
		},
	}))
	blockStop := []byte(constant.AnthropicContentBlockStopData)
	messageDelta := lo.Must1(sonic.Marshal(&dto.AnthropicSSEMessageDelta{
		Delta: dto.AnthropicSSEMessageDeltaPayload{StopReason: msg.StopReason},
		Usage: &dto.AnthropicUsage{},
	}))
	events := []presetEvent{
		{event: enum.AnthropicSSEEventTypeMessageStart, data: messageStart},
		{event: enum.AnthropicSSEEventTypeContentBlockStart, data: blockStart},
		{event: enum.AnthropicSSEEventTypeContentBlockDelta, data: textDelta},
		{event: enum.AnthropicSSEEventTypeContentBlockStop, data: blockStop},
		{event: enum.AnthropicSSEEventTypeMessageDelta, data: messageDelta},
		{event: enum.AnthropicSSEEventTypeMessageStop, data: []byte(constant.AnthropicMessageStopData)},
	}
	return presetStreamResult(events, enum.ProtocolKindAnthropic)
}

// ==================== OpenAI Chat Completion ====================

// NewOpenAIChatCaptureReply 构造 OpenAI Chat 协议 capture 固定回复。
//
// stream=true 时输出 role chunk → content chunk → finish chunk → [DONE]。
func NewOpenAIChatCaptureReply(reply, exposedModel string, stream bool) port.Result {
	id := fmt.Sprintf(constant.OpenAIChunkIDTemplate, uuid.NewString())
	created := time.Now().Unix()
	usage := &dto.OpenAICompletionUsage{}

	if !stream {
		completion := &dto.OpenAIChatCompletion{
			ID:      id,
			Object:  enum.CompletionObjectChatCompletion,
			Created: created,
			Model:   exposedModel,
			Choices: []*dto.OpenAIChatCompletionChoice{{
				Index:        0,
				FinishReason: enum.FinishReasonStop,
				Message: &dto.OpenAIChatCompletionMessageParam{
					Role:    enum.RoleAssistant,
					Content: &dto.OpenAIMessageContent{Text: reply},
				},
			}},
			Usage: usage,
		}
		return &port.JSONResult{
			StatusCode: http.StatusOK,
			Headers:    captureJSONHeaders(),
			Body:       lo.Must1(sonic.Marshal(completion)),
			Protocol:   enum.ProtocolKindOpenAI,
		}
	}

	marshalChunk := func(delta *dto.OpenAIChatCompletionChunkDelta, finish *enum.FinishReason) []byte {
		return lo.Must1(sonic.Marshal(&dto.OpenAIChatCompletionChunk{
			ID:      id,
			Object:  enum.CompletionObjectChatCompletionChunk,
			Created: created,
			Model:   exposedModel,
			Choices: []*dto.OpenAIChatCompletionChunkChoice{{
				Index:        0,
				Delta:        delta,
				FinishReason: finish,
			}},
		}))
	}
	roleChunk := marshalChunk(&dto.OpenAIChatCompletionChunkDelta{Role: enum.RoleAssistant}, nil)
	contentChunk := marshalChunk(&dto.OpenAIChatCompletionChunkDelta{Content: lo.ToPtr(reply)}, nil)
	finishChunk := marshalChunk(&dto.OpenAIChatCompletionChunkDelta{}, lo.ToPtr(enum.FinishReasonStop))
	events := []presetEvent{
		{data: roleChunk},
		{data: contentChunk},
		{data: finishChunk},
		{data: []byte(constant.SSEDoneSignal)},
	}
	return presetStreamResult(events, enum.ProtocolKindOpenAI)
}

// ==================== OpenAI Response API ====================

// NewOpenAIResponseCaptureReply 构造 OpenAI Response 协议 capture 固定回复。
//
// stream=true 时输出 response.created → output_item.added →
// content_part.added → output_text.delta → output_text.done →
// content_part.done → output_item.done → response.completed。
func NewOpenAIResponseCaptureReply(reply, exposedModel string, stream bool) port.Result {
	responseID := uuid.NewString()
	itemID := fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)
	rsp := &dto.OpenAICreateResponseRsp{
		ID:        fmt.Sprintf(constant.ResponseIDTemplate, responseID),
		Object:    constant.ResponseObjectValue,
		CreatedAt: time.Now().Unix(),
		Status:    enum.ResponseStatusCompleted,
		Model:     exposedModel,
		Output: []*dto.ResponseInputItem{{
			Type:   lo.ToPtr(enum.ResponseInputItemTypeMessage),
			ID:     lo.ToPtr(itemID),
			Status: lo.ToPtr(constant.ResponseStreamFieldStatusCompleted),
			Role:   lo.ToPtr(enum.RoleAssistant),
			Content: &dto.ResponseInputMessageContent{
				Parts: []*dto.ResponseInputContent{{
					Type: constant.ResponseStreamFieldOutputTextType,
					Text: lo.ToPtr(reply),
				}},
			},
		}},
		Usage: &dto.ResponseUsage{},
	}
	if !stream {
		return &port.JSONResult{
			StatusCode: http.StatusOK,
			Headers:    captureJSONHeaders(),
			Body:       lo.Must1(sonic.Marshal(rsp)),
			Protocol:   enum.ProtocolKindOpenAI,
		}
	}

	marshalEvent := func(payload map[string]any) []byte {
		return lo.Must1(sonic.Marshal(payload))
	}
	outputItem := map[string]any{
		constant.ResponseStreamFieldID:     itemID,
		constant.ResponseStreamFieldType:   constant.ResponseStreamFieldTypeValue,
		constant.ResponseStreamFieldStatus: constant.ResponseStreamFieldStatusCompleted,
		constant.ResponseStreamFieldRole:   enum.RoleAssistant,
		constant.ResponseStreamFieldContent: []map[string]any{{
			constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
			constant.ResponseStreamFieldText:        reply,
			constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
		}},
	}
	events := []presetEvent{
		{event: enum.ResponseStreamEventCreated, data: marshalEvent(map[string]any{
			constant.ResponseStreamFieldType: enum.ResponseStreamEventCreated,
			constant.ResponseStreamFieldResponse: map[string]any{
				constant.ResponseStreamFieldID:     rsp.ID,
				constant.ResponseStreamFieldType:   constant.ResponseObjectValue,
				constant.ResponseStreamFieldStatus: enum.ResponseStatusInProgress,
			},
		})},
		{event: enum.ResponseStreamEventOutputItemAdded, data: marshalEvent(map[string]any{
			constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemAdded,
			constant.ResponseStreamFieldOutputItem: 0,
			constant.ResponseStreamFieldItem: map[string]any{
				constant.ResponseStreamFieldID:      itemID,
				constant.ResponseStreamFieldType:    constant.ResponseStreamFieldTypeValue,
				constant.ResponseStreamFieldStatus:  constant.ResponseStreamFieldStatusInProgress,
				constant.ResponseStreamFieldRole:    enum.RoleAssistant,
				constant.ResponseStreamFieldContent: []any{},
			},
		})},
		{event: enum.ResponseStreamEventContentPartAdded, data: marshalEvent(map[string]any{
			constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartAdded,
			constant.ResponseStreamFieldItemID:       itemID,
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldPart: map[string]any{
				constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
				constant.ResponseStreamFieldText:        "",
				constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
			},
		})},
		{event: enum.ResponseStreamEventOutputTextDelta, data: marshalEvent(map[string]any{
			constant.ResponseStreamFieldType:         enum.ResponseStreamEventOutputTextDelta,
			constant.ResponseStreamFieldItemID:       itemID,
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldDelta:        reply,
		})},
		{event: enum.ResponseStreamEventOutputTextDone, data: marshalEvent(map[string]any{
			constant.ResponseStreamFieldType:         enum.ResponseStreamEventOutputTextDone,
			constant.ResponseStreamFieldItemID:       itemID,
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldText:         reply,
		})},
		{event: enum.ResponseStreamEventContentPartDone, data: marshalEvent(map[string]any{
			constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartDone,
			constant.ResponseStreamFieldItemID:       itemID,
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldPart: map[string]any{
				constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
				constant.ResponseStreamFieldText:        reply,
				constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
			},
		})},
		{event: enum.ResponseStreamEventOutputItemDone, data: marshalEvent(map[string]any{
			constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemDone,
			constant.ResponseStreamFieldOutputItem: 0,
			constant.ResponseStreamFieldItem:       outputItem,
		})},
		{event: enum.ResponseStreamEventCompleted, data: lo.Must1(sonic.Marshal(
			&dto.ResponseStreamTerminalEvent{Type: enum.ResponseStreamEventCompleted, Response: rsp},
		))},
	}
	return presetStreamResult(events, enum.ProtocolKindOpenAI)
}
