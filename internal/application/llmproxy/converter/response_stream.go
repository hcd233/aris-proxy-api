package converter

import (
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// StreamItemState 流式 output item 状态跟踪。
// 跟踪 message output item 和 tool call output item 的初始化状态，
// 确保每个独立 item 只发一次 output_item.added 事件。
type StreamItemState struct {
	initializedMessages  map[int]bool
	initializedToolCalls map[string]bool
	toolCallIndexToID    map[int]string
}

// NewStreamItemState 创建流式 item 状态跟踪器。
func NewStreamItemState() *StreamItemState {
	return &StreamItemState{
		initializedMessages:  make(map[int]bool),
		initializedToolCalls: make(map[string]bool),
		toolCallIndexToID:    make(map[int]string),
	}
}

// WriteResponseDeltaFromChatChunk 将 ChatCompletion 流式 chunk 转换为 Response API SSE 事件。
//
// 对文本和推理内容，使用 choice.Index 对应的 message output item。
// 对工具调用，为每个工具调用创建独立的 output item（function_call / custom_tool_call），
// 发送 output_item.added 携带函数名和 call_id，然后发送 arguments.delta / input.delta。
func WriteResponseDeltaFromChatChunk(sink port.EventSink, chunk *dto.OpenAIChatCompletionChunk, state *StreamItemState, responseID string, conv *ResponseProtocolConverter) (bool, error) {
	if chunk == nil {
		return false, nil
	}
	wroteDelta := false
	for _, choice := range chunk.Choices {
		if choice == nil || choice.Delta == nil {
			continue
		}
		delta := choice.Delta
		messageItemID := fmt.Sprintf(constant.ResponseItemIDTemplate, responseID)
		outputIndex := choice.Index

		// 文本 / 推理内容：使用 message output item
		if hasTextOrReasoning(delta) {
			if err := initMessageOutputItem(sink, state, messageItemID, outputIndex); err != nil {
				return wroteDelta, err
			}
		}

		wrote, err := writeDeltaField(sink, enum.ResponseStreamEventOutputTextDelta, delta.Content, messageItemID, outputIndex, 0)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote

		wrote, err = writeDeltaField(sink, enum.ResponseStreamEventReasoningTextDelta, delta.ReasoningContent, messageItemID, outputIndex, 0)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote

		// 工具调用：每个 tool call 使用独立的 output item
		wrote, err = writeToolCallDeltas(sink, delta.ToolCalls, state, conv)
		if err != nil {
			return wroteDelta || wrote, err
		}
		wroteDelta = wroteDelta || wrote
	}
	return wroteDelta, nil
}

// FinalizeResponseFromChatCompletion 在流式结束时，
// 将最终的 ChatCompletion 转换为 Response API 响应，
// 并发送所有 output item 的 done 事件 + response.completed 终态事件。
func FinalizeResponseFromChatCompletion(sink port.EventSink, completion *dto.OpenAIChatCompletion, exposedModel, responseID string, conv *ResponseProtocolConverter) *dto.OpenAICreateResponseRsp {
	if completion == nil {
		return nil
	}
	completion.Model = exposedModel
	rsp, _ := conv.ToResponseResponse(completion) //nolint:errcheck // best-effort conversion, don't block stream finalize
	if rsp == nil {
		return nil
	}
	rsp.ID = responseID

	for _, item := range rsp.Output {
		if item == nil || item.Type == nil {
			continue
		}
		itemType := *item.Type
		ensureItemID(item, itemType, responseID)
		switch itemType {
		case enum.ResponseInputItemTypeMessage:
			writeMessageOutputItemDone(sink, item)
		case enum.ResponseInputItemTypeFunctionCall, enum.ResponseInputItemTypeCustomToolCall,
			enum.ResponseInputItemTypeLocalShellCall:
			writeToolCallOutputItemDone(sink, item, itemType)
		case enum.ResponseInputItemTypeReasoning:
			writeReasoningOutputItemDone(sink, item)
		}
	}

	_ = writeResponseTerminalEvent(sink, enum.ResponseStreamEventCompleted, rsp) //nolint:errcheck // best-effort write on stream close
	return rsp
}

// hasTextOrReasoning 判断 delta 是否包含文本或推理内容
func hasTextOrReasoning(delta *dto.OpenAIChatCompletionChunkDelta) bool {
	if delta == nil {
		return false
	}
	if v := lo.FromPtr(delta.Content); v != "" {
		return true
	}
	if v := lo.FromPtr(delta.ReasoningContent); v != "" {
		return true
	}
	return false
}

// initMessageOutputItem 初始化 message output item（发送 output_item.added + content_part.added）
func initMessageOutputItem(sink port.EventSink, state *StreamItemState, itemID string, outputIndex int) error {
	if state.initializedMessages[outputIndex] {
		return nil
	}
	state.initializedMessages[outputIndex] = true
	if err := writeOutputItemAddedEvent(sink, itemID, outputIndex, constant.ResponseStreamFieldTypeValue); err != nil {
		return err
	}
	return writeContentPartAddedEvent(sink, itemID, outputIndex)
}

// toolCallItemID 生成工具调用 output item 的 ID
func toolCallItemID(callID string) string {
	id := lo.FromPtr(&callID)
	if id == "" {
		id = "call_" + uuid.New().String()
	}
	return "fc_" + id
}

// ensureItemID 为 output item 设置 ID（如果缺失）
// message item 用 msg_{responseID}，tool call item 用 fc_{callID}
func ensureItemID(item *dto.ResponseInputItem, itemType, responseID string) {
	if item.ID != nil && *item.ID != "" {
		return
	}
	switch itemType {
	case enum.ResponseInputItemTypeFunctionCall, enum.ResponseInputItemTypeCustomToolCall,
		enum.ResponseInputItemTypeLocalShellCall:
		callID := lo.FromPtr(item.CallID)
		item.ID = lo.ToPtr(toolCallItemID(callID))
	case enum.ResponseInputItemTypeMessage:
		item.ID = lo.ToPtr(fmt.Sprintf(constant.ResponseItemIDTemplate, responseID))
	default:
		item.ID = lo.ToPtr(fmt.Sprintf(constant.ResponseItemIDTemplate, responseID))
	}
}

// writeToolCallDeltas 处理工具调用 delta：
// - 首次出现时发送 output_item.added（携带 type/call_id/name）
// - 发送 arguments.delta（function）或 input.delta（custom）
func writeToolCallDeltas(sink port.EventSink, toolCalls []*dto.OpenAIChatCompletionMessageToolCall, state *StreamItemState, conv *ResponseProtocolConverter) (bool, error) {
	wrote := false
	for _, tc := range toolCalls {
		if tc == nil {
			continue
		}
		info := extractToolCallInfo(tc)
		if info == nil {
			continue
		}

		itemID, ok := resolveToolCallItemID(state, info, lo.FromPtr(tc.Index))
		if !ok {
			continue
		}

		if !state.initializedToolCalls[itemID] {
			state.initializedToolCalls[itemID] = true
			itemType := resolveToolCallOutputType(info.name, conv.ToolTypeMap())
			outputIndex := len(state.initializedToolCalls)
			name, ns := splitNamespacedName(info.name, conv.NamespaceMap())
			if err := writeToolCallOutputItemAdded(sink, itemID, outputIndex, itemType, info.callID, name, ns); err != nil {
				return wrote, err
			}
			wrote = true
		}

		w2, err := writeToolCallArgumentDelta(sink, tc, info, itemID)
		if err != nil {
			return wrote || w2, err
		}
		wrote = wrote || w2
	}
	return wrote, nil
}

type toolCallInfo struct {
	name     string
	callID   string
	isCustom bool
}

func extractToolCallInfo(tc *dto.OpenAIChatCompletionMessageToolCall) *toolCallInfo {
	switch {
	case tc.Function != nil:
		return &toolCallInfo{name: tc.Function.Name, callID: lo.FromPtr(tc.ID)}
	case tc.Custom != nil:
		return &toolCallInfo{name: tc.Custom.Name, callID: lo.FromPtr(tc.ID), isCustom: true}
	}
	return nil
}

func resolveToolCallItemID(state *StreamItemState, info *toolCallInfo, tcIdx int) (string, bool) {
	if existingID, ok := state.toolCallIndexToID[tcIdx]; ok {
		return existingID, true
	}
	if info.callID == "" && info.name == "" {
		return "", false
	}
	itemID := toolCallItemID(info.callID)
	state.toolCallIndexToID[tcIdx] = itemID
	return itemID, true
}

func writeToolCallArgumentDelta(sink port.EventSink, tc *dto.OpenAIChatCompletionMessageToolCall, info *toolCallInfo, itemID string) (bool, error) {
	if info.isCustom && tc.Custom != nil && tc.Custom.Input != "" {
		if err := writeResponseDeltaEvent(sink, enum.ResponseStreamEventCustomToolCallInputDelta, tc.Custom.Input, itemID, 0, 0); err != nil {
			return true, err
		}
		return true, nil
	}
	if !info.isCustom && tc.Function != nil && tc.Function.Arguments != "" {
		if err := writeResponseDeltaEvent(sink, enum.ResponseStreamEventFunctionCallArgumentsDelta, tc.Function.Arguments, itemID, 0, 0); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

// writeDeltaField 写入增量事件（值为空时跳过）
func writeDeltaField(sink port.EventSink, event enum.ResponseStreamEventType, value *string, itemID string, outputIndex, contentIndex int) (bool, error) {
	if value == nil || *value == "" {
		return false, nil
	}
	if err := writeResponseDeltaEvent(sink, event, *value, itemID, outputIndex, contentIndex); err != nil {
		return false, err
	}
	return true, nil
}

// writeResponseDeltaEvent 写入增量 SSE 事件
func writeResponseDeltaEvent(sink port.EventSink, event enum.ResponseStreamEventType, delta, itemID string, outputIndex, contentIndex int) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         event,
		constant.ResponseStreamFieldDelta:        delta,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: contentIndex,
	}))
	return sink.WriteEvent(event, payload)
}

// writeOutputItemAddedEvent 写入 message 类型的 output_item.added 事件
func writeOutputItemAddedEvent(sink port.EventSink, itemID string, outputIndex int, itemType string) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemAdded,
		constant.ResponseStreamFieldOutputItem: outputIndex,
		constant.ResponseStreamFieldItem: map[string]any{
			constant.ResponseStreamFieldID:      itemID,
			constant.ResponseStreamFieldType:    itemType,
			constant.ResponseStreamFieldStatus:  constant.ResponseStreamFieldStatusInProgress,
			constant.ResponseStreamFieldRole:    enum.RoleAssistant,
			constant.ResponseStreamFieldContent: []any{},
		},
	}))
	return sink.WriteEvent(enum.ResponseStreamEventOutputItemAdded, payload)
}

// writeToolCallOutputItemAdded 写入工具调用的 output_item.added 事件
func writeToolCallOutputItemAdded(sink port.EventSink, itemID string, outputIndex int, itemType, callID, name, namespace string) error {
	item := map[string]any{
		constant.ResponseStreamFieldID:     itemID,
		constant.ResponseStreamFieldType:   itemType,
		constant.ResponseStreamFieldStatus: constant.ResponseStreamFieldStatusInProgress,
	}
	if callID != "" {
		item[constant.ResponseStreamFieldCallID] = callID
	}
	if name != "" {
		item[constant.ResponseStreamFieldName] = name
	}
	if namespace != "" {
		item[constant.ResponseStreamFieldNamespace] = namespace
	}
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemAdded,
		constant.ResponseStreamFieldOutputItem: outputIndex,
		constant.ResponseStreamFieldItem:       item,
	}))
	return sink.WriteEvent(enum.ResponseStreamEventOutputItemAdded, payload)
}

// writeContentPartAddedEvent 写入 content_part.added 事件
func writeContentPartAddedEvent(sink port.EventSink, itemID string, outputIndex int) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartAdded,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: 0,
		constant.ResponseStreamFieldPart: map[string]any{
			constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
			constant.ResponseStreamFieldText:        "",
			constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
		},
	}))
	return sink.WriteEvent(enum.ResponseStreamEventContentPartAdded, payload)
}

// writeMessageOutputItemDone 写入 message 的 done 事件序列
func writeMessageOutputItemDone(sink port.EventSink, item *dto.ResponseInputItem) {
	if item.Content == nil {
		return
	}
	text := item.Content.Text
	if len(item.Content.Parts) > 0 {
		texts := lo.FilterMap(item.Content.Parts, func(p *dto.ResponseInputContent, _ int) (string, bool) {
			if p == nil || p.Text == nil {
				return "", false
			}
			return *p.Text, true
		})
		text = strings.Join(texts, "")
	}

	itemID := lo.FromPtr(item.ID)
	outputIndex := 0

	_ = writeOutputTextDoneEvent(sink, itemID, outputIndex, text)  //nolint:errcheck // best-effort write on stream close
	_ = writeContentPartDoneEvent(sink, itemID, outputIndex, text) //nolint:errcheck // best-effort write on stream close
	content := []map[string]any{{
		constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
		constant.ResponseStreamFieldText:        text,
		constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
	}}
	_ = writeOutputItemDoneEvent(sink, itemID, outputIndex, content) //nolint:errcheck // best-effort write on stream close
}

// writeToolCallOutputItemDone 写入工具调用的 output_item.done 事件
func writeToolCallOutputItemDone(sink port.EventSink, item *dto.ResponseInputItem, itemType string) {
	itemID := lo.FromPtr(item.ID)
	outputIndex := 0

	doneItem := map[string]any{
		constant.ResponseStreamFieldID:     itemID,
		constant.ResponseStreamFieldType:   itemType,
		constant.ResponseStreamFieldStatus: constant.ResponseStreamFieldStatusCompleted,
	}
	if item.CallID != nil {
		doneItem[constant.ResponseStreamFieldCallID] = *item.CallID
	}
	if item.Name != nil {
		doneItem[constant.ResponseStreamFieldName] = *item.Name
	}
	if item.Namespace != nil && *item.Namespace != "" {
		doneItem[constant.ResponseStreamFieldNamespace] = *item.Namespace
	}
	if item.Arguments != nil {
		doneItem[constant.ResponseStreamFieldArguments] = *item.Arguments
	}
	if item.Input != nil {
		doneItem[constant.ResponseStreamFieldInput] = *item.Input
	}

	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemDone,
		constant.ResponseStreamFieldOutputItem: outputIndex,
		constant.ResponseStreamFieldItem:       doneItem,
	}))
	_ = sink.WriteEvent(enum.ResponseStreamEventOutputItemDone, payload) //nolint:errcheck // best-effort write on stream close
}

// writeReasoningOutputItemDone 写入推理内容的 output_item.done 事件
func writeReasoningOutputItemDone(sink port.EventSink, item *dto.ResponseInputItem) {
	itemID := lo.FromPtr(item.ID)
	outputIndex := 0

	summaryTexts := lo.FilterMap(lo.FromPtr(item.Summary), func(s *dto.ResponseReasoningSummary, _ int) (string, bool) {
		if s == nil || s.Text == "" {
			return "", false
		}
		return s.Text, true
	})

	doneItem := map[string]any{
		constant.ResponseStreamFieldID:     itemID,
		constant.ResponseStreamFieldType:   enum.ResponseInputItemTypeReasoning,
		constant.ResponseStreamFieldStatus: constant.ResponseStreamFieldStatusCompleted,
		constant.ResponseStreamFieldSummary: lo.Map(lo.FromPtr(item.Summary), func(s *dto.ResponseReasoningSummary, _ int) map[string]any {
			if s == nil {
				return nil
			}
			return map[string]any{
				constant.ResponseStreamFieldType: s.Type,
				constant.ResponseStreamFieldText: s.Text,
			}
		}),
	}
	_ = summaryTexts

	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemDone,
		constant.ResponseStreamFieldOutputItem: outputIndex,
		constant.ResponseStreamFieldItem:       doneItem,
	}))
	_ = sink.WriteEvent(enum.ResponseStreamEventOutputItemDone, payload) //nolint:errcheck // best-effort write on stream close
}

// writeOutputTextDoneEvent 写入 output_text.done 事件
func writeOutputTextDoneEvent(sink port.EventSink, itemID string, outputIndex int, text string) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         enum.ResponseStreamEventOutputTextDone,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: 0,
		constant.ResponseStreamFieldText:         text,
	}))
	return sink.WriteEvent(enum.ResponseStreamEventOutputTextDone, payload)
}

// writeContentPartDoneEvent 写入 content_part.done 事件
func writeContentPartDoneEvent(sink port.EventSink, itemID string, outputIndex int, text string) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:         enum.ResponseStreamEventContentPartDone,
		constant.ResponseStreamFieldItemID:       itemID,
		constant.ResponseStreamFieldOutputIndex:  outputIndex,
		constant.ResponseStreamFieldContentIndex: 0,
		constant.ResponseStreamFieldPart: map[string]any{
			constant.ResponseStreamFieldType:        constant.ResponseStreamFieldOutputTextType,
			constant.ResponseStreamFieldText:        text,
			constant.ResponseStreamFieldAnnotations: constant.ResponseStreamFieldAnnotationsEmpty,
		},
	}))
	return sink.WriteEvent(enum.ResponseStreamEventContentPartDone, payload)
}

// writeOutputItemDoneEvent 写入 message 类型的 output_item.done 事件
func writeOutputItemDoneEvent(sink port.EventSink, itemID string, outputIndex int, content []map[string]any) error {
	payload := lo.Must1(sonic.Marshal(map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemDone,
		constant.ResponseStreamFieldOutputItem: outputIndex,
		constant.ResponseStreamFieldItem: map[string]any{
			constant.ResponseStreamFieldID:      itemID,
			constant.ResponseStreamFieldType:    constant.ResponseStreamFieldTypeValue,
			constant.ResponseStreamFieldStatus:  constant.ResponseStreamFieldStatusCompleted,
			constant.ResponseStreamFieldRole:    enum.RoleAssistant,
			constant.ResponseStreamFieldContent: content,
		},
	}))
	return sink.WriteEvent(enum.ResponseStreamEventOutputItemDone, payload)
}

// writeResponseTerminalEvent 写入终态事件
func writeResponseTerminalEvent(sink port.EventSink, event enum.ResponseStreamEventType, rsp *dto.OpenAICreateResponseRsp) error {
	payload := lo.Must1(sonic.Marshal(&dto.ResponseStreamTerminalEvent{Type: event, Response: rsp}))
	return sink.WriteEvent(event, payload)
}
