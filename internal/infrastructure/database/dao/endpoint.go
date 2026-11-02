package dao

import dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"

// EndpointDAO Endpoint表DAO
type EndpointDAO struct {
	baseDAO[dbmodel.Endpoint]
}
