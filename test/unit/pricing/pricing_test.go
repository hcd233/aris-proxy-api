package pricing

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// mustPricing 构造已计价（USD）的定价值对象，构造失败即终止测试。
func mustPricing(t *testing.T, rules []vo.PricingRule) vo.Pricing {
	t.Helper()
	p, err := vo.NewPricing(enum.CurrencyUSD, rules)
	if err != nil {
		t.Fatalf("vo.NewPricing() error = %v", err)
	}
	return p
}

var defaultRule = vo.PricingRule{InputMicro: 2_000_000, OutputMicro: 8_000_000}

func TestNewPricingValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		currency enum.Currency
		rules    []vo.PricingRule
		wantErr  bool
	}{
		{"未计价且无规则", enum.CurrencyNone, nil, false},
		{"未计价却有规则", enum.CurrencyNone, []vo.PricingRule{defaultRule}, true},
		{"计价但无规则", enum.CurrencyUSD, nil, true},
		{"非法币种", enum.Currency("JPY"), []vo.PricingRule{defaultRule}, true},
		{"已废弃币种CNY", enum.Currency("CNY"), []vo.PricingRule{defaultRule}, true},
		{"单条默认规则", enum.CurrencyUSD, []vo.PricingRule{defaultRule}, false},
		{"两条无条件规则", enum.CurrencyUSD, []vo.PricingRule{defaultRule, defaultRule}, true},
		{"无默认规则区间连续无界末档", enum.CurrencyUSD, []vo.PricingRule{{ContextMax: 200_000, InputMicro: 1}, {ContextMin: 200_000, InputMicro: 2}}, false},
		{"无默认规则区间连续有界末档", enum.CurrencyUSD, []vo.PricingRule{{ContextMax: 200_000, InputMicro: 1}}, false},
		{"无默认规则区间断档", enum.CurrencyUSD, []vo.PricingRule{{ContextMax: 100_000, InputMicro: 1}, {ContextMin: 200_000, ContextMax: 300_000, InputMicro: 2}}, true},
		{"无默认规则区间不从零起", enum.CurrencyUSD, []vo.PricingRule{{ContextMin: 100_000, ContextMax: 200_000, InputMicro: 1}}, true},
		{"无默认规则纯时段规则", enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: []vo.TimeWindow{{Start: "09:00", End: "10:00"}}, InputMicro: 1}}, true},
		{"负价", enum.CurrencyUSD, []vo.PricingRule{{InputMicro: -1}}, true},
		{"超上限", enum.CurrencyUSD, []vo.PricingRule{{InputMicro: constant.PricingMaxPriceMicro + 1}}, true},
		{"区间非法", enum.CurrencyUSD, []vo.PricingRule{{ContextMin: 10, ContextMax: 10, InputMicro: 1}, defaultRule}, true},
		{"窗口时刻非法", enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: []vo.TimeWindow{{Start: "9:00", End: "10:00"}}, InputMicro: 1}, defaultRule}, true},
		{"窗口时区非法", enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: []vo.TimeWindow{{Start: "09:00", End: "10:00", Timezone: "Mars/Base"}}, InputMicro: 1}, defaultRule}, true},
		{"窗口起止相等", enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: []vo.TimeWindow{{Start: "09:00", End: "09:00"}}, InputMicro: 1}, defaultRule}, true},
		{"窗口days越界", enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: []vo.TimeWindow{{Start: "09:00", End: "10:00", Days: []int{0}}}, InputMicro: 1}, defaultRule}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := vo.NewPricing(tc.currency, tc.rules)
			if (err != nil) != tc.wantErr {
				t.Fatalf("vo.NewPricing() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestPricingMatchFirstHitAndDefault(t *testing.T) {
	t.Parallel()
	p := mustPricing(t, []vo.PricingRule{
		{ContextMax: 200_000, InputMicro: 1_000_000}, // 有条件
		{InputMicro: 3_000_000},                      // 默认
	})
	if got := p.Match(time.Now(), 100).InputMicro; got != 1_000_000 {
		t.Fatalf("first-hit InputMicro = %d", got)
	}
	if got := p.Match(time.Now(), 500_000).InputMicro; got != 3_000_000 {
		t.Fatalf("fallback InputMicro = %d", got)
	}
}

func TestPricingMatchBeyondTopBandClampsToTopTier(t *testing.T) {
	t.Parallel()
	// 无默认规则：区间 [0,272000)∪[272000,1050000) 平铺模型上下文，
	// 超出末档上界的理论命中回落末档价（末档延伸语义），不按零价计费
	p := mustPricing(t, []vo.PricingRule{
		{ContextMax: 272_000, InputMicro: 10_000_000, OutputMicro: 50_000_000},
		{ContextMin: 272_000, ContextMax: 1_050_000, InputMicro: 20_000_000, OutputMicro: 75_000_000},
	})
	if got := p.Match(time.Now(), 100_000).InputMicro; got != 10_000_000 {
		t.Fatalf("low band InputMicro = %d", got)
	}
	if got := p.Match(time.Now(), 500_000).InputMicro; got != 20_000_000 {
		t.Fatalf("top band InputMicro = %d", got)
	}
	if got := p.Match(time.Now(), 1_200_000).InputMicro; got != 20_000_000 {
		t.Fatalf("beyond top band must clamp to top tier, got %d", got)
	}
}

func TestPricingMatchTimeWindow(t *testing.T) {
	t.Parallel()
	// 22:00–02:00 跨午夜 + 仅周五/周六 + UTC
	p := mustPricing(t, []vo.PricingRule{
		{TimeWindows: []vo.TimeWindow{{Days: []int{5, 6}, Start: "22:00", End: "02:00"}}, InputMicro: 1},
		defaultRule,
	})
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("parse time: %v", err)
		}
		return v
	}
	// 2026-10-02 是周五
	if got := p.Match(at("2026-10-02T23:00:00Z"), 0).InputMicro; got != 1 {
		t.Fatalf("friday 23:00 should hit window, got %d", got)
	}
	// 跨午夜落到周六 01:00，窗口 days 含 6 → 命中
	if got := p.Match(at("2026-10-03T01:00:00Z"), 0).InputMicro; got != 1 {
		t.Fatalf("cross-midnight saturday 01:00 should hit window, got %d", got)
	}
	if got := p.Match(at("2026-10-02T21:00:00Z"), 0).InputMicro; got != 2_000_000 {
		t.Fatalf("friday 21:00 should miss window, got %d", got)
	}
	if got := p.Match(at("2026-10-04T23:00:00Z"), 0).InputMicro; got != 2_000_000 {
		t.Fatalf("sunday 23:00 should miss window (days=[5,6]), got %d", got)
	}
}

func TestPricingMatchTimeWindowTimezoneDST(t *testing.T) {
	t.Parallel()
	// America/New_York 09:00–10:00：冬令时 = 14:00Z 附近，夏令时 = 13:00Z 附近
	p := mustPricing(t, []vo.PricingRule{
		{TimeWindows: []vo.TimeWindow{{Start: "09:00", End: "10:00", Timezone: "America/New_York"}}, InputMicro: 7},
		{InputMicro: 1},
	})
	winter, err := time.Parse(time.RFC3339, "2026-01-15T14:30:00Z")
	if err != nil {
		t.Fatalf("parse winter: %v", err)
	}
	summer, err := time.Parse(time.RFC3339, "2026-07-15T13:30:00Z")
	if err != nil {
		t.Fatalf("parse summer: %v", err)
	}
	if got := p.Match(winter, 0).InputMicro; got != 7 {
		t.Fatalf("winter 09:30 ET should hit, got %d", got)
	}
	if got := p.Match(summer, 0).InputMicro; got != 7 {
		t.Fatalf("summer 09:30 ET should hit, got %d", got)
	}
	if got := p.Match(winter.Add(-time.Hour), 0).InputMicro; got != 1 {
		t.Fatalf("winter 08:30 ET should miss, got %d", got)
	}
}

func TestPricingMatchContextTierHalfOpen(t *testing.T) {
	t.Parallel()
	p := mustPricing(t, []vo.PricingRule{
		{ContextMin: 200_000, InputMicro: 2}, // >200k 档（无上限）
		{ContextMax: 200_000, InputMicro: 1}, // ≤200k 档
		{InputMicro: 3},
	})
	if got := p.Match(time.Now(), 199_999).InputMicro; got != 1 {
		t.Fatalf("199999 tier = %d", got)
	}
	if got := p.Match(time.Now(), 200_000).InputMicro; got != 2 {
		t.Fatalf("200000 falls to next tier = %d", got)
	}
	if got := p.Match(time.Now(), 250_000).InputMicro; got != 2 {
		t.Fatalf("250000 tier = %d", got)
	}
}

func TestPricingRuleCostBreakdown(t *testing.T) {
	t.Parallel()
	rule := vo.PricingRule{
		InputMicro:       800_000, // $0.8/1M
		OutputMicro:      4_000_000,
		CacheCreateMicro: 1_000_000,
		CacheReadMicro:   80_000,
	}
	// 1000 in + 500 out + 0 + 250 cacheRead
	// 800000*1000/1e6=800; 4000000*500/1e6=2000; 80000*250/1e6=20 → 2820
	b := rule.CostBreakdown(1000, 500, 0, 250)
	if b.InputMicro != 800 || b.OutputMicro != 2000 || b.CacheCreateMicro != 0 || b.CacheReadMicro != 20 || b.Total() != 2820 {
		t.Fatalf("CostBreakdown = %+v total=%d, want 800/2000/0/20 total 2820", b, b.Total())
	}
	// 半入：500000 tokens × 1 微单位/1M = 0.5 → 1
	if got := (vo.PricingRule{InputMicro: 1}).CostBreakdown(500_000, 0, 0, 0).Total(); got != 1 {
		t.Fatalf("round half = %d, want 1", got)
	}
	// 免费规则
	if got := (vo.PricingRule{}).CostBreakdown(1_000_000, 1_000_000, 1_000_000, 1_000_000).Total(); got != 0 {
		t.Fatalf("free rule = %d, want 0", got)
	}
	// 大数不溢出：1e12 微单位 × 1e9 tokens / 1e6 = 1e15
	if got := (vo.PricingRule{InputMicro: constant.PricingMaxPriceMicro}).CostBreakdown(1_000_000_000, 0, 0, 0).Total(); got != 1_000_000_000_000_000 {
		t.Fatalf("big = %d", got)
	}
}

func TestPricingBaseRule(t *testing.T) {
	t.Parallel()
	// 默认规则在后、时段规则在前：基础档必须跳过时段规则取无条件默认
	withDefault := mustPricing(t, []vo.PricingRule{
		{TimeWindows: []vo.TimeWindow{{Start: "00:00", End: "08:00"}}, InputMicro: 1},
		{InputMicro: 3},
	})
	if r, ok := withDefault.BaseRule(); !ok || r.InputMicro != 3 {
		t.Fatalf("BaseRule with default = %+v ok=%v, want InputMicro 3", r, ok)
	}
	// 无默认规则的连续区间：基础档为起点 0 的区间
	bands := mustPricing(t, []vo.PricingRule{
		{ContextMin: 200_000, InputMicro: 2},
		{ContextMax: 200_000, InputMicro: 1},
	})
	if r, ok := bands.BaseRule(); !ok || r.InputMicro != 1 {
		t.Fatalf("BaseRule with bands = %+v ok=%v, want InputMicro 1", r, ok)
	}
	if _, ok := (vo.Pricing{}).BaseRule(); ok {
		t.Fatal("unpriced must have no base rule")
	}
}

func TestNewPricingLimits(t *testing.T) {
	t.Parallel()
	tooManyRules := make([]vo.PricingRule, constant.PricingMaxRules+1)
	for i := range tooManyRules {
		tooManyRules[i] = vo.PricingRule{TimeWindows: []vo.TimeWindow{{Start: "00:00", End: "01:00"}}}
	}
	tooManyRules[0] = defaultRule
	if _, err := vo.NewPricing(enum.CurrencyUSD, tooManyRules); err == nil {
		t.Fatal("rules over limit must be rejected")
	}
	if _, err := vo.NewPricing(enum.CurrencyUSD, tooManyRules[:constant.PricingMaxRules]); err != nil {
		t.Fatalf("rules at limit must pass: %v", err)
	}

	windows := make([]vo.TimeWindow, constant.PricingMaxTimeWindows+1)
	for i := range windows {
		windows[i] = vo.TimeWindow{Start: "00:00", End: "01:00", Timezone: "Asia/Shanghai"}
	}
	if _, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: windows}, defaultRule}); err == nil {
		t.Fatal("time windows over limit must be rejected")
	}
	if _, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: windows[:constant.PricingMaxTimeWindows]}, defaultRule}); err != nil {
		t.Fatalf("time windows at limit must pass: %v", err)
	}

	days := []vo.TimeWindow{{Start: "00:00", End: "01:00", Days: []int{1, 2, 3, 4, 5, 6, 7, 1}}}
	if _, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{TimeWindows: days}, defaultRule}); err == nil {
		t.Fatal("days over limit must be rejected")
	}
}

func TestNewPricingInvalidTimezoneNotCached(t *testing.T) {
	t.Parallel()
	// 时区缓存只存成功结果：非法时区重复构造必须每次都报错
	rules := []vo.PricingRule{{TimeWindows: []vo.TimeWindow{{Start: "09:00", End: "10:00", Timezone: "Mars/Base"}}}, defaultRule}
	for range 2 {
		if _, err := vo.NewPricing(enum.CurrencyUSD, rules); err == nil {
			t.Fatal("invalid timezone must be rejected every time")
		}
	}
}
