package constant

// 定价与计费常量（定价规则校验、时段窗口解析、费用计算）。
const (
	// PricingMaxPriceMicro 单价上限（微单位/1M tokens = 1e6 货币单位/1M tokens），
	// 保证费用计算两段乘法均在 int64 内（tokens ≤ 1e9）。
	PricingMaxPriceMicro int64 = 1_000_000_000_000
	// PricingTokensPerMillion 计价分母：单价语义为「每 1M tokens」。
	PricingTokensPerMillion int64 = 1_000_000
	// PricingMinutesPerHour 时段窗口的小时→分钟换算系数。
	PricingMinutesPerHour = 60
	// PricingTimeBoundMaxHour HH:MM 小时上限（23）。
	PricingTimeBoundMaxHour = 23
	// PricingTimeBoundMaxMinute HH:MM 分钟上限（59）。
	PricingTimeBoundMaxMinute = 59
)
