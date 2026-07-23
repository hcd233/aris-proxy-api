package proxyutil

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/samber/lo"
)

// SendAnthropicModelNotFoundError 发送Anthropic模型不存在错误
//
//	@param model string
//	@return rsp
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func SendAnthropicModelNotFoundError(model string) (rsp *huma.StreamResponse) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(http.StatusNotFound)
			humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
			_, _ = humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{ //nolint:errcheck // best-effort write in error handler
				Type: constant.AnthropicInternalErrorBodyType,
				Error: &dto.AnthropicError{
					Type:    constant.AnthropicNotFoundErrorType,
					Message: fmt.Sprintf(constant.AnthropicModelNotFoundMessageTemplate, model),
				},
			})))
		},
	}
}

// SendAnthropicInternalError 发送Anthropic内部错误
//
//	@return rsp
//	@author centonhuang
//	@update 2026-03-17 10:00:00
func SendAnthropicInternalError() (rsp *huma.StreamResponse) {
	return &huma.StreamResponse{
		Body: func(humaCtx huma.Context) {
			humaCtx.SetStatus(http.StatusInternalServerError)
			humaCtx.SetHeader(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
			_, _ = humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{ //nolint:errcheck // best-effort write in error handler
				Type: constant.AnthropicInternalErrorBodyType,
				Error: &dto.AnthropicError{
					Type:    constant.AnthropicInternalErrorType,
					Message: constant.AnthropicInternalErrorMessage,
				},
			})))
		},
	}
}

// blockState tracks content blocks by index during SSE event accumulation
type blockState struct {
	block         *dto.AnthropicContentBlock
	textParts     []string
	thinkingParts []string
	inputParts    []string // input_json_delta
}

// ConcatAnthropicSSEEvents 合并 Anthropic SSE 事件为完整的 AnthropicMessage 响应
//
// Anthropic SSE 事件类型：
//
//   - message_start: 包含 message 对象（id, model, role, usage等）
//
//   - content_block_start: 包含 content_block 对象（type, text 初始值）
//
//   - content_block_delta: 包含 delta 对象（type, text增量）
//
//   - content_block_stop: content block 结束
//
//   - message_delta: 包含 delta 对象（stop_reason）和 usage
//
//   - message_stop: 消息结束
//
//   - ping: 心跳
//
//     @param events []dto.AnthropicSSEEvent
//     @return *dto.AnthropicMessage
//     @return error
//     @author centonhuang
//     @update 2026-03-18 10:00:00
func ConcatAnthropicSSEEvents(events []dto.AnthropicSSEEvent) (*dto.AnthropicMessage, error) {
	msg := &dto.AnthropicMessage{}
	blocks := make(map[int]*blockState)
	blockOrder := make([]int, 0)

	for _, event := range events {
		if err := processAnthropicSSEEvent(event, msg, blocks, &blockOrder); err != nil {
			return nil, err
		}
	}

	content, err := buildAnthropicContentBlocks(blocks, blockOrder)
	if err != nil {
		return nil, err
	}
	msg.Content = content
	return msg, nil
}

// processAnthropicSSEEvent processes a single SSE event and updates the message and blocks
func processAnthropicSSEEvent(event dto.AnthropicSSEEvent, msg *dto.AnthropicMessage, blocks map[int]*blockState, blockOrder *[]int) error {
	switch event.Event {
	case enum.AnthropicSSEEventTypeMessageStart:
		var payload dto.AnthropicSSEMessageStart
		if err := sonic.Unmarshal(event.Data, &payload); err != nil {
			return ierr.Wrap(ierr.ErrSSEParse, err, "unmarshal message_start")
		}
		if payload.Message != nil {
			msg.ID = payload.Message.ID
			msg.Type = payload.Message.Type
			msg.Role = payload.Message.Role
			msg.Model = payload.Message.Model
			msg.Usage = payload.Message.Usage
		}

	case enum.AnthropicSSEEventTypeContentBlockStart:
		var payload dto.AnthropicSSEContentBlockStart
		if err := sonic.Unmarshal(event.Data, &payload); err != nil {
			return ierr.Wrap(ierr.ErrSSEParse, err, "unmarshal content_block_start")
		}
		blocks[payload.Index] = &blockState{
			block: payload.ContentBlock,
		}
		*blockOrder = append(*blockOrder, payload.Index)

	case enum.AnthropicSSEEventTypeContentBlockDelta:
		var payload dto.AnthropicSSEContentBlockDelta
		if err := sonic.Unmarshal(event.Data, &payload); err != nil {
			return ierr.Wrap(ierr.ErrSSEParse, err, "unmarshal content_block_delta")
		}
		bs, ok := blocks[payload.Index]
		if !ok {
			return ierr.Newf(ierr.ErrSSEParse, "content_block_delta for unknown index %d", payload.Index)
		}
		switch payload.Delta.Type {
		case enum.AnthropicDeltaTypeTextDelta:
			bs.textParts = append(bs.textParts, payload.Delta.Text)
		case enum.AnthropicDeltaTypeThinkingDelta:
			bs.thinkingParts = append(bs.thinkingParts, payload.Delta.Thinking)
		case enum.AnthropicDeltaTypeInputJSONDelta:
			bs.inputParts = append(bs.inputParts, payload.Delta.PartialJSON)
		case enum.AnthropicDeltaTypeSignatureDelta:
			if bs.block != nil {
				bs.block.Signature = lo.ToPtr(lo.FromPtr(bs.block.Signature) + payload.Delta.Text)
			}
		}

	case enum.AnthropicSSEEventTypeMessageDelta:
		var payload dto.AnthropicSSEMessageDelta
		if err := sonic.Unmarshal(event.Data, &payload); err != nil {
			return ierr.Wrap(ierr.ErrSSEParse, err, "unmarshal message_delta")
		}
		msg.StopReason = payload.Delta.StopReason
		msg.StopSequence = payload.Delta.StopSequence
		if payload.Usage != nil && msg.Usage != nil {
			msg.Usage.OutputTokens = payload.Usage.OutputTokens
		}

	case enum.AnthropicSSEEventTypeContentBlockStop, enum.AnthropicSSEEventTypeMessageStop, enum.AnthropicSSEEventTypePing:
		// 无需处理

	default:
		return ierr.Newf(ierr.ErrSSEUnknownEvent, "unknown SSE event type: %q", event.Event)
	}
	return nil
}

// buildAnthropicContentBlocks assembles final content blocks from accumulated block state
func buildAnthropicContentBlocks(blocks map[int]*blockState, blockOrder []int) ([]*dto.AnthropicContentBlock, error) {
	content := make([]*dto.AnthropicContentBlock, 0, len(blockOrder))
	for _, idx := range blockOrder {
		bs := blocks[idx]
		if bs.block == nil {
			continue
		}

		block := bs.block

		switch block.Type {
		case enum.AnthropicContentBlockTypeText:
			if len(bs.textParts) > 0 {
				text := strings.Join(bs.textParts, "")
				block.Text = &text
			}
		case enum.AnthropicContentBlockTypeThinking:
			if len(bs.thinkingParts) > 0 {
				s := strings.Join(bs.thinkingParts, "")
				block.Thinking = &s
			}
		case enum.AnthropicContentBlockTypeToolUse, enum.AnthropicContentBlockTypeServerToolUse:
			if len(bs.inputParts) > 0 {
				inputJSON := strings.Join(bs.inputParts, "")
				var input sonic.NoCopyRawMessage
				if err := sonic.UnmarshalString(inputJSON, &input); err != nil {
					return nil, ierr.Wrapf(ierr.ErrSSEParse, err, "unmarshal accumulated tool_use input for block[%d]", idx)
				}
				block.Input = input
			}
		}

		content = append(content, block)
	}
	return content, nil
}

// ExtractAnthropicMetadata 将 Anthropic 元数据转换为通用 map
//
//	@param meta *dto.AnthropicMetadata
//	@return map[string]string
//	@author centonhuang
//	@update 2026-03-29 10:00:00
func ExtractAnthropicMetadata(meta *dto.AnthropicMetadata) map[string]string {
	if meta == nil {
		return nil
	}
	m := make(map[string]string)
	if lo.FromPtr(meta.UserID) != "" {
		m["user_id"] = lo.FromPtr(meta.UserID)
	}

	if len(m) == 0 {
		return nil
	}
	return m
}
