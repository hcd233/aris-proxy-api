// Package dao Message DAO
//
//	author centonhuang
//	update 2026-03-10 10:00:00
package dao

import (
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/samber/lo"
	"gorm.io/gorm"
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

// FilterTerminalToolCallIDs 从候选 message ID 中筛出 role=assistant 且 tool_calls 非空的 ID
//
//	判定下推到 SQL，避免把 message JSON 全量载入内存反序列化。
//	生产实测：2080 个候选中仅 30 个命中，下推后 IO 由 5063 kB 降至约 1 KB。
//
//	谓词使用 PostgreSQL 专有的 ::jsonb 强转（message 为 text 列），sqlite 不可用，
//	故 SQL 本身由 e2e 覆盖，单测只覆盖空输入短路。
//
//	@receiver dao *MessageDAO
//	@param db *gorm.DB
//	@param ids []uint 候选 message ID
//	@return []uint 命中的 message ID
//	@return error
//	@author centonhuang
//	@update 2026-08-19 10:00:00
func (dao *MessageDAO) FilterTerminalToolCallIDs(db *gorm.DB, ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var records []*dbmodel.Message
	err := db.Model(&dbmodel.Message{}).
		Select([]string{constant.FieldID}).
		Where(constant.DBConditionWhereIDIn, ids).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.DBJSONConditionAssistantRole).
		Where(constant.DBJSONConditionHasToolCalls).
		Find(&records).Error
	if err != nil {
		return nil, err
	}

	return lo.Map(records, func(m *dbmodel.Message, _ int) uint { return m.ID }), nil
}
