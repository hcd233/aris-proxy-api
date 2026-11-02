package dao

import dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"

// ModelDAO Model表DAO
type ModelDAO struct {
	baseDAO[dbmodel.Model]
}
