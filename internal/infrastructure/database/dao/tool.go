// Package dao Tool DAO
//
//	author centonhuang
//	update 2026-03-18 10:00:00
package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// ToolDAO 工具数据访问对象
//
//	@author centonhuang
//	@update 2026-03-18 10:00:00
type ToolDAO struct {
	baseDAO[dbmodel.Tool]
}
