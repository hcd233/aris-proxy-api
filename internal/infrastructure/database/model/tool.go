// Package model defines the database schema for the model.
//
//	update 2026-03-18 10:00:00
package model

import "github.com/hcd233/aris-proxy-api/internal/common/vo"

// Tool 工具数据库模型
//
//	@author centonhuang
//	@update 2026-08-10 10:00:00
//
// 索引说明（bugfix/tool-checksum-dedup-2026-08-10）：
//   - idx_tool_checksum_deleted 为 (check_sum, deleted_at) 复合唯一索引，语义是
//     “活跃行内 checksum 唯一”，与去重查询的 deleted_at = 0 条件对齐，
//     并为将来引入 tool 软删除留出空间。
//   - DeletedAt 在此重声明以覆盖 BaseModel.DeletedAt，仅为给本表挂唯一索引 tag，
//     避免污染其他继承 BaseModel 的表。
type Tool struct {
	BaseModel
	ID        uint            `json:"id" gorm:"column:id;primary_key;auto_increment;comment:工具ID"`
	Tool      *vo.UnifiedTool `json:"tool" gorm:"column:tool;not null;comment:工具;serializer:json"`
	CheckSum  string          `json:"check_sum" gorm:"column:check_sum;not null;default:'';uniqueIndex:idx_tool_checksum_deleted,priority:1;comment:校验和"`
	DeletedAt int64           `json:"deleted_at" gorm:"column:deleted_at;default:0;uniqueIndex:idx_tool_checksum_deleted,priority:2;comment:删除时间，默认为0"`
}
