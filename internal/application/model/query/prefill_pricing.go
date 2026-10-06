package query

import (
	"context"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// prefillPricingHandler 从公开定价源导入表单初值（仅填充，永不自动改价）
type prefillPricingHandler struct {
	provider port.PricingQuoteProvider
}

// NewPrefillPricingHandler 构造定价导入查询处理器
//
//	@param provider port.PricingQuoteProvider 公开定价来源
//	@return port.PrefillPricingHandler
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func NewPrefillPricingHandler(provider port.PricingQuoteProvider) port.PrefillPricingHandler {
	return &prefillPricingHandler{provider: provider}
}

// Handle 按 upstream_model 精确匹配；未命中/拉取失败一律 found=false（录入不被阻塞）
//
//	@receiver h *prefillPricingHandler
//	@param ctx context.Context
//	@param q port.PrefillPricingQuery
//	@return *port.PrefillPricingResult
//	@return error 恒为 nil
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (h *prefillPricingHandler) Handle(ctx context.Context, q port.PrefillPricingQuery) (*port.PrefillPricingResult, error) {
	name := strings.TrimSpace(q.UpstreamModel)
	if name == "" {
		return &port.PrefillPricingResult{}, nil
	}
	quote, ok, err := h.provider.Quote(ctx, name)
	// 拉取失败/未命中统一降级为未命中（录入不被阻塞），因此只走正向分支
	if err == nil && ok {
		return &port.PrefillPricingResult{
			Found:              true,
			Currency:           enum.CurrencyUSD,
			InputPrice:         quote.Input,
			OutputPrice:        quote.Output,
			CacheCreationPrice: quote.CacheCreation,
			CacheReadPrice:     quote.CacheRead,
		}, nil
	}
	return &port.PrefillPricingResult{}, nil
}
