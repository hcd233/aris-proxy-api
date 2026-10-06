package port

import "context"

// PricingTier 公开定价的一档（USD/1M tokens）：ContextMin 起（含）的 prompt 量适用本档
type PricingTier struct {
	ContextMin    int64
	Input         float64
	Output        float64
	CacheCreation float64
	CacheRead     float64
}

// PricingQuote 公开定价（USD/1M tokens）：按上下文区间分档，Tiers 按 ContextMin 升序且首档为 0
type PricingQuote struct {
	Tiers []PricingTier
}

// PricingQuoteProvider 公开定价来源（由 infrastructure/modelsdev 实现）
type PricingQuoteProvider interface {
	// Quote 按模型 ID 精确匹配查询；未命中 ok=false；拉取失败返回 err
	Quote(ctx context.Context, modelID string) (PricingQuote, bool, error)
}
