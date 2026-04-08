// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

import "time"

// Session 会话数据库模型
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type Session struct {
	BaseModel
	ID             uint              `json:"id" gorm:"column:id;primary_key;auto_increment;comment:会话ID"`
	APIKeyName     string            `json:"api_key_name" gorm:"column:api_key_name;not null;default:'';comment:API密钥名称"`
	MessageIDs     []uint            `json:"message_ids" gorm:"column:message_ids;not null;comment:消息ID列表;serializer:json"`
	ToolIDs        []uint            `json:"tool_ids" gorm:"column:tool_ids;not null;comment:工具ID列表;serializer:json"`
	Summary        string            `json:"summary" gorm:"column:summary;not null;default:'';comment:会话总结(5-10字)"`
	Metadata       map[string]string `json:"metadata" gorm:"column:metadata;comment:请求元数据;serializer:json"`
	CoherenceScore float64           `json:"coherence_score" gorm:"column:coherence_score;default:0;comment:连贯性评分(1-10)"`
	DepthScore     float64           `json:"depth_score" gorm:"column:depth_score;default:0;comment:深度评分(1-10)"`
	ValueScore     float64           `json:"value_score" gorm:"column:value_score;default:0;comment:价值评分(1-10)"`
	TotalScore     float64           `json:"total_score" gorm:"column:total_score;default:0;comment:总分(1-10)"`
	ScoreVersion   string            `json:"score_version" gorm:"column:score_version;not null;default:'';comment:评分算法版本"`
	ScoredAt       *time.Time        `json:"scored_at" gorm:"column:scored_at;comment:评分时间"`
	ScoreError     string            `json:"score_error" gorm:"column:score_error;not null;default:'';comment:评分失败原因"`
	SummarizeError string            `json:"summarize_error" gorm:"column:summarize_error;not null;default:'';comment:总结失败原因"`
}
