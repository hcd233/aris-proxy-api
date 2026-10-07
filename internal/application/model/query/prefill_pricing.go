package query

import (
	"context"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
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

// Handle 按 upstream_model 精确匹配；未命中/拉取失败一律 found=false（录入不被阻塞）。
// 报价的上下文分档映射为区间规则：前档 [ContextMin, 下一档 ContextMin)，末档上界取
// ContextLength（模型最大上下文），其后补一条同价无条件默认规则兜底；
// 时段窗口 models.dev 无数据，不在导入范围（用户手填）。
//
//	@receiver h *prefillPricingHandler
//	@param ctx context.Context
//	@param q port.PrefillPricingQuery
//	@return *port.PrefillPricingResult
//	@return error 恒为 nil
//	@author centonhuang
//	@update 2026-10-07 10:00:00
func (h *prefillPricingHandler) Handle(ctx context.Context, q port.PrefillPricingQuery) (*port.PrefillPricingResult, error) {
	name := strings.TrimSpace(q.UpstreamModel)
	if name == "" {
		return &port.PrefillPricingResult{}, nil
	}
	quote, ok, err := h.provider.Quote(ctx, name)
	// 拉取失败/未命中统一降级为未命中（录入不被阻塞），因此只走正向分支
	if err == nil && ok {
		return &port.PrefillPricingResult{
			Found:    true,
			Currency: enum.CurrencyUSD,
			Rules:    tiersToRules(quote.Tiers, q.ContextLength),
		}, nil
	}
	return &port.PrefillPricingResult{}, nil
}

// tiersToRules 分档报价 → 区间规则：前档 [min, 下一档 min)；多档且 ContextLength 大于末档
// 起点时，末档区间为 [末档 min, ContextLength)，其后追加价格相同的无条件默认规则兜底
// （满足「恰一条默认规则」校验，承接 ≥ContextLength 的理论命中，计费语义与末档一致）；
// 单档或 ContextLength 不足时，末档清零区间归为无条件默认规则（原行为）。
func tiersToRules(tiers []port.PricingTier, contextLength int64) []dto.PricingRuleDTO {
	rules := make([]dto.PricingRuleDTO, 0, len(tiers)+1)
	for i, tier := range tiers {
		rule := dto.PricingRuleDTO{
			ContextMin:         tier.ContextMin,
			InputPrice:         tier.Input,
			OutputPrice:        tier.Output,
			CacheCreationPrice: tier.CacheCreation,
			CacheReadPrice:     tier.CacheRead,
		}
		if i < len(tiers)-1 {
			rule.ContextMax = tiers[i+1].ContextMin
		}
		rules = append(rules, rule)
	}
	if len(rules) == 0 {
		return rules
	}
	last := &rules[len(rules)-1]
	if len(tiers) > 1 && contextLength > last.ContextMin {
		last.ContextMax = contextLength
		fallback := *last
		fallback.ContextMin = 0
		fallback.ContextMax = 0
		rules = append(rules, fallback)
		return rules
	}
	last.ContextMin = 0
	last.ContextMax = 0
	return rules
}
