package enum

// SSEDataType SSE数据类型
//
//	@author centonhuang
//	@update 2025-11-08 04:20:42
type SSEDataType string

const (
	// SSEDataTypeError 错误数据
	//	@author centonhuang
	//	@update 2025-11-08 04:39:06
	SSEDataTypeError = "error"

	// SSEDataTypeHeartBeat 心跳数据
	//	@author centonhuang
	//	@update 2025-11-08 04:39:27
	SSEDataTypeHeartBeat = "heartbeat"
)
