package enum

// BreakerState 熔断器状态。
//
//	@author centonhuang
//	@update 2026-08-20 10:00:00
type BreakerState = uint8

const (

	// StateClosed 关闭：请求正常放行，统计滑动窗口错误率。
	//
	//	@author centonhuang
	//	@update 2026-08-20 10:00:00
	StateClosed BreakerState = iota

	// StateOpen 打开：请求快速失败，等待 OpenTimeout 后进入半开。
	//
	//	@author centonhuang
	//	@update 2026-08-20 10:00:00
	StateOpen

	// StateHalfOpen 半开：限量放行探测请求，成功即关闭，失败重新打开。
	//
	//	@author centonhuang
	//	@update 2026-08-20 10:00:00
	StateHalfOpen
)
