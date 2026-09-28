// Package aggregate Session 域聚合根
package aggregate

import (
	"maps"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/common/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// Session 会话聚合根
//
// 封装一次对话会话的核心状态：所有者（APIKeyID，名称仅展示）、消息/工具 ID
// 列表、元数据、评分。评分通过 UpdateScore 方法更新，
// 替代基础设施直接字段落盘模式，保持聚合的一致性边界。
//
// Session 持有 MessageID/ToolID 的弱引用（值对象不跨聚合强引用），
// 一致性边界仅在 Session 自身。
//
//	@author centonhuang
//	@update 2026-04-24 14:00:00
type Session struct {
	aggregate.Base

	ownerID    vo.APIKeyOwnerID
	ownerName  string
	messageIDs []uint
	toolIDs    []uint
	metadata   map[string]string
	score      vo.SessionScore
	createdAt  time.Time
	updatedAt  time.Time
}

// CreateSession 创建新 Session 聚合
//
//	@param ownerID vo.APIKeyOwnerID 归属 API Key ID（鉴权唯一依据），不得为 0
//	@param ownerName string 归属 API Key 名称（仅展示与审计可读性），允许为空
//	@param messageIDs []uint 去重后的消息 ID 列表
//	@param toolIDs []uint 去重后的工具 ID 列表
//	@param metadata map[string]string 可选请求元数据
//	@return *Session
//	@return error
//	@author centonhuang
//	@update 2026-09-27 10:00:00
func CreateSession(ownerID vo.APIKeyOwnerID, ownerName string, messageIDs, toolIDs []uint, metadata map[string]string, now time.Time) (*Session, error) {
	if ownerID.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "session api key owner id is empty")
	}
	if hasDuplicateIDs(messageIDs) {
		return nil, ierr.New(ierr.ErrValidation, "session message IDs contain duplicates")
	}
	if hasDuplicateIDs(toolIDs) {
		return nil, ierr.New(ierr.ErrValidation, "session tool IDs contain duplicates")
	}
	return &Session{
		ownerID:    ownerID,
		ownerName:  ownerName,
		messageIDs: messageIDs,
		toolIDs:    toolIDs,
		metadata:   metadata,
		createdAt:  now,
		updatedAt:  now,
	}, nil
}

// hasDuplicateIDs 检查 ID 切片是否存在重复值
//
//	@param ids []uint
//	@return bool
func hasDuplicateIDs(ids []uint) bool {
	return len(lo.Uniq(ids)) < len(ids)
}

// RestoreSession 从仓储重建聚合
//
//	@param id uint
//	@param ownerID vo.APIKeyOwnerID 归属 API Key ID（存量可为 0，不校验）
//	@param ownerName string 归属 API Key 名称（仅展示）
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
func RestoreSession(id uint, ownerID vo.APIKeyOwnerID, ownerName string, messageIDs, toolIDs []uint,
	metadata map[string]string, score vo.SessionScore,
	createdAt, updatedAt time.Time) *Session {
	s := &Session{
		ownerID:    ownerID,
		ownerName:  ownerName,
		messageIDs: messageIDs,
		toolIDs:    toolIDs,
		metadata:   metadata,
		score:      score,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
	s.SetID(id)
	return s
}

// UpdateScore 更新会话人工评分
//
//	@receiver s *Session
//	@param score vo.SessionScore
//	@param now time.Time
//	@author centonhuang
//	@update 2026-06-03 10:00:00
func (s *Session) UpdateScore(score vo.SessionScore, now time.Time) {
	s.score = score
	s.updatedAt = now
}

// OwnerID 返回归属 API Key ID（鉴权依据）
func (s *Session) OwnerID() vo.APIKeyOwnerID { return s.ownerID }

// OwnerName 返回归属 API Key 名称（仅展示用，不参与鉴权）
func (s *Session) OwnerName() string { return s.ownerName }

// MessageIDs 返回消息 ID 列表的防御性副本
func (s *Session) MessageIDs() []uint {
	out := make([]uint, len(s.messageIDs))
	copy(out, s.messageIDs)
	return out
}

// ToolIDs 返回工具 ID 列表的防御性副本
func (s *Session) ToolIDs() []uint {
	out := make([]uint, len(s.toolIDs))
	copy(out, s.toolIDs)
	return out
}

// Metadata 返回请求元数据的防御性副本
func (s *Session) Metadata() map[string]string {
	return maps.Clone(s.metadata)
}

// Score 返回评分值对象
func (s *Session) Score() vo.SessionScore { return s.score }

// CreatedAt 返回创建时间
func (s *Session) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt 返回更新时间
func (s *Session) UpdatedAt() time.Time { return s.updatedAt }

// IsOwnedByID 判断指定 API Key ID 是否为会话所有者。
//
// 归属为 0（存量空归属）或请求方 ID 为 0（认证缺失）时恒返回 false：
// 两个零值不得互相匹配，否则空归属会话会对认证异常的请求全部放行。
//
//	@receiver s *Session
//	@param apiKeyID uint
//	@return bool
//	@author centonhuang
//	@update 2026-09-27 10:00:00
func (s *Session) IsOwnedByID(apiKeyID uint) bool {
	if s.ownerID.IsEmpty() || apiKeyID == 0 {
		return false
	}
	return s.ownerID.Uint() == apiKeyID
}
