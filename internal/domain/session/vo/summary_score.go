// Package vo Session 域值对象
package vo

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

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
