package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

type BlockedDAO struct {
	baseDAO[dbmodel.Blocked]
}
