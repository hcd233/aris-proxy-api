package enum

// ConfigMissing 模型配置缺失项标识（model/list 与 upstream/list 的 configMissing 成员）。
type ConfigMissing string

const (
	// ConfigMissingPricing 未计价（pricing_currency 为空；免费模型不算）
	ConfigMissingPricing ConfigMissing = "pricing"
	// ConfigMissingSpec 未填规格（context_length 为 0）
	ConfigMissingSpec ConfigMissing = "spec"
)
