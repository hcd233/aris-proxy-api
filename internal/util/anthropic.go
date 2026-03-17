package util

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"
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
			humaCtx.SetHeader("Content-Type", "application/json")
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
				Type: "error",
				Error: &dto.AnthropicError{
					Type:    "not_found_error",
					Message: fmt.Sprintf("model: %s", model),
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
			humaCtx.SetHeader("Content-Type", "application/json")
			humaCtx.BodyWriter().Write(lo.Must1(sonic.Marshal(&dto.AnthropicErrorResponse{
				Type: "error",
				Error: &dto.AnthropicError{
					Type:    "api_error",
					Message: "Internal server error",
				},
			})))
		},
	}
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
//     @param events []AnthropicSSEEvent
//     @return *dto.AnthropicMessage
//     @author centonhuang
//     @update 2026-03-17 10:00:00
func ConcatAnthropicSSEEvents(events []AnthropicSSEEvent) *dto.AnthropicMessage {
	msg := &dto.AnthropicMessage{}

	// Track content blocks by index
	type blockState struct {
		blockType string
		textParts []string
		rawStart  json.RawMessage // The initial content_block from content_block_start
	}
	blocks := make(map[int]*blockState)
	blockOrder := make([]int, 0)

	for _, event := range events {
		switch event.Event {
		case "message_start":
			// data: {"type":"message_start","message":{...}}
			var payload struct {
				Message *dto.AnthropicMessage `json:"message"`
			}
			if err := sonic.Unmarshal(event.Data, &payload); err == nil && payload.Message != nil {
				msg.ID = payload.Message.ID
				msg.Type = payload.Message.Type
				msg.Role = payload.Message.Role
				msg.Model = payload.Message.Model
				msg.Usage = payload.Message.Usage
			}

		case "content_block_start":
			// data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
			var payload struct {
				Index        int             `json:"index"`
				ContentBlock json.RawMessage `json:"content_block"`
			}
			if err := sonic.Unmarshal(event.Data, &payload); err == nil {
				var blockInfo struct {
					Type string `json:"type"`
				}
				sonic.Unmarshal(payload.ContentBlock, &blockInfo)
				bs := &blockState{
					blockType: blockInfo.Type,
					rawStart:  payload.ContentBlock,
				}
				blocks[payload.Index] = bs
				blockOrder = append(blockOrder, payload.Index)
			}

		case "content_block_delta":
			// data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"..."}}
			var payload struct {
				Index int `json:"index"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := sonic.Unmarshal(event.Data, &payload); err == nil {
				if bs, ok := blocks[payload.Index]; ok {
					if payload.Delta.Text != "" {
						bs.textParts = append(bs.textParts, payload.Delta.Text)
					}
				}
			}

		case "message_delta":
			// data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}
			var payload struct {
				Delta struct {
					StopReason   *string `json:"stop_reason"`
					StopSequence *string `json:"stop_sequence"`
				} `json:"delta"`
				Usage *dto.AnthropicUsage `json:"usage"`
			}
			if err := sonic.Unmarshal(event.Data, &payload); err == nil {
				msg.StopReason = payload.Delta.StopReason
				msg.StopSequence = payload.Delta.StopSequence
				if payload.Usage != nil && msg.Usage != nil {
					msg.Usage.OutputTokens = payload.Usage.OutputTokens
				}
			}
		}
	}

	// Build final content blocks
	msg.Content = make([]json.RawMessage, 0, len(blockOrder))
	for _, idx := range blockOrder {
		bs := blocks[idx]
		if bs.blockType == "text" && len(bs.textParts) > 0 {
			// Reconstruct the text block with accumulated text
			block := map[string]any{
				"type": "text",
				"text": strings.Join(bs.textParts, ""),
			}
			data, _ := sonic.Marshal(block)
			msg.Content = append(msg.Content, data)
		} else {
			// For non-text blocks (thinking, tool_use, etc.), use the raw start block
			msg.Content = append(msg.Content, bs.rawStart)
		}
	}

	return msg
}

// AnthropicSSEEvent 表示一个解析后的 Anthropic SSE 事件
//
//	@author centonhuang
//	@update 2026-03-17 10:00:00
type AnthropicSSEEvent struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}
