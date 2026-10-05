# 模型计费（成本估算）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Model 加定价规则表（时段窗口 × 上下文区间 × 四类单价），LLM 调用审计按当时价格落库估算费用，并在 upstream/audit/Dashboard 展示成本。

**Architecture:** 计价规则作为值对象挂 `llmproxy.Model` 聚合（`models` 表 JSON 列），审计收尾 seam `recordModelCall` 匹配规则并计算微单位费用写入 `model_call_audits`（`cost_micro` + `pricing_currency` 快照），成本查询走现有 audit 仓储 SQL 聚合（分币种、分组），wire 层只出现展示单位浮点。

**Tech Stack:** Go（GORM/PostgreSQL、Huma v2、fx、Redis）、Next.js（静态导出）；测试用标准库 `testing`（go.mod 无 testify）。

**Spec:** `docs/superpowers/specs/2026-10-05-model-pricing-cost-design.md`

**Spec 微调（已在计划中采纳，实现时以此为准）：** spec 写 "`CreateModel` 入参追加 pricing"，实现改为聚合 `UpdatePricing` 注入（仓储补水与创建命令共用，与既有 `SetUserID`/`SetTimestamps` 补水模式一致，避免 8 参构造继续膨胀）；`ComputeCost` 舍入用整数半入（`(frac+5e5)/1e6`）替代 `math.Round`，非负域等价且零浮点。其余按 spec 执行。

## Global Constraints

- 金额一律 int64 微单位（1e-6 货币单位）入账；**禁止** `int64(v*scale+0.5)` 截断与浮点入库；wire 层金额只用展示单位浮点 + `currency`
- 币种枚举仅 `""`/`CNY`/`USD`；`currency==""` ⇔ 未计价（`cost_micro` NULL）；`currency!=""` 且四价全 0 ⇔ 免费（`cost_micro`=0）
- 规则匹配：顺序第一命中；**恰好一条无条件默认规则**；区间按 `promptTokens = input+cacheCreation+cacheRead` 整段跳档；时段窗口 `[start,end)` 半开、`end<start` 跨午夜、IANA 时区
- 单价上限 `MaxPriceMicro = 1_000_000_000_000` 微单位/1M tokens；`time_windows.days` 成员 ∈ [1,7]（1=周一…7=周日）
- DTO 遵循 `huma-dto-conventions` skill（wire 平铺、Body 包装、错误 422）；改 `internal/dto/**` 或 huma 路由前先读该 skill
- 编辑任何 Go 文件前先跑 `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <file>`（不许截断输出）；涉及 lo/mo 用法加载对应 skill
- 不新增第三方依赖；不引入新表/新索引；DB 变更仅加列（AutoMigrate）
- fx 装配三处同步：handler 构造、`bootstrap/modules` Provide、`bootstrap/router.go` 的 `routeParams` 结构与赋值（漏一处运行时 nil panic，build 不报）
- 每任务：先写失败测试 → 跑出失败 → 最小实现 → 跑绿 → commit（conventional commits，中文摘要）；提交前 `make lint` 相关子集不红
- 全局收尾才跑 `make test` 全量与 ponytail-review

## File Structure

| 文件 | 职责 |
|---|---|
| `internal/common/enum/currency.go`（新） | 币种枚举 |
| `internal/domain/llmproxy/vo/pricing.go`（新） | Pricing/PricingRule/TimeWindow 值对象、校验、Match、ComputeCost |
| `internal/infrastructure/database/model/model.go` | Model 加 `pricing_rules`/`pricing_currency` 列与行结构 |
| `internal/common/constant/sql.go` | 字段常量 + `ModelRepoFieldsFull` 扩列 |
| `internal/domain/llmproxy/aggregate/model.go` | 聚合挂 pricing、`UpdatePricing`/`Pricing()` |
| `internal/infrastructure/repository/model_repository.go` | DB↔聚合 pricing 映射（含 `updateModelTx` map 键） |
| `internal/domain/modelcall/aggregate/audit.go`、`internal/dto/asynctask.go`、`internal/infrastructure/pool/store_pool.go`、`internal/infrastructure/repository/audit_repository.go`、`internal/application/llmproxy/usecase/recorder.go` | 计费落库链路 |
| `internal/dto/pricing.go`（新）、`internal/dto/model.go`、`internal/dto/upstream.go`、`internal/dto/audit.go`、`internal/dto/audit_stats.go` | wire 契约 |
| `internal/application/model/port/handler.go`、`command/create_model.go`、`command/update_model.go`、`internal/handler/model.go` | CRUD 接线 |
| `internal/infrastructure/modelsdev/client.go`（新）、`internal/application/model/query/prefill_pricing.go`（新） | models.dev 导入辅助 |
| `internal/application/audit/query/cost_summary.go`、`cost_distribution.go`、`service.go`、`internal/router/audit.go`、`internal/router/model.go` | 成本查询与路由 |
| `cmd/server/main.go` | tzdata 内嵌 |
| `CONTEXT.md`、`web/CONTEXT.md` | 领域词汇 |
| `test/e2e/model_pricing/`（新） | E2E |
| `web/src/lib/types.ts`、`web/src/lib/money.ts`（新）、`web/src/app/(dashboard)/upstream/pricing-editor.tsx`（新）、`model-dialog.tsx`、`audit` 页组件、`page.tsx`、`web/src/locales/*.json`、`web/src/lib/api-client.ts` | 前端 |

---

### Task 1: 币种枚举 + 定价值对象（校验/匹配/计价）

**Files:**
- Create: `internal/common/enum/currency.go`
- Create: `internal/domain/llmproxy/vo/pricing.go`
- Test: `internal/domain/llmproxy/vo/pricing_test.go`

**Interfaces:**
- Consumes: `internal/common/ierr`（`ierr.New(ierr.ErrValidation, ...)`）
- Produces:
  - `enum.Currency`（`CurrencyNone/CurrencyCNY/CurrencyUSD`，`Valid() bool`）
  - `vo.NewPricing(currency enum.Currency, rules []PricingRule) (Pricing, error)`
  - `vo.Pricing`：`IsPriced() bool` / `Currency() enum.Currency` / `Rules() []PricingRule` / `Match(at time.Time, promptTokens int64) PricingRule` / `ComputeCost(rule PricingRule, input, output, cacheCreate, cacheRead int64) int64`
  - `vo.PricingRule{TimeWindows []TimeWindow; ContextMin, ContextMax, InputMicro, OutputMicro, CacheCreateMicro, CacheReadMicro int64}`、`vo.TimeWindow{Days []int; Start, End, Timezone string}`
  - 常量 `vo.MaxPriceMicro int64 = 1_000_000_000_000`

- [ ] **Step 1: 写失败测试** `internal/domain/llmproxy/vo/pricing_test.go`

```go
package vo

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func mustPricing(t *testing.T, currency enum.Currency, rules []PricingRule) Pricing {
	t.Helper()
	p, err := NewPricing(currency, rules)
	if err != nil {
		t.Fatalf("NewPricing() error = %v", err)
	}
	return p
}

var defaultRule = PricingRule{InputMicro: 2_000_000, OutputMicro: 8_000_000}

func TestNewPricingValidation(t *testing.T) {
	cases := []struct {
		name     string
		currency enum.Currency
		rules    []PricingRule
		wantErr  bool
	}{
		{"未计价且无规则", enum.CurrencyNone, nil, false},
		{"未计价却有规则", enum.CurrencyNone, []PricingRule{defaultRule}, true},
		{"计价但无规则", enum.CurrencyCNY, nil, true},
		{"非法币种", enum.Currency("JPY"), []PricingRule{defaultRule}, true},
		{"单条默认规则", enum.CurrencyUSD, []PricingRule{defaultRule}, false},
		{"两条无条件规则", enum.CurrencyUSD, []PricingRule{defaultRule, defaultRule}, true},
		{"缺默认规则", enum.CurrencyUSD, []PricingRule{{ContextMax: 200_000, InputMicro: 1}, defaultRule}, true},
		{"缺默认规则仅条件", enum.CurrencyUSD, []PricingRule{{ContextMax: 200_000, InputMicro: 1}}, true},
		{"负价", enum.CurrencyUSD, []PricingRule{{InputMicro: -1}}, true},
		{"超上限", enum.CurrencyUSD, []PricingRule{{InputMicro: MaxPriceMicro + 1}}, true},
		{"区间非法", enum.CurrencyUSD, []PricingRule{{ContextMin: 10, ContextMax: 10, InputMicro: 1}, defaultRule}, true},
		{"窗口时刻非法", enum.CurrencyUSD, []PricingRule{{TimeWindows: []TimeWindow{{Start: "9:00", End: "10:00"}}, InputMicro: 1}, defaultRule}, true},
		{"窗口时区非法", enum.CurrencyUSD, []PricingRule{{TimeWindows: []TimeWindow{{Start: "09:00", End: "10:00", Timezone: "Mars/Base"}}, InputMicro: 1}, defaultRule}, true},
		{"窗口起止相等", enum.CurrencyUSD, []PricingRule{{TimeWindows: []TimeWindow{{Start: "09:00", End: "09:00"}}, InputMicro: 1}, defaultRule}, true},
		{"窗口days越界", enum.CurrencyUSD, []PricingRule{{TimeWindows: []TimeWindow{{Start: "09:00", End: "10:00", Days: []int{0}}}, InputMicro: 1}, defaultRule}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPricing(tc.currency, tc.rules)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewPricing() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestPricingMatchFirstHitAndDefault(t *testing.T) {
	p := mustPricing(t, enum.CurrencyUSD, []PricingRule{
		{ContextMax: 200_000, InputMicro: 1_000_000}, // 有条件
		{InputMicro: 3_000_000},                      // 默认（位置在前也不影响兜底语义之外的顺序）
	})
	// 顺序敏感：有条件规则在前，命中区间即第一条
	if got := p.Match(time.Now(), 100).InputMicro; got != 1_000_000 {
		t.Fatalf("first-hit InputMicro = %d", got)
	}
	if got := p.Match(time.Now(), 500_000).InputMicro; got != 3_000_000 {
		t.Fatalf("fallback InputMicro = %d", got)
	}
}

func TestPricingMatchTimeWindow(t *testing.T) {
	// 22:00–02:00 跨午夜 + 仅周五/周六 + UTC
	p := mustPricing(t, enum.CurrencyCNY, []PricingRule{
		{TimeWindows: []TimeWindow{{Days: []int{5, 6}, Start: "22:00", End: "02:00"}}, InputMicro: 1},
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
	if got := p.Match(at("2026-10-03T01:00:00Z"), 0).InputMicro; got != 1 {
		t.Fatalf("cross-midnight saturday 01:00 should hit window (weekday=sat), got %d", got)
	}
	if got := p.Match(at("2026-10-02T21:00:00Z"), 0).InputMicro; got != 2_000_000 {
		t.Fatalf("friday 21:00 should miss window, got %d", got)
	}
	if got := p.Match(at("2026-10-04T23:00:00Z"), 0).InputMicro; got != 2_000_000 {
		t.Fatalf("sunday 23:00 should miss window (days=[5,6]), got %d", got)
	}
}

func TestPricingMatchTimeWindowTimezoneDST(t *testing.T) {
	// America/New_York 09:00–10:00：冬令时 = 14:00Z，夏令时 = 13:00Z
	p := mustPricing(t, enum.CurrencyUSD, []PricingRule{
		{TimeWindows: []TimeWindow{{Start: "09:00", End: "10:00", Timezone: "America/New_York"}}, InputMicro: 7},
		{InputMicro: 1},
	})
	winter, _ := time.Parse(time.RFC3339, "2026-01-15T14:30:00Z")
	summer, _ := time.Parse(time.RFC3339, "2026-07-15T13:30:00Z")
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
	p := mustPricing(t, enum.CurrencyUSD, []PricingRule{
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

func TestPricingComputeCost(t *testing.T) {
	p := mustPricing(t, enum.CurrencyUSD, []PricingRule{defaultRule})
	rule := PricingRule{
		InputMicro:       800_000,  // $0.8/1M
		OutputMicro:      4_000_000, // $4/1M
		CacheCreateMicro: 1_000_000,
		CacheReadMicro:   80_000,
	}
	// 1000 in + 500 out + 0 + 250 cacheRead
	// 800000*1000/1e6=800; 4000000*500/1e6=2000; 80000*250/1e6=20 → 2820
	if got := p.ComputeCost(rule, 1000, 500, 0, 250); got != 2820 {
		t.Fatalf("ComputeCost = %d, want 2820", got)
	}
	// 舍入半入：1 token × 0.5 微单位/1M 不为负、0.5 按 1 计
	if got := p.ComputeCost(PricingRule{InputMicro: 1}, 500_000, 0, 0, 0); got != 1 {
		t.Fatalf("round half = %d, want 1", got)
	}
	// 免费规则
	if got := p.ComputeCost(PricingRule{}, 1_000_000, 1_000_000, 1_000_000, 1_000_000); got != 0 {
		t.Fatalf("free rule = %d, want 0", got)
	}
	// 大数不溢出
	if got := p.ComputeCost(PricingRule{InputMicro: MaxPriceMicro}, 1_000_000_000, 0, 0, 0); got != 1_000_000_000_000_000_000/1_000_000 {
		t.Fatalf("big = %d", got)
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/domain/llmproxy/vo/...`
Expected: FAIL（`undefined: NewPricing` 等编译错误）

- [ ] **Step 3: 实现** `internal/common/enum/currency.go`

```go
package enum

// Currency 计价币种；空值表示未计价。
type Currency string

const (
	CurrencyNone Currency = ""
	CurrencyCNY  Currency = "CNY"
	CurrencyUSD  Currency = "USD"
)

// Valid 币种是否合法（含空值=未计价）。
func (c Currency) Valid() bool {
	switch c {
	case CurrencyNone, CurrencyCNY, CurrencyUSD:
		return true
	default:
		return false
	}
}
```

实现 `internal/domain/llmproxy/vo/pricing.go`（完整实现要点，与测试一一对应）：

```go
package vo

import (
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// MaxPriceMicro 单价上限（微单位/1M tokens = 1e6 货币单位/1M tokens），
// 保证 roundCost 两段乘法均在 int64 内（tokens ≤ 1e9）。
const MaxPriceMicro int64 = 1_000_000_000_000

const tokensPerMillion int64 = 1_000_000

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

type normWindow struct {
	days     map[int]struct{}
	startMin int
	endMin   int
	loc      *time.Location
}

type normRule struct {
	raw     PricingRule
	windows []normWindow
}

// NewPricing 构造并校验 Pricing。
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
	return Pricing{currency: currency, rules: rules, norms: norms}, nil
}

// IsPriced 是否已计价（currency 非空）。
func (p Pricing) IsPriced() bool { return p.currency != enum.CurrencyNone }

// Currency 计价币种。
func (p Pricing) Currency() enum.Currency { return p.currency }

// Rules 规则表（只读快照）。
func (p Pricing) Rules() []PricingRule { return lo.Slice(p.rules, 0, len(p.rules)) }

// Match 按顺序返回第一条「时段命中 ∧ 区间命中」的规则；
// 条件规则全不中时返回无条件默认规则（构造校验保证存在）。
func (p Pricing) Match(at time.Time, promptTokens int64) PricingRule {
	for _, n := range p.norms {
		if n.matchTime(at) && n.raw.matchContext(promptTokens) {
			return n.raw
		}
	}
	return PricingRule{}
}

// ComputeCost 按命中规则计算估算费用（微单位）；整段跳档由调用方 Match 选档。
func (p Pricing) ComputeCost(rule PricingRule, input, output, cacheCreate, cacheRead int64) int64 {
	return roundCost(rule.InputMicro, input) +
		roundCost(rule.OutputMicro, output) +
		roundCost(rule.CacheCreateMicro, cacheCreate) +
		roundCost(rule.CacheReadMicro, cacheRead)
}

func (r PricingRule) isDefault() bool {
	return len(r.TimeWindows) == 0 && r.ContextMin == 0 && r.ContextMax == 0
}

func (r PricingRule) matchContext(promptTokens int64) bool {
	if promptTokens < r.ContextMin {
		return false
	}
	return r.ContextMax == 0 || promptTokens < r.ContextMax
}

func (n normRule) matchTime(at time.Time) bool {
	if len(n.windows) == 0 {
		return true
	}
	return lo.SomeBy(n.windows, func(w normWindow) bool { return w.contains(at) })
}

func (w normWindow) contains(at time.Time) bool {
	local := at.In(w.loc)
	if len(w.days) > 0 {
		iso := int(local.Weekday())
		if iso == 0 {
			iso = 7
		}
		if _, ok := w.days[iso]; !ok {
			return false
		}
	}
	mins := local.Hour()*60 + local.Minute()
	if w.endMin <= w.startMin { // 跨午夜：命中起点之后或终点之前
		return mins >= w.startMin || mins < w.endMin
	}
	return mins >= w.startMin && mins < w.endMin
}

func validateRule(r PricingRule) error {
	for _, v := range []int64{r.InputMicro, r.OutputMicro, r.CacheCreateMicro, r.CacheReadMicro} {
		if v < 0 || v > MaxPriceMicro {
			return ierr.New(ierr.ErrValidation, "pricing out of range")
		}
	}
	if r.ContextMin < 0 {
		return ierr.New(ierr.ErrValidation, "context_min cannot be negative")
	}
	if r.ContextMax != 0 && r.ContextMax <= r.ContextMin {
		return ierr.New(ierr.ErrValidation, "context_max must be 0 or greater than context_min")
	}
	return nil
}

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
		days := make(map[int]struct{}, len(w.Days))
		for _, d := range w.Days {
			if d < 1 || d > 7 {
				return nil, ierr.New(ierr.ErrValidation, "time window day out of range")
			}
			days[d] = struct{}{}
		}
		out = append(out, normWindow{days: days, startMin: start, endMin: end, loc: loc})
	}
	return out, nil
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, ierr.New(ierr.ErrValidation, "invalid time bound: "+s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, ierr.New(ierr.ErrValidation, "invalid time bound: "+s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, ierr.New(ierr.ErrValidation, "invalid time bound: "+s)
	}
	return h*60 + m, nil
}

// roundCost priceMicro × tokens / 1e6 的四舍五入（非负域整数半入，等价 math.Round）。
// 拆「整数段 + 余数段」两段计算，避免 (priceMicro × tokens) 直接乘法溢出。
func roundCost(priceMicro, tokens int64) int64 {
	whole := (priceMicro / tokensPerMillion) * tokens
	frac := (priceMicro % tokensPerMillion) * tokens
	return whole + (frac+tokensPerMillion/2)/tokensPerMillion
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/domain/llmproxy/vo/... ./internal/common/enum/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/common/enum/currency.go internal/domain/llmproxy/vo/pricing.go internal/domain/llmproxy/vo/pricing_test.go
git commit -m "feat(pricing): 定价值对象——币种枚举、规则表校验、时段/区间匹配、费用计算"
```

---

### Task 2: Model 聚合与持久化（DB 列 + 仓储映射）

**Files:**
- Modify: `internal/infrastructure/database/model/model.go`（Model 加列 + 行结构）
- Modify: `internal/common/constant/sql.go`（字段常量 + `ModelRepoFieldsFull`）
- Modify: `internal/domain/llmproxy/aggregate/model.go`（pricing 字段、`UpdatePricing`、`Pricing()`）
- Modify: `internal/infrastructure/repository/model_repository.go`（`toModelAggregate`/`toModelDBModel`/`updateModelTx`）
- Test: `internal/domain/llmproxy/aggregate/model_pricing_test.go`

**Interfaces:**
- Consumes: Task 1 的 `vo.NewPricing`/`vo.Pricing`/`enum.Currency`
- Produces:
  - `aggregate.Model.UpdatePricing(pricing vo.Pricing) error`、`aggregate.Model.Pricing() vo.Pricing`
  - DB 列 `models.pricing_rules`（JSON，`serializer:json`）、`models.pricing_currency`
  - `constant.FieldModelPricingRules` / `constant.FieldModelPricingCurrency`，并纳入 `constant.ModelRepoFieldsFull`
  - DB 行结构 `dbmodel.ModelPricingRule` / `dbmodel.ModelTimeWindow`（snake_case JSON 键，与 spec 存储示例一致）

- [ ] **Step 1: 写失败测试** `internal/domain/llmproxy/aggregate/model_pricing_test.go`

```go
package aggregate

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

func TestModelUpdatePricing(t *testing.T) {
	m, err := CreateModel(1, vo.EndpointAlias("gpt"), "gpt-4", 2, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel() error = %v", err)
	}
	if m.Pricing().IsPriced() {
		t.Fatalf("new model should be unpriced")
	}
	p, err := vo.NewPricing(enum.CurrencyCNY, []vo.PricingRule{{InputMicro: 1_000_000}})
	if err != nil {
		t.Fatalf("NewPricing() error = %v", err)
	}
	if err := m.UpdatePricing(p); err != nil {
		t.Fatalf("UpdatePricing() error = %v", err)
	}
	if got := m.Pricing().Currency(); got != enum.CurrencyCNY {
		t.Fatalf("Currency() = %v", got)
	}
	// 清空定价（未计价）
	if err := m.UpdatePricing(vo.Pricing{}); err != nil {
		t.Fatalf("UpdatePricing(empty) error = %v", err)
	}
	if m.Pricing().IsPriced() {
		t.Fatalf("pricing should be cleared")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/domain/llmproxy/aggregate/...`
Expected: FAIL（`m.Pricing undefined`）

- [ ] **Step 3: 实现**

`internal/infrastructure/database/model/model.go`：Model 结构体 `Capabilities` 字段后追加，并在同文件新增行结构：

```go
	PricingRules    []ModelPricingRule `json:"pricing_rules" gorm:"column:pricing_rules;not null;default:'[]';comment:定价规则(微单位单价,数组顺序=匹配优先级);serializer:json"`
	PricingCurrency string             `json:"pricing_currency" gorm:"column:pricing_currency;not null;default:'';comment:计价币种(''/CNY/USD,空=未计价)"`

// ModelTimeWindow 时段窗口 DB 形态
type ModelTimeWindow struct {
	Days     []int  `json:"days"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Timezone string `json:"timezone"`
}

// ModelPricingRule 定价规则 DB 形态（四价：微单位/1M tokens）
type ModelPricingRule struct {
	TimeWindows             []ModelTimeWindow `json:"time_windows"`
	ContextMin              int64             `json:"context_min"`
	ContextMax              int64             `json:"context_max"`
	InputPriceMicro         int64             `json:"input_price_micro"`
	OutputPriceMicro        int64             `json:"output_price_micro"`
	CacheCreationPriceMicro int64             `json:"cache_creation_price_micro"`
	CacheReadPriceMicro     int64             `json:"cache_read_price_micro"`
}
```

`internal/common/constant/sql.go`：字段常量区追加 `FieldModelPricingRules = "pricing_rules"`、`FieldModelPricingCurrency = "pricing_currency"`，并把两字段插进 `ModelRepoFieldsFull`（`FieldModelCapabilities` 之后、`FieldCreatedAt` 之前）——**漏改白名单会导致读路径取不到定价列**。

`internal/domain/llmproxy/aggregate/model.go`：

```go
	pricing vo.Pricing // 模型定价（currency + 规则表）
```

新增方法（`Capabilities()` getter 之后）：

```go
// Pricing 模型定价
func (m *Model) Pricing() vo.Pricing { return m.pricing }

// UpdatePricing 更新模型定价（vo 构造已校验，此处仅替换）
func (m *Model) UpdatePricing(pricing vo.Pricing) error {
	m.pricing = pricing
	return nil
}
```

`internal/infrastructure/repository/model_repository.go`：

`toModelAggregate` 内 `model.SetTimestamps(...)` 前追加（DB→vo 转换函数 `pricingFromDB` 同文件新增）：

```go
	pricing := pricingFromDB(m.PricingRules, m.PricingCurrency)
	if err := model.UpdatePricing(pricing); err != nil {
		return nil, err
	}
```

`toModelDBModel` 返回结构体追加两字段（vo→DB 转换 `pricingToDB` 同文件新增）：

```go
		PricingRules:    pricingToDB(m.Pricing()),
		PricingCurrency: string(m.Pricing().Currency()),
```

`updateModelTx` 的 `updates` map 追加两个键（**map 更新可写零值，支持清空定价**；`Updates(map)` 不走 serializer，规则需手动序列化，与 capabilities 同款）：

```go
	ruleJSON, _ := sonic.Marshal(pricingToDB(m.Pricing())) //nolint:errcheck // 结构体序列化不会失败，值已经聚合校验
	updates[constant.FieldModelPricingRules] = string(ruleJSON)
	updates[constant.FieldModelPricingCurrency] = string(m.Pricing().Currency())
```

同文件新增两个转换函数（字段名与 `dbmodel.ModelPricingRule` 的 JSON 键一一对应）：

```go
func pricingFromDB(rules []dbmodel.ModelPricingRule, currency string) vo.Pricing {
	if currency == "" {
		return vo.Pricing{}
	}
	vr := lo.Map(rules, func(r dbmodel.ModelPricingRule, _ int) vo.PricingRule {
		return vo.PricingRule{
			TimeWindows: lo.Map(r.TimeWindows, func(w dbmodel.ModelTimeWindow, _ int) vo.TimeWindow {
				return vo.TimeWindow{Days: w.Days, Start: w.Start, End: w.End, Timezone: w.Timezone}
			}),
			ContextMin:       r.ContextMin,
			ContextMax:       r.ContextMax,
			InputMicro:       r.InputPriceMicro,
			OutputMicro:      r.OutputPriceMicro,
			CacheCreateMicro: r.CacheCreationPriceMicro,
			CacheReadMicro:   r.CacheReadPriceMicro,
		}
	})
	p, err := vo.NewPricing(enum.Currency(currency), vr)
	if err != nil {
		return vo.Pricing{} // 库内数据非法时按未计价降级，不阻断读路径
	}
	return p
}

func pricingToDB(p vo.Pricing) []dbmodel.ModelPricingRule {
	return lo.Map(p.Rules(), func(r vo.PricingRule, _ int) dbmodel.ModelPricingRule {
		return dbmodel.ModelPricingRule{
			TimeWindows: lo.Map(r.TimeWindows, func(w vo.TimeWindow, _ int) dbmodel.ModelTimeWindow {
				return dbmodel.ModelTimeWindow{Days: w.Days, Start: w.Start, End: w.End, Timezone: w.Timezone}
			}),
			ContextMin:              r.ContextMin,
			ContextMax:              r.ContextMax,
			InputPriceMicro:         r.InputMicro,
			OutputPriceMicro:        r.OutputMicro,
			CacheCreationPriceMicro: r.CacheCreateMicro,
			CacheReadPriceMicro:     r.CacheReadMicro,
		}
	})
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/domain/llmproxy/... ./internal/infrastructure/repository/...`
Expected: PASS（仓储包若编译失败说明映射签名不一致，修到绿）

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/database/model/model.go internal/common/constant/sql.go internal/domain/llmproxy/aggregate/model.go internal/domain/llmproxy/aggregate/model_pricing_test.go internal/infrastructure/repository/model_repository.go
git commit -m "feat(pricing): Model 聚合与持久化挂载定价规则（models 加 pricing_rules/两列映射）"
```

---

### Task 3: 审计计费落库（收尾 seam 计算 + 快照写入）

**Files:**
- Modify: `internal/domain/modelcall/aggregate/audit.go`（cost 字段 + `RecordCallInput` + getter）
- Modify: `internal/dto/asynctask.go`（`ModelCallAuditTask` 加 `CostMicro`/`PricingCurrency`/`CreatedAt`）
- Modify: `internal/infrastructure/pool/store_pool.go`（`RecordCallInput` 补映射、`now` 取 `task.CreatedAt`）
- Modify: `internal/infrastructure/repository/audit_repository.go`（`Save` 落两列 + 查询映射）
- Modify: `internal/infrastructure/database/model/model_call_audit.go`（加两列）
- Modify: `internal/application/llmproxy/usecase/recorder.go`（计价计算）
- Test: `internal/application/llmproxy/usecase/recorder_cost_test.go`

**Interfaces:**
- Consumes: Task 1 `vo.Pricing.Match/ComputeCost`、Task 2 `aggregate.Model.Pricing()`
- Produces:
  - `dto.ModelCallAuditTask.CostMicro *int64`、`PricingCurrency string`、`CreatedAt time.Time`
  - `mcaggregate.RecordCallInput.CostMicro *int64`、`PricingCurrency enum.Currency`；`ModelCallAudit.GetCostMicro() *int64`、`GetPricingCurrency() enum.Currency`
  - DB 列 `model_call_audits.cost_micro`（可空）、`pricing_currency`

- [ ] **Step 1: 写失败测试** `internal/application/llmproxy/usecase/recorder_cost_test.go`

```go
package usecase

import (
	"context"
	"net/http"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type fakeSubmitter struct{ task *dto.ModelCallAuditTask }

func (f *fakeSubmitter) SubmitModelCallAuditTask(task *dto.ModelCallAuditTask) error {
	f.task = task
	return nil
}

//nolint:unused // TaskSubmitter 的其他方法不在本测试路径
func (f *fakeSubmitter) SubmitMessageStoreTask(*dto.MessageStoreTask) error { return nil }

func pricedModel(t *testing.T) *aggregate.Model {
	t.Helper()
	m, err := aggregate.CreateModel(1, vo.EndpointAlias("gpt"), "gpt-4", 2, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel() error = %v", err)
	}
	p, err := vo.NewPricing(enum.CurrencyUSD, []vo.PricingRule{{InputMicro: 1_000_000, OutputMicro: 2_000_000}})
	if err != nil {
		t.Fatalf("NewPricing() error = %v", err)
	}
	_ = m.UpdatePricing(p)
	return m
}

func TestRecordModelCall_CostAttachedOnSuccess(t *testing.T) {
	sub := &fakeSubmitter{}
	recordModelCall(context.Background(), sub, nil, callOutcome{
		model:         pricedModel(t),
		successStatus: true,
		usage: openAITokenUsage{usage: &dto.OpenAICompletionUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
		}},
	})
	task := sub.task
	if task == nil {
		t.Fatal("audit task not submitted")
	}
	if task.CostMicro == nil {
		t.Fatal("CostMicro should not be nil for priced success call")
	}
	// 1000×1/1e6=1000 + 500×2/1e6=1000 → 2000
	if *task.CostMicro != 2000 {
		t.Fatalf("CostMicro = %d, want 2000", *task.CostMicro)
	}
	if task.PricingCurrency != string(enum.CurrencyUSD) {
		t.Fatalf("PricingCurrency = %q", task.PricingCurrency)
	}
}

func TestRecordModelCall_NoCostOnFailureOrUnpriced(t *testing.T) {
	sub := &fakeSubmitter{}
	recordModelCall(context.Background(), sub, nil, callOutcome{
		model:         pricedModel(t),
		successStatus: false,
		err:           &dtoErr{},
	})
	if sub.task.CostMicro != nil {
		t.Fatalf("failed call must not be priced")
	}

	sub2 := &fakeSubmitter{}
	m, _ := aggregate.CreateModel(1, vo.EndpointAlias("gpt"), "gpt-4", 2, true, 1, 1, []enum.InputModality{enum.InputModalityText})
	recordModelCall(context.Background(), sub2, nil, callOutcome{model: m, successStatus: true})
	if sub2.task.CostMicro != nil {
		t.Fatalf("unpriced model must not be priced")
	}
}

// dtoErr 构造可被 extractUpstreamStatusAndError 归类的失败错误（借用 status=0 的未知错误路径）
type dtoErr struct{}

func (e *dtoErr) Error() string { return "boom" }
```

注：`openAITokenUsage` 字段与 `TaskSubmitter` 接口方法集以 `internal/application/llmproxy/usecase/port.go` 实际声明为准；若接口含更多方法，fake 按接口补齐空实现。`TaskSubmitter` 全部方法名在实现前用 `rg "type TaskSubmitter" -A 10 internal/application/llmproxy/usecase/port.go` 核对。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/application/llmproxy/usecase/... -run TestRecordModelCall`
Expected: FAIL（`task.CostMicro undefined`）

- [ ] **Step 3: 实现**

`internal/domain/modelcall/aggregate/audit.go`：结构体加 `costMicro *int64`、`pricingCurrency enum.Currency`；`RecordCallInput` 加同名两字段；`newAudit` 赋值；新增 getter：

```go
// GetCostMicro 估算费用（微单位）；nil=未计价
func (a *ModelCallAudit) GetCostMicro() *int64 { return a.costMicro }

// GetPricingCurrency 计价币种快照
func (a *ModelCallAudit) GetPricingCurrency() enum.Currency { return a.pricingCurrency }
```

`internal/dto/asynctask.go` 的 `ModelCallAuditTask` 追加：

```go
	CostMicro       *int64
	PricingCurrency string
	CreatedAt       time.Time
```

`internal/infrastructure/pool/store_pool.go` 的 `SubmitModelCallAuditTask`：

```go
		audit := mcaggregate.RecordCall(mcaggregate.RecordCallInput{
			// ...既有字段照旧...
			CostMicro:       task.CostMicro,
			PricingCurrency: enum.Currency(task.PricingCurrency),
		}, task.CreatedAt)
```

（`time.Now()` 改为 `task.CreatedAt`，保证计价时点=审计时点；task 构造处负责填 `CreatedAt`。）

`internal/infrastructure/database/model/model_call_audit.go` 追加两列（`Error string` 字段后）：

```go
	CostMicro       *int64 `json:"cost_micro" gorm:"column:cost_micro;comment:估算费用(微单位,NULL=未计价)"`
	PricingCurrency string `json:"pricing_currency" gorm:"column:pricing_currency;not null;default:'';comment:计价币种快照(''/CNY/USD)"`
```

`internal/infrastructure/repository/audit_repository.go` 的 `Save`（约 :51 构造 `dbmodel.ModelCallAudit` 处）追加两字段映射，读路径（`paginate` 的 Scan → 聚合重建处）同步带回 `CostMicro`/`PricingCurrency` 到聚合（`RecordCall` 或等价重建函数补两入参）。

`internal/application/llmproxy/usecase/recorder.go`：`recordModelCall` 中 task 构造处补 `CreatedAt: time.Now()`；在状态归一化之后、`SubmitModelCallAuditTask` 之前插入计价块：

```go
	// 计费：仅成功调用计价（失败不计费）；未计价模型跳过。
	if task.UpstreamStatusCode == http.StatusOK && out.model.Pricing().IsPriced() {
		p := out.model.Pricing()
		promptTokens := int64(task.InputTokens) + int64(task.CacheCreationInputTokens) + int64(task.CacheReadInputTokens)
		rule := p.Match(task.CreatedAt, promptTokens)
		cost := p.ComputeCost(rule, int64(task.InputTokens), int64(task.OutputTokens),
			int64(task.CacheCreationInputTokens), int64(task.CacheReadInputTokens))
		task.CostMicro = &cost
		task.PricingCurrency = string(p.Currency())
	}
```

注意 `responseStatus` 的 in-band 失败（`SetErrorFromResponseStatus`）会改写 `UpstreamStatusCode`，计价块必须放在其后，天然满足"失败不计费"。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/application/llmproxy/... ./internal/infrastructure/... ./internal/domain/modelcall/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/modelcall/aggregate/audit.go internal/dto/asynctask.go internal/infrastructure/pool/store_pool.go internal/infrastructure/repository/audit_repository.go internal/infrastructure/database/model/model_call_audit.go internal/application/llmproxy/usecase/recorder.go internal/application/llmproxy/usecase/recorder_cost_test.go
git commit -m "feat(pricing): 审计收尾 seam 按当时价计费并落库（cost_micro+币种快照）"
```

---

### Task 4: DTO wire 契约 + Model CRUD 接线

**Files:**
- Create: `internal/dto/pricing.go`
- Modify: `internal/dto/model.go`（`CreateModelReqBody`/`UpdateModelReqBody`/`ModelListItem` 加 Pricing）
- Modify: `internal/dto/upstream.go`（`UpstreamModelItem` 加 Pricing）
- Modify: `internal/application/model/port/handler.go`（命令加 Pricing）
- Modify: `internal/application/model/command/create_model.go`、`update_model.go`
- Modify: `internal/handler/model.go`（pricingFromDTO/pricingToDTO + 命令映射）、`internal/handler/upstream.go`（`UpstreamModelItem` 装配处映射，`rg -n "UpstreamModelItem{" internal` 定位）
- Test: `internal/dto/pricing_test.go`、`internal/handler/model_pricing_test.go`

**Interfaces:**
- Consumes: Task 1 `vo.NewPricing/vo.Pricing/vo.PricingRule/vo.TimeWindow`、Task 2 `Model.UpdatePricing/Pricing()`
- Produces:
  - `dto.PricingDTO{Currency enum.Currency; Rules []PricingRuleDTO}`、`dto.PricingRuleDTO`、`dto.TimeWindowDTO`
  - `dto.PriceMicroFromDisplay(v float64) int64`、`dto.PriceDisplayFromMicro(m int64) float64`
  - handler：`pricingFromDTO(*dto.PricingDTO) (vo.Pricing, error)`、`pricingToDTO(vo.Pricing) *dto.PricingDTO`
  - `port.CreateModelCommand.Pricing vo.Pricing`、`port.UpdateModelCommand.Pricing *vo.Pricing`（nil=不改）

- [ ] **Step 1: 写失败测试** `internal/dto/pricing_test.go`

```go
package dto

import "testing"

func TestPriceMicroRoundTrip(t *testing.T) {
	cases := []float64{0, 0.8, 4, 0.000001, 2.5, 1_000_000}
	for _, v := range cases {
		if got := PriceDisplayFromMicro(PriceMicroFromDisplay(v)); got != v {
			t.Fatalf("round trip %v -> %v", v, got)
		}
	}
	if got := PriceMicroFromDisplay(0.5); got != 500_000 {
		t.Fatalf("0.5 display = %d micro", got)
	}
}
```

`internal/handler/model_pricing_test.go`：

```go
package handler

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestPricingFromDTO(t *testing.T) {
	// nil → 未计价
	p, err := pricingFromDTO(nil)
	if err != nil || p.IsPriced() {
		t.Fatalf("nil dto: p=%+v err=%v", p, err)
	}
	// 合法：条件规则 + 默认规则
	p, err = pricingFromDTO(&dto.PricingDTO{
		Currency: enum.CurrencyCNY,
		Rules: []dto.PricingRuleDTO{
			{TimeWindows: []dto.TimeWindowDTO{{Days: []int{1}, Start: "00:30", End: "08:30", Timezone: "UTC"}},
				ContextMax: 200000, InputPrice: 1},
			{InputPrice: 2},
		},
	})
	if err != nil || !p.IsPriced() {
		t.Fatalf("valid dto: err=%v", err)
	}
	// 非法：>0 单价但缺币种
	if _, err := pricingFromDTO(&dto.PricingDTO{Rules: []dto.PricingRuleDTO{{InputPrice: 1}}}); err == nil {
		t.Fatal("missing currency should be rejected")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/dto/... ./internal/handler/... -run 'TestPriceMicro|TestPricingFromDTO'`
Expected: FAIL（`undefined: PriceMicroFromDisplay` 等）

- [ ] **Step 3: 实现**

`internal/dto/pricing.go`（完整）：

```go
package dto

import (
	"math"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// PricingDTO 模型定价（wire 展示单位：货币单位每 1M tokens）。
// currency=="" ⇔ 未计价；非空时 rules 须恰好含一条无条件默认规则
// （time_windows 空 + context_min=0 + context_max=0）。
type PricingDTO struct {
	Currency enum.Currency     `json:"currency,omitempty" enum:",CNY,USD" doc:"计价币种；空=未计价"`
	Rules    []PricingRuleDTO  `json:"rules,omitempty" doc:"定价规则（数组顺序=匹配优先级）"`
}

// TimeWindowDTO 时段窗口（循环生效）
type TimeWindowDTO struct {
	Days     []int  `json:"days,omitempty" minimum:"1" maximum:"7" doc:"周几（1=周一…7=周日），空=每天"`
	Start    string `json:"start" minLength:"5" maxLength:"5" doc:"起始时刻 HH:MM（含）"`
	End      string `json:"end" minLength:"5" maxLength:"5" doc:"结束时刻 HH:MM（不含）；小于 start 表示跨午夜"`
	Timezone string `json:"timezone,omitempty" doc:"IANA 时区，缺省 UTC"`
}

// PricingRuleDTO 定价规则（单价：货币单位/1M tokens）
type PricingRuleDTO struct {
	TimeWindows        []TimeWindowDTO `json:"time_windows,omitempty" doc:"时段窗口（空=全时段，多窗口 OR）"`
	ContextMin         int64           `json:"context_min,omitempty" minimum:"0" doc:"prompt token 下界（含）"`
	ContextMax         int64           `json:"context_max,omitempty" minimum:"0" doc:"上界（不含），0=无上限"`
	InputPrice         float64         `json:"input_price" minimum:"0" maximum:"1000000" doc:"输入单价"`
	OutputPrice        float64         `json:"output_price" minimum:"0" maximum:"1000000" doc:"输出单价"`
	CacheCreationPrice float64         `json:"cache_creation_price" minimum:"0" maximum:"1000000" doc:"缓存创建单价"`
	CacheReadPrice     float64         `json:"cache_read_price" minimum:"0" maximum:"1000000" doc:"缓存读取单价"`
}

// PriceMicroFromDisplay 展示单位 → 微单位（1e-6 货币单位），半入。
func PriceMicroFromDisplay(v float64) int64 {
	return int64(math.Round(v * 1e6))
}

// PriceDisplayFromMicro 微单位 → 展示单位。
func PriceDisplayFromMicro(m int64) float64 {
	return float64(m) / 1e6
}
```

`internal/handler/model.go` 新增映射（放文件内既有 dto↔command 映射函数旁）：

```go
// pricingFromDTO wire 定价 → 值对象；校验统一走 vo.NewPricing（错误为 ErrValidation → 422）。
func pricingFromDTO(d *dto.PricingDTO) (vo.Pricing, error) {
	if d == nil {
		return vo.Pricing{}, nil
	}
	rules := make([]vo.PricingRule, 0, len(d.Rules))
	for _, r := range d.Rules {
		rules = append(rules, vo.PricingRule{
			TimeWindows: lo.Map(r.TimeWindows, func(w dto.TimeWindowDTO, _ int) vo.TimeWindow {
				return vo.TimeWindow{Days: w.Days, Start: w.Start, End: w.End, Timezone: w.Timezone}
			}),
			ContextMin:       r.ContextMin,
			ContextMax:       r.ContextMax,
			InputMicro:       dto.PriceMicroFromDisplay(r.InputPrice),
			OutputMicro:      dto.PriceMicroFromDisplay(r.OutputPrice),
			CacheCreateMicro: dto.PriceMicroFromDisplay(r.CacheCreationPrice),
			CacheReadMicro:   dto.PriceMicroFromDisplay(r.CacheReadPrice),
		})
	}
	return vo.NewPricing(d.Currency, rules)
}

// pricingToDTO 值对象 → wire 定价（未计价返回 nil，响应省略字段）。
func pricingToDTO(p vo.Pricing) *dto.PricingDTO {
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
				ContextMin:         r.ContextMin,
				ContextMax:         r.ContextMax,
				InputPrice:         dto.PriceDisplayFromMicro(r.InputMicro),
				OutputPrice:        dto.PriceDisplayFromMicro(r.OutputMicro),
				CacheCreationPrice: dto.PriceDisplayFromMicro(r.CacheCreateMicro),
				CacheReadPrice:     dto.PriceDisplayFromMicro(r.CacheReadMicro),
			}
		}),
	}
}
```

DTO 结构体字段追加：

- `CreateModelReqBody` 加 `Pricing *PricingDTO \`json:"pricing,omitempty" doc:"定价（缺省=未计价）"\``
- `UpdateModelReqBody` 加 `Pricing *PricingDTO \`json:"pricing,omitempty" doc:"定价（缺省=不修改；currency 传空串+rules 空数组=清空为未计价）"\``
- `ModelListItem` 加 `Pricing *PricingDTO \`json:"pricing,omitempty" doc:"定价"\``
- `UpstreamModelItem` 同样加 `Pricing *PricingDTO`

命令扩展（`internal/application/model/port/handler.go`）：

```go
// CreateModelCommand 追加（结构体内）：
	Pricing vo.Pricing // 模型定价（未计价传 vo.Pricing{}）

// UpdateModelCommand 追加（结构体内）：
	Pricing *vo.Pricing // nil=不修改定价
```

（port 文件需补 `vo` import。）

`internal/application/model/command/create_model.go`：`aggregate.CreateModel(...)` 之后、`repo.Create` 之前：

```go
	if err := m.UpdatePricing(cmd.Pricing); err != nil {
		return nil, err
	}
```

`internal/application/model/command/update_model.go`：`m.Update(...)` 之后：

```go
	if cmd.Pricing != nil {
		if err := m.UpdatePricing(*cmd.Pricing); err != nil {
			return llmproxy.ModelIDSyncCounts{}, err
		}
	}
```

`internal/handler/model.go` 请求映射处：Create 的 command 构造补

```go
	pricing, perr := pricingFromDTO(body.Pricing)
	if perr != nil {
		return nil, perr // ierr ErrValidation → 422
	}
```

（`cmd.Pricing = pricing`；Update 同理，`body.Pricing == nil` 时 `cmd.Pricing` 留 nil。）列表/上游响应装配处补 `Pricing: pricingToDTO(...)`（按 `rg -n "ModelListItem{\|UpstreamModelItem{" internal` 定位装配点，取数据源为 `aggregate.Model.Pricing()`）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/dto/... ./internal/handler/... ./internal/application/model/...`
Expected: PASS；`go build ./...` 编译通过（命令结构体新字段的既有调用点补齐）

- [ ] **Step 5: Commit**

```bash
git add internal/dto/pricing.go internal/dto/model.go internal/dto/upstream.go internal/application/model/port/handler.go internal/application/model/command/ internal/handler/ internal/dto/pricing_test.go internal/handler/model_pricing_test.go
git commit -m "feat(pricing): 定价 wire 契约与 Model CRUD 接线（微单位换算、422 校验）"
```

---

### Task 5: models.dev 定价导入辅助（prefill）

**Files:**
- Create: `internal/application/model/port/provider.go`（端口）
- Create: `internal/infrastructure/modelsdev/client.go`
- Create: `internal/application/model/query/prefill_pricing.go`
- Modify: `internal/application/model/port/handler.go`（Prefill 查询/结果/Handler 接口）
- Modify: `internal/dto/model.go`（Prefill Req/Rsp）
- Modify: `internal/handler/model.go`（HandlePrefillPricing）
- Modify: `internal/router/model.go`（GET /pricing/prefill）
- Modify: `internal/bootstrap/modules/`（fx Provide：modelsdev.Client + PrefillPricingHandler）与 `internal/bootstrap/router.go`（routeParams 同步）
- Test: `internal/infrastructure/modelsdev/client_test.go`、`internal/application/model/query/prefill_pricing_test.go`

**Interfaces:**
- Consumes: 无（独立能力）
- Produces:
  - `port.PricingQuote{Input, Output, CacheCreation, CacheRead float64}`、`port.PricingQuoteProvider interface{ Quote(ctx, modelID string) (PricingQuote, bool, error) }`
  - `port.PrefillPricingQuery{UpstreamModel string}`、`port.PrefillPricingResult{Found bool; Currency enum.Currency; InputPrice, OutputPrice, CacheCreationPrice, CacheReadPrice float64}`、`port.PrefillPricingHandler`
  - `modelsdev.NewClient(hc *http.Client, rdb redis.UniversalClient) *modelsdev.Client`（实现 `port.PricingQuoteProvider`）
  - 路由 `GET /api/web/v1/model/pricing/prefill`，OperationID `prefillModelPricing`

- [ ] **Step 1: 写失败测试** `internal/infrastructure/modelsdev/client_test.go`

```go
package modelsdev

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQuoteExactMatchAndMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"gpt-4":{"cost":{"input":"3","output":"15","cache_read":"0.3","cache_write":"3.75"}}}`))
	}))
	defer srv.Close()

	c := &Client{Source: srv.URL, http: srv.Client()}
	q, ok, err := c.Quote(context.Background(), "gpt-4")
	if err != nil || !ok {
		t.Fatalf("quote: ok=%v err=%v", ok, err)
	}
	if q.Input != 3 || q.Output != 15 || q.CacheCreation != 3.75 || q.CacheRead != 0.3 {
		t.Fatalf("quote = %+v", q)
	}
	if _, ok, _ := c.Quote(context.Background(), " gpt-4 "); ok {
		t.Fatal("must match exactly after trim in caller; client should miss")
	}
}

func TestQuoteFetchErrorIsSwallowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := &Client{Source: srv.URL, http: srv.Client()}
	if _, ok, err := c.Quote(context.Background(), "x"); ok || err == nil {
		t.Fatalf("fetch failure must return err, ok=false; got ok=%v err=%v", ok, err)
	}
}
```

`internal/application/model/query/prefill_pricing_test.go`：

```go
package query

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
)

type fakeQuoteProvider struct {
	quote port.PricingQuote
	ok    bool
	err   error
}

func (f *fakeQuoteProvider) Quote(_ context.Context, _ string) (port.PricingQuote, bool, error) {
	return f.quote, f.ok, f.err
}

func TestPrefillPricing(t *testing.T) {
	h := NewPrefillPricingHandler(&fakeQuoteProvider{quote: port.PricingQuote{Input: 3}, ok: true})
	rsp, err := h.Handle(context.Background(), port.PrefillPricingQuery{UpstreamModel: " gpt-4 "})
	if err != nil || !rsp.Found {
		t.Fatalf("found case: %+v err=%v", rsp, err)
	}
	// 上游失败降级为 not found，不报错
	h2 := NewPrefillPricingHandler(&fakeQuoteProvider{err: errors.New("boom")})
	rsp2, err := h2.Handle(context.Background(), port.PrefillPricingQuery{UpstreamModel: "x"})
	if err != nil || rsp2.Found {
		t.Fatalf("error case must degrade: %+v err=%v", rsp2, err)
	}
	// 空模型名短路
	rsp3, _ := h.Handle(context.Background(), port.PrefillPricingQuery{UpstreamModel: "  "})
	if rsp3.Found {
		t.Fatal("blank model should miss")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/infrastructure/modelsdev/... ./internal/application/model/query/... -run TestQuote -run TestPrefill`
Expected: FAIL（包不存在）

- [ ] **Step 3: 实现**

`internal/application/model/port/provider.go`：

```go
package port

import "context"

// PricingQuote 公开定价（展示单位：USD/1M tokens）
type PricingQuote struct {
	Input         float64
	Output        float64
	CacheCreation float64
	CacheRead     float64
}

// PricingQuoteProvider 公开定价来源（由 infrastructure/modelsdev 实现）
type PricingQuoteProvider interface {
	// Quote 按模型 ID 精确匹配查询；未命中 ok=false；拉取失败返回 err
	Quote(ctx context.Context, modelID string) (PricingQuote, bool, error)
}
```

`internal/infrastructure/modelsdev/client.go`（要点：`Source` 可注入便于测试；redis nil-safe；文档缓存 24h；`cost` 值容忍 string/number 两种 JSON 形态）：

```go
package modelsdev

// SourceURL models.dev 公开定价数据源。
const SourceURL = "https://models.dev/api.json"

const (
	cacheKey = "pricing:modelsdev:doc"
	docTTL   = 24 * time.Hour
	maxDoc   = 16 << 20 // 安全上限，防异常大响应
)

// Client models.dev 定价查询：拉取公开文档（Redis 缓存 24h），按模型 ID 精确匹配。
type Client struct {
	Source string // 测试注入用；空 = SourceURL
	http   *http.Client
	redis  redis.UniversalClient // nil = 不缓存
}

// NewClient 构造 models.dev 定价客户端（实现 port.PricingQuoteProvider）。
func NewClient(hc *http.Client, rdb redis.UniversalClient) *Client {
	return &Client{http: hc, redis: rdb}
}

// Quote 按模型 ID 精确匹配公开定价。
func (c *Client) Quote(ctx context.Context, modelID string) (port.PricingQuote, bool, error) {
	raw, err := c.doc(ctx)
	if err != nil {
		return port.PricingQuote{}, false, err
	}
	var doc map[string]struct {
		Cost map[string]any `json:"cost"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return port.PricingQuote{}, false, err
	}
	entry, ok := doc[modelID]
	if !ok {
		return port.PricingQuote{}, false, nil
	}
	return port.PricingQuote{
		Input:         numCost(entry.Cost["input"]),
		Output:        numCost(entry.Cost["output"]),
		CacheCreation: numCost(entry.Cost["cache_write"]),
		CacheRead:     numCost(entry.Cost["cache_read"]),
	}, true, nil
}

func (c *Client) doc(ctx context.Context) ([]byte, error) {
	if c.redis != nil {
		if cached, err := c.redis.Get(ctx, cacheKey).Bytes(); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}
	url := c.Source
	if url == "" {
		url = SourceURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("models.dev fetch failed: " + resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDoc))
	if err != nil {
		return nil, err
	}
	if c.redis != nil {
		_ = c.redis.Set(ctx, cacheKey, body, docTTL).Err() // 缓存失败不影响查询
	}
	return body, nil
}

// numCost 兼容 models.dev 中 string/number 两种计价形态。
func numCost(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
```

`internal/application/model/port/handler.go` 追加：

```go
// PrefillPricingQuery 定价导入查询
type PrefillPricingQuery struct {
	UpstreamModel string
}

// PrefillPricingResult 定价导入结果（USD/1M tokens；found=false 时其余字段为零值）
type PrefillPricingResult struct {
	Found              bool
	Currency           enum.Currency
	InputPrice         float64
	OutputPrice        float64
	CacheCreationPrice float64
	CacheReadPrice     float64
}

// PrefillPricingHandler 定价导入处理器
type PrefillPricingHandler interface {
	Handle(ctx context.Context, q PrefillPricingQuery) (*PrefillPricingResult, error)
}
```

`internal/application/model/query/prefill_pricing.go`：

```go
package query

// prefillPricingHandler 从公开定价源导入表单初值（仅填充，永不自动改价）。
type prefillPricingHandler struct {
	provider port.PricingQuoteProvider
}

// NewPrefillPricingHandler 构造定价导入查询处理器。
func NewPrefillPricingHandler(provider port.PricingQuoteProvider) port.PrefillPricingHandler {
	return &prefillPricingHandler{provider: provider}
}

// Handle 按 upstream_model 精确匹配；未命中/拉取失败一律 found=false（录入不被阻塞）。
func (h *prefillPricingHandler) Handle(ctx context.Context, q port.PrefillPricingQuery) (*port.PrefillPricingResult, error) {
	name := strings.TrimSpace(q.UpstreamModel)
	if name == "" {
		return &port.PrefillPricingResult{}, nil
	}
	quote, ok, err := h.provider.Quote(ctx, name)
	if err != nil || !ok {
		return &port.PrefillPricingResult{}, nil
	}
	return &port.PrefillPricingResult{
		Found:              true,
		Currency:           enum.CurrencyUSD,
		InputPrice:         quote.Input,
		OutputPrice:        quote.Output,
		CacheCreationPrice: quote.CacheCreation,
		CacheReadPrice:     quote.CacheRead,
	}, nil
}
```

`internal/dto/model.go` 追加：

```go
// ModelPricingPrefillReq 定价导入请求
type ModelPricingPrefillReq struct {
	UpstreamModel string `query:"upstreamModel" required:"true" maxLength:"200" doc:"上游模型名（与 models.dev 模型 ID 精确匹配）"`
}

// ModelPricingPrefillRsp 定价导入响应（USD/1M tokens；found=false 表示未命中/上游不可达）
type ModelPricingPrefillRsp struct {
	CommonRsp
	Found              bool          `json:"found" doc:"是否命中公开定价"`
	Currency           enum.Currency `json:"currency,omitempty" doc:"币种（命中时 USD）"`
	InputPrice         float64       `json:"inputPrice,omitempty" doc:"输入单价"`
	OutputPrice        float64       `json:"outputPrice,omitempty" doc:"输出单价"`
	CacheCreationPrice float64       `json:"cacheCreationPrice,omitempty" doc:"缓存创建单价"`
	CacheReadPrice     float64       `json:"cacheReadPrice,omitempty" doc:"缓存读取单价"`
}
```

`internal/handler/model.go` 新增 handler 方法（沿用既有 ModelHandler 接口加方法 + 实现）：

```go
// HandlePrefillPricing models.dev 定价导入（表单填充用）
func (h *modelHandler) HandlePrefillPricing(ctx context.Context, req *dto.ModelPricingPrefillReq) (*dto.ModelPricingPrefillRsp, error) {
	rsp := &dto.ModelPricingPrefillRsp{}
	res, err := h.prefillPricing.Handle(ctx, port.PrefillPricingQuery{UpstreamModel: req.UpstreamModel})
	if err != nil {
		return rsp, nil // 降级为未命中，不阻塞录入
	}
	rsp.Found = res.Found
	if res.Found {
		rsp.Currency = res.Currency
		rsp.InputPrice = res.InputPrice
		rsp.OutputPrice = res.OutputPrice
		rsp.CacheCreationPrice = res.CacheCreationPrice
		rsp.CacheReadPrice = res.CacheReadPrice
	}
	return rsp, nil
}
```

（`modelHandler` 结构体加 `prefillPricing port.PrefillPricingHandler` 字段，构造函数签名同步。）

路由注册（`internal/router/model.go` 的 `initModelRouter` 内追加）：

```go
	huma.Register(modelGroup, huma.Operation{
		OperationID: "prefillModelPricing",
		Method:      http.MethodGet,
		Path:        "/pricing/prefill",
		Summary:     "PrefillModelPricing",
		Description: "Fetch public pricing quote from models.dev (form prefill only)",
		Tags:        []string{constant.TagModel},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("prefillModelPricing", enum.PermissionUser),
		},
	}, modelHandler.HandlePrefillPricing)
```

fx 装配三处同步（Global Constraints 第 6 条）：

1. `internal/bootstrap/modules/`：`fx.Provide` 注册 `modelsdev.NewClient`（infra 组，复用全局 `*http.Client` 与 `*redis.Client`）、`query.NewPrefillPricingHandler`（application 组）
2. `internal/handler/` 的 ModelHandler 构造参数补 `port.PrefillPricingHandler`
3. `internal/bootstrap/router.go` 的 `routeParams` 与 `initModelRouter` 参数同步（若路由函数加参）

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/infrastructure/modelsdev/... ./internal/application/model/... ./internal/handler/... && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/model/port/ internal/infrastructure/modelsdev/ internal/application/model/query/prefill_pricing.go internal/application/model/query/prefill_pricing_test.go internal/dto/model.go internal/handler/model.go internal/router/model.go internal/bootstrap/
git commit -m "feat(pricing): models.dev 定价导入辅助端点（24h 缓存、精确匹配、失败降级）"
```

---

### Task 6: 成本查询（审计行费用 + summary + distribution）

**Files:**
- Create: `internal/domain/modelcall/cost.go`（点类型与分组枚举）
- Modify: `internal/domain/modelcall/repository.go`（AuditRepository 加 3 个查询方法）
- Modify: `internal/infrastructure/repository/audit_repository.go`（SQL 实现）
- Modify: `internal/common/constant/sql.go`（`FieldCostMicro`/`FieldPricingCurrency` 等常量）
- Create: `internal/application/audit/query/cost_summary.go`、`cost_distribution.go`
- Modify: `internal/application/audit/query/service.go`（派发）与 `internal/application/audit/port`（服务接口）
- Modify: `internal/dto/audit.go`（AuditLogItem 加 cost/currency）、`internal/dto/audit_stats.go`（cost Req/Rsp）
- Modify: `internal/handler/audit.go`、`internal/router/audit.go`、`internal/bootstrap/`（装配）
- Test: `internal/application/audit/query/cost_query_test.go`

**Interfaces:**
- Consumes: Task 3 的 `cost_micro`/`pricing_currency` 列、既有 `dateTruncSQL(granularity)`、`modelcall.AuditRelation`（含 UserID/UserName/UserEmail）、`apiKeyIDs.LookupIDsByUserID`
- Produces:
  - `modelcall.CostTotal{Currency string; CostMicro int64}`、`CostPoint{Time time.Time; Currency string; CostMicro int64}`、`CostGroupBy`（`CostGroupByUser/APIKey/Model`）、`CostDistributionPoint{ID, Name, Currency string; CostMicro int64}`
  - `AuditRepository.SumCostByCurrency/QueryCostSeries/QueryCostDistribution`
  - 路由 `GET /api/web/v1/audit/cost/summary`、`GET /api/web/v1/audit/cost/distribution`（OperationID `auditCostSummary`/`auditCostDistribution`）
  - `AuditLogItem.Cost *float64`、`AuditLogItem.PricingCurrency enum.Currency`

- [ ] **Step 1: 写失败测试** `internal/application/audit/query/cost_query_test.go`（fake 仓储断言派发与微单位→展示单位换算、`group_by=user` 的 user 越权拒绝）

```go
package query

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

type fakeCostRepo struct {
	totals []*modelcall.CostTotal
	series []*modelcall.CostPoint
	dist   []*modelcall.CostDistributionPoint
}

func (f *fakeCostRepo) SumCostByCurrency(context.Context, []uint, time.Time, time.Time) ([]*modelcall.CostTotal, error) {
	return f.totals, nil
}
func (f *fakeCostRepo) QueryCostSeries(context.Context, []uint, time.Time, time.Time, enum.Granularity) ([]*modelcall.CostPoint, error) {
	return f.series, nil
}
func (f *fakeCostRepo) QueryCostDistribution(context.Context, []uint, modelcall.CostGroupBy, time.Time, time.Time, int) ([]*modelcall.CostDistributionPoint, error) {
	return f.dist, nil
}

// 注意：fake 需满足 handler 对仓储的最小依赖面；若 handler 依赖完整
// modelcall.AuditRepository，按接口补齐其余方法的空实现（下文不再重复）。

func TestCostSummaryConvertsMicroToDisplay(t *testing.T) {
	repo := &fakeCostRepo{totals: []*modelcall.CostTotal{{Currency: "USD", CostMicro: 2820}}}
	h := NewCostSummaryHandler(repo)
	rsp, err := h.Handle(context.Background(), CostSummaryQuery{})
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if len(rsp.Totals) != 1 || rsp.Totals[0].Cost != 0.00282 {
		t.Fatalf("totals = %+v", rsp.Totals)
	}
}

func TestCostDistributionRejectsUserGroupForPlainUser(t *testing.T) {
	// 非 admin 请求 group_by=user：应在 handler/派发层拒绝（ErrNoPermission）
	_, err := NewCostDistributionHandler(&fakeCostRepo{}).Handle(context.Background(),
		CostDistributionQuery{GroupBy: modelcall.CostGroupByUser, IsAdmin: false})
	if err == nil {
		t.Fatal("group_by=user must be rejected for non-admin")
	}
}
```

（`CostSummaryQuery`/`CostDistributionQuery`/`CostSummaryResult` 等 handler 层入参出参在实现中定义，字段含 `UserID *uint`/`IsAdmin bool`/`StartTime`/`EndTime`/`Granularity`/`Limit`；测试断言字段与实现保持一致。）

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./internal/application/audit/... -run TestCost`
Expected: FAIL（`undefined: NewCostSummaryHandler`）

- [ ] **Step 3: 实现**

`internal/domain/modelcall/cost.go`：

```go
package modelcall

import "time"

// CostTotal 按币种费用合计（微单位）
type CostTotal struct {
	Currency  string
	CostMicro int64
}

// CostPoint 费用趋势点（微单位）
type CostPoint struct {
	Time      time.Time
	Currency  string
	CostMicro int64
}

// CostGroupBy 成本分布分组维度
type CostGroupBy string

const (
	CostGroupByUser   CostGroupBy = "user"
	CostGroupByAPIKey CostGroupBy = "api_key"
	CostGroupByModel  CostGroupBy = "model"
)

// CostDistributionPoint 成本分布行（每行 = 分组 × 币种）
type CostDistributionPoint struct {
	ID        string
	Name      string
	Currency  string
	CostMicro int64
}
```

`internal/domain/modelcall/repository.go` 的 `AuditRepository` 追加：

```go
	// SumCostByCurrency 时间范围内费用合计（按币种；cost_micro IS NOT NULL 口径）
	SumCostByCurrency(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time) ([]*CostTotal, error)
	// QueryCostSeries 费用趋势（时间桶 × 币种）
	QueryCostSeries(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*CostPoint, error)
	// QueryCostDistribution 成本分布（每币种取前 limit 行，费用降序；ID=分组键原始值）
	QueryCostDistribution(ctx context.Context, apiKeyIDs []uint, groupBy CostGroupBy, startTime, endTime time.Time, limit int) ([]*CostDistributionPoint, error)
```

`internal/infrastructure/repository/audit_repository.go` 实现（镜像 `QueryRequestRate` 的骨架：apiKeyIDs 空短路、`DBConditionDeletedAtZero`、`dateTruncSQL`）：

```go
func (r *auditRepository) SumCostByCurrency(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time) ([]*modelcall.CostTotal, error) {
	if apiKeyIDs != nil && len(apiKeyIDs) == 0 {
		return []*modelcall.CostTotal{}, nil
	}
	db := r.db.WithContext(ctx).Model(&dbmodel.ModelCallAudit{}).
		Where(constant.FieldCreatedAt+" >= ? AND "+constant.FieldCreatedAt+" <= ?", startTime, endTime).
		Where(constant.DBConditionDeletedAtZero).
		Where(constant.FieldCostMicro + " IS NOT NULL")
	if len(apiKeyIDs) > 0 {
		db = db.Where(constant.FieldAPIKeyID+" IN ?", apiKeyIDs)
	}
	var results []*modelcall.CostTotal
	if err := db.Select(constant.FieldPricingCurrency + " AS currency, SUM(" + constant.FieldCostMicro + ") AS cost_micro").
		Group(constant.FieldPricingCurrency).
		Scan(&results).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "sum cost by currency")
	}
	return results, nil
}
```

`QueryCostSeries` 同骨架，Select 换 `dateTruncSQL(granularity) + " AS time, " + FieldPricingCurrency + " AS currency, SUM(" + FieldCostMicro + ") AS cost_micro"`，`Group("time, " + constant.FieldPricingCurrency)`，`Order("time")`。

`QueryCostDistribution`：

```go
func (r *auditRepository) QueryCostDistribution(ctx context.Context, apiKeyIDs []uint, groupBy modelcall.CostGroupBy, startTime, endTime time.Time, limit int) ([]*modelcall.CostDistributionPoint, error) {
	if apiKeyIDs != nil && len(apiKeyIDs) == 0 {
		return []*modelcall.CostDistributionPoint{}, nil
	}
	col := constant.FieldAPIKeyID // CostGroupByUser / CostGroupByAPIKey 均按 key 分组，user 由调用方经 AuditRelation 归并
	if groupBy == modelcall.CostGroupByModel {
		col = constant.FieldModelID
	}
	// 每币种取前 limit 行：窗口函数按币种分区排名（PG；项目 SQL 已用 date_trunc/FILTER，非跨库）
	sql := "SELECT id, currency, cost_micro FROM (" +
		"SELECT " + col + "::text AS id, " + constant.FieldPricingCurrency + " AS currency, " +
		"SUM(" + constant.FieldCostMicro + ") AS cost_micro, " +
		"ROW_NUMBER() OVER (PARTITION BY " + constant.FieldPricingCurrency + " ORDER BY SUM(" + constant.FieldCostMicro + ") DESC) AS rn " +
		"FROM " + constant.TableModelCallAudits + " WHERE " + constant.DBConditionDeletedAtZero +
		" AND " + constant.FieldCostMicro + " IS NOT NULL" +
		" AND " + constant.FieldCreatedAt + " >= ? AND " + constant.FieldCreatedAt + " <= ?"
	args := []any{startTime, endTime}
	if len(apiKeyIDs) > 0 {
		sql += " AND " + constant.FieldAPIKeyID + " IN ?"
		args = append(args, apiKeyIDs)
	}
	sql += " GROUP BY " + col + ", " + constant.FieldPricingCurrency +
		") t WHERE rn <= ? ORDER BY currency, cost_micro DESC"
	args = append(args, limit)

	var results []*modelcall.CostDistributionPoint
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query cost distribution")
	}
	return results, nil
}
```

常量（`internal/common/constant/sql.go`）：`FieldCostMicro = "cost_micro"`、`FieldPricingCurrency = "pricing_currency"`；`TableModelCallAudits` 若无现成常量则以实际表名常量/字面量为准（`rg -n "model_call_audits" internal/common/constant`）。

`internal/application/audit/query/cost_summary.go`（admin 与 ByUser 两个 handler 同文件，模式镜像 `token_usage.go`）：

```go
// CostSummaryQuery 成本合计查询
type CostSummaryQuery struct {
	UserID      *uint // 非 nil = user 视角（限定名下 key）
	StartTime   time.Time
	EndTime     time.Time
	Granularity enum.Granularity
}

// CostTotalItem 成本合计行（展示单位）
type CostTotalItem struct {
	Currency enum.Currency
	Cost     float64
}

// CostSeriesPoint 成本趋势点（展示单位）
type CostSeriesPoint struct {
	BucketTime time.Time
	Currency   enum.Currency
	Cost       float64
}

// CostSummaryResult 成本合计结果
type CostSummaryResult struct {
	Totals []*CostTotalItem
	Series []*CostSeriesPoint
}

// CostSummaryHandler 成本合计处理器（admin 全量视角）
type CostSummaryHandler interface {
	Handle(ctx context.Context, q CostSummaryQuery) (*CostSummaryResult, error)
}
// CostSummaryByUserHandler 成本合计处理器（user 视角，限定名下 key）
type CostSummaryByUserHandler interface {
	Handle(ctx context.Context, q CostSummaryQuery) (*CostSummaryResult, error)
}
```

实现要点：ByUser 版先 `apiKeyIDs.LookupIDsByUserID(ctx, *q.UserID)`（既有 `ListAuditLogsByUserHandler` 同款），再查仓储；`CostMicro` → `Cost: dto.PriceDisplayFromMicro(...)`，`Currency: enum.Currency(row.Currency)`；查询结果按币种归并 totals（series 直出）。仓储两查（totals + series）可合并为一次 series 查询后 Go 侧求和，减少一次 DB 往返——实现取一种即可，测试按所选形态断言。

`cost_distribution.go`：

```go
// CostDistributionQuery 成本分布查询
type CostDistributionQuery struct {
	UserID    *uint
	IsAdmin   bool // group_by=user 仅 admin/demo 全量视角
	GroupBy   modelcall.CostGroupBy
	StartTime time.Time
	EndTime   time.Time
	Limit     int // 默认 10、上限 100（handler 层 clamp）
}

// CostDistributionItem 成本分布行（展示单位）
type CostDistributionItem struct {
	ID       string
	Name     string
	Currency enum.Currency
	Cost     float64
}

// CostDistributionResult 成本分布结果
type CostDistributionResult struct {
	GroupBy modelcall.CostGroupBy
	Items   []*CostDistributionItem
}
```

实现要点：

1. `!q.IsAdmin && q.GroupBy == modelcall.CostGroupByUser` → `ierr.New(ierr.ErrNoPermission, ...)`
2. user 视角（`q.UserID != nil`）先取 `apiKeyIDs`，空则返回空结果（防越权）
3. `GroupBy == CostGroupByUser`：仓储按 `api_key_id` 分组查询 → `BatchGetRelations(ctx, keyIDs)`（`modelcall.AuditRelation` 有 `UserID/UserName/UserEmail`）→ Go 侧按 `UserID` 归并求和，`ID=strconv.FormatUint(UserID, 10)`、`Name=UserName`；`CostGroupByAPIKey`：`ID=strconv.FormatUint(keyID,10)`、`Name=APIKeyName`；`CostGroupByModel`：`ID=Name=model_id`
4. demo 全量视角的用户脱敏沿用 `option_list` 三态模式（demo 置 `IsAdmin=false` 且 `UserID` 为其自身，`group_by=user` 自然拒绝）

派发（`internal/application/audit/query/service.go`）：`auditService` 加 `costSummary`/`costSummaryByUser`、`costDistribution`/`costDistributionByUser` 字段与构造参数，公开方法按 `modelTrend`/`modelTrendByUser` 的派发模式选 admin/user 版本；`internal/application/audit/port` 服务接口加对应方法。

DTO（`internal/dto/audit_stats.go`）：

```go
// AuditCostSummaryReq 成本合计查询请求
type AuditCostSummaryReq struct {
	StartTime   time.Time         `query:"startTime"`
	EndTime     time.Time         `query:"endTime"`
	Granularity enum.Granularity  `query:"granularity" enum:"minute,hour,day,week" default:"day"`
}

// AuditCostTotalItem 成本合计行
type AuditCostTotalItem struct {
	Currency enum.Currency `json:"currency" doc:"币种"`
	Cost     float64       `json:"cost" doc:"估算费用（展示单位）"`
}

// AuditCostSeriesPoint 成本趋势点
type AuditCostSeriesPoint struct {
	BucketTime time.Time     `json:"bucketTime" doc:"时间桶（RFC3339 UTC）"`
	Currency   enum.Currency `json:"currency" doc:"币种"`
	Cost       float64       `json:"cost" doc:"估算费用（展示单位）"`
}

// AuditCostSummaryRsp 成本合计响应
type AuditCostSummaryRsp struct {
	CommonRsp
	Totals []*AuditCostTotalItem   `json:"totals" doc:"费用合计（按币种）"`
	Series []*AuditCostSeriesPoint `json:"series" doc:"费用趋势（时间桶 × 币种）"`
}

// AuditCostDistributionReq 成本分布请求
type AuditCostDistributionReq struct {
	GroupBy   string    `query:"groupBy" required:"true" enum:"user,api_key,model" doc:"分组维度（user 仅管理员）"`
	StartTime time.Time `query:"startTime"`
	EndTime   time.Time `query:"endTime"`
	Limit     int       `query:"limit" minimum:"1" maximum:"100" default:"10" doc:"每币种返回行数"`
}

// AuditCostDistributionItem 成本分布行
type AuditCostDistributionItem struct {
	ID       string        `json:"id" doc:"分组键（用户ID/API Key ID/模型ID）"`
	Name     string        `json:"name" doc:"展示名"`
	Currency enum.Currency `json:"currency" doc:"币种"`
	Cost     float64       `json:"cost" doc:"估算费用（展示单位）"`
}

// AuditCostDistributionRsp 成本分布响应
type AuditCostDistributionRsp struct {
	CommonRsp
	GroupBy string                       `json:"groupBy" doc:"分组维度"`
	Items   []*AuditCostDistributionItem `json:"items" doc:"分布行（每行=分组×币种，币种内费用降序）"`
}
```

`internal/dto/audit.go` 的 `AuditLogItem` 追加：

```go
	Cost            *float64      `json:"cost,omitempty" doc:"估算费用（展示单位，null=未计价）"`
	PricingCurrency enum.Currency `json:"pricingCurrency,omitempty" doc:"计价币种"`
```

（列表装配点 `rg -n "AuditLogItem{" internal` 定位，从聚合 `GetCostMicro()`/`GetPricingCurrency()` 映射：`nil` → `Cost=nil`，非 nil → `dto.PriceDisplayFromMicro(*cost)`。）

路由（`internal/router/audit.go` 追加两条，鉴权/限流中间件与相邻统计路由一致）：

```go
	huma.Register(auditGroup, huma.Operation{
		OperationID: "auditCostSummary",
		Method:      http.MethodGet,
		Path:        "/cost/summary",
		Summary:     "AuditCostSummary",
		Description: "Estimated cost totals and series grouped by currency",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{ {constant.SecuritySchemeJWT: {}} },
	}, auditHandler.HandleCostSummary)

	huma.Register(auditGroup, huma.Operation{
		OperationID: "auditCostDistribution",
		Method:      http.MethodGet,
		Path:        "/cost/distribution",
		Summary:     "AuditCostDistribution",
		Description: "Estimated cost distribution by user/api_key/model (per currency top-N)",
		Tags:        []string{constant.TagAudit},
		Security:    []map[string][]string{ {constant.SecuritySchemeJWT: {}} },
	}, auditHandler.HandleCostDistribution)
```

（handler 方法在 `internal/handler/audit.go` 实现：请求→query、结果→rsp；`IsAdmin` 由 `util.CtxValue` 的权限字段判定，镜像既有列表接口的判定方式 `rg -n "PermissionAdmin" internal/handler/audit.go`。）

fx 装配三处同步同 Task 5 要求（handler 构造、modules Provide、routeParams）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./internal/application/audit/... ./internal/infrastructure/repository/... ./internal/dto/... && go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/modelcall/ internal/infrastructure/repository/audit_repository.go internal/common/constant/sql.go internal/application/audit/ internal/dto/audit.go internal/dto/audit_stats.go internal/handler/audit.go internal/router/audit.go internal/bootstrap/
git commit -m "feat(pricing): 成本查询——审计行费用列、分币种合计/趋势、分组成本分布"
```

---

### Task 7: tzdata 内嵌 + 领域词汇回写

**Files:**
- Modify: `cmd/server/main.go`（`import _ "time/tzdata"`）
- Modify: `CONTEXT.md`、`web/CONTEXT.md`

**Interfaces:**
- Consumes: Task 1 的时区匹配（`time.LoadLocation`）
- Produces: 生产镜像缺 zoneinfo 时非 UTC 时区规则仍可加载；领域词汇与代码同步

- [ ] **Step 1: tzdata 内嵌**

`cmd/server/main.go` import 块追加（与标准库 import 归组）：

```go
	_ "time/tzdata" // 内嵌 IANA 时区库：生产镜像可能缺 zoneinfo，定价时段规则的非 UTC 时区依赖它
```

Run: `go build ./cmd/server/...`
Expected: PASS

- [ ] **Step 2: CONTEXT.md 词条**

在 `CONTEXT.md` 的 LLM Proxy 小节（ModelCapabilities 词条之后）追加：

```markdown
**ModelPricing（模型定价）**:
挂在 Model 上的计费配置：币种（CNY/USD，空=未计价）+ 定价规则表。价格单位为「每 1M tokens 的微单位」（1e-6 货币单位，int64 入账，禁浮点）。`currency` 为空 ⇔ 规则为空 ⇔ 未计价（审计 cost 记 NULL）；`currency` 非空且四价全 0 ⇔ 免费模型（审计 cost 记 0）。计价只覆盖 LLM 代理成功调用（失败不计费）。
_Avoid_: price config, billing config, rate card

**PricingRule（定价规则）**:
一条「可选时段条件 + 可选上下文区间条件 + 四类单价（输入/输出/缓存创建/缓存读取）」的计价规则，多条组成规则表，**数组顺序 = 匹配优先级（第一命中）**，且必须恰好包含一条无条件默认规则兜底。计价按整段跳档：命中哪条规则，全部 token 按该规则单价计。
_Avoid_: price tier, rate rule

**TimeWindow（时段窗口）**:
定价规则的时段条件：`[start, end)` 半开区间（HH:MM），`end < start` 表示跨午夜；`days` 为空=每天、成员 1=周一…7=周日（按调用时刻在窗口时区下的周几判定）；`timezone` 为 IANA 时区名（默认 UTC）。多窗口为 OR 关系。
_Avoid_: schedule, time slot

**ContextTier（上下文区间）**:
定价规则的上下文条件：按本次调用的 prompt 总 token（input + cacheCreation + cacheRead）落档，`context_min ≤ promptTokens < context_max`（`context_max=0` 表无上限），**整段跳档**（不做累进分段）——与 Gemini/Claude 官方分档口径一致。
_Avoid_: context pricing, token bracket

**EstimatedCost（估算费用）**:
一次模型调用的费用估算（`model_call_audits.cost_micro`，微单位，NULL=未计价），按调用时刻与 prompt 总 token 匹配定价规则后计算：四项分别「单价 × tokens / 1e6 四舍五入」求和。请求时计算并落库（不随改价漂移），同时快照 `pricing_currency`。统计聚合按币种分组，不做汇率换算。
_Avoid_: cost, billing amount, charge

**PricingPrefill（定价导入辅助）**:
从 models.dev 公开定价按 `upstream_model` 精确匹配（trim 后、区分大小写）查询 USD 单价，仅用于填充录入表单的默认规则行，**永不自动改价**；未命中/上游不可达一律降级为「未找到，可手填」。
_Avoid_: price import, auto pricing
```

Model 词条补充一句：「`Pricing`（见 ModelPricing）随聚合装配，未配置定价的模型不产生估算费用。」

`web/CONTEXT.md` 补对应前端术语（定价规则编辑器、费用列、成本合计/趋势/分布、本期成本卡）。

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go CONTEXT.md web/CONTEXT.md
git commit -m "feat(pricing): 内嵌 tzdata 保障非 UTC 时区规则；回写定价领域词汇"
```

---

### Task 8: E2E（test/e2e/model_pricing/）

**Files:**
- Create: `test/e2e/model_pricing/pricing_test.go`

**Interfaces:**
- Consumes: Task 4/5/6 的 HTTP 契约（proxy 调用、web JWT 审计/成本接口）
- Produces: 黑盒回归用例（离线 skip，同仓 E2E 惯例：`BASE_URL`/`API_KEY` 缺失即 `t.Skip`）

前置环境（文档化在文件头注释）：`BASE_URL`（网关地址）、`API_KEY`（Proxy Key）、`MODEL_ALIAS`（已配置定价的模型别名）、`MODEL_ALIAS_UNPRICED`（未配置定价的模型别名）、`WEB_JWT`（web JWT，缺省时跳过成本断言）。

- [ ] **Step 1: 写用例** `test/e2e/model_pricing/pricing_test.go`

```go
// Package model_pricing 模型计费 E2E。
//
// 前置：目标环境已配置带定价的模型别名 MODEL_ALIAS（如 currency=CNY、
// input=1 元/1M、output=2 元/1M）与未计价别名 MODEL_ALIAS_UNPRICED；
// WEB_JWT 提供 web 管理视角（缺省跳过成本断言）。默认离线 skip，不打生产。
package model_pricing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func mustEnv(t *testing.T, keys ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			t.Skipf("%s is required for e2e test", k)
		}
		out[k] = v
	}
	return out
}

func newE2EClient() *http.Client { return &http.Client{Timeout: 60 * time.Second} }

// postChat 发起一次非流式 chat 调用，返回响应体。
func postChat(t *testing.T, baseURL, apiKey, model string) map[string]any {
	t.Helper()
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":"hi"}]}`, model)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/api/openai/v1/chat/completions", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, raw)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return obj
}

// getJSON 带 JWT 的 web GET；返回解析后的 JSON。
func getJSON(t *testing.T, baseURL, jwt, path string) map[string]any {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		strings.TrimRight(baseURL, "/")+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, body = %s", path, resp.StatusCode, raw)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return obj
}

// latestAuditCost 查审计列表最新一行的 cost/currency（轮询等待异步落库）。
func latestAuditCost(t *testing.T, baseURL, jwt, model string) (cost any, currency string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		obj := getJSON(t, baseURL, jwt,
			fmt.Sprintf("/api/web/v1/audit/list?page=1&pageSize=20&query=%s", model))
		logs, _ := obj["logs"].([]any)
		for _, l := range logs {
			item, _ := l.(map[string]any)
			if item["modelId"] == model {
				return item["cost"], fmt.Sprint(item["pricingCurrency"])
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("no audit row for model %s within deadline", model)
	return nil, ""
}

func TestPricing_CostRecordedOnSuccess(t *testing.T) {
	env := mustEnv(t, "BASE_URL", "API_KEY", "MODEL_ALIAS", "WEB_JWT")
	postChat(t, env["BASE_URL"], env["API_KEY"], env["MODEL_ALIAS"])
	cost, currency := latestAuditCost(t, env["BASE_URL"], env["WEB_JWT"], env["MODEL_ALIAS"])
	if cost == nil {
		t.Fatalf("priced model call must have cost, got null (currency=%s)", currency)
	}
	if currency == "" || currency == "null" {
		t.Fatalf("pricing currency snapshot missing")
	}
}

func TestPricing_UnpricedModelHasNullCost(t *testing.T) {
	env := mustEnv(t, "BASE_URL", "API_KEY", "MODEL_ALIAS_UNPRICED", "WEB_JWT")
	postChat(t, env["BASE_URL"], env["API_KEY"], env["MODEL_ALIAS_UNPRICED"])
	cost, _ := latestAuditCost(t, env["BASE_URL"], env["WEB_JWT"], env["MODEL_ALIAS_UNPRICED"])
	if cost != nil {
		t.Fatalf("unpriced model call must have null cost, got %v", cost)
	}
}

func TestPricing_CostSummaryAndDistribution(t *testing.T) {
	env := mustEnv(t, "BASE_URL", "WEB_JWT")
	summary := getJSON(t, env["BASE_URL"], env["WEB_JWT"], "/api/web/v1/audit/cost/summary?granularity=day")
	totals, _ := summary["totals"].([]any)
	if totals == nil {
		t.Fatalf("summary.totals missing: %v", summary)
	}
	dist := getJSON(t, env["BASE_URL"], env["WEB_JWT"],
		"/api/web/v1/audit/cost/distribution?groupBy=model&limit=10")
	if dist["groupBy"] != "model" {
		t.Fatalf("distribution.groupBy = %v", dist["groupBy"])
	}
}
```

注：审计列表路由以 `internal/router/audit.go` 实际 path 为准（`rg -n "Path:" internal/router/audit.go`）；若为 `/api/web/v1/audit` + `/logs` 之类，同步调整 `latestAuditCost` 的 URL 与 query 参数名（`ListAuditLogsReq` 的 `query` 字段）。

- [ ] **Step 2: 离线确认 skip、在线跑通**

Run: `go test ./test/e2e/model_pricing/... -v`
Expected: 无环境变量时全部 SKIP；配置环境后全 PASS

- [ ] **Step 3: Commit**

```bash
git add test/e2e/model_pricing/
git commit -m "test(e2e): 模型计费回归——计价落库、未计价 NULL、成本合计与分布"
```

---

### Task 9: 前端（定价规则编辑器 + 成本展示）

**Files:**
- Modify: `web/src/lib/types.ts`（定价/成本类型镜像）
- Create: `web/src/lib/money.ts`（金额格式化）
- Create: `web/src/app/(dashboard)/upstream/pricing-editor.tsx`（规则列表编辑器）
- Modify: `web/src/app/(dashboard)/upstream/model-dialog.tsx`（嵌入编辑器、提交载荷）
- Create: `web/src/app/(dashboard)/audit/cost-panel.tsx`（成本合计卡 + 趋势 + 分布块）
- Modify: `web/src/app/(dashboard)/audit/`（列表费用列 + 挂 cost-panel）、`web/src/app/(dashboard)/page.tsx`（本期成本卡）
- Modify: `web/src/lib/api-client.ts`（prefill/cost 接口 + 模型载荷带 pricing）
- Modify: `web/src/locales/zh.json`、`en.json`、`ja.json`（i18n key）

**Interfaces:**
- Consumes: Task 4/5/6 的 HTTP 契约
- Produces: `PricingDTO`/`PricingRuleDTO`/`TimeWindowDTO` TS 镜像、`formatCost(v: number, currency: string): string`、`<PricingEditor value onChange onPrefill>`、`prefillModelPricing/upstreamModelPricing 等 api-client 方法`

- [ ] **Step 1: 类型与工具**

`web/src/lib/types.ts` 追加（与 dto 字段名一致）：

```ts
export interface TimeWindowDTO {
  days?: number[];
  start: string;
  end: string;
  timezone?: string;
}
export interface PricingRuleDTO {
  time_windows?: TimeWindowDTO[];
  context_min?: number;
  context_max?: number;
  input_price: number;
  output_price: number;
  cache_creation_price: number;
  cache_read_price: number;
}
export interface PricingDTO {
  currency?: "" | "CNY" | "USD";
  rules?: PricingRuleDTO[];
}
export interface PricingPrefillRsp extends CommonRsp {
  found: boolean;
  currency?: "USD";
  inputPrice?: number;
  outputPrice?: number;
  cacheCreationPrice?: number;
  cacheReadPrice?: number;
}
export interface AuditCostSummaryRsp extends CommonRsp {
  totals: { currency: string; cost: number }[];
  series: { bucketTime: string; currency: string; cost: number }[];
}
export interface AuditCostDistributionRsp extends CommonRsp {
  groupBy: string;
  items: { id: string; name: string; currency: string; cost: number }[];
}
```

（`AuditLogItem` 加 `cost?: number | null`、`pricingCurrency?: string`；`ModelListItem`/`UpstreamModelItem`/模型创建更新载荷加 `pricing?: PricingDTO`——按文件内既有镜像风格摆放。）

`web/src/lib/money.ts`：

```ts
// formatCost 金额展示：去尾零、最多 6 位小数，规避浮点尾差直出（0.30000000000000004）
export function formatCost(v: number | null | undefined, currency?: string): string {
  if (v === null || v === undefined) return "—";
  const symbol = currency === "CNY" ? "¥" : currency === "USD" ? "$" : "";
  const s = v.toFixed(6).replace(/\.?0+$/, "");
  return `${symbol}${s === "" ? "0" : s}`;
}
```

- [ ] **Step 2: 定价规则编辑器** `web/src/app/(dashboard)/upstream/pricing-editor.tsx`

组件契约：`<PricingEditor value={PricingDTO | undefined} onChange={(p: PricingDTO | undefined) => void} onPrefill={async () => PricingPrefillRsp | null} />`。核心行为（完整实现按此语义写，配 shadcn 风格 Input/Select/Button——沿用 `model-dialog.tsx` 既有表单组件）：

```tsx
// PricingEditor 定价规则编辑器。
// - 币种下拉（无 / CNY / USD）；币种为空 = 未计价（规则区隐藏）
// - 规则行：时段窗口组（可增删：周几多选 + 起止时刻 + 时区）、上下文区间（min/max，max 空=无上限）、四价
// - 行操作：增、删、上移/下移（数组顺序 = 匹配优先级）
// - 默认规则行（time_windows 空 + context 0/0）标注「默认」，保存前校验恰好一条
// - 「从 models.dev 导入」按钮调 onPrefill，命中则填充默认规则行四价（仅填充，可改）
export function PricingEditor({ value, onChange, onPrefill }: PricingEditorProps) {
  // 状态即 value（受控）；所有修改经 onChange 产出新 PricingDTO。
  // 校验（保存前在 model-dialog 触发，非法行内红字）：
  //   - currency 非空时规则非空且恰一条无条件规则
  //   - start/end 为 HH:MM 且不相等；days ∈ 1..7
  //   - context_max = 0 或 > context_min；价格 ≥ 0
}
```

实现细节（必须落进代码）：

1. 「导入」按钮调 `onPrefill` → `found=true` 时：`currency` 设 USD（若当前为空）、默认规则行四价写入 `inputPrice/outputPrice/cacheCreationPrice/cacheReadPrice`；`found=false` 时 toast「未找到，可手动填写」
2. `context_max` 输入框空串 ↔ `0`（0=无上限）互转；价格输入框 `number` step=`0.000001`
3. 移动按钮用数组 swap 产出新 `rules`

`model-dialog.tsx` 接线：

1. 表单 state 加 `pricing: PricingDTO | undefined`
2. 提交载荷 `pricing`（undefined 时省略字段；用户显式清空币种时传 `{currency:"", rules:[]}` 表示清空为未计价）
3. 「从 models.dev 导入」的 `onPrefill` 实现调 api-client 的 `prefillModelPricing(upstreamModel 字段值)`
4. 编辑回填：`item.pricing` → 表单

- [ ] **Step 3: api-client 与成本展示**

`web/src/lib/api-client.ts` 追加（沿用既有请求封装与错误 toast 机制）：

```ts
// prefillModelPricing 按上游模型名查询公开定价（表单填充用）
async prefillModelPricing(upstreamModel: string): Promise<PricingPrefillRsp> {
  return this.request<PricingPrefillRsp>(
    "GET", `/api/web/v1/model/pricing/prefill?upstreamModel=${encodeURIComponent(upstreamModel)}`);
}
// getAuditCostSummary 成本合计与趋势（按币种）
async getAuditCostSummary(params: { startTime?: string; endTime?: string; granularity?: string }): Promise<AuditCostSummaryRsp> { ... }
// getAuditCostDistribution 成本分布（groupBy: user|api_key|model）
async getAuditCostDistribution(params: { groupBy: string; startTime?: string; endTime?: string; limit?: number }): Promise<AuditCostDistributionRsp> { ... }
```

`web/src/app/(dashboard)/audit/cost-panel.tsx`（新组件）：

1. 成本合计卡：`totals` 按币种分行（`formatCost`）
2. 成本趋势线：`series` 按币种拆线（复用 `web/src/components/charts/` 既有折线图组件，图例币种命名）
3. 成本分布块：分组切换（模型 / API Key，admin 多「用户」）→ `getAuditCostDistribution`，条形/表格展示 `name` + `formatCost`（按币种分段）
4. 时间范围与页面顶部筛选联动（沿用 audit 页既有 time-range 状态）

`audit` 页接线：列表加「费用」列（`formatCost(row.cost, row.pricingCurrency)`，null → `—`）；统计区挂 `<CostPanel />`。

`page.tsx`（Dashboard 首页）：本期成本卡（`getAuditCostSummary`，按币种分行，点击跳 `/audit`）。

- [ ] **Step 4: i18n 与校验**

`web/src/locales/zh.json`/`en.json`/`ja.json` 补 key（三份都要）：

```json
{
  "upstream.pricing.title": "定价",
  "upstream.pricing.currency": "计价币种",
  "upstream.pricing.currency.none": "未计价",
  "upstream.pricing.rule.default": "默认",
  "upstream.pricing.rule.add": "添加规则",
  "upstream.pricing.rule.moveUp": "上移",
  "upstream.pricing.rule.moveDown": "下移",
  "upstream.pricing.prefill": "从 models.dev 导入",
  "upstream.pricing.prefill.miss": "未找到，可手动填写",
  "upstream.pricing.window.add": "添加时段窗口",
  "upstream.pricing.context.max.placeholder": "无上限",
  "audit.cost.title": "估算成本",
  "audit.cost.column": "费用",
  "audit.cost.summary": "成本合计",
  "audit.cost.trend": "成本趋势",
  "audit.cost.distribution": "成本分布",
  "audit.cost.groupBy.user": "按用户",
  "audit.cost.groupBy.apiKey": "按 API Key",
  "audit.cost.groupBy.model": "按模型",
  "audit.cost.unpriced": "未计价",
  "dashboard.costCard": "本期成本"
}
```

（ja.json 用对应日文翻译，勿留英文占位。）

- [ ] **Step 5: 前端校验与提交**

Run: `cd web && npm run lint && npm run build`
Expected: PASS

```bash
git add web/src web/src/locales
git commit -m "feat(web): 定价规则编辑器与成本展示（费用列/合计/趋势/分布/首页卡片）"
```

---

### Task 10: 全量验证与收尾

- [ ] **Step 1: 全量测试与 lint**

Run: `make lint && make test`（E2E 需 embed 占位 `internal/web/dist/index.html`；web 构建产物按需 `cp -r web/out internal/web/dist`）
Expected: 全绿

- [ ] **Step 2: Web 运行时验证**

按 `next-dev-loop` skill 跑编辑/验证循环：upstream 编辑弹窗（加规则/导入/保存回显）、audit 费用列与成本图表、Dashboard 成本卡。仅 lint/build 不算验证通过。

- [ ] **Step 3: ponytail-review**

对本次 diff 跑 `ponytail-review`，逐行列出投机抽象/重复造轮子/死代码并清理；刻意简化处按约定补 `// ponytail: <ceiling>, <upgrade path>` 注释（例：prefill 不做模糊匹配、分布不做分页）。

- [ ] **Step 4: Serena 沉淀**

`serena_write_memory` 记录：定价规则表设计（第一命中/默认兜底/整段跳档）、微单位整数半入公式与溢出拆解、`updateModelTx` map 更新可清零值、`ModelRepoFieldsFull` 白名单坑、tzdata 内嵌原因。不写临时状态与凭据。

- [ ] **Step 5: 汇报与合并询问**

汇报验证证据（命令与结果），询问用户提 MR 或直接合并——禁止擅自 push/merge。

---

## Self-Review 记录

- **Spec 覆盖**：定价规则模型（T1/T2）、落库计费（T3）、CRUD+prefill（T4/T5）、成本查询三件套（T6）、tzdata+词汇（T7）、E2E（T8）、前端四块（T9）、验证（T10）——spec 各章节均有对应任务；spec 微调两处已在文首声明。
- **占位符**：无 TBD/TODO；`rg` 定位点均给出实际命令；Task 9 编辑器给出组件契约与必落实现细节（受控组件按既有表单风格实现）。
- **类型一致性**：`PriceMicroFromDisplay/PriceDisplayFromMicro`（T4 定义，T6/T9 使用）、`port.PricingQuoteProvider`（T5 内自洽）、`modelcall.Cost*` 与仓储签名（T6 内自洽）、`dto.Cost *float64` 与 `GetCostMicro()`（T3→T6）已核对。
