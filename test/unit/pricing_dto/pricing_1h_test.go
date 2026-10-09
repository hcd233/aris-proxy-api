package pricing_dto

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestPricingDTO1hRoundTrip(t *testing.T) {
	t.Parallel()
	in := &dto.PricingDTO{
		Currency: "USD",
		Rules: []dto.PricingRuleDTO{{
			InputPrice:           1.0,
			OutputPrice:          2.0,
			CacheCreationPrice:   1.25,
			CacheCreation1hPrice: 2.5,
			CacheReadPrice:       0.1,
		}},
	}
	p, err := port.PricingFromDTO(in)
	if err != nil {
		t.Fatalf("PricingFromDTO: %v", err)
	}
	out := port.PricingToDTO(p)
	if out == nil || len(out.Rules) != 1 {
		t.Fatalf("rules = %+v, want 1", out)
	}
	if out.Rules[0].CacheCreation1hPrice != 2.5 {
		t.Fatalf("CacheCreation1hPrice = %v, want 2.5", out.Rules[0].CacheCreation1hPrice)
	}
}

func TestPricingDTO1hDefaultZero(t *testing.T) {
	t.Parallel()
	in := &dto.PricingDTO{Currency: "USD", Rules: []dto.PricingRuleDTO{{InputPrice: 1.0, CacheCreationPrice: 1.25}}}
	p, err := port.PricingFromDTO(in)
	if err != nil {
		t.Fatalf("PricingFromDTO: %v", err)
	}
	out := port.PricingToDTO(p)
	if out == nil || len(out.Rules) != 1 {
		t.Fatalf("rules = %+v, want 1", out)
	}
	if got := out.Rules[0].CacheCreation1hPrice; got != 0 {
		t.Fatalf("CacheCreation1hPrice = %v, want 0 (回落语义)", got)
	}
}
