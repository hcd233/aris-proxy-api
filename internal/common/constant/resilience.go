package constant

// ResilienceWindowBucketCount 熔断滑动窗口固定时间桶数（窗口按此均分，统计误差 ≤ 桶宽）。
//
//	@update 2026-08-20 10:00:00
const ResilienceWindowBucketCount = 6

// CircuitOpenErrorTemplate 熔断打开错误消息模板（参数：key、retry after 时长）。
//
//	@update 2026-08-20 10:00:00
const CircuitOpenErrorTemplate = "circuit breaker open for upstream %s, retry after %s"
