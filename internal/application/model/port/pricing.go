package port

import (
	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// PricingFromDTO wire 定价 → 值对象；校验统一走 vo.NewPricing（错误为 ErrValidation → 422）。
// 放在 application port 层：handler 只依赖 application，不得直触 domain。
//
//	@param d *dto.PricingDTO wire 定价（nil=未计价）
//	@return vo.Pricing
//	@return error
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func PricingFromDTO(d *dto.PricingDTO) (vo.Pricing, error) {
	if d == nil {
		return vo.Pricing{}, nil
	}
	rules := make([]vo.PricingRule, 0, len(d.Rules))
	for _, r := range d.Rules {
		rules = append(rules, vo.PricingRule{
			TimeWindows: lo.Map(r.TimeWindows, func(w dto.TimeWindowDTO, _ int) vo.TimeWindow {
				return vo.TimeWindow{Days: w.Days, Start: w.Start, End: w.End, Timezone: w.Timezone}
			}),
			ContextMin:         r.ContextMin,
			ContextMax:         r.ContextMax,
			InputMicro:         dto.PriceMicroFromDisplay(r.InputPrice),
			OutputMicro:        dto.PriceMicroFromDisplay(r.OutputPrice),
			CacheCreateMicro:   dto.PriceMicroFromDisplay(r.CacheCreationPrice),
			CacheCreate1hMicro: dto.PriceMicroFromDisplay(r.CacheCreation1hPrice),
			CacheReadMicro:     dto.PriceMicroFromDisplay(r.CacheReadPrice),
		})
	}
	return vo.NewPricing(d.Currency, rules)
}

// PricingToDTO 值对象 → wire 定价（未计价返回 nil，响应省略字段）。
//
//	@param p vo.Pricing
//	@return *dto.PricingDTO
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func PricingToDTO(p vo.Pricing) *dto.PricingDTO {
	if !p.IsPriced() {
		return nil
	}
	return &dto.PricingDTO{
		Currency: p.Currency(),
		Rules: lo.Map(p.Rules(), func(r vo.PricingRule, _ int) dto.PricingRuleDTO {
			return dto.PricingRuleDTO{
				TimeWindows: lo.Map(r.TimeWindows, func(w vo.TimeWindow, _ int) dto.TimeWindowDTO {
					return dto.TimeWindowDTO{Days: w.Days, Start: w.Start, End: w.End, Timezone: w.Timezone}
				}),
				ContextMin:           r.ContextMin,
				ContextMax:           r.ContextMax,
				InputPrice:           dto.PriceDisplayFromMicro(r.InputMicro),
				OutputPrice:          dto.PriceDisplayFromMicro(r.OutputMicro),
				CacheCreationPrice:   dto.PriceDisplayFromMicro(r.CacheCreateMicro),
				CacheCreation1hPrice: dto.PriceDisplayFromMicro(r.CacheCreate1hMicro),
				CacheReadPrice:       dto.PriceDisplayFromMicro(r.CacheReadMicro),
			}
		}),
	}
}
