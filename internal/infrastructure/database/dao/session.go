// Package dao Message DAO
//
//	author centonhuang
//	update 2026-03-10 10:00:00
package dao

import (
	"strconv"
	"time"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// MessageDAO 消息数据访问对象
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type SessionDAO struct {
	baseDAO[dbmodel.Session]
}

// SessionPurgeView 软删除清理所需的会话数据视图
type SessionPurgeView struct {
	ID         uint
	MessageIDs []uint
	ToolIDs    []uint
}

// SessionTerminalScanView 终态清理窗口扫描所需的会话数据视图
//
//	cron 包不允许依赖 dbmodel（lintconv architecture.database_model_dependency），
//	窗口扫描类查询经此视图暴露。
type SessionTerminalScanView struct {
	ID         uint
	MessageIDs []uint
}

// FindAllForPurge 查询会话数据用于软删除清理
func (dao *SessionDAO) FindAllForPurge(db *gorm.DB, softDeleted bool) ([]SessionPurgeView, error) {
	var models []*dbmodel.Session
	query := db
	if softDeleted {
		query = query.Unscoped().Where(constant.DBConditionDeletedAtNotZero)
	} else {
		query = query.Where(constant.DBConditionDeletedAtZero)
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	views := lo.Map(models, func(m *dbmodel.Session, _ int) SessionPurgeView {
		return SessionPurgeView{ID: m.ID, MessageIDs: m.MessageIDs, ToolIDs: m.ToolIDs}
	})
	return views, nil
}

// GroupForUpdateQuery 构造同组会话的行锁查询（不执行）
//
//	首条消息 ID 是同一对话快照集合的分组键：历史消息按 checksum 去重复用行，
//	第 k 轮的 MessageIDs 是第 k+1 轮的前缀。查询结果不含 MessageIDs 为空的行
//	（'[]' 的 ->>0 为 NULL），与去重算法跳过空 MessageIDs 的语义一致。
//
//	@param db *gorm.DB
//	@param firstMessageID uint 组键（首条消息 ID）
//	@return *gorm.DB
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) GroupForUpdateQuery(db *gorm.DB, firstMessageID uint) *gorm.DB {
	return db.Select(constant.SessionRepoFieldsDedup).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.SessionFirstMessageIDCondition, strconv.FormatUint(uint64(firstMessageID), 10)).
		Clauses(clause.Locking{Strength: constant.DBLockStrengthUpdate})
}

// FindGroupForUpdate 锁定并返回同组（首条消息 ID 相同）的全部活跃会话
//
//	FOR UPDATE 串行化同组并发写入（插入路径去重与多副本并发）。
//	::jsonb 为 PG 专有语法，sqlite 不可用，行为由 e2e 覆盖。
//
//	@param db *gorm.DB
//	@param firstMessageID uint 组键
//	@return []*dbmodel.Session
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) FindGroupForUpdate(db *gorm.DB, firstMessageID uint) ([]*dbmodel.Session, error) {
	var models []*dbmodel.Session
	if err := dao.GroupForUpdateQuery(db, firstMessageID).Find(&models).Error; err != nil {
		return nil, err
	}
	return models, nil
}

// CreatedSinceQuery 构造 24h 窗口扫描查询（不执行）
//
//	@param db *gorm.DB
//	@param since time.Time 创建时间下界
//	@return *gorm.DB
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) CreatedSinceQuery(db *gorm.DB, since time.Time) *gorm.DB {
	return db.Select(constant.SessionRepoFieldsTerminalScan).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.WhereCreatedAtGTE, since)
}

// FindCreatedSince 查询指定时间之后创建的活跃会话（终态清理扫描窗口）
//
//	依赖 idx_sessions_created_at（Session 模型重声明 CreatedAt 挂 tag，AutoMigrate 自动建）。
//
//	@param db *gorm.DB
//	@param since time.Time 创建时间下界
//	@return []SessionTerminalScanView
//	@return error
//	@author centonhuang
//	@update 2026-08-29 10:00:00
func (dao *SessionDAO) FindCreatedSince(db *gorm.DB, since time.Time) ([]SessionTerminalScanView, error) {
	var models []*dbmodel.Session
	if err := dao.CreatedSinceQuery(db, since).Find(&models).Error; err != nil {
		return nil, err
	}
	views := lo.Map(models, func(m *dbmodel.Session, _ int) SessionTerminalScanView {
		return SessionTerminalScanView{ID: m.ID, MessageIDs: m.MessageIDs}
	})
	return views, nil
}
