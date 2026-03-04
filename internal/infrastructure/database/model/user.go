// Package model defines the database schema for the model.
//
//	update 2024-06-22 09:33:43
package model

import (
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// User 用户数据库模型
//
//	author centonhuang
//	update 2024-06-22 09:36:22
type User struct {
	BaseModel
	ID           uint            `json:"id" gorm:"column:id;primary_key;auto_increment;comment:用户ID"`
	Name         string          `json:"name" gorm:"column:name;not null;comment:用户名"`
	Email        string          `json:"email" gorm:"column:email;not null;comment:邮箱"`
	Avatar       string          `json:"avatar" gorm:"column:avatar;not null;comment:头像"`
	Permission   enum.Permission `json:"permission" gorm:"column:permission;not null;default:'reader';comment:权限"`
	LastLogin    time.Time       `json:"last_login" gorm:"column:last_login;comment:最后登录时间"`
	GithubBindID string          `json:"-" gorm:"unique;comment:Github绑定ID"`
	// QQBindID     string     `json:"-" gorm:"unique;comment:QQ绑定ID"`
	GoogleBindID string `json:"-" gorm:"unique;comment:Google绑定ID"`
}
