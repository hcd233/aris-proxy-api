package proxyutil

import (
	"context"
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

// presetEvent 预生成的 SSE 事件（event 为空表示 data-only 帧）。
type presetEvent struct {
	event string
	data  []byte
}

// presetStream 按预生成事件列表依次输出的 port.Stream 实现。
// 用于短路响应的 SSE 形态：不建立上游连接，Read 依序写入事件后返回 nil。
type presetStream struct {
	events []presetEvent
}

func (s *presetStream) Read(_ context.Context, sink port.EventSink) error {
	for _, e := range s.events {
		if err := sink.WriteEvent(e.event, e.data); err != nil {
			return err
		}
	}
	return nil
}

func (s *presetStream) Close() error { return nil }

// BuildOpenAIChatContentFilter 构造 OpenAI Chat 内容拦截消息（HTTP 200）。
//
// deny 敏感词命中时替代 403 返回协议原生 content_filter 形态：
// stream=true 返回 SSE 流（role → content → finish_reason=content_filter → [DONE]），
// 否则返回 JSON（choices[0].finish_reason=content_filter）。
//
//	@param model string 响应中暴露的模型名（请求模型）
//	@param stream bool 请求是否流式
//	@return port.Result
//	@author centonhuang
//	@update 2026-08-15 10:00:00
func BuildOpenAIChatContentFilter(model string, stream bool) port.Result {
	if stream {
		return buildOpenAIChatContentFilterStream(model)
	}
	return buildOpenAIChatContentFilterJSON(model)
}

func buildOpenAIChatContentFilterJSON(model string) *port.JSONResult {
	completion := &dto.OpenAIChatCompletion{
		ID:      fmt.Sprintf(constant.OpenAIChunkIDTemplate, uuid.New().String()),
		Object:  enum.CompletionObjectChatCompletion,
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []*dto.OpenAIChatCompletionChoice{{
			Index: 0,
			Message: &dto.OpenAIChatCompletionMessageParam{
				Role:    enum.RoleAssistant,
				Content: &dto.OpenAIMessageContent{Text: constant.TriggerContentFilterMessage},
			},
			FinishReason: enum.FinishReasonContentFilter,
		}},
		Usage: &dto.OpenAICompletionUsage{},
	}
	body := lo.Must1(sonic.Marshal(completion))
	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       body,
		Protocol:   enum.ProtocolKindOpenAI,
	}
}

func buildOpenAIChatContentFilterStream(model string) *port.StreamResult {
	chunkID := fmt.Sprintf(constant.OpenAIChunkIDTemplate, uuid.New().String())
	created := time.Now().Unix()
	base := func(choice *dto.OpenAIChatCompletionChunkChoice) *dto.OpenAIChatCompletionChunk {
		return &dto.OpenAIChatCompletionChunk{
			ID:      chunkID,
			Object:  enum.CompletionObjectChatCompletionChunk,
			Created: created,
			Model:   model,
			Choices: []*dto.OpenAIChatCompletionChunkChoice{choice},
		}
	}
	chunks := []*dto.OpenAIChatCompletionChunk{
		base(&dto.OpenAIChatCompletionChunkChoice{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{Role: enum.RoleAssistant},
		}),
		base(&dto.OpenAIChatCompletionChunkChoice{
			Index: 0,
			Delta: &dto.OpenAIChatCompletionChunkDelta{Content: lo.ToPtr(constant.TriggerContentFilterMessage)},
		}),
		base(&dto.OpenAIChatCompletionChunkChoice{
			Index:        0,
			Delta:        &dto.OpenAIChatCompletionChunkDelta{},
			FinishReason: lo.ToPtr(enum.FinishReasonContentFilter),
		}),
	}
	events := make([]presetEvent, 0, len(chunks)+1)
	for _, chunk := range chunks {
		events = append(events, presetEvent{data: lo.Must1(sonic.Marshal(chunk))})
	}
	events = append(events, presetEvent{data: []byte(constant.SSEDoneSignal)})
	return &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Open: func(context.Context) (port.Stream, error) {
			return &presetStream{events: events}, nil
		},
	}
}

// BuildOpenAIResponseContentFilter 构造 OpenAI Response API 内容拦截消息（HTTP 200）。
//
// deny 敏感词命中时替代 403 返回协议原生 content_filter 形态：
// stream=true 返回 SSE 流（created → output_item.added → content_part.added/done →
// output_item.done → completed），否则返回 JSON（output 含 refusal part、
// incomplete_details.reason=content_filter）。
//
//	@param model string 响应中暴露的模型名（请求模型）
//	@param stream bool 请求是否流式
//	@return port.Result
//	@author centonhuang
//	@update 2026-08-15 10:00:00
func BuildOpenAIResponseContentFilter(model string, stream bool) port.Result {
	if stream {
		return buildOpenAIResponseContentFilterStream(model)
	}
	return buildOpenAIResponseContentFilterJSON(model)
}

func buildOpenAIResponseContentFilterJSON(model string) *port.JSONResult {
	rsp := buildOpenAIResponseContentFilterRsp(model)
	body := lo.Must1(sonic.Marshal(rsp))
	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       body,
		Protocol:   enum.ProtocolKindOpenAI,
	}
}

// buildOpenAIResponseContentFilterRsp 构造 Response API 拦截响应对象。
// responseID 生成一次，供 item id（msg_{responseID}）复用。
func buildOpenAIResponseContentFilterRsp(model string) *dto.OpenAICreateResponseRsp {
	responseID := uuid.New().String()
	return &dto.OpenAICreateResponseRsp{
		ID:        fmt.Sprintf(constant.ResponseIDTemplate, responseID),
		Object:    enum.CompletionObjectResponse,
		CreatedAt: time.Now().Unix(),
		Status:    constant.ResponseStreamFieldStatusCompleted,
		Model:     model,
		Output: []*dto.ResponseInputItem{{
			Type:   lo.ToPtr(enum.ResponseInputItemTypeMessage),
			ID:     lo.ToPtr(fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)),
			Status: lo.ToPtr(constant.ResponseStreamFieldStatusCompleted),
			Role:   lo.ToPtr(enum.RoleAssistant),
			Content: &dto.ResponseInputMessageContent{Parts: []*dto.ResponseInputContent{{
				Type:    enum.ResponseContentTypeRefusal,
				Refusal: lo.ToPtr(constant.TriggerContentFilterMessage),
			}}},
		}},
		IncompleteDetails: &dto.ResponseIncomplete{Reason: enum.ResponseIncompleteReasonContentFilter},
		Usage:             &dto.ResponseUsage{},
	}
}

func buildOpenAIResponseContentFilterStream(model string) *port.StreamResult {
	rsp := buildOpenAIResponseContentFilterRsp(model)
	itemID := lo.FromPtr(rsp.Output[0].ID)

	// response.created 事件中的 response 对象 output 为空（与原生流一致）
	createdRsp := *rsp
	createdRsp.Output = nil

	refusalPart := map[string]any{
		constant.ResponseStreamFieldType:    enum.ResponseContentTypeRefusal,
		constant.ResponseStreamFieldRefusal: constant.TriggerContentFilterMessage,
	}
	item := map[string]any{
		constant.ResponseStreamFieldID:      itemID,
		constant.ResponseStreamFieldType:    constant.ResponseStreamFieldTypeValue,
		constant.ResponseStreamFieldStatus:  constant.ResponseStreamFieldStatusCompleted,
		constant.ResponseStreamFieldRole:    enum.RoleAssistant,
		constant.ResponseStreamFieldContent: []map[string]any{refusalPart},
	}

	events := []presetEvent{
		{event: enum.ResponseStreamEventCreated, data: lo.Must1(sonic.Marshal(&dto.ResponseStreamTerminalEvent{Type: enum.ResponseStreamEventCreated, Response: &createdRsp}))},
		{event: enum.ResponseStreamEventOutputItemAdded, data: lo.Must1(sonic.Marshal(map[string]any{
			constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemAdded,
			constant.ResponseStreamFieldOutputItem: 0,
			constant.ResponseStreamFieldItem: map[string]any{
				constant.ResponseStreamFieldID:      itemID,
				constant.ResponseStreamFieldType:    constant.ResponseStreamFieldTypeValue,
				constant.ResponseStreamFieldStatus:  constant.ResponseStreamFieldStatusInProgress,
				constant.ResponseStreamFieldRole:    enum.RoleAssistant,
				constant.ResponseStreamFieldContent: []any{},
			},
		}))},
		{event: enum.ResponseStreamEventContentPartAdded, data: lo.Must1(sonic.Marshal(map[string]any{
			constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartAdded,
			constant.ResponseStreamFieldItemID:       itemID,
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldPart:         refusalPart,
		}))},
		{event: enum.ResponseStreamEventContentPartDone, data: lo.Must1(sonic.Marshal(map[string]any{
			constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartDone,
			constant.ResponseStreamFieldItemID:       itemID,
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldPart:         refusalPart,
		}))},
		{event: enum.ResponseStreamEventOutputItemDone, data: lo.Must1(sonic.Marshal(map[string]any{
			constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemDone,
			constant.ResponseStreamFieldOutputItem: 0,
			constant.ResponseStreamFieldItem:       item,
		}))},
		{event: enum.ResponseStreamEventCompleted, data: lo.Must1(sonic.Marshal(&dto.ResponseStreamTerminalEvent{Type: enum.ResponseStreamEventCompleted, Response: rsp}))},
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindOpenAI,
		Open: func(context.Context) (port.Stream, error) {
			return &presetStream{events: events}, nil
		},
	}
}

// BuildAnthropicContentFilter 构造 Anthropic 内容拦截消息（HTTP 200）。
//
// deny 敏感词命中时替代 403 返回协议原生 refusal 形态：
// stream=true 返回 SSE 流（message_start → content_block_start/delta/stop →
// message_delta(stop_reason=refusal) → message_stop），否则返回 JSON
// （stop_reason=refusal + stop_details）。
//
//	@param model string 响应中暴露的模型名（请求模型）
//	@param stream bool 请求是否流式
//	@return port.Result
//	@author centonhuang
//	@update 2026-08-15 10:00:00
func BuildAnthropicContentFilter(model string, stream bool) port.Result {
	if stream {
		return buildAnthropicContentFilterStream(model)
	}
	return buildAnthropicContentFilterJSON(model)
}

// buildAnthropicContentFilterMessage 构造 Anthropic 拦截消息对象。
func buildAnthropicContentFilterMessage(model string) *dto.AnthropicMessage {
	return &dto.AnthropicMessage{
		ID:   fmt.Sprintf(constant.AnthropicMessageIDTemplate, uuid.New().String()),
		Type: constant.AnthropicMessageType,
		Role: enum.RoleAssistant,
		Content: []*dto.AnthropicContentBlock{{
			Type: enum.AnthropicContentBlockTypeText,
			Text: lo.ToPtr(constant.TriggerContentFilterMessage),
		}},
		Model:      model,
		StopReason: lo.ToPtr(enum.AnthropicStopReasonRefusal),
		StopDetails: &dto.AnthropicRefusalStopDetails{
			Type:        enum.AnthropicStopDetailsTypeRefusal,
			Explanation: lo.ToPtr(constant.TriggerContentFilterMessage),
		},
		Usage: &dto.AnthropicUsage{},
	}
}

func buildAnthropicContentFilterJSON(model string) *port.JSONResult {
	msg := buildAnthropicContentFilterMessage(model)
	body := lo.Must1(sonic.Marshal(msg))
	return &port.JSONResult{
		StatusCode: http.StatusOK,
		Headers:    map[string]string{constant.HTTPHeaderContentType: constant.HTTPContentTypeJSON},
		Body:       body,
		Protocol:   enum.ProtocolKindAnthropic,
	}
}

func buildAnthropicContentFilterStream(model string) *port.StreamResult {
	msg := buildAnthropicContentFilterMessage(model)
	textBlock := msg.Content[0]
	stopDetails := msg.StopDetails

	events := []presetEvent{
		{event: enum.AnthropicSSEEventTypeMessageStart, data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEMessageStart{Message: msg}))},
		{event: enum.AnthropicSSEEventTypeContentBlockStart, data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockStart{Index: 0, ContentBlock: textBlock}))},
		{event: enum.AnthropicSSEEventTypeContentBlockDelta, data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEContentBlockDelta{
			Index: 0,
			Delta: dto.AnthropicSSEContentBlockDeltaPayload{Type: enum.AnthropicDeltaTypeTextDelta, Text: constant.TriggerContentFilterMessage},
		}))},
		{event: enum.AnthropicSSEEventTypeContentBlockStop, data: lo.Must1(sonic.Marshal(map[string]any{
			constant.ResponseStreamFieldType:  enum.AnthropicSSEEventTypeContentBlockStop,
			constant.ResponseStreamFieldIndex: 0,
		}))},
		{event: enum.AnthropicSSEEventTypeMessageDelta, data: lo.Must1(sonic.Marshal(&dto.AnthropicSSEMessageDelta{
			Delta: dto.AnthropicSSEMessageDeltaPayload{
				StopReason:  lo.ToPtr(enum.AnthropicStopReasonRefusal),
				StopDetails: stopDetails,
			},
			Usage: &dto.AnthropicUsage{},
		}))},
		{event: enum.AnthropicSSEEventTypeMessageStop, data: []byte(constant.AnthropicMessageStopData)},
	}
	return &port.StreamResult{
		Protocol: enum.ProtocolKindAnthropic,
		Open: func(context.Context) (port.Stream, error) {
			return &presetStream{events: events}, nil
		},
	}
}
