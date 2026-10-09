// Package enum provides common enums for the application.
package enum

// DecisionAnswerType Decision API 答案类型
//
//	@author centonhuang
//	@update 2026-10-09 10:00:00
type DecisionAnswerType = string

const (
	// DecisionAnswerTypeRefusal 模型拒答（触发词拦截时代答亦使用该类型）
	//
	//	@author centonhuang
	//	@update 2026-10-09 10:00:00
	DecisionAnswerTypeRefusal DecisionAnswerType = "refusal"
)
