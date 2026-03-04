package enum

// SSEStatus SSE状态
//
//	@author centonhuang
//	@update 2025-11-13 17:50:42
type SSEStatus string

const (
	// SSEStatusStart SSE状态开始
	//	@author centonhuang
	//	@update 2025-11-13 17:50:42
	SSEStatusStart SSEStatus = "start"

	// SSEStatusStreaming SSEStatus 流式
	//	@update 2025-11-13 19:07:55
	SSEStatusStreaming SSEStatus = "streaming"

	// SSEStatusError SSEStatus 错误
	//	@update 2025-11-13 19:07:49
	SSEStatusError SSEStatus = "error"
	// SSEStatusEnd SSE状态结束
	//	@author centonhuang
	//	@update 2025-11-13 17:50:42
	SSEStatusEnd SSEStatus = "end"
)
