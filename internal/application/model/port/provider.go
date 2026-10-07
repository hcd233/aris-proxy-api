package port

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

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

// ModelSpec 公开模型规格 + 分档报价
type ModelSpec struct {
	ContextLength   int64                // 上下文窗口（tokens），无数据为 0
	MaxOutputTokens int64                // 最大输出（tokens），无数据为 0
	InputModalities []enum.InputModality // 输入模态（枚举序，未知值已丢弃），无数据为 nil
	Quote           PricingQuote         // 分档报价
}

// ModelSpecProvider 公开模型规格来源（由 infrastructure/modelsdev 实现）
type ModelSpecProvider interface {
	// Describe 按模型 ID 精确匹配查询；未命中 ok=false；拉取失败返回 err
	Describe(ctx context.Context, modelID string) (ModelSpec, bool, error)
}
