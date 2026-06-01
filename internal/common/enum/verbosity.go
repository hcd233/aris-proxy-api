// Package enum provides common enums for the application.
package enum

// Verbosity 详细程度
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type Verbosity = string

const (

	// VerbosityLow 简洁响应
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	VerbosityLow Verbosity = "low"

	// VerbosityMedium 中等详细程度
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	VerbosityMedium Verbosity = "medium"

	// VerbosityHigh 详细响应
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	VerbosityHigh Verbosity = "high"
)
