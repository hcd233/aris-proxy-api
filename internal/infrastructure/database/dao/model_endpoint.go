package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// ModelEndpointDAO 模型端点DAO
//
//	@author centonhuang
//	@update 2026-04-04 10:00:00
type ModelEndpointDAO struct {
	baseDAO[dbmodel.ModelEndpoint]
}
