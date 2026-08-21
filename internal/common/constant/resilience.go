package constant

import "time"

// ResilienceWindowBucketCount 熔断滑动窗口固定时间桶数（窗口按此均分，统计误差 ≤ 桶宽）。
//
//	@update 2026-08-20 10:00:00
const ResilienceWindowBucketCount = 6

// ResilienceMinWindow 熔断滑动窗口最小值。窗口小于桶数秒数时单桶宽度为 0，
// record 的桶索引计算除零 panic；配置读取处以此值 clamp 下界。
//
//	@update 2026-08-21 10:00:00
const ResilienceMinWindow = ResilienceWindowBucketCount * time.Second

// CircuitOpenErrorTemplate 熔断打开错误消息模板（参数：key、retry after 时长）。
//
//	@update 2026-08-20 10:00:00
const CircuitOpenErrorTemplate = "circuit breaker open for upstream %s, retry after %s"

// BulkheadFullErrorTemplate 信号量满载错误消息模板（参数：key、并发上限）。
//
//	@update 2026-08-20 10:00:00
const BulkheadFullErrorTemplate = "bulkhead full for upstream %s (limit %d)"
