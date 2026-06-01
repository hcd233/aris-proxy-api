// Package enum provides common enums for the application.
package enum

// ReasoningEffort 推理努力级别
//
//	@author centonhuang
//	@update 2026-03-06 10:00:00
type ReasoningEffort = string

const (

	// ReasoningEffortNone 不执行推理
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ReasoningEffortNone ReasoningEffort = "none"

	// ReasoningEffortMinimal 最小推理努力
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ReasoningEffortMinimal ReasoningEffort = "minimal"

	// ReasoningEffortLow 低推理努力
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ReasoningEffortLow ReasoningEffort = "low"

	// ReasoningEffortMedium 中等推理努力
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ReasoningEffortMedium ReasoningEffort = "medium"

	// ReasoningEffortHigh 高推理努力
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ReasoningEffortHigh ReasoningEffort = "high"

	// ReasoningEffortXHigh 极高推理努力
	//
	//	@author centonhuang
	//	@update 2026-03-06 10:00:00
	ReasoningEffortXHigh ReasoningEffort = "xhigh"
)
