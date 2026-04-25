// Package vo Session 域值对象
package vo

import (
	"fmt"
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
// 三维度评分：Coherence（连贯性）+ Depth（深度）+ Value（价值）均在 1-10；
// Total 为三者均值（float）。字段私有以保证值对象不可变。
//
//	@author centonhuang
//	@update 2026-04-24 14:00:00
type SessionScore struct {
	coherence float64
	depth     float64
	value     float64
	total     float64
	version   string
	at        *time.Time
	errMsg    string
}

// Coherence 返回连贯性分
func (s SessionScore) Coherence() float64 { return s.coherence }

// Depth 返回深度分
func (s SessionScore) Depth() float64 { return s.depth }

// Value 返回价值分
func (s SessionScore) Value() float64 { return s.value }

// Total 返回总分（三维度均值）
func (s SessionScore) Total() float64 { return s.total }

// Version 返回评分算法版本
func (s SessionScore) Version() string { return s.version }

// At 返回评分时间；未评分返回 nil
func (s SessionScore) At() *time.Time { return s.at }

// Error 返回失败原因（成功时为空字符串）
func (s SessionScore) Error() string { return s.errMsg }

// IsEmpty 判断是否未评分
//
//	@receiver s SessionScore
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (s SessionScore) IsEmpty() bool { return s.at == nil && s.errMsg == "" }

// Failed 判断评分是否失败
//
//	@receiver s SessionScore
//	@return bool
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func (s SessionScore) Failed() bool { return s.errMsg != "" }

// NewSessionScore 构造评分值对象（均值自动计算）
//
//	@param coherence float64 连贯性评分（1-10）
//	@param depth float64 深度评分（1-10）
//	@param value float64 价值评分（1-10）
//	@param version string
//	@param at time.Time
//	@return SessionScore
//	@return error 任一维度不在 1-10 范围内时返回 ierr.ErrValidation
//	@author centonhuang
//	@update 2026-04-26 10:00:00
func NewSessionScore(coherence, depth, value float64, version string, at time.Time) (SessionScore, error) {
	if coherence < 1 || coherence > 10 {
		return SessionScore{}, ierr.New(ierr.ErrValidation, fmt.Sprintf("coherence score %.1f out of range [1,10]", coherence))
	}
	if depth < 1 || depth > 10 {
		return SessionScore{}, ierr.New(ierr.ErrValidation, fmt.Sprintf("depth score %.1f out of range [1,10]", depth))
	}
	if value < 1 || value > 10 {
		return SessionScore{}, ierr.New(ierr.ErrValidation, fmt.Sprintf("value score %.1f out of range [1,10]", value))
	}
	return SessionScore{
		coherence: coherence,
		depth:     depth,
		value:     value,
		total:     (coherence + depth + value) / 3.0,
		version:   version,
		at:        &at,
	}, nil
}

// NewFailedSessionScore 构造失败的评分值对象
//
//	@param reason string
//	@param at time.Time
//	@return SessionScore
//	@author centonhuang
//	@update 2026-04-22 19:30:00
func NewFailedSessionScore(reason string, at time.Time) SessionScore {
	return SessionScore{
		errMsg: reason,
		at:     &at,
	}
}

// RestoreSessionScore 从持久化字段重建评分值对象（供仓储使用，不重新计算 total）
//
//	@param coherence float64
//	@param depth float64
//	@param value float64
//	@param total float64
//	@param version string
//	@param at *time.Time
//	@param errMsg string
//	@return SessionScore
//	@author centonhuang
//	@update 2026-04-24 14:00:00
func RestoreSessionScore(coherence, depth, value, total float64, version string, at *time.Time, errMsg string) SessionScore {
	return SessionScore{
		coherence: coherence,
		depth:     depth,
		value:     value,
		total:     total,
		version:   version,
		at:        at,
		errMsg:    errMsg,
	}
}
