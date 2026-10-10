package vo

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"

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
// CacheCreateMicro 为 5m 档缓存创建价；CacheCreate1hMicro 为 1h 档价，0=回落 5m 档。
type PricingRule struct {
	TimeWindows        []TimeWindow
	ContextMin         int64
	ContextMax         int64
	InputMicro         int64
	OutputMicro        int64
	CacheCreateMicro   int64
	CacheCreate1hMicro int64
	CacheReadMicro     int64
}

// Pricing 模型定价：币种 + 规则表（顺序=匹配优先级）。
// currency=="" ⇔ rules 为空 ⇔ 未计价；非空时任意 prompt 有价可依：
// 要么恰含一条无条件默认规则，要么无时段规则的上下文区间从 0 连续覆盖（末档可有界）。
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
	if len(rules) > constant.PricingMaxRules {
		return Pricing{}, ierr.New(ierr.ErrValidation, "too many pricing rules")
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
	if defaults > 1 {
		return Pricing{}, ierr.New(ierr.ErrValidation, "pricing rules must contain at most one unconditional default rule")
	}
	if defaults == 0 {
		if err := validateContiguousBands(rules); err != nil {
			return Pricing{}, err
		}
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
// 全不中时回落最高档（无时段规则中 ContextMin 最大者，末档延伸语义），
// 计费永不落空为零价；未计价（规则为空）返回零值规则。
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
	return p.topBand()
}

// topBand 无时段规则中 ContextMin 最大者（超出末档上界时兜底计价）；不存在返回零值规则。
func (p Pricing) topBand() PricingRule {
	var (
		top   PricingRule
		found bool
	)
	for _, r := range p.rules {
		if len(r.TimeWindows) != 0 {
			continue
		}
		if !found || r.ContextMin > top.ContextMin {
			top, found = r, true
		}
	}
	return top
}

// BaseRule 基础档：prompt 为 0、不受时段限制时适用的规则（默认规则或起点为 0 的无时段区间），
// 用于对外导出单一参考价；未计价返回 false。
//
//	@receiver p Pricing
//	@return PricingRule
//	@return bool
//	@author centonhuang
//	@update 2026-10-08 10:00:00
func (p Pricing) BaseRule() (PricingRule, bool) {
	return lo.Find(p.rules, func(r PricingRule) bool {
		return len(r.TimeWindows) == 0 && r.matchContext(0)
	})
}

// CostBreakdown 费用四维拆分（微单位）：输入/输出/缓存创建/缓存读取。
type CostBreakdown struct {
	InputMicro       int64
	OutputMicro      int64
	CacheCreateMicro int64
	CacheReadMicro   int64
}

// Total 四维合计（微单位）。
func (b CostBreakdown) Total() int64 {
	return b.InputMicro + b.OutputMicro + b.CacheCreateMicro + b.CacheReadMicro
}

// CostBreakdown 按本规则单价计算四维费用拆分（微单位）；整段跳档由调用方 Match 选档。
// 缓存创建按 5m/1h 分档计价：CacheCreate1hMicro==0 时 1h 回落 5m 价；
// CacheCreateMicro 保持合计语义（5m 费用 + 1h 费用）。
//
//	@receiver r PricingRule
//	@param input int64 输入 token
//	@param output int64 输出 token
//	@param cacheCreate5m int64 5m 缓存创建 token
//	@param cacheCreate1h int64 1h 缓存创建 token
//	@param cacheRead int64 缓存读取 token
//	@return CostBreakdown
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func (r PricingRule) CostBreakdown(input, output, cacheCreate5m, cacheCreate1h, cacheRead int64) CostBreakdown {
	price1h := r.CacheCreate1hMicro
	if price1h == 0 {
		price1h = r.CacheCreateMicro
	}
	return CostBreakdown{
		InputMicro:       roundCost(r.InputMicro, input),
		OutputMicro:      roundCost(r.OutputMicro, output),
		CacheCreateMicro: roundCost(r.CacheCreateMicro, cacheCreate5m) + roundCost(price1h, cacheCreate1h),
		CacheReadMicro:   roundCost(r.CacheReadMicro, cacheRead),
	}
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

// validateContiguousBands 无默认规则时校验无时段规则的上下文区间从 0 连续覆盖（末档可有界）：
// 区间内直接命中，超出末档上界的理论命中由 Match 回落末档价兜底，永不按零价计费。
func validateContiguousBands(rules []PricingRule) error {
	bands := lo.Filter(rules, func(r PricingRule, _ int) bool {
		return len(r.TimeWindows) == 0
	})
	if len(bands) == 0 {
		return ierr.New(ierr.ErrValidation, "pricing rules must contain at least one unconditional context band")
	}
	slices.SortFunc(bands, func(a, b PricingRule) int {
		return cmp.Compare(a.ContextMin, b.ContextMin)
	})
	if bands[0].ContextMin != 0 {
		return ierr.New(ierr.ErrValidation, "pricing context bands must start at zero")
	}
	for i := 1; i < len(bands); i++ {
		if bands[i].ContextMin != bands[i-1].ContextMax {
			return ierr.New(ierr.ErrValidation, "pricing context bands must be contiguous")
		}
	}
	return nil
}

// normalizeWindows 校验并归一化时段窗口（解析时区与 HH:MM）。
func normalizeWindows(ws []TimeWindow) ([]normWindow, error) {
	if len(ws) > constant.PricingMaxTimeWindows {
		return nil, ierr.New(ierr.ErrValidation, "too many time windows")
	}
	out := make([]normWindow, 0, len(ws))
	for _, w := range ws {
		loc, err := loadLocation(w.Timezone)
		if err != nil {
			return nil, err
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
		if len(w.Days) > constant.PricingDaysPerWeek {
			return nil, ierr.New(ierr.ErrValidation, "too many time window days")
		}
		for _, d := range w.Days {
			if d < 1 || d > constant.PricingDaysPerWeek {
				return nil, ierr.New(ierr.ErrValidation, "time window day out of range")
			}
		}
		out = append(out, normWindow{days: slices.Clone(w.Days), startMin: start, endMin: end, loc: loc})
	}
	return out, nil
}

// locationCache IANA 时区名 → *time.Location。time.LoadLocation 每次读 zoneinfo 文件且不缓存，
// 而 NewPricing 在每次从库还原模型（含代理热路径）时都会执行，故做进程级缓存；
// 只缓存解析成功的时区，非法名每次重新报错。
var locationCache sync.Map

// loadLocation 解析时区名（空按 UTC），带进程级缓存。
func loadLocation(name string) (*time.Location, error) {
	if name == "" {
		return time.UTC, nil
	}
	if v, ok := locationCache.Load(name); ok {
		return v.(*time.Location), nil //nolint:forcetypeassert // 仅本函数写入，类型恒定
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, ierr.New(ierr.ErrValidation, "invalid timezone: "+name)
	}
	locationCache.Store(name, loc)
	return loc, nil
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
