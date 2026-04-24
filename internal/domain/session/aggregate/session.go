// Package aggregate Session 域聚合根
package aggregate

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// Session 会话聚合根
//
// 封装一次对话会话的核心状态：所有者（APIKeyName）、消息/工具 ID 列表、
// 元数据、摘要、评分。摘要与评分通过 UpdateSummary / UpdateScore 方法更新，
// 替代基础设施直接字段落盘模式，保持聚合的一致性边界。
//
// Session 持有 MessageID/ToolID 的弱引用（值对象不跨聚合强引用），
// 一致性边界仅在 Session 自身。
//
//	@author centonhuang
//	@update 2026-04-24 14:00:00
type Session struct {
	aggregate.Base

	owner      vo.APIKeyOwner
	messageIDs []uint
	toolIDs    []uint
	metadata   map[string]string
	summary    vo.SessionSummary
	score      vo.SessionScore
	createdAt  time.Time
	updatedAt  time.Time
}

// CreateSession 创建新 Session 聚合
//
//	@param owner vo.APIKeyOwner
//	@param messageIDs []uint 去重后的消息 ID 列表
//	@param toolIDs []uint 去重后的工具 ID 列表
//	@param metadata map[string]string 可选请求元数据
//	@return *Session
//	@return error
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func CreateSession(owner vo.APIKeyOwner, messageIDs, toolIDs []uint, metadata map[string]string) (*Session, error) {
	if owner.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "session api key owner is empty")
	}
	now := time.Now().UTC()
	return &Session{
		owner:      owner,
		messageIDs: messageIDs,
		toolIDs:    toolIDs,
		metadata:   metadata,
		createdAt:  now,
		updatedAt:  now,
	}, nil
}

// RestoreSession 从仓储重建聚合
//
//	@param id uint
//	@param owner vo.APIKeyOwner
//	@param messageIDs []uint
//	@param toolIDs []uint
//	@param metadata map[string]string
//	@param summary vo.SessionSummary
//	@param score vo.SessionScore
//	@param createdAt time.Time
//	@param updatedAt time.Time
//	@return *Session
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func RestoreSession(id uint, owner vo.APIKeyOwner, messageIDs, toolIDs []uint,
	metadata map[string]string, summary vo.SessionSummary, score vo.SessionScore,
	createdAt, updatedAt time.Time) *Session {
	s := &Session{
		owner:      owner,
		messageIDs: messageIDs,
		toolIDs:    toolIDs,
		metadata:   metadata,
		summary:    summary,
		score:      score,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
	s.SetID(id)
	return s
}

// UpdateSummary 更新会话摘要
//
// 由 SummarizeAgent 在完成总结后调用，替代基础设施直接写入 DB 字段。
//
//	@receiver s *Session
//	@param summary vo.SessionSummary
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func (s *Session) UpdateSummary(summary vo.SessionSummary) {
	s.summary = summary
	s.updatedAt = time.Now().UTC()
}

// UpdateScore 更新会话评分
//
// 由 ScoreAgent 在完成评分后调用，替代基础设施直接写入 DB 字段。
//
//	@receiver s *Session
//	@param score vo.SessionScore
//	@author centonhuang
//	@update 2026-04-24 20:00:00
func (s *Session) UpdateScore(score vo.SessionScore) {
	s.score = score
	s.updatedAt = time.Now().UTC()
}

// AggregateType 实现 aggregate.Root 接口
func (*Session) AggregateType() string { return constant.AggregateTypeSession }

// Owner 返回所属 API Key 名称
func (s *Session) Owner() vo.APIKeyOwner { return s.owner }

// MessageIDs 返回消息 ID 列表（按原始对话顺序）
func (s *Session) MessageIDs() []uint { return s.messageIDs }

// ToolIDs 返回工具 ID 列表
func (s *Session) ToolIDs() []uint { return s.toolIDs }

// Metadata 返回请求元数据
func (s *Session) Metadata() map[string]string { return s.metadata }

// Summary 返回总结值对象
func (s *Session) Summary() vo.SessionSummary { return s.summary }

// Score 返回评分值对象
func (s *Session) Score() vo.SessionScore { return s.score }

// CreatedAt 返回创建时间
func (s *Session) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt 返回更新时间
func (s *Session) UpdatedAt() time.Time { return s.updatedAt }

// IsOwnedBy 判断指定 APIKeyName 是否为会话所有者
//
//	@receiver s *Session
//	@param apiKeyName string
//	@return bool
//	@author centonhuang
//	@update 2026-04-23 10:45:00
func (s *Session) IsOwnedBy(apiKeyName string) bool {
	return s.owner.String() == apiKeyName
}
