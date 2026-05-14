package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

// ModelDAO Model表DAO
type ModelDAO struct {
	baseDAO[dbmodel.Model]
}
