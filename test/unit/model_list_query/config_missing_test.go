package model_list_query

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

func TestConfigMissing(t *testing.T) {
	t.Parallel()
	m, err := aggregate.CreateModel(1, "gpt-4", "gpt-4-0613", 10, true, 0, 0, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	got := m.MissingConfig()
	want := []enum.ConfigMissing{enum.ConfigMissingPricing, enum.ConfigMissingSpec}
	if !slices.Equal(got, want) {
		t.Fatalf("ConfigMissing = %v, want %v", got, want)
	}
}

func TestConfigMissingFullyConfigured(t *testing.T) {
	t.Parallel()
	m, err := aggregate.CreateModel(2, "gpt-4", "gpt-4-0613", 10, true, 128000, 4096, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	priced, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{InputMicro: 1}})
	if err != nil {
		t.Fatalf("NewPricing: %v", err)
	}
	m.UpdatePricing(priced)
	if got := m.MissingConfig(); len(got) != 0 {
		t.Fatalf("ConfigMissing = %v, want empty", got)
	}
}

func TestConfigMissingFreeModelNotMissingPricing(t *testing.T) {
	t.Parallel()
	m, err := aggregate.CreateModel(3, "gpt-4", "gpt-4-0613", 10, true, 128000, 4096, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	free, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{}}) // 四价全 0 = 免费模型
	if err != nil {
		t.Fatalf("NewPricing: %v", err)
	}
	m.UpdatePricing(free)
	if got := m.MissingConfig(); len(got) != 0 {
		t.Fatalf("免费模型不应报缺失, got %v", got)
	}
}
