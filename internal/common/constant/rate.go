package constant

import "time"

const (

	// PeriodOAuth2Callback OAuth2回调限频周期
	//	@update 2025-11-12 11:27:05
	PeriodOAuth2Callback = 5 * time.Second

	// LimitOAuth2Callback OAuth2回调限频
	//	@update 2025-11-12 11:26:56
	LimitOAuth2Callback = 16
)
