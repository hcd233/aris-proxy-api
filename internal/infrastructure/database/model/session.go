// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

import "time"

// Session 会话数据库模型
//
// 列设计说明（refactor/session-list-baseline-perf-2026-06-07）：
//
//   - MessageCount / ToolCount 是 message_ids / tool_ids 长度的物化冗余列，
//     由 sessionRepository.Save 在写入路径同步维护，
//     存量数据由 database.PostMigrate 一次性回填。
//     原因：旧实现读路径一直走 COALESCE(jsonb_array_length(message_ids::jsonb), 0)，
//     planner 无法对这类表达式列建索引，sort by message_count/tool_count 永远全表扫；
//     物化列后投影直接读 int 列，sort 也可以走真实列。
//
//     @author centonhuang
//     @update 2026-06-07 21:50:00
type Session struct {
	BaseModel
	ID           uint              `json:"id" gorm:"column:id;primary_key;auto_increment;comment:会话ID"`
	APIKeyName   string            `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:API密钥名称"`
	MessageIDs   []uint            `json:"message_ids" gorm:"column:message_ids;not null;comment:消息ID列表;serializer:json"`
	ToolIDs      []uint            `json:"tool_ids" gorm:"column:tool_ids;not null;comment:工具ID列表;serializer:json"`
	MessageCount int               `json:"message_count" gorm:"column:message_count;not null;default:0;comment:消息数量(冗余 message_ids 长度)"`
	ToolCount    int               `json:"tool_count" gorm:"column:tool_count;not null;default:0;comment:工具数量(冗余 tool_ids 长度)"`
	Questions    []uint            `json:"questions" gorm:"column:questions;comment:用户提问消息ID列表(仅role=user且tool_call_id为空);serializer:json"`
	Metadata     map[string]string `json:"metadata" gorm:"column:metadata;comment:请求元数据;serializer:json"`
	Score        *int              `json:"score" gorm:"column:score;comment:人工评分(1-5)"`
	ScoredAt     *time.Time        `json:"scored_at" gorm:"column:scored_at;comment:评分时间"`
}
