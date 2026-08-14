package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

type TriggerDAO struct {
	baseDAO[dbmodel.Trigger]
}
