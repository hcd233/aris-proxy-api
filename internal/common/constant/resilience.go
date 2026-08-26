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

// ── 配置钳制下界/上界（2026-08-26 CR：极端配置会静默改变组件行为）──

// ResilienceMinMinRequests 熔断最少请求数下界。0/负值会让单次失败立即触发
// 熔断判定（含 0/0 NaN 的语义歧义）。
//
//	@update 2026-08-26 10:00:00
const ResilienceMinMinRequests = 1

// ResilienceMinErrorThreshold / ResilienceMaxErrorThreshold 熔断错误率阈值
// 的合法区间。≤0 时任何含失败计数的窗口都满足判定（阈值失效），>1 时永不触发。
//
//	@update 2026-08-26 10:00:00
const (
	ResilienceMinErrorThreshold = 0.01
	ResilienceMaxErrorThreshold = 1.0
)

// ResilienceMinOpenTimeout 熔断打开保持时长下界。过小（如 0）会让 Open 状态
// 立即转 HalfOpen，熔断失去隔离意义。
//
//	@update 2026-08-26 10:00:00
const ResilienceMinOpenTimeout = 1 * time.Second

// ResilienceMinHalfOpenMaxRequests 半开探测并发下界。0 会让每个 OpenTimeout
// 周期只放行 1 个探测且成功即关闭，行为退化。
//
//	@update 2026-08-26 10:00:00
const ResilienceMinHalfOpenMaxRequests = 1

// ResilienceMinMaxConcurrent bulkhead 每 key 并发上限下界。≤0 时 channel
// 容量为 0/负值，所有请求等满 AcquireTimeout 被拒（服务自锁）。
//
//	@update 2026-08-26 10:00:00
const ResilienceMinMaxConcurrent = 1

// ResilienceMinAcquireTimeout bulkhead 获取槽位等待时长下界。过小时
// time.After 与 send 的 select 竞态不可控，行为不确定。
//
//	@update 2026-08-26 10:00:00
const ResilienceMinAcquireTimeout = 100 * time.Millisecond

// ResilienceRegistryMaxKeys 容错注册表（per-key breaker / 信号量槽）软上限。
// key 数量正常等于 endpoint 数（个位数到几十）；超限整体重建仅丢失熔断窗口
// （可重新积累），防 endpoint 高频增删场景下的无界增长。
//
//	@update 2026-08-26 10:00:00
const ResilienceRegistryMaxKeys = 1024

// CircuitOpenErrorTemplate 熔断打开错误消息模板（参数：key、retry after 时长）。
//
//	@update 2026-08-20 10:00:00
const CircuitOpenErrorTemplate = "circuit breaker open for upstream %s, retry after %s"

// BulkheadFullErrorTemplate 信号量满载错误消息模板（参数：key、并发上限）。
//
//	@update 2026-08-20 10:00:00
const BulkheadFullErrorTemplate = "bulkhead full for upstream %s (limit %d)"
