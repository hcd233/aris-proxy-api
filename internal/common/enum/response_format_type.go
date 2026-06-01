// Package enum provides common enums for the application.
package enum

// ResponseFormatType 响应格式类型
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type ResponseFormatType = string

const (

	// ResponseFormatTypeText 默认文本响应格式
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ResponseFormatTypeText ResponseFormatType = "text"

	// ResponseFormatTypeJSONObject JSON对象响应格式（旧版JSON模式）
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ResponseFormatTypeJSONObject ResponseFormatType = "json_object"

	// ResponseFormatTypeJSONSchema JSON Schema响应格式（结构化输出）
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ResponseFormatTypeJSONSchema ResponseFormatType = "json_schema"
)
