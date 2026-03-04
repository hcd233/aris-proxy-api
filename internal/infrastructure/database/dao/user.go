package dao

import (
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// UserDAO 用户DAO
//
//	author centonhuang
//	update 2024-10-17 02:30:24
type UserDAO struct {
	baseDAO[model.User]
}
