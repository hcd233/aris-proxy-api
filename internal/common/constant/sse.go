package constant

import "time"

const (
	SSEHeartbeatCount = 30
	HeartbeatInterval = 1 * time.Second

	SSEDataPrefix  = "data: "
	SSEDoneSignal  = "[DONE]"
	SSEEventPrefix = "event: "

	AnthropicMessageStopSSEFrame  = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	AnthropicMessageStopData      = `{"type":"message_stop"}`
	AnthropicContentBlockStopData = `{"type":"content_block_stop","index":0}`

	// ResponseObjectValue Response API 响应对象的 object 字段值
	ResponseObjectValue = "response"

	SSEDataFrameTemplate        = "data: %s\n\n"
	SSEEventFrameTemplate       = "event: %s\ndata: %s\n\n"
	SSEEventLineTemplate        = "event: %s\n"
	SSEDataLineTemplate         = "data: %s\n\n"
	SSEOpenAIUpstreamErrorFrame = "data: {\"error\":{\"message\":\"upstream returned status %d\",\"type\":\"server_error\",\"code\":\"upstream_error\"}}\n\n"
	SSEOpenAIInternalErrorFrame = "data: {\"error\":{\"message\":\"internal server error\",\"type\":\"server_error\",\"code\":\"internal_error\"}}\n\n"
	SSEOpenAIUpstreamErrorData  = `{"error":{"message":"upstream returned status %d","type":"server_error","code":"upstream_error"}}`
	SSEOpenAIInternalErrorData  = `{"error":{"message":"internal server error","type":"server_error","code":"internal_error"}}`
	SSEOpenAIShuttingDownData   = `{"error":{"message":"server is restarting, please retry","type":"server_error","code":"server_shutting_down"}}`

	SSEProviderOpenAI    = "openai"
	SSEProviderAnthropic = "anthropic"
)
