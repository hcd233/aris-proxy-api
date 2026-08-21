package model

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// DemoConfig Demo 演示配置数据库模型（单行表，固定 ID=1）
//
// Modules 用 text + serializer:json 持久化字符串数组；更新必须走 struct Save
// 而非 Updates(map)，否则 serializer 不触发（同 Model.Capabilities 的约束）。
type DemoConfig struct {
	ID           uint      `json:"id" gorm:"column:id;primary_key;auto_increment;comment:配置ID"`
	LoginEnabled bool      `json:"login_enabled" gorm:"column:login_enabled;not null;default:false;comment:Demo登录入口开关"`
	Modules      []string  `json:"modules" gorm:"column:modules;not null;default:'[]';comment:开放模块列表;serializer:json"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"comment:更新时间"`
}

func (DemoConfig) TableName() string {
	return constant.DemoConfigTableName
}
