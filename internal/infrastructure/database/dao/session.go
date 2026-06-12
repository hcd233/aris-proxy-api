// Package dao Message DAO
//
//	author centonhuang
//	update 2026-03-10 10:00:00
package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm"

	"github.com/samber/lo"

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
