// Package enum provides common enums for the application.
package enum

// SearchContextSize 搜索上下文大小
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type SearchContextSize = string

const (

	// SearchContextSizeLow 低上下文大小
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	SearchContextSizeLow SearchContextSize = "low"

	// SearchContextSizeMedium 中等上下文大小
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	SearchContextSizeMedium SearchContextSize = "medium"

	// SearchContextSizeHigh 高上下文大小
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	SearchContextSizeHigh SearchContextSize = "high"
)
