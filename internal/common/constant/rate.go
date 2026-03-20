package constant

import "time"

const (

	// PeriodOAuth2Callback OAuth2回调限频周期
	//	@update 2025-11-12 11:27:05
	PeriodOAuth2Callback = 5 * time.Second

	// LimitOAuth2Callback OAuth2回调限频
	//	@update 2025-11-12 11:26:56
	LimitOAuth2Callback = 16

	// PeriodCallProxyLLM 调用代理LLM限频周期
	//	@update 2025-11-12 11:27:05
	PeriodCallProxyLLM = 1 * time.Second

	// LimitCallProxyLLM 调用代理LLM限频
	//	@update 2025-11-12 11:26:56
	LimitCallProxyLLM = 100
)
