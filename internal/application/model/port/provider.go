package port

import "context"

// PricingQuote 公开定价（展示单位：USD/1M tokens）
type PricingQuote struct {
	Input         float64
	Output        float64
	CacheCreation float64
	CacheRead     float64
}

// PricingQuoteProvider 公开定价来源（由 infrastructure/modelsdev 实现）
type PricingQuoteProvider interface {
	// Quote 按模型 ID 精确匹配查询；未命中 ok=false；拉取失败返回 err
	Quote(ctx context.Context, modelID string) (PricingQuote, bool, error)
}
