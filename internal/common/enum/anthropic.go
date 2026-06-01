// Package enum provides common enums for the application.
package enum

// ==================== Anthropic SSE Event Types ====================

// AnthropicSSEEventType Anthropic SSE 事件类型
//
//	@author centonhuang
//	@update 2026-04-01 10:00:00
type AnthropicSSEEventType = string

const (
	// AnthropicSSEEventTypeMessageStart message_start 事件
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicSSEEventTypeMessageStart AnthropicSSEEventType = "message_start"

	// AnthropicSSEEventTypeContentBlockStart content_block_start 事件
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicSSEEventTypeContentBlockStart AnthropicSSEEventType = "content_block_start"

	// AnthropicSSEEventTypeContentBlockDelta content_block_delta 事件
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicSSEEventTypeContentBlockDelta AnthropicSSEEventType = "content_block_delta"

	// AnthropicSSEEventTypeContentBlockStop content_block_stop 事件
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicSSEEventTypeContentBlockStop AnthropicSSEEventType = "content_block_stop"

	// AnthropicSSEEventTypeMessageDelta message_delta 事件
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicSSEEventTypeMessageDelta AnthropicSSEEventType = "message_delta"

	// AnthropicSSEEventTypeMessageStop message_stop 事件
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicSSEEventTypeMessageStop AnthropicSSEEventType = "message_stop"

	// AnthropicSSEEventTypePing ping 心跳事件
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicSSEEventTypePing AnthropicSSEEventType = "ping"
)

// ==================== Anthropic Content Block Types ====================

// AnthropicContentBlockType Anthropic 内容块类型
//
//	@author centonhuang
//	@update 2026-04-01 10:00:00
type AnthropicContentBlockType = string

const (
	// AnthropicContentBlockTypeText 文本内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeText AnthropicContentBlockType = "text"

	// AnthropicContentBlockTypeThinking 思考内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeThinking AnthropicContentBlockType = "thinking"

	// AnthropicContentBlockTypeRedactedThinking 编辑后的思考内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeRedactedThinking AnthropicContentBlockType = "redacted_thinking"

	// AnthropicContentBlockTypeToolUse 工具使用内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeToolUse AnthropicContentBlockType = "tool_use"

	// AnthropicContentBlockTypeServerToolUse 服务器工具使用内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeServerToolUse AnthropicContentBlockType = "server_tool_use"

	// AnthropicContentBlockTypeToolResult 工具结果内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeToolResult AnthropicContentBlockType = "tool_result"

	// AnthropicContentBlockTypeImage 图片内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeImage AnthropicContentBlockType = "image"

	// AnthropicContentBlockTypeDocument 文档内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeDocument AnthropicContentBlockType = "document"

	// AnthropicContentBlockTypeSearchResult 搜索结果内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeSearchResult AnthropicContentBlockType = "search_result"

	// AnthropicContentBlockTypeContainerUpload 容器上传内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeContainerUpload AnthropicContentBlockType = "container_upload"

	// AnthropicContentBlockTypeWebSearchToolResult Web搜索工具结果内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeWebSearchToolResult AnthropicContentBlockType = "web_search_tool_result"

	// AnthropicContentBlockTypeCodeExecutionToolResult 代码执行工具结果内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeCodeExecutionToolResult AnthropicContentBlockType = "code_execution_tool_result"

	// AnthropicContentBlockTypeWebFetchToolResult Web获取工具结果内容块
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicContentBlockTypeWebFetchToolResult AnthropicContentBlockType = "web_fetch_tool_result"
)

// ==================== Anthropic Delta Types ====================

// AnthropicDeltaType Anthropic Delta 类型
//
//	@author centonhuang
//	@update 2026-04-01 10:00:00
type AnthropicDeltaType = string

const (
	// AnthropicDeltaTypeTextDelta 文本增量
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicDeltaTypeTextDelta AnthropicDeltaType = "text_delta"

	// AnthropicDeltaTypeThinkingDelta 思考增量
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicDeltaTypeThinkingDelta AnthropicDeltaType = "thinking_delta"

	// AnthropicDeltaTypeInputJSONDelta 输入JSON增量
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicDeltaTypeInputJSONDelta AnthropicDeltaType = "input_json_delta"

	// AnthropicDeltaTypeSignatureDelta 签名增量
	//
	//	@author centonhuang
	//	@update 2026-04-01 10:00:00
	AnthropicDeltaTypeSignatureDelta AnthropicDeltaType = "signature_delta"
)
