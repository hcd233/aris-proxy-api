package model

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// DemoSession demo 会话白名单（admin 选取供 demo 只读访问的会话）
type DemoSession struct {
	ID        uint      `json:"id" gorm:"column:id;primary_key;auto_increment;comment:ID"`
	SessionID uint      `json:"session_id" gorm:"column:session_id;uniqueIndex;not null;comment:会话ID"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at;comment:创建时间"`
}

func (DemoSession) TableName() string {
	return constant.DemoSessionTableName
}
