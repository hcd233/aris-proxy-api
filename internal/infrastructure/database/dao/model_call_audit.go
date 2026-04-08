// Package dao ModelCallAudit DAO
//
//	author centonhuang
//	update 2026-04-09 10:00:00
package dao

import (
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// ModelCallAuditDAO 模型调用审计DAO
//
//	@author centonhuang
//	@update 2026-04-09 10:00:00
type ModelCallAuditDAO struct {
	baseDAO[dbmodel.ModelCallAudit]
}
