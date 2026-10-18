package dao

import dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"

// EndpointOAuthCredentialDAO OAuth凭据表DAO
type EndpointOAuthCredentialDAO struct {
	baseDAO[dbmodel.EndpointOAuthCredential]
}
