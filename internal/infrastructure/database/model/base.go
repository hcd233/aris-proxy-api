package model

import (
	"time"
)

// BaseModel 基础模型
//
//	@author centonhuang
//	@update 2025-09-30 16:37:21
type BaseModel struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;comment:ID"`
	CreatedAt time.Time `json:"created_at" gorm:"comment:创建时间"`
	UpdatedAt time.Time `json:"updated_at" gorm:"comment:更新时间"`
	DeletedAt int64     `json:"deleted_at" gorm:"column:deleted_at;default:0;comment:删除时间，默认为0"`
}

// Models undefined
//
//	update 2024-10-29 12:43:4
var Models = []interface{}{
	&User{},
}
