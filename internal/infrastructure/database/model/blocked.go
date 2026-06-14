package model

import "github.com/hcd233/aris-proxy-api/internal/common/constant"

type Blocked struct {
	BaseModel
	Word     string `json:"word" gorm:"column:word;type:varchar(512);not null;uniqueIndex:idx_word_deleted_at,priority:1;comment:敏感词"`
	HitCount uint   `json:"hit_count" gorm:"column:hit_count;not null;default:0;comment:命中次数"`
}

func (Blocked) TableName() string {
	return constant.BlockedTableName
}
