// Package dao Image DAO
//
//	author centonhuang
//	update 2026-04-07 10:00:00
package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// ImageDAO 图片数据访问对象
//
//	@author centonhuang
//	@update 2026-04-07 10:00:00
type ImageDAO struct {
	baseDAO[dbmodel.Image]
}
