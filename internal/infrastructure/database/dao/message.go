// Package dao Message DAO
//
//	author centonhuang
//	update 2026-03-10 10:00:00
package dao

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm/clause"
)

// MessageDAO 消息数据访问对象
//
//	@author centonhuang
//	@update 2026-03-10 10:00:00
type MessageDAO struct {
	baseDAO[dbmodel.Message]
}

// ChecksumConflict 返回按 checksum 去重插入时的冲突策略
//
//	messages 表唯一索引为 idx_message_checksum check_sum 单列索引，
//	与 tools 表的 (check_sum, deleted_at) 复合索引形态不同，故冲突目标只指定一列。
//
//	DoNothing 的事务安全考量见 ToolDAO.ChecksumConflict 的说明。
//
//	@receiver dao *MessageDAO
//	@return clause.Expression
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func (dao *MessageDAO) ChecksumConflict() clause.Expression {
	return clause.OnConflict{
		Columns:   []clause.Column{{Name: constant.FieldCheckSum}},
		DoNothing: true,
	}
}
