package dao

import dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"

type TriggerDAO struct {
	baseDAO[dbmodel.Trigger]
}
