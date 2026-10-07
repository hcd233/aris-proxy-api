package query

import (
	"context"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// prefillSpecHandler 从公开规格源导入表单初值（仅填充，永不自动改价）
type prefillSpecHandler struct {
	provider port.ModelSpecProvider
}

// NewPrefillSpecHandler 构造模型规格导入查询处理器
//
//	@param provider port.ModelSpecProvider 公开规格来源
//	@return port.PrefillSpecHandler
//	@author centonhuang
//	@update 2026-10-07 18:00:00
func NewPrefillSpecHandler(provider port.ModelSpecProvider) port.PrefillSpecHandler {
	return &prefillSpecHandler{provider: provider}
}

// Handle 按 upstream_model 精确匹配；未命中/拉取失败一律 found=false（录入不被阻塞）。
// 规格三件套（上下文/最大输出/输入模态）原样透传；报价的上下文分档映射为区间规则：
// 前档 [ContextMin, 下一档 ContextMin)，多档时末档上界取 spec.ContextLength（模型最大上下文），
// 区间平铺 [0, ContextLength) 不再追加兜底默认规则（超出末档上界由计价回落末档价）；
// 时段窗口 models.dev 无数据，不在导入范围（用户手填）。
//
//	@receiver h *prefillSpecHandler
//	@param ctx context.Context
//	@param q port.PrefillSpecQuery
//	@return *port.PrefillSpecResult
//	@return error 恒为 nil
//	@author centonhuang
//	@update 2026-10-07 18:00:00
func (h *prefillSpecHandler) Handle(ctx context.Context, q port.PrefillSpecQuery) (*port.PrefillSpecResult, error) {
	name := strings.TrimSpace(q.UpstreamModel)
	if name == "" {
		return &port.PrefillSpecResult{}, nil
	}
	spec, ok, err := h.provider.Describe(ctx, name)
	// 拉取失败/未命中统一降级为未命中（录入不被阻塞），因此只走正向分支
	if err == nil && ok {
		return &port.PrefillSpecResult{
			Found:           true,
			ContextLength:   spec.ContextLength,
			MaxOutputTokens: spec.MaxOutputTokens,
			InputModalities: spec.InputModalities,
			Currency:        enum.CurrencyUSD,
			Rules:           tiersToRules(spec.Quote.Tiers, spec.ContextLength),
		}, nil
	}
	return &port.PrefillSpecResult{}, nil
}

// tiersToRules 分档报价 → 区间规则：前档 [min, 下一档 min)；多档且 ContextLength 大于末档
// 起点时，末档区间为 [末档 min, ContextLength)，区间平铺 [0, ContextLength)；
// 单档或 ContextLength 不足时，末档清零区间归为无条件默认规则。
func tiersToRules(tiers []port.PricingTier, contextLength int64) []dto.PricingRuleDTO {
	rules := make([]dto.PricingRuleDTO, 0, len(tiers))
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
		return rules
	}
	last.ContextMin = 0
	last.ContextMax = 0
	return rules
}
