// Package model defines the database schema for the model.
//
//	update 2026-03-18 10:00:00
package model

import "github.com/hcd233/aris-proxy-api/internal/dto"

// Tool 工具数据库模型
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type Tool struct {
	BaseModel
	ID       uint             `json:"id" gorm:"column:id;primary_key;auto_increment;comment:工具ID"`
	Tool     *dto.UnifiedTool `json:"tool" gorm:"column:tool;not null;comment:工具;serializer:json"`
	CheckSum string           `json:"check_sum" gorm:"column:check_sum;not null;default:'';comment:校验和"`
}
