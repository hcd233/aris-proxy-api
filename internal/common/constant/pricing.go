package constant

import "time"

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
	// PricingModelsDevCacheTTL models.dev 定价文档的 Redis 缓存时长。
	PricingModelsDevCacheTTL = 24 * time.Hour
	// PricingModelsDevMaxDocBytes models.dev 定价文档读取上限（防异常大响应）。
	PricingModelsDevMaxDocBytes = 16 << 20
	// PricingParseBitSize 价格字符串解析的浮点位宽。
	PricingParseBitSize = 64
	// PricingModelsDevParsedTTL models.dev 解析结果的进程内缓存时长（避免每次请求重解析数 MB 文档）。
	PricingModelsDevParsedTTL = 1 * time.Hour
	// PricingMaxRules 单模型定价规则条数上限。
	PricingMaxRules = 32
	// PricingMaxTimeWindows 单条规则时段窗口数上限。
	PricingMaxTimeWindows = 8
	// PricingDaysPerWeek 时段窗口星期取值上限（1=周一…7=周日），也是 days 数组长度上限。
	PricingDaysPerWeek = 7
)

// ModelsDevOfficialProviders models.dev 报价去重优先级：官方渠道优先（列表序靠前者胜）。
var ModelsDevOfficialProviders = []string{"openai", "anthropic", "google", "deepseek", "moonshotai", "zhipuai"}
