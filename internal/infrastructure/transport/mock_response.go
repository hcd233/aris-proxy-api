package transport

import (
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// MockResponseEvent 一个 Responses / Anthropic 的 SSE 事件（事件名 + JSON 负载）。
type MockResponseEvent struct {
	Event string
	Data  []byte
}

// buildResponseItem 构造输出项：一条完成的 assistant 文本消息。
// content 用 output_text 数组形态（与真协议及本仓 fixture 一致）；字符串形态虽被
// ResponseInputMessageContent.UnmarshalJSON 兼容，但严格 SDK 无法解析。
func buildResponseItem(itemID string, spec MockSpec) *dto.ResponseInputItem {
	msgType, role, status := enum.ResponseInputItemTypeMessage, enum.RoleAssistant, enum.ResponseStatusCompleted
	text := string(spec.Chars)
	return &dto.ResponseInputItem{
		Type:   &msgType,
		ID:     &itemID,
		Role:   &role,
		Status: &status,
		Content: &dto.ResponseInputMessageContent{
			Parts: []*dto.ResponseInputContent{{Type: enum.ResponseContentTypeOutputText, Text: &text}},
		},
	}
}

// buildOutputItemAddedPayload 构造 output_item.added 负载：item 为 in_progress 占位
// （content 为空数组，正文经后续 delta 到达），与真协议一致（converter 的
// writeOutputItemAddedEvent 同形）。
//
// 用 map 而非 DTO 的原因：ResponseInputMessageContent.MarshalJSON 只在 Parts 非空时
// 序列化成数组，空 Parts 会退化为字符串 ""；负载带 type 判别字段（严格 SDK 据此分发）。
func buildOutputItemAddedPayload(itemID string) map[string]any {
	return map[string]any{
		constant.ResponseStreamFieldType:       enum.ResponseStreamEventOutputItemAdded,
		constant.ResponseStreamFieldOutputItem: 0,
		constant.ResponseStreamFieldItem: map[string]any{
			constant.ResponseStreamFieldID:      itemID,
			constant.ResponseStreamFieldType:    enum.ResponseInputItemTypeMessage,
			constant.ResponseStreamFieldStatus:  enum.ResponseStatusInProgress,
			constant.ResponseStreamFieldRole:    enum.RoleAssistant,
			constant.ResponseStreamFieldContent: []any{},
		},
	}
}

// ResponseStreamLen 流式事件总数：created + output_item.added + N×delta +
// output_item.done + completed。
func ResponseStreamLen(spec MockSpec) int { return spec.ChunkCount() + 4 }

// BuildResponseEventAt 按需构造第 i 个流式事件（i ∈ [0, ResponseStreamLen)）：
// created → output_item.added → N×output_text.delta → output_item.done → completed。
// itemID 由调用方生成并传入，保证 added/delta/done 三条事件归属同一 item；
// output_item.done 与 completed 承载完整 item/response，是存储链路的内容来源
// （见 responseNativeStream.handleOutputItemDone / handleTerminalEvent）。
// 逐事件按需构造，不预物化整条流（16K 字符上限 = 4096 个 delta）。
func BuildResponseEventAt(spec MockSpec, model, responseID, itemID string, i int) MockResponseEvent {
	chunkCount := spec.ChunkCount()
	switch {
	case i == 0:
		return mockEvent(enum.ResponseStreamEventCreated, map[string]any{
			constant.ResponseStreamFieldType:   enum.ResponseStreamEventCreated,
			constant.ResponseStreamFieldID:     responseID,
			constant.ResponseStreamFieldObject: enum.CompletionObjectResponse,
			constant.ResponseStreamFieldStatus: enum.ResponseStatusInProgress,
			constant.ResponseStreamFieldModel:  model,
		})
	case i == 1:
		return mockEvent(enum.ResponseStreamEventOutputItemAdded, buildOutputItemAddedPayload(itemID))
	case i < chunkCount+2:
		return mockEvent(enum.ResponseStreamEventOutputTextDelta, map[string]any{
			constant.ResponseStreamFieldType:         enum.ResponseStreamEventOutputTextDelta,
			constant.ResponseStreamFieldItemID:       itemID,
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldDelta:        spec.ChunkAt(i - 2),
		})
	case i == chunkCount+2:
		return mockEvent(enum.ResponseStreamEventOutputItemDone, &dto.ResponseStreamOutputItemDoneEvent{
			Type:        enum.ResponseStreamEventOutputItemDone,
			OutputIndex: 0,
			Item:        buildResponseItem(itemID, spec),
		})
	default:
		return mockEvent(enum.ResponseStreamEventCompleted, &dto.ResponseStreamTerminalEvent{
			Type:     enum.ResponseStreamEventCompleted,
			Response: BuildResponse(spec, model),
		})
	}
}

// BuildResponse 生成非流式 Responses 响应。
func BuildResponse(spec MockSpec, model string) *dto.OpenAICreateResponseRsp {
	return &dto.OpenAICreateResponseRsp{
		ID:        fmt.Sprintf(constant.ResponseIDTemplate, strconv.FormatInt(time.Now().UnixNano(), 10)),
		Object:    enum.CompletionObjectResponse,
		CreatedAt: time.Now().Unix(),
		Status:    enum.ResponseStatusCompleted,
		Model:     model,
		Output:    []*dto.ResponseInputItem{buildResponseItem(mockID(constant.ResponseItemIDTemplate), spec)},
		Usage:     buildResponseUsage(spec),
	}
}

// buildResponseUsage 按 1 token ≈ 4 字符口径估算 usage；输入维由调用方按请求 body 填入 Spec.InputTokens。
func buildResponseUsage(spec MockSpec) *dto.ResponseUsage {
	out := spec.Tokens()
	return &dto.ResponseUsage{
		InputTokens:  spec.InputTokens,
		OutputTokens: out,
		TotalTokens:  spec.InputTokens + out,
	}
}

// mockEvent 把事件负载序列化为 SSE data 字段。
func mockEvent(event string, payload any) MockResponseEvent {
	data, err := sonic.Marshal(payload)
	if err != nil {
		panic(err) // 负载为本包内构造的结构体/字面量，序列化失败属编程错误
	}
	return MockResponseEvent{Event: event, Data: data}
}
