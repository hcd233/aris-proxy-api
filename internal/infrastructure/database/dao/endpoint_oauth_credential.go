package dao

import dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"

// EndpointOAuthCredentialDAO OAuth凭据表DAO
type EndpointOAuthCredentialDAO struct {
	baseDAO[dbmodel.EndpointOAuthCredential]
}
