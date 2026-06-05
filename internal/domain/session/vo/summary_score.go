// Package vo Session 域值对象
package vo

import (
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// SessionSummary 会话摘要值对象
//
// 摘要约定：5-10 字中文描述，由 Summarizer 领域服务生成。为空字符串表示
// 尚未总结（or 总结失败）。携带 error 字段便于审计和重试策略。
//
// 字段私有以保证值对象不可变；通过构造函数 + 只读 getter 访问。
//
//	@author centonhuang
//	@update 2026-04-24 14:00:00
type SessionSummary struct {
	text   string
	errMsg string
}

// NewSessionSummary 构造摘要值对象
//
//	@param text string 摘要文本
//	@param errMsg string 失败原因（为空表示成功）
//	@return SessionSummary
//	@author centonhuang
//	@update 2026-04-24 14:00:00
func NewSessionSummary(text, errMsg string) SessionSummary {
	return SessionSummary{text: text, errMsg: errMsg}
}

// Text 返回摘要文本
//
//	@receiver s SessionSummary
//	@return string
//	@author centonhuang
//	@update 2026-04-24 14:00:00
func (s SessionSummary) Text() string { return s.text }

// Error 返回失败原因（成功时为空字符串）
//
//	@receiver s SessionSummary
//	@return string
//	@author centonhuang
//	@update 2026-04-24 14:00:00
func (s SessionSummary) Error() string { return s.errMsg }

// IsEmpty 判断是否为空
//
//	@receiver s SessionSummary
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (s SessionSummary) IsEmpty() bool { return strings.TrimSpace(s.text) == "" }

// Failed 判断总结是否失败
//
//	@receiver s SessionSummary
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (s SessionSummary) Failed() bool { return s.errMsg != "" }

// SessionScore 会话评分值对象
//
// 人工评分，范围 1-5（整数），nil 表示未评分。
// 字段私有以保证值对象不可变。
//
//	@author centonhuang
//	@update 2026-06-03 10:00:00
type SessionScore struct {
	score *int
	at    *time.Time
}

// Score 返回评分值；未评分返回 nil
func (s SessionScore) Score() *int { return s.score }

// At 返回评分时间；未评分返回 nil
func (s SessionScore) At() *time.Time { return s.at }

// IsEmpty 判断是否未评分
func (s SessionScore) IsEmpty() bool { return s.score == nil }

// NewSessionScore 构造人工评分值对象
//
//	@param score int 评分值，需在 1-5 范围内
//	@param at time.Time 评分时间
//	@return SessionScore
//	@return error score 不在 1-5 范围内时返回 ierr.ErrValidation
func NewSessionScore(score int, at time.Time) (SessionScore, error) {
	if score < 1 || score > 5 {
		return SessionScore{}, ierr.Newf(ierr.ErrValidation, "score %d out of range [1,5]", score)
	}
	if at.IsZero() {
		return SessionScore{}, ierr.New(ierr.ErrValidation, "scoredAt must not be zero")
	}
	return SessionScore{
		score: &score,
		at:    &at,
	}, nil
}

// RestoreSessionScore 从持久化字段重建评分值对象
func RestoreSessionScore(score *int, at *time.Time) SessionScore {
	return SessionScore{
		score: score,
		at:    at,
	}
}
