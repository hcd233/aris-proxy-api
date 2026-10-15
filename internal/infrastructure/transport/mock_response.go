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

// buildResponseItem 构造 mock 输出项：一条 assistant 文本消息。
func buildResponseItem(spec MockSpec) *dto.ResponseInputItem {
	msgType, role, status := enum.ResponseInputItemTypeMessage, enum.RoleAssistant, enum.ResponseStatusCompleted
	return &dto.ResponseInputItem{
		Type:    &msgType,
		Role:    &role,
		Status:  &status,
		Content: &dto.ResponseInputMessageContent{Text: string(spec.Chars)},
	}
}

// BuildResponseStream 生成 Responses API 流式事件序列：
// created → output_item.added → N×output_text.delta → output_item.done → completed。
// 其中 output_item.done 与 completed 承载完整 item/response，是存储链路的内容来源
// （见 responseNativeStream.handleOutputItemDone / handleTerminalEvent）。
func BuildResponseStream(spec MockSpec, model, responseID string) []MockResponseEvent {
	pieces := spec.Chunks()
	item := buildResponseItem(spec)
	out := make([]MockResponseEvent, 0, len(pieces)+4)

	out = append(out,
		mockEvent(enum.ResponseStreamEventCreated, map[string]any{
			constant.ResponseStreamFieldID:     responseID,
			constant.ResponseStreamFieldObject: enum.CompletionObjectResponse,
			constant.ResponseStreamFieldStatus: enum.ResponseStatusInProgress,
			constant.ResponseStreamFieldModel:  model,
		}),
		mockEvent(enum.ResponseStreamEventOutputItemAdded, map[string]any{
			constant.ResponseStreamFieldOutputIndex: 0,
			constant.ResponseStreamFieldItem:        item,
		}),
	)
	for _, piece := range pieces {
		out = append(out, mockEvent(enum.ResponseStreamEventOutputTextDelta, map[string]any{
			constant.ResponseStreamFieldOutputIndex:  0,
			constant.ResponseStreamFieldContentIndex: 0,
			constant.ResponseStreamFieldDelta:        piece,
		}))
	}
	out = append(out,
		mockEvent(enum.ResponseStreamEventOutputItemDone, &dto.ResponseStreamOutputItemDoneEvent{
			Type:        enum.ResponseStreamEventOutputItemDone,
			OutputIndex: 0,
			Item:        item,
		}),
		mockEvent(enum.ResponseStreamEventCompleted, &dto.ResponseStreamTerminalEvent{
			Type:     enum.ResponseStreamEventCompleted,
			Response: BuildResponse(spec, model),
		}),
	)
	return out
}

// BuildResponse 生成非流式 Responses 响应。
func BuildResponse(spec MockSpec, model string) *dto.OpenAICreateResponseRsp {
	return &dto.OpenAICreateResponseRsp{
		ID:        fmt.Sprintf(constant.ResponseIDTemplate, strconv.FormatInt(time.Now().UnixNano(), 10)),
		Object:    enum.CompletionObjectResponse,
		CreatedAt: time.Now().Unix(),
		Status:    enum.ResponseStatusCompleted,
		Model:     model,
		Output:    []*dto.ResponseInputItem{buildResponseItem(spec)},
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
