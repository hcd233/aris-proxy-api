// Package dao Tool DAO
//
//	author centonhuang
//	update 2026-03-18 10:00:00
package dao

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm/clause"
)

// ToolDAO 工具数据访问对象
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type ToolDAO struct {
	baseDAO[dbmodel.Tool]
}

// ChecksumConflict 返回按 checksum 去重插入时的冲突策略
//
//	tools 表唯一索引为 idx_tool_checksum_deleted (check_sum, deleted_at) 复合索引，
//	冲突目标需同时指定两列。
//
//	DoNothing 让冲突行被静默跳过而不报错，这对事务安全是必要的：PostgreSQL 中任一语句
//	报错后事务即进入 aborted 状态，后续任何语句（包括补查已存在记录）都会失败。
//
//	@receiver dao *ToolDAO
//	@return clause.Expression
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (dao *ToolDAO) ChecksumConflict() clause.Expression {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: constant.FieldCheckSum}, {Name: constant.FieldDeletedAt}},
		DoNothing: true,
	}
}
