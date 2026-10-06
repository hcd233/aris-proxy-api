package vo

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// TimeWindow 时段窗口：[Start,End) 半开区间，End<Start 表示跨午夜；
// Days 空=每天，成员 1=周一…7=周日；Timezone 为 IANA 时区名（空按 UTC）。
type TimeWindow struct {
	Days     []int
	Start    string
	End      string
	Timezone string
}

// PricingRule 定价规则。四价单位：微单位/1M tokens。
// 区间命中：ContextMin ≤ promptTokens < ContextMax（ContextMax==0 表无上限）。
type PricingRule struct {
	TimeWindows      []TimeWindow
	ContextMin       int64
	ContextMax       int64
	InputMicro       int64
	OutputMicro      int64
	CacheCreateMicro int64
	CacheReadMicro   int64
}

// Pricing 模型定价：币种 + 规则表（顺序=匹配优先级）。
// currency=="" ⇔ rules 为空 ⇔ 未计价；非空时恰含一条无条件默认规则兜底。
type Pricing struct {
	currency enum.Currency
	rules    []PricingRule
	norms    []normRule
}

// normWindow 时段窗口的归一化形态（构造时一次性解析，匹配路径零分配）。
type normWindow struct {
	days     []int
	startMin int
	endMin   int
	loc      *time.Location
}

// normRule 规则 + 归一化窗口。
type normRule struct {
	raw     PricingRule
	windows []normWindow
}

// NewPricing 构造并校验 Pricing。
//
//	@param currency enum.Currency 计价币种（空=未计价）
//	@param rules []PricingRule 规则表（顺序=匹配优先级）
//	@return Pricing
//	@return error 非法配置返回 ierr.ErrValidation
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func NewPricing(currency enum.Currency, rules []PricingRule) (Pricing, error) {
	if !currency.Valid() {
		return Pricing{}, ierr.New(ierr.ErrValidation, "invalid pricing currency")
	}
	if currency == enum.CurrencyNone {
		if len(rules) != 0 {
			return Pricing{}, ierr.New(ierr.ErrValidation, "pricing rules must be empty when unpriced")
		}
		return Pricing{}, nil
	}
	if len(rules) == 0 {
		return Pricing{}, ierr.New(ierr.ErrValidation, "pricing rules cannot be empty")
	}
	defaults := 0
	norms := make([]normRule, 0, len(rules))
	for _, r := range rules {
		if err := validateRule(r); err != nil {
			return Pricing{}, err
		}
		if r.isDefault() {
			defaults++
		}
		windows, err := normalizeWindows(r.TimeWindows)
		if err != nil {
			return Pricing{}, err
		}
		norms = append(norms, normRule{raw: r, windows: windows})
	}
	if defaults != 1 {
		return Pricing{}, ierr.New(ierr.ErrValidation, "pricing rules must contain exactly one unconditional default rule")
	}
	return Pricing{currency: currency, rules: slices.Clone(rules), norms: norms}, nil
}

// IsPriced 是否已计价（currency 非空）。
//
//	@receiver p Pricing
//	@return bool
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (p Pricing) IsPriced() bool {
	return p.currency != enum.CurrencyNone
}

// Currency 计价币种。
//
//	@receiver p Pricing
//	@return enum.Currency
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (p Pricing) Currency() enum.Currency {
	return p.currency
}

// Rules 规则表快照（副本）。
//
//	@receiver p Pricing
//	@return []PricingRule
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (p Pricing) Rules() []PricingRule {
	return slices.Clone(p.rules)
}

// Match 按顺序返回第一条「时段命中 ∧ 区间命中」的规则；
// 条件规则全不中时返回无条件默认规则（构造校验保证存在）。
//
//	@receiver p Pricing
//	@param at time.Time 调用时刻
//	@param promptTokens int64 prompt 总 token（input+cacheCreation+cacheRead）
//	@return PricingRule
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (p Pricing) Match(at time.Time, promptTokens int64) PricingRule {
	for _, n := range p.norms {
		if n.matchTime(at) && n.raw.matchContext(promptTokens) {
			return n.raw
		}
	}
	return PricingRule{}
}

// ComputeCost 按命中规则计算估算费用（微单位）；整段跳档由调用方 Match 选档。
//
//	@receiver p Pricing
//	@param rule PricingRule 命中的规则
//	@param input int64 输入 token
//	@param output int64 输出 token
//	@param cacheCreate int64 缓存创建 token
//	@param cacheRead int64 缓存读取 token
//	@return int64 估算费用（微单位）
//	@author centonhuang
//	@update 2026-10-05 10:00:00
func (p Pricing) ComputeCost(rule PricingRule, input, output, cacheCreate, cacheRead int64) int64 {
	return roundCost(rule.InputMicro, input) +
		roundCost(rule.OutputMicro, output) +
		roundCost(rule.CacheCreateMicro, cacheCreate) +
		roundCost(rule.CacheReadMicro, cacheRead)
}

// isDefault 是否无条件默认规则（全时段 + 全区间）。
func (r PricingRule) isDefault() bool {
	return len(r.TimeWindows) == 0 && r.ContextMin == 0 && r.ContextMax == 0
}

// matchContext 上下文区间命中（半开区间，ContextMax==0 表无上限）。
func (r PricingRule) matchContext(promptTokens int64) bool {
	if promptTokens < r.ContextMin {
		return false
	}
	return r.ContextMax == 0 || promptTokens < r.ContextMax
}

// matchTime 时段命中：无窗口=全时段，多窗口 OR。
func (n normRule) matchTime(at time.Time) bool {
	if len(n.windows) == 0 {
		return true
	}
	return slices.ContainsFunc(n.windows, func(w normWindow) bool {
		return w.contains(at)
	})
}

// contains 时刻是否落在窗口内（[start,end) 半开；end<=start 跨午夜）。
func (w normWindow) contains(at time.Time) bool {
	local := at.In(w.loc)
	if len(w.days) > 0 {
		iso := int(local.Weekday())
		if iso == 0 {
			iso = 7
		}
		if !slices.Contains(w.days, iso) {
			return false
		}
	}
	mins := local.Hour()*constant.PricingMinutesPerHour + local.Minute()
	if w.endMin <= w.startMin { // 跨午夜：命中起点之后或终点之前
		return mins >= w.startMin || mins < w.endMin
	}
	return mins >= w.startMin && mins < w.endMin
}

// validateRule 单条规则字段校验（区间与单价范围；窗口由 normalizeWindows 校验）。
func validateRule(r PricingRule) error {
	for _, v := range []int64{r.InputMicro, r.OutputMicro, r.CacheCreateMicro, r.CacheReadMicro} {
		if v < 0 || v > constant.PricingMaxPriceMicro {
			return ierr.New(ierr.ErrValidation, "pricing out of range")
		}
	}
	if r.ContextMin < 0 {
		return ierr.New(ierr.ErrValidation, "context min cannot be negative")
	}
	if r.ContextMax != 0 && r.ContextMax <= r.ContextMin {
		return ierr.New(ierr.ErrValidation, "context max must be 0 or greater than context min")
	}
	return nil
}

// normalizeWindows 校验并归一化时段窗口（解析时区与 HH:MM）。
func normalizeWindows(ws []TimeWindow) ([]normWindow, error) {
	out := make([]normWindow, 0, len(ws))
	for _, w := range ws {
		loc := time.UTC
		if w.Timezone != "" {
			l, err := time.LoadLocation(w.Timezone)
			if err != nil {
				return nil, ierr.New(ierr.ErrValidation, "invalid timezone: "+w.Timezone)
			}
			loc = l
		}
		start, err := parseHHMM(w.Start)
		if err != nil {
			return nil, err
		}
		end, err := parseHHMM(w.End)
		if err != nil {
			return nil, err
		}
		if start == end {
			return nil, ierr.New(ierr.ErrValidation, "time window start equals end")
		}
		for _, d := range w.Days {
			if d < 1 || d > 7 {
				return nil, ierr.New(ierr.ErrValidation, "time window day out of range")
			}
		}
		out = append(out, normWindow{days: slices.Clone(w.Days), startMin: start, endMin: end, loc: loc})
	}
	return out, nil
}

// parseHHMM 解析 "HH:MM" 为当日分钟数。
func parseHHMM(s string) (int, error) {
	invalid := func() (int, error) {
		return 0, ierr.New(ierr.ErrValidation, "invalid time bound: "+s)
	}
	parts := strings.Split(s, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return invalid()
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > constant.PricingTimeBoundMaxHour {
		return invalid()
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > constant.PricingTimeBoundMaxMinute {
		return invalid()
	}
	return h*constant.PricingMinutesPerHour + m, nil
}

// roundCost priceMicro × tokens / 1e6 的四舍五入（非负域整数半入，等价 math.Round）。
// 拆「整数段 + 余数段」两段计算，避免 (priceMicro × tokens) 直接乘法溢出。
func roundCost(priceMicro, tokens int64) int64 {
	whole := (priceMicro / constant.PricingTokensPerMillion) * tokens
	frac := (priceMicro % constant.PricingTokensPerMillion) * tokens
	return whole + (frac+constant.PricingTokensPerMillion/2)/constant.PricingTokensPerMillion
}
