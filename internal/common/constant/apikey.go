package constant

import "time"

const (
	APIKeyPrefix       = "sk-aris-"
	APIKeyCharset      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	APIKeyRandomLength = 24
	APIKeyMaxCount     = 5

	PeriodManageAPIKey = 1 * time.Minute
	LimitManageAPIKey  = 20
)
