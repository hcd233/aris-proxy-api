package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// ProxyAPIKeyDAO 代理API密钥DAO
//
//	@author centonhuang
//	@update 2026-04-04 10:00:00
type ProxyAPIKeyDAO struct {
	baseDAO[dbmodel.ProxyAPIKey]
}
