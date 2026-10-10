# 端点调度与成本增强（#13~#17）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 aris-proxy-api 落地 5 个功能：Claude 缓存 5m/1h 分档计价、模型配置健康检测、优先级/权重调度 + 跨端点 fallback、端点亲和性路由、Playground 调试台。

**Architecture:** 沿 DDD 分层做增量扩展：`dto`（wire 解析）→ `domain/llmproxy/vo`（计价）+ `domain/llmproxy/service`（端点解析）→ `application/llmproxy/usecase`（转发编排）→ `infrastructure`（DB 列 / Redis 亲和）→ `web`（弹窗与新页）。不重构既有链路，只扩字段、扩签名、加循环。

**Tech Stack:** Go 1.25 · Fiber v3 · Huma v2 · GORM · Redis · sonic · samber/lo · Next.js 16 / React 19 / TS / Tailwind 4

**Spec:** `docs/superpowers/specs/2026-10-09-endpoint-scheduling-pricing-design.md`

## Global Constraints

- 业务错误统一 `internal/common/ierr`（`ierr.New` / `ierr.Wrap`）；禁止 `errors.New` / `fmt.Errorf`（判定第三方 sentinel 的 `errors.Is/As` 除外）。
- JSON 一律 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`。
- DTO 层禁止 `any` / `interface{}`（用具体结构体或 `sonic.NoCopyRawMessage`）；禁止导入 `internal/infrastructure/database/model`。
- 业务包禁止本地 `const` 块：常量进 `internal/common/constant/`（字符串模板进 `string.go`、SQL 字段名进 `sql.go`），枚举进 `internal/common/enum/`。
- 价格一律 int64 微单位（1e-6 货币单位/1M tokens）；禁止浮点计价。展示换算用 `dto.PriceMicroFromDisplay` / `dto.PriceDisplayFromMicro`。
- Token 四维互斥口径：`净输入 + 缓存创建(总量) + 缓存读取 = 上游口径输入总量`；新增的 1h 明细是「缓存创建总量」的子集（5m = 总量 − 1h），不改变互斥式。
- 存量兼容：`cache_creation_input_tokens` 列保持总量语义；`cache_creation_price` 保持存在（语义扩为 5m 价 + 1h 回落价）。禁止破坏性列变更，全部改动走 AutoMigrate 加列。
- 测试只放 `test/unit/<topic>/` 与 `test/e2e/<topic>/`；`*_test.go` 不进 `internal/`；只用标准库 `testing`（禁 testify/gomock）；禁止 `time.Sleep` 同步。
- 每个 Task 开工前跑 `sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <将编辑的文件>` 并遵循返回指南（禁止截断输出）；与本约束冲突时以本约束为准。
- Context 契约：函数从调用方接收 `ctx`；异步任务用 `util.CopyContextValues(ctx)`；context key 注册到 `internal/common/constant/ctx.go`；读值用 `util.CtxValueString/Uint/Bool`。
- 日志 `logger.WithCtx(ctx)`，消息前缀 `[PascalCaseModule]`；secret 用 `util.MaskSecret`。
- 路由规范：Web 端 `/api/web/v1/{resource}[/{action}]`，操作走 path 段，按 ID 的更新/删除用 `?id=` query。
- 提交信息用中文描述；每 Task 结束提交一次（`git add` 精确到文件）。
- Ponytail：不建投机抽象；刻意简化处标注 `// ponytail: <ceiling>, <upgrade path>`。

## File Structure

| 文件 | 动作 | 职责 |
|---|---|---|
| `internal/dto/anthropic/anthropic.go` | 修改 | `AnthropicUsage` 解析 `cache_creation{5m,1h}` |
| `internal/dto/asynctask.go` | 修改 | `ModelCallAuditTask` 带 1h 明细 |
| `internal/domain/llmproxy/vo/pricing.go` | 修改 | `PricingRule.CacheCreate1hMicro`、`CostBreakdown` 分档计算 |
| `internal/domain/llmproxy/vo/token_breakdown.go` | 修改 | `cacheCreation1h` 字段与访问器 |
| `internal/domain/llmproxy/aggregate/model.go` | 修改 | Model 聚合带 `priority`/`weight` |
| `internal/domain/llmproxy/service/resolver.go` | 重写 | `ResolveCandidates` 排序 + 亲和置顶 |
| `internal/application/llmproxy/usecase/recorder.go` | 修改 | `PriceModelCall` 分档计价 |
| `internal/application/llmproxy/usecase/common.go` | 修改 | `canSwitchEndpoint` 判定 |
| `internal/application/llmproxy/usecase/openai.go` | 修改 | 候选遍历 + fallback（2 入口） |
| `internal/application/llmproxy/usecase/anthropic.go` | 修改 | 候选遍历 + fallback（1 入口） |
| `internal/application/llmproxy/usecase/affinity.go` | 新建 | 亲和键提取 + Redis 读写 |
| `internal/application/model/port/list_model.go` | 修改 | `ListModelView.ConfigMissing` |
| `internal/application/model/port/pricing.go` | 修改 | DTO↔vo 1h 价映射 |
| `internal/application/model/query/list_model.go` | 修改 | configMissing 判定 |
| `internal/application/model/command/create_model.go` / `update_model.go` | 修改 | priority/weight 参数 |
| `internal/infrastructure/database/model/model.go` | 修改 | `models.priority/weight`、`ModelPricingRule.CacheCreation1hPriceMicro` |
| `internal/infrastructure/database/model/model_call_audit.go` | 修改 | `cache_creation_1h_input_tokens` 列 |
| `internal/infrastructure/repository/model_repository.go` | 修改 | `pricingFromDB/pricingToDB`、`ModelRepoFieldsFull` |
| `internal/infrastructure/repository/audit_repository.go` | 修改 | 1h 列读写映射 |
| `internal/common/constant/ctx.go` | 修改 | `CtxKeyPlayground`（若需标识；见 Task 11 说明） |
| `internal/common/constant/string.go` | 修改 | Redis 亲和 key 模板 |
| `internal/router/playground.go` + `internal/handler/playground.go` | 新建 | Playground 路由与 handler |
| `internal/dto/pricing.go` | 修改 | `PricingRuleDTO.CacheCreation1hPrice` |
| `internal/dto/model.go` | 修改 | model DTO 的 priority/weight/configMissing |
| `web/src/app/(dashboard)/upstream/pricing-editor.tsx` | 修改 | 1h 价输入 |
| `web/src/app/(dashboard)/upstream/model-editor.tsx`（及 model 表单所在组件） | 修改 | priority/weight 输入 |
| `web/src/app/(dashboard)/upstream/page.tsx`（列表所在组件） | 修改 | configMissing 徽标 + 筛选 |
| `web/src/app/(dashboard)/playground/page.tsx` | 新建 | Playground 页面 |
| `web/src/components/nav.tsx`（实际导航组件） | 修改 | Playground 入口 |
| `test/unit/model_call_pricing/` | 修改/新增 | 5m/1h 计价与归一化测试 |
| `test/unit/endpoint_resolver/` | 修改 | 候选排序/亲和测试 |
| `test/unit/affinity/` | 新建 | 亲和键与 Redis 亲和测试 |
| `test/unit/model_list_query/` | 修改 | configMissing 判定测试 |
| `test/e2e/endpoint_fallback/` | 新建 | 跨端点 fallback E2E |
| `test/e2e/playground/` | 新建 | Playground E2E |
| `CONTEXT.md` | 修改 | TokenAccounting/ModelPricing/新概念词条 |

---

## Task 1: Anthropic 5m/1h usage 归一化采集

**Files:**
- Modify: `internal/dto/anthropic/anthropic.go`（`AnthropicUsage`，约 465 行处）
- Modify: `internal/dto/asynctask.go`（`ModelCallAuditTask` + `SetTokensFromAnthropicUsage`）
- Test: `test/unit/model_call_pricing/usage_5m1h_test.go`

**Interfaces:**
- Consumes: `dto.AnthropicMessage`（转发链路已解析）、`lo.FromPtr`
- Produces: `dto.AnthropicCacheCreation{Ephemeral5mInputTokens, Ephemeral1hInputTokens *int}`；`AnthropicUsage.CacheCreation *AnthropicCacheCreation`；`ModelCallAuditTask.CacheCreation1hInputTokens int`（后续 Task 2/3 消费）。不变量：`CacheCreationInputTokens` 恒为总量（5m+1h，旧格式时为原值）。

- [ ] **Step 1: 写失败测试**

`test/unit/model_call_pricing/usage_5m1h_test.go`：

```go
package model_call_pricing

import (
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/dto/anthropic"
)

func TestSetTokensFromAnthropicUsage5m1h(t *testing.T) {
	tests := []struct {
		name        string
		usageJSON   string
		wantTotal   int
		want1h      int
	}{
		{
			name:      "新版 cache_creation 对象 5m+1h",
			usageJSON: `{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":60,"cache_creation":{"ephemeral_5m_input_tokens":40,"ephemeral_1h_input_tokens":20}}`,
			wantTotal: 60,
			want1h:    20,
		},
		{
			name:      "旧版仅总量字段全部归 5m",
			usageJSON: `{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":50}`,
			wantTotal: 50,
			want1h:    0,
		},
		{
			name:      "cache_creation 对象优先于总量字段",
			usageJSON: `{"input_tokens":100,"output_tokens":10,"cache_creation_input_tokens":999,"cache_creation":{"ephemeral_5m_input_tokens":30,"ephemeral_1h_input_tokens":30}}`,
			wantTotal: 60,
			want1h:    30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var usage anthropic.AnthropicUsage
			if err := sonic.UnmarshalString(tt.usageJSON, &usage); err != nil {
				t.Fatalf("unmarshal usage: %v", err)
			}
			task := &dto.ModelCallAuditTask{}
			task.SetTokensFromAnthropicUsage(&anthropic.AnthropicMessage{Usage: &usage})
			if task.CacheCreationInputTokens != tt.wantTotal {
				t.Fatalf("CacheCreationInputTokens = %d, want %d", task.CacheCreationInputTokens, tt.wantTotal)
			}
			if task.CacheCreation1hInputTokens != tt.want1h {
				t.Fatalf("CacheCreation1hInputTokens = %d, want %d", task.CacheCreation1hInputTokens, tt.want1h)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/model_call_pricing/ -run TestSetTokensFromAnthropicUsage5m1h -v`
Expected: 编译失败 `unknown field CacheCreation` / `unknown field CacheCreation1hInputTokens`

- [ ] **Step 3: 最小实现**

`internal/dto/anthropic/anthropic.go`，在 `AnthropicUsage` 前加结构、并给 `AnthropicUsage` 加字段：

```go
// AnthropicCacheCreation 缓存创建明细（新版 usage.cache_creation 对象）。
type AnthropicCacheCreation struct {
	Ephemeral5mInputTokens *int `json:"ephemeral_5m_input_tokens,omitempty"`
	Ephemeral1hInputTokens *int `json:"ephemeral_1h_input_tokens,omitempty"`
}
```

`AnthropicUsage` 新增字段（放在 `CacheCreationInputTokens` 之后）：

```go
	CacheCreation *AnthropicCacheCreation `json:"cache_creation,omitempty"`
```

`internal/dto/asynctask.go`，`ModelCallAuditTask` 在 `CacheCreationInputTokens` 后新增：

```go
	CacheCreation1hInputTokens int
```

`SetTokensFromAnthropicUsage` 中把 `t.CacheCreationInputTokens = lo.FromPtr(msg.Usage.CacheCreationInputTokens)` 替换为：

```go
	switch {
	case msg.Usage.CacheCreation != nil:
		t.CacheCreation1hInputTokens = lo.FromPtr(msg.Usage.CacheCreation.Ephemeral1hInputTokens)
		t.CacheCreationInputTokens = lo.FromPtr(msg.Usage.CacheCreation.Ephemeral5mInputTokens) + t.CacheCreation1hInputTokens
	default:
		t.CacheCreationInputTokens = lo.FromPtr(msg.Usage.CacheCreationInputTokens)
	}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/model_call_pricing/ -v`
Expected: PASS（含既有用例）

- [ ] **Step 5: 提交**

```bash
git add internal/dto/anthropic/anthropic.go internal/dto/asynctask.go test/unit/model_call_pricing/usage_5m1h_test.go
git commit -m "feat(dto): Anthropic usage 解析缓存创建 5m/1h 明细"
```

---

## Task 2: 计价分档（PricingRule 1h 价 + CostBreakdown + PriceModelCall）

**Files:**
- Modify: `internal/domain/llmproxy/vo/pricing.go`（`PricingRule`、`CostBreakdown` 方法，约 29/215 行）
- Modify: `internal/application/llmproxy/usecase/recorder.go`（`PriceModelCall`，约 180 行）
- Test: `test/unit/model_call_pricing/cost_5m1h_test.go`

**Interfaces:**
- Consumes: Task 1 的 `ModelCallAuditTask.CacheCreation1hInputTokens`
- Produces: `vo.PricingRule.CacheCreate1hMicro int64`（0=回落 5m 价）；`PricingRule.CostBreakdown(input, output, cacheCreate5m, cacheCreate1h, cacheRead int64) CostBreakdown`（**签名由 4 参改 5 参**，`CostBreakdown.CacheCreateMicro` 保持合计语义）；`vo.TokenBreakdown` 新增 `cacheCreation1h`（见 Step 3）。

- [ ] **Step 1: 写失败测试**

`test/unit/model_call_pricing/cost_5m1h_test.go`：

```go
package model_call_pricing

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
)

func TestCostBreakdown5m1h(t *testing.T) {
	rule := vo.PricingRule{
		InputMicro:         1_000_000, // 1.00/1M
		OutputMicro:        2_000_000,
		CacheCreateMicro:   1_250_000, // 5m 价
		CacheCreate1hMicro: 2_500_000,
		CacheReadMicro:     100_000,
	}
	b := rule.CostBreakdown(100, 50, 40, 20, 40)
	// 40×1.25 + 20×2.5 = 50 + 50 = 100（整除，无舍入歧义）
	if b.CacheCreateMicro != 100 {
		t.Fatalf("CacheCreateMicro = %d, want 100", b.CacheCreateMicro)
	}
	if b.Total() != b.InputMicro+b.OutputMicro+b.CacheCreateMicro+b.CacheReadMicro {
		t.Fatalf("Total inconsistent")
	}
}

func TestCostBreakdown1hFallback(t *testing.T) {
	rule := vo.PricingRule{CacheCreateMicro: 1_250_000} // 1h 价未配置
	b := rule.CostBreakdown(0, 0, 8, 8, 0)
	// 1h 回落 5m 价：8×1.25 + 8×1.25 = 20（整除）
	if b.CacheCreateMicro != 20 {
		t.Fatalf("CacheCreateMicro = %d, want 20", b.CacheCreateMicro)
	}
}

func TestPriceModelCall5m1h(t *testing.T) {
	pricing, err := vo.NewPricing("USD", []vo.PricingRule{{
		InputMicro:         1_000_000,
		OutputMicro:        2_000_000,
		CacheCreateMicro:   1_250_000,
		CacheCreate1hMicro: 2_500_000,
		CacheReadMicro:     100_000,
	}})
	if err != nil {
		t.Fatalf("NewPricing: %v", err)
	}
	task := &dto.ModelCallAuditTask{
		InputTokens:                100,
		OutputTokens:               50,
		CacheCreationInputTokens:   30,
		CacheCreation1hInputTokens: 10,
		CacheReadInputTokens:       0,
		UpstreamStatusCode:         200,
		CreatedAt:                  time.Now(),
	}
	usecase.PriceModelCall(task, pricing)
	if task.CacheCreateCostMicro == nil {
		t.Fatal("CacheCreateCostMicro is nil")
	}
	// 20×1.25 + 10×2.5 = 50
	if *task.CacheCreateCostMicro != 50 {
		t.Fatalf("CacheCreateCostMicro = %d, want 50", *task.CacheCreateCostMicro)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/model_call_pricing/ -run 'TestCostBreakdown|TestPriceModelCall5m1h' -v`
Expected: 编译失败 `unknown field CacheCreate1hMicro` / `too many arguments in call to rule.CostBreakdown`

- [ ] **Step 3: 最小实现**

`internal/domain/llmproxy/vo/pricing.go`：

`PricingRule` 加字段（`CacheCreateMicro` 后）：

```go
	CacheCreate1hMicro int64
```

`CostBreakdown` 方法改为 5 参（替换原实现）：

```go
// CostBreakdown 按本规则单价计算四维费用拆分（微单位）；整段跳档由调用方 Match 选档。
// cacheCreate5m/cacheCreate1h 分档计价；CacheCreate1hMicro==0 时 1h 回落 5m 价。
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
```

`internal/application/llmproxy/usecase/recorder.go`，`PriceModelCall` 中 `promptTokens` 行后与 `rule.CostBreakdown(...)` 调用改为：

```go
	cacheCreate1h := int64(task.CacheCreation1hInputTokens)
	cacheCreate5m := int64(task.CacheCreationInputTokens) - cacheCreate1h
	breakdown := rule.CostBreakdown(
		int64(task.InputTokens),
		int64(task.OutputTokens),
		cacheCreate5m,
		cacheCreate1h,
		int64(task.CacheReadInputTokens),
	)
```

同步修复其他 `CostBreakdown` 调用点（Step 4 前执行定位）：

```bash
grep -rn "\.CostBreakdown(" internal/ test/ --include="*.go"
```

逐个补第 4 参（`cacheCreate5m`）与第 5 参（原 cacheRead）；`NewTokenBreakdown` 调用点同步扩为 5 参（`internal/domain/llmproxy/vo/token_breakdown.go` 的构造函数增加 `cacheCreation1h int` 参数并存入新字段，新增访问器 `CacheCreation1h() int`，`CacheCreation()` 改为返回 `cacheCreation`（总量语义不变）；`IsZero` 并入新字段）：

```go
func NewTokenBreakdown(input, output, cacheCreation, cacheCreation1h, cacheRead int) TokenBreakdown {
	return TokenBreakdown{input: input, output: output, cacheCreation: cacheCreation, cacheCreation1h: cacheCreation1h, cacheRead: cacheRead}
}

func (t TokenBreakdown) CacheCreation1h() int { return t.cacheCreation1h }
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/model_call_pricing/ ./test/unit/pricing/ ./test/unit/model_call_audit/ -v`
Expected: PASS；若 `grep` 出的调用点遗漏会编译失败，补齐后重跑。

- [ ] **Step 5: 提交**

```bash
git add internal/domain/llmproxy/vo/pricing.go internal/domain/llmproxy/vo/token_breakdown.go internal/application/llmproxy/usecase/recorder.go test/unit/model_call_pricing/cost_5m1h_test.go
git commit -m "feat(pricing): 缓存创建 5m/1h 分档计价与费用拆分"
```

---

## Task 3: 持久化与 wire 契约（DB 列 + DTO 映射）

**Files:**
- Modify: `internal/infrastructure/database/model/model.go`（`ModelPricingRule`）
- Modify: `internal/infrastructure/database/model/model_call_audit.go`（加列）
- Modify: `internal/infrastructure/repository/model_repository.go`（`pricingFromDB`/`pricingToDB`，83/110 行）
- Modify: `internal/infrastructure/repository/audit_repository.go`（1h 列映射）
- Modify: `internal/dto/pricing.go`（`PricingRuleDTO`）
- Modify: `internal/application/model/port/pricing.go`（`PricingFromDTO`/`PricingToDTO`）
- Test: `test/unit/pricing_dto/`（扩展）、`test/unit/model_repository/`（扩展）

**Interfaces:**
- Consumes: Task 2 的 `vo.PricingRule.CacheCreate1hMicro`
- Produces: DB JSON 列字段 `cache_creation_1h_price_micro`；审计列 `cache_creation_1h_input_tokens`；wire 字段 `cache_creation_1h_price`（展示单位，可选，0=回落）。DTO 校验语义：`cache_creation_1h_price` 缺省 0。

- [ ] **Step 1: 写失败测试**

`test/unit/pricing_dto/pricing_1h_test.go`：

```go
package pricing_dto

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

func TestPricingDTO1hRoundTrip(t *testing.T) {
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
	if len(out.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(out.Rules))
	}
	if out.Rules[0].CacheCreation1hPrice != 2.5 {
		t.Fatalf("CacheCreation1hPrice = %v, want 2.5", out.Rules[0].CacheCreation1hPrice)
	}
}

func TestPricingDTO1hDefaultZero(t *testing.T) {
	in := &dto.PricingDTO{Currency: "USD", Rules: []dto.PricingRuleDTO{{InputPrice: 1.0, CacheCreationPrice: 1.25}}}
	p, err := port.PricingFromDTO(in)
	if err != nil {
		t.Fatalf("PricingFromDTO: %v", err)
	}
	if got := port.PricingToDTO(p).Rules[0].CacheCreation1hPrice; got != 0 {
		t.Fatalf("CacheCreation1hPrice = %v, want 0 (回落语义)", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/pricing_dto/ -run 1h -v`
Expected: 编译失败 `unknown field CacheCreation1hPrice`

- [ ] **Step 3: 最小实现**

1. `internal/dto/pricing.go` `PricingRuleDTO` 加字段（`CacheCreationPrice` 后）：

```go
	CacheCreation1hPrice float64 `json:"cache_creation_1h_price,omitempty" minimum:"0" maximum:"1000000" doc:"1h 缓存创建单价（可选；0=回落 5m 档 cache_creation_price）"`
```

2. `internal/application/model/port/pricing.go`：`PricingFromDTO` 的 `vo.PricingRule` 字面量加一行、`PricingToDTO` 加一行：

```go
			CacheCreate1hMicro: dto.PriceMicroFromDisplay(r.CacheCreation1hPrice),
```
```go
				CacheCreation1hPrice: dto.PriceDisplayFromMicro(r.CacheCreate1hMicro),
```

3. `internal/infrastructure/database/model/model.go` `ModelPricingRule` 加字段（`CacheCreationPriceMicro` 后）：

```go
	CacheCreation1hPriceMicro int64 `json:"cache_creation_1h_price_micro"`
```

4. `internal/infrastructure/repository/model_repository.go`：`pricingFromDB` 的 `vo.PricingRule` 字面量加 `CacheCreate1hMicro: r.CacheCreation1hPriceMicro`；`pricingToDB` 的 `dbmodel.ModelPricingRule` 字面量加 `CacheCreation1hPriceMicro: r.CacheCreate1hMicro`。

5. `internal/infrastructure/database/model/model_call_audit.go` 加列（`CacheCreationInputTokens` 后）：

```go
	CacheCreation1hInputTokens int `json:"cache_creation_1h_input_tokens" gorm:"column:cache_creation_1h_input_tokens;not null;default:0;comment:1h 缓存写入token数(存量=0,5m=总量-1h)"`
```

6. `internal/infrastructure/repository/audit_repository.go`：审计实体映射处（`CacheCreationInputTokens` 的读写行附近）同步 `CacheCreation1hInputTokens` 读写；`dto.ModelCallAuditTask` → DB 与 DB → 查询 DTO 两处都补。用 `grep -n "CacheCreationInputTokens" internal/infrastructure/repository/audit_repository.go internal/application/audit -r` 定位全部映射点，与既有字段同形补齐。

- [ ] **Step 4: 运行测试确认通过（含 AutoMigrate）**

Run: `go test ./test/unit/pricing_dto/ ./test/unit/model_repository/ ./test/unit/model_pricing_agg/ ./test/unit/audit_repo/ -v`
Expected: PASS（sqlite 测试自动建新列）

- [ ] **Step 5: 提交**

```bash
git add internal/dto/pricing.go internal/application/model/port/pricing.go internal/infrastructure/database/model/model.go internal/infrastructure/database/model/model_call_audit.go internal/infrastructure/repository/model_repository.go internal/infrastructure/repository/audit_repository.go test/unit/pricing_dto/pricing_1h_test.go
git commit -m "feat(persistence): 1h 缓存计价与审计明细持久化映射"
```

---

## Task 4: 前端定价弹窗 1h 价

**Files:**
- Modify: `web/src/app/(dashboard)/upstream/pricing-editor.tsx`
- Test: `web/src/app/(dashboard)/upstream/__tests__/`（沿用现有测试位置）

**Interfaces:**
- Consumes: wire 字段 `cache_creation_1h_price`（Task 3）
- Produces: 定价规则编辑行的 `cacheCreation1hPrice` 表单状态（与现有四价同构）。

- [ ] **Step 0: 定位组件导出形态**

Run: `grep -n "export" web/src/app/\(dashboard\)/upstream/pricing-editor.tsx | head`
记录导出的组件名与 props 签名（下文以 `PricingEditor` 指代实际导出组件，props 以实际为准）；同时看该目录 `__tests__/` 下现有测试文件的渲染方式（wrapper/Provider），测试写法对齐。

- [ ] **Step 1: 写失败测试**

`web/src/app/(dashboard)/upstream/__tests__/pricing-editor-1h.test.tsx`（沿用该目录既有测试的渲染方式，断言字段存在与回落文案）：

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PricingEditor } from "../pricing-editor";

describe("PricingEditor 1h 缓存创建价", () => {
  it("渲染 1h 价输入框占位说明回落语义", () => {
    render(<PricingEditor value={[]} onChange={() => {}} />);
    expect(screen.getByLabelText("1h 缓存创建价")).toBeTruthy();
    expect(screen.getByPlaceholderText(/回落 5m/)).toBeTruthy();
  });
});
```

（若 `PricingEditor` props 形态不同，以现有组件导出签名为准调整渲染参数，断言不变。）

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && npx vitest run src/app/\(dashboard\)/upstream/__tests__/pricing-editor-1h.test.tsx`
Expected: FAIL（找不到 label「1h 缓存创建价」）

- [ ] **Step 3: 最小实现**

在 `pricing-editor.tsx` 每条规则行的「缓存创建价」输入旁加同构输入：label「1h 缓存创建价」、`type="number"`、`min={0}`、placeholder「留空回落 5m」、值绑定规则对象的 `cacheCreation1hPrice`（与 `cacheCreationPrice` 同样的受控更新路径）；规则的序列化/反序列化（若有 `micro` 换算）沿用 `cacheCreationPrice` 的换算函数。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd web && npx vitest run && npm run lint`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add web/src/app/\(dashboard\)/upstream/pricing-editor.tsx web/src/app/\(dashboard\)/upstream/__tests__/pricing-editor-1h.test.tsx
git commit -m "feat(web): 定价弹窗支持 1h 缓存创建价"
```

---

## Task 5: 模型配置健康检测（configMissing）

**Files:**
- Modify: `internal/application/model/port/list_model.go`（`ListModelView`）
- Modify: `internal/application/model/query/list_model.go`（`toListModelView`，95 行）
- Modify: `internal/handler/model.go`（响应 DTO 组装，192 行附近）
- Modify: `internal/dto/model.go`（列表项 DTO）
- Modify: `web/src/app/(dashboard)/upstream/`（列表徽标 + 筛选）
- Test: `test/unit/model_list_query/`（扩展）

**Interfaces:**
- Consumes: `Model.Pricing().IsPriced()`、`Model.ContextLength()`
- Produces: `ListModelView.ConfigMissing []string`（成员 `"pricing"` / `"spec"`）；wire 字段 `config_missing []string`。

- [ ] **Step 0: 定位工厂签名**

Run: `grep -n "func CreateModel" -A 3 internal/domain/llmproxy/aggregate/model.go`
记录真实参数序（下文测试代码按实际签名对齐，断言不变）；同时看 `test/unit/model_command/` 现有测试的仓储 stub 构造方式。

- [ ] **Step 1: 写失败测试**

`test/unit/model_list_query/config_missing_test.go`（沿用该目录既有 fixture 构造方式，直接单测判定函数）：

```go
package model_list_query

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

func TestConfigMissing(t *testing.T) {
	m, err := aggregate.CreateModel(1, "gpt-4", "gpt-4-0613", 10, true, 0, 0, []string{"text"}, vo.Pricing{})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	got := configMissing(m)
	want := []string{"pricing", "spec"}
	if !slices.Equal(got, want) {
		t.Fatalf("configMissing = %v, want %v", got, want)
	}

	priced, _ := vo.NewPricing("USD", []vo.PricingRule{{InputMicro: 1}})
	m2, _ := aggregate.CreateModel(2, "gpt-4", "gpt-4-0613", 10, true, 128000, 4096, []string{"text"}, priced)
	if got := configMissing(m2); len(got) != 0 {
		t.Fatalf("configMissing = %v, want empty", got)
	}
}
```

（`CreateModel` 参数序以 Step 0 grep 结果为准对齐；测试断言不变。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/model_list_query/ -run TestConfigMissing -v`
Expected: 编译失败 `undefined: configMissing`

- [ ] **Step 3: 最小实现**

`internal/application/model/query/list_model.go` 新增判定函数（`toListModelView` 上方）：

```go
// configMissing 计算模型配置缺失项：pricing=未计价（currency 空），spec=未填规格（context_length=0）。
func configMissing(m *llmagg.Model) []string {
	var missing []string
	if !m.Pricing().IsPriced() {
		missing = append(missing, "pricing")
	}
	if m.ContextLength() == 0 {
		missing = append(missing, "spec")
	}
	return missing
}
```

`ListModelView` 加字段 `ConfigMissing []string`；`toListModelView` 组装 `ConfigMissing: configMissing(m)`；`internal/handler/model.go` 列表响应 DTO 加 `ConfigMissing []string \`json:"config_missing"\`` 并映射；`internal/dto/model.go` 列表项 DTO 同步字段。demo 脱敏逻辑不涉及此字段（缺失项本身无敏感信息）。

前端：列表行加徽标（`pricing` → 「未定价」、`spec` → 「未填规格」，样式沿用现有 badge 组件）；筛选控件加「仅看配置缺失」toggle，命中 `config_missing.length > 0` 的行。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/model_list_query/ ./test/unit/model_handler/ -v && cd web && npx vitest run && npm run lint`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/application/model/port/list_model.go internal/application/model/query/list_model.go internal/handler/model.go internal/dto/model.go web/src/app/\(dashboard\)/upstream/ test/unit/model_list_query/config_missing_test.go
git commit -m "feat(model): 模型配置缺失检测与前端提示"
```

---

## Task 6: models 表 priority/weight（数据模型 + 命令 + 表单）

**Files:**
- Modify: `internal/infrastructure/database/model/model.go`（`Model` struct）
- Modify: `internal/common/constant/sql.go`（`ModelRepoFieldsFull`）
- Modify: `internal/domain/llmproxy/aggregate/model.go`（字段 + `CreateModel`/`Update`/访问器）
- Modify: `internal/application/model/command/create_model.go` / `update_model.go` + `port/handler.go`（命令字段）
- Modify: `internal/handler/model.go`、`internal/dto/model.go`（wire 字段）
- Modify: `web/src/app/(dashboard)/upstream/`（模型表单两个输入）
- Test: `test/unit/model_command/`（扩展）

**Interfaces:**
- Consumes: 无（基础数据模型）
- Produces: `Model.Priority() int`（数字小=优先级高）、`Model.Weight() int`（>0）；DB 列 `models.priority`（default 0）、`models.weight`（default 1）；wire `priority`/`weight`。Task 7 的排序消费这两个访问器。

- [ ] **Step 1: 写失败测试**

`test/unit/model_command/priority_weight_test.go`（沿用该目录既有 command 测试骨架）：

```go
package model_command

import "testing"

func TestCreateModelWithPriorityWeight(t *testing.T) {
	// 沿用本目录既有测试的仓储 stub 与 handler 构造方式
	h := newTestCreateModelHandler(t)
	res, err := h.Handle(t.Context(), newCreateModelCmd(t, "gpt-4", 2, 10))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	m := h.mustLoad(t, res.ModelID)
	if m.Priority() != 2 {
		t.Fatalf("Priority = %d, want 2", m.Priority())
	}
	if m.Weight() != 10 {
		t.Fatalf("Weight = %d, want 10", m.Weight())
	}
}

func TestCreateModelDefaultsAndValidation(t *testing.T) {
	h := newTestCreateModelHandler(t)
	res, err := h.Handle(t.Context(), newCreateModelCmd(t, "gpt-4", 0, 0)) // weight 0 → 默认 1
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	m := h.mustLoad(t, res.ModelID)
	if m.Priority() != 0 || m.Weight() != 1 {
		t.Fatalf("defaults = (%d,%d), want (0,1)", m.Priority(), m.Weight())
	}
	if _, err := h.Handle(t.Context(), newCreateModelCmd(t, "gpt-5", 0, -1)); err == nil {
		t.Fatal("weight=-1 应被拒绝")
	}
}
```

（`newTestCreateModelHandler`/`newCreateModelCmd` 若已有同名 helper 则复用；否则按本目录既有测试的构造方式实现 helper：内存 sqlite 仓储 + handler，`mustLoad` 经 repository 读回聚合。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/model_command/ -run PriorityWeight -v`
Expected: 编译失败（`Priority`/`Weight` 不存在）

- [ ] **Step 3: 最小实现**

1. DB：`internal/infrastructure/database/model/model.go` `Model` 加两列（`Enabled` 后）：

```go
	Priority int `json:"priority" gorm:"column:priority;not null;default:0;comment:调度优先级,数字小=优先级高"`
	Weight   int `json:"weight" gorm:"column:weight;not null;default:1;comment:同优先级加权随机权重(>0)"`
```

2. `internal/common/constant/sql.go`：`FieldModelPriority = "priority"`、`FieldModelWeight = "weight"`（`string.go` 声明常量名），并插入 `ModelRepoFieldsFull` 切片（`FieldEnabled` 后）。

3. 聚合 `internal/domain/llmproxy/aggregate/model.go`：struct 加私有 `priority int`/`weight int`；`CreateModel` 签名尾部加 `priority, weight int`（weight<=0 归一为 1；weight<0 返回 `ierr.New(ierr.ErrValidation, "model weight must be positive")`）；`Update` 加 `priority, weight *int` 参数分支；访问器 `Priority() int`/`Weight() int`；`SetScheduling(priority, weight int)` 供 repository 恢复（与 `SetUserID` 同模式）。

4. 命令链：`port/handler.go` 的 `CreateModelCommand`/`UpdateModelCommand` 加 `Priority int`/`Weight int`（update 为 `*int`）；`create_model.go`/`update_model.go` 透传；repository 的 `toDBModel`/`toDomainModel`（`model_repository.go`）补两列映射（含 `SetScheduling`）。

5. wire：`internal/dto/model.go` 的模型 DTO 加 `Priority int \`json:"priority" minimum:"-100" maximum:"100"\``、`Weight int \`json:"weight" minimum:"1" maximum:"1000"\``；`internal/handler/model.go` 创建/更新/详情组装同步。

6. 前端模型表单：priority 数字输入（默认 0，help「数字小=优先级高」）、weight 数字输入（默认 1，min 1）。

7. **同步修正受影响的既有测试**：`grep -rn "aggregate.CreateModel(" test/ internal/ --include="*.go"` 逐个补 `0, 1` 两参。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/model_command/ ./test/unit/endpoint_resolver/ ./test/unit/model_repository/ ./test/unit/domain_llmproxy/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/infrastructure/database/model/model.go internal/common/constant/ internal/domain/llmproxy/aggregate/model.go internal/application/model/ internal/handler/model.go internal/dto/model.go internal/infrastructure/repository/model_repository.go web/src/app/\(dashboard\)/upstream/ test/unit/model_command/
git commit -m "feat(model): 端点调度 priority/weight 数据模型"
```

---

## Task 7: EndpointResolver.ResolveCandidates（排序 + 加权洗牌）

**Files:**
- Modify: `internal/domain/llmproxy/service/resolver.go`（重写 Resolve 为 ResolveCandidates）
- Test: `test/unit/endpoint_resolver/`（扩展）

**Interfaces:**
- Consumes: Task 6 的 `Model.Priority()/Weight()`
- Produces:

```go
type Candidate struct {
	Endpoint *aggregate.Endpoint
	Model    *aggregate.Model
}

type EndpointResolver interface {
	ResolveCandidates(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) ([]Candidate, error)
}
```

（旧 `Resolve` 删除；Task 8 的 3 个调用点迁移。Task 10 将在此排序尾部加亲和置顶。）

- [ ] **Step 1: 写失败测试**

`test/unit/endpoint_resolver/candidates_test.go`（沿用该目录既有 fixture 构造方式——该目录已有 resolver 测试，复用其仓储 stub）：

```go
package endpoint_resolver

import (
	"context"
	"testing"
)

func TestResolveCandidatesPriorityOrder(t *testing.T) {
	r := newTestResolver(t,
		cand{alias: "gpt-4", endpoint: "ep-low", priority: 5, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-high1", priority: 1, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-high2", priority: 1, weight: 1},
	)
	got, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", nil)
	if err != nil {
		t.Fatalf("ResolveCandidates: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[2].Endpoint.Name() != "ep-low" {
		t.Fatalf("last = %s, want ep-low", got[2].Endpoint.Name())
	}
}

func TestResolveCandidatesWeightedShuffle(t *testing.T) {
	// weight 9:1，1000 次抽样中重端点应显著更多
	r := newTestResolver(t,
		cand{alias: "gpt-4", endpoint: "ep-heavy", priority: 0, weight: 9},
		cand{alias: "gpt-4", endpoint: "ep-light", priority: 0, weight: 1},
	)
	heavyFirst := 0
	for range 1000 {
		got, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", nil)
		if err != nil {
			t.Fatalf("ResolveCandidates: %v", err)
		}
		if got[0].Endpoint.Name() == "ep-heavy" {
			heavyFirst++
		}
	}
	if heavyFirst < 750 {
		t.Fatalf("heavy first = %d/1000, want >= 750", heavyFirst)
	}
}

func TestResolveCandidatesEmpty(t *testing.T) {
	r := newTestResolver(t)
	if _, err := r.ResolveCandidates(context.Background(), 1, "gpt-4", nil); err == nil {
		t.Fatal("无候选应返回错误")
	}
}
```

（`newTestResolver`/`cand` helper 按该目录既有测试的 stub 模式实现：内存 sqlite 或现有仓储 fake + `NewEndpointResolver(repo, repo, true)`。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/endpoint_resolver/ -run Candidates -v`
Expected: 编译失败 `r.ResolveCandidates undefined`

- [ ] **Step 3: 最小实现**

`internal/domain/llmproxy/service/resolver.go`：

```go
// Candidate 端点解析候选：一次 alias 解析中可用的 (Endpoint, Model) 组合。
type Candidate struct {
	Endpoint *aggregate.Endpoint
	Model    *aggregate.Model
}

// ResolveCandidates 按 alias 解析出全部可用候选，按调度优先级有序返回。
//
//  1. 查 model 表（按 alias，限定用户租户；未命中且开启共享池回退时查共享池）
//  2. 过滤 enabled 与 matcher（matcher 为 nil 表示不筛选）
//  3. priority 升序分档（数字小=优先级高）；同档内按 weight 做 A-Res 加权洗牌
//     （key = rand^(1/weight)，key 大者在前）
//  4. 无候选返回 ErrDataNotExists
func (r *endpointResolver) ResolveCandidates(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) ([]Candidate, error) {
	if alias.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "endpoint alias is empty")
	}
	tenantID := userID
	models, err := r.modelRepo.FindByAlias(ctx, alias, &tenantID)
	if err != nil {
		return nil, err
	}
	if len(models) == 0 && r.sharedPoolFallback {
		sharedPoolID := constant.SharedPoolUserID
		models, err = r.modelRepo.FindByAlias(ctx, alias, &sharedPoolID)
		if err != nil {
			return nil, err
		}
	}
	if len(models) == 0 {
		return nil, ierr.Newf(ierr.ErrDataNotExists, "model %q not found", alias.String())
	}
	scope := &tenantID
	if models[0].UserID() == constant.SharedPoolUserID {
		sharedPoolID := constant.SharedPoolUserID
		scope = &sharedPoolID
	}

	cands := make([]Candidate, 0, len(models))
	for _, m := range models {
		if !m.Enabled() {
			continue
		}
		ep, findErr := r.endpointRepo.FindByID(ctx, m.EndpointID(), scope)
		if findErr != nil {
			return nil, findErr
		}
		if ep == nil {
			continue
		}
		if matcher == nil || matcher(ep) {
			cands = append(cands, Candidate{Endpoint: ep, Model: m})
		}
	}
	if len(cands) == 0 {
		return nil, ierr.Newf(ierr.ErrDataNotExists, "model %q has no endpoint supporting requested API", alias.String())
	}

	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Model.Priority() < cands[j].Model.Priority() })
	// 档内 A-Res 加权洗牌：key = rand^(1/weight)，key 大者在前
	for i := 0; i < len(cands); {
		j := i + 1
		for j < len(cands) && cands[j].Model.Priority() == cands[i].Model.Priority() {
			j++
		}
		type keyed struct {
			idx int
			key float64
		}
		ks := make([]keyed, 0, j-i)
		for idx := i; idx < j; idx++ {
			w := float64(cands[idx].Model.Weight())
			ks = append(ks, keyed{idx: idx, key: math.Pow(rand.Float64(), 1/w)}) //nolint:gosec // 调度洗牌无需加密随机
		}
		sort.SliceStable(ks, func(a, b int) bool { return ks[a].key > ks[b].key })
		perm := make([]Candidate, 0, j-i)
		for _, k := range ks {
			perm = append(perm, cands[k.idx])
		}
		copy(cands[i:j], perm)
		i = j
	}
	return cands, nil
}
```

imports 补 `math`、`sort`。原 `Resolve` 方法与接口签名删除；`rand.Perm` 用法随删除消失（`math/rand/v2` 保留给洗牌）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/endpoint_resolver/ -v`
Expected: PASS（既有测试若调用旧 `Resolve`，按新签名改断言：取 `got[0]`）

- [ ] **Step 5: 提交**

```bash
git add internal/domain/llmproxy/service/resolver.go test/unit/endpoint_resolver/
git commit -m "feat(resolver): 端点候选按优先级排序与权重洗牌"
```

---

## Task 8: usecase 候选遍历 + 跨端点 fallback

**Files:**
- Modify: `internal/application/llmproxy/usecase/common.go`（`canSwitchEndpoint`）
- Modify: `internal/application/llmproxy/usecase/openai.go`（`CreateChatCompletion`、`CreateResponse`）
- Modify: `internal/application/llmproxy/usecase/anthropic.go`（`CreateMessage`）
- Test: `test/unit/llmproxy_usecase/`（扩展）、`test/e2e/endpoint_fallback/`（新建）

**Interfaces:**
- Consumes: Task 7 的 `ResolveCandidates`；`transport.IsRetryableError`；`model.CircuitOpenError`/`model.BulkheadFullError`；`port.ProxyError.Cause`
- Produces: `canSwitchEndpoint(err error) bool`（usecase 包内函数）；转发失败且可切换时自动换候选的行为。

- [ ] **Step 1: 写失败测试（单测）**

`test/unit/llmproxy_usecase/fallback_test.go`（沿用该目录既有 fake proxy/submitter 构造方式）：

```go
package llmproxy_usecase

import (
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

func TestCanSwitchEndpoint(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"连接错误", &model.UpstreamConnectionError{Cause: errors.New("dial")}, true},
		{"上游 5xx", &model.UpstreamError{StatusCode: 502}, true},
		{"上游 429", &model.UpstreamError{StatusCode: 429}, true},
		{"上游 400 不可切换", &model.UpstreamError{StatusCode: 400}, false},
		{"熔断打开", &model.CircuitOpenError{}, true},
		{"信号量满载", &model.BulkheadFullError{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := usecase.CanSwitchEndpoint(tc.err); got != tc.want {
				t.Fatalf("CanSwitchEndpoint = %v, want %v", got, tc.want)
			}
		})
	}
}
```

（导出形态：如坚持包内小写，则此测试放 `internal` 同包测试不可行（测试只进 test/），故 `canSwitchEndpoint` 导出为 `CanSwitchEndpoint` 供测试与三入口共用。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/llmproxy_usecase/ -run CanSwitch -v`
Expected: 编译失败 `usecase.CanSwitchEndpoint undefined`

- [ ] **Step 3: 最小实现（判定 + 三入口循环）**

`internal/application/llmproxy/usecase/common.go`：

```go
// CanSwitchEndpoint 判断转发失败是否可切换到下一个候选端点。
//
// 可切换：连接错误 / 5xx / 429（transport.IsRetryableError），
// 以及 Guard 熔断打开、信号量满载（端点熔断正是换端点的时机）。
// errors.As 穿透 ProxyError.Cause 后判定。
func CanSwitchEndpoint(err error) bool {
	if err == nil {
		return false
	}
	if transport.IsRetryableError(err) {
		return true
	}
	var proxyErr *port.ProxyError
	if errors.As(err, &proxyErr) && proxyErr.Cause != nil {
		if transport.IsRetryableError(proxyErr.Cause) {
			return true
		}
	}
	var circuitErr *model.CircuitOpenError
	if errors.As(err, &circuitErr) {
		return true
	}
	var bulkheadErr *model.BulkheadFullError
	if errors.As(err, &bulkheadErr) {
		return true
	}
	return false
}
```

（imports 补 `internal/infrastructure/transport`、`errors`。）

三入口改造以 `CreateChatCompletion`（`internal/application/llmproxy/usecase/openai.go`）为例——把「Resolve → 触发词检查 → switch compatRoute 转发」改为「ResolveCandidates → 触发词检查（一次，审计端点名取首个候选）→ 候选循环转发」：

```go
	candidates, err := u.resolver.ResolveCandidates(ctx, userID, vo.EndpointAlias(req.Body.Model), func(ep *aggregate.Endpoint) bool {
		return SelectCompatRoute(enum.ProxyAPIOpenAIChat, ep) != enum.CompatRouteUnsupported
	})
	if err != nil {
		log.Error("[OpenAIUseCase] Model not found or unsupported for chat completion", zap.String("model", req.Body.Model), zap.Error(err))
		return nil, proxyutil.SendOpenAIModelNotFoundError(req.Body.Model)
	}
```

触发词分支内原 `ep.Name()` 改为 `candidates[0].Endpoint.Name()`（deny/capture 审计），转发前进入候选循环（替换原 `switch compatRoute` 尾部）：

```go
	var lastErr error
	for i, cand := range candidates {
		compatRoute = SelectCompatRoute(enum.ProxyAPIOpenAIChat, cand.Endpoint)
		upstream := toTransportEndpoint(cand.Model, cand.Endpoint, false)
		result, fwdErr := u.dispatchChat(ctx, req, cand.Model, cand.Endpoint, upstream, compatRoute)
		if fwdErr == nil {
			return result, nil
		}
		lastErr = fwdErr
		if i == len(candidates)-1 || !CanSwitchEndpoint(fwdErr) {
			return nil, fwdErr
		}
		log.Warn("[OpenAIUseCase] Upstream failed, switching endpoint",
			zap.String("model", req.Body.Model),
			zap.String("from", cand.Endpoint.Name()),
			zap.String("to", candidates[i+1].Endpoint.Name()),
			zap.Error(fwdErr),
		)
	}
	return nil, lastErr
```

`dispatchChat` 是把原 `switch compatRoute` 各分支（`forwardChatNative`/`forwardChatViaAnthropic`/`forwardChatViaResponse`，含原 default 分支错误）提取为方法（参数含 `m *aggregate.Model, ep *aggregate.Endpoint, upstream vo.UpstreamEndpoint, compatRoute enum.CompatRoute`），返回 `(port.Result, error)`。`CreateResponse` 与 `CreateMessage` 同构改造：分别提取 `dispatchResponse`/`dispatchMessage`，matcher 分别用 `enum.ProxyAPIOpenAIResponse`/`enum.ProxyAPIAnthropicMessage`，日志前缀 `[OpenAIUseCase]`（response）/`[AnthropicUseCase]`。

注意：候选循环中 `toTransportEndpoint(m, ep, ...)` 的第三参（anthropic 路径标识）沿用各 dispatch 分支原取值；`auditFailure` 的 endpoint 参数传 `cand.Endpoint.Name()`。

- [ ] **Step 4: 单测通过 + E2E 编写**

Run: `go test ./test/unit/llmproxy_usecase/ -v`
Expected: PASS

E2E `test/e2e/endpoint_fallback/fallback_test.go`：进程内 `httptest.NewServer` 作假上游（路径 `/v1/chat/completions`）：第一个创建的端点指向返回 500 的 server、第二个指向正常返回 SSE 的 server；经管理 API（ADMIN_TOKEN）创建两个 endpoint + 两个 model 记录（同 alias、priority 0 与 1），再以 USER_TOKEN + ProxyAPIKey 发 `POST /api/openai/v1/chat/completions`，断言：
1. 响应 200 且 body 是正常 completion（fallback 成功）；
2. 审计 `GET /api/web/v1/audit/model/log/list` 最新一条 endpoint 为正常端点。

测试骨架沿用 `test/e2e/apikeys` 的 env/e2eguard 模式（`BASE_URL`/`ADMIN_TOKEN`/`USER_TOKEN` + `e2eguard.GuardLiveTarget`），mock server 响应体放 `fixtures/requests/`。若假上游无法从服务进程访问（网络隔离），该 E2E 标注 `t.Skip` 条件并保留用例（本地/CI 可跑时执行）。

- [ ] **Step 5: 提交**

```bash
git add internal/application/llmproxy/usecase/ test/unit/llmproxy_usecase/fallback_test.go test/e2e/endpoint_fallback/
git commit -m "feat(llmproxy): 转发失败跨端点 fallback"
```

---

## Task 9: 亲和键提取与 Redis 存储

**Files:**
- Create: `internal/application/llmproxy/usecase/affinity.go`
- Modify: `internal/common/constant/string.go`（Redis key 模板）
- Test: `test/unit/affinity/`（新建）

**Interfaces:**
- Consumes: `constant.HTTPHeaderOpencodeSession` / `HTTPHeaderSessionID`、`util.CtxValueUint(CtxKeyUserID)`、redis client
- Produces:

```go
// AffinityKey 组合亲和键：会话头优先，回退 alias+首条 user 消息指纹。
func AffinityKey(ctx context.Context, alias string, firstUserText string) (string, bool)
// AffinityStore 亲和映射存取（TTL 5min，失败 fail-open）。
func (s *AffinityStore) Get(ctx context.Context, userID uint, alias, key string) (uint, bool)
func (s *AffinityStore) Put(ctx context.Context, userID uint, alias, key string, endpointID uint)
```

Redis key 模板：`constant.AffinityKeyTemplate = "affinity:%d:%s:%s"`（userID, alias, key）；TTL 常量 `constant.AffinityTTL = 5 * time.Minute`（若业务包禁 const，放 `constant/` 包并由 bootstrap 注入 store）。

- [ ] **Step 1: 写失败测试**

`test/unit/affinity/affinity_test.go`：

```go
package affinity

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func TestAffinityKeyPrefersSessionHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set(constant.HTTPHeaderSessionID, "sess-1")
	ctx := context.WithValue(req.Context(), constant.CtxKeyPassthroughHeaders, req.Header)
	key, ok := usecase.AffinityKey(ctx, "gpt-4", "hello")
	if !ok || key != "sess-1" {
		t.Fatalf("key = %q ok=%v, want sess-1", key, ok)
	}
}

func TestAffinityKeyFingerprintStable(t *testing.T) {
	ctx := context.Background()
	k1, ok1 := usecase.AffinityKey(ctx, "gpt-4", "hello")
	k2, ok2 := usecase.AffinityKey(ctx, "gpt-4", "hello")
	k3, _ := usecase.AffinityKey(ctx, "gpt-4", "other")
	if !ok1 || !ok2 || k1 != k2 {
		t.Fatalf("同输入指纹应稳定: %q %q", k1, k2)
	}
	if k1 == k3 {
		t.Fatal("不同输入指纹应不同")
	}
}

func TestAffinityKeyNoInput(t *testing.T) {
	if _, ok := usecase.AffinityKey(context.Background(), "gpt-4", ""); ok {
		t.Fatal("无会话头且无文本应返回 ok=false")
	}
}
```

（会话头的读取来源以 `applyPassthroughRequestHeaders` 的 context 存取方式为准：若头经 `constant.CtxKeyPassthroughHeaders` 传入则如上；若 usecase 另有请求头访问路径，测试构造方式对齐该路径，断言不变。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/affinity/ -v`
Expected: 编译失败 `usecase.AffinityKey undefined`

- [ ] **Step 3: 最小实现**

`internal/common/constant/string.go`：`AffinityKeyTemplate = "affinity:%d:%s:%s"`；`internal/common/constant/` 时长常量按现有模式放（`grep -rn "5 \* time.Minute" internal/common/constant/` 看放哪个文件，同文件加 `AffinityTTL = 5 * time.Minute`）。

`internal/application/llmproxy/usecase/affinity.go`：

```go
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
)

// AffinityKey 组合亲和键：会话头（x-opencode-session / X-Session-Id）优先；
// 无会话头时回退 sha256(alias + NUL + 首条 user 文本) 指纹；两者皆空返回 ok=false（不参与亲和）。
// 头读取来源 constant.CtxKeyPassthroughHeaders（类型 map[string]string，
// 由 middleware/header_passthrough.go 写入），大小写不敏感匹配。
func AffinityKey(ctx context.Context, alias, firstUserText string) (string, bool) {
	headers, _ := util.CtxValue(ctx, constant.CtxKeyPassthroughHeaders).(map[string]string)
	for name, v := range headers {
		lname := strings.ToLower(name)
		if (lname == "x-opencode-session" || lname == "x-session-id") && v != "" {
			return v, true
		}
	}
	if firstUserText == "" {
		return "", false
	}
	sum := sha256.Sum256([]byte(alias + "\x00" + firstUserText))
	return hex.EncodeToString(sum[:8]), true
}

// AffinityStore 端点亲和映射（Redis，TTL 5 分钟；读写失败 fail-open）。
type AffinityStore struct {
	rdb redis.UniversalClient
}

func NewAffinityStore(rdb redis.UniversalClient) *AffinityStore { return &AffinityStore{rdb: rdb} }

func (s *AffinityStore) Get(ctx context.Context, userID uint, alias, key string) (uint, bool) {
	val, err := s.rdb.Get(ctx, fmt.Sprintf(constant.AffinityKeyTemplate, userID, alias, key)).Uint64()
	if err != nil {
		return 0, false
	}
	return uint(val), true
}

func (s *AffinityStore) Put(ctx context.Context, userID uint, alias, key string, endpointID uint) {
	if err := s.rdb.Set(ctx, fmt.Sprintf(constant.AffinityKeyTemplate, userID, alias, key), endpointID, constant.AffinityTTL).Err(); err != nil {
		logger.WithCtx(ctx).Warn("[AffinityStore] put affinity failed", zap.Error(err))
	}
}
```

（imports 补 `strings`、`fmt`、`crypto/sha256`、`encoding/hex`；头读取断言方式与 `internal/util/context.go:49` 一致（`map[string]string`）。）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/affinity/ -v && go build ./...`
Expected: PASS（store 的 Redis 路径用 miniredis 或现有 redis 测试先例；若 `test/unit/cache` 有 redis 测试夹具则复用）

- [ ] **Step 5: 提交**

```bash
git add internal/application/llmproxy/usecase/affinity.go internal/common/constant/ test/unit/affinity/
git commit -m "feat(affinity): 端点亲和键提取与 Redis 存储"
```

---

## Task 10: 亲和集成（候选置顶 + 成功写入）

**Files:**
- Modify: `internal/domain/llmproxy/service/resolver.go`（`ResolveCandidates` 尾部置顶）
- Modify: `internal/application/llmproxy/usecase/openai.go` / `anthropic.go`（成功后 Put 亲和）
- Test: `test/unit/endpoint_resolver/`（扩展）

**Interfaces:**
- Consumes: Task 7 `ResolveCandidates`、Task 9 `AffinityStore`
- Produces: 亲和行为——同一亲和键的后续请求优先命中上次成功的端点（TTL 内）。

- [ ] **Step 1: 写失败测试**

`test/unit/endpoint_resolver/affinity_test.go`：

```go
package endpoint_resolver

import (
	"context"
	"testing"
)

func TestResolveCandidatesAffinityFirst(t *testing.T) {
	r := newTestResolver(t,
		cand{alias: "gpt-4", endpoint: "ep-a", priority: 0, weight: 1},
		cand{alias: "gpt-4", endpoint: "ep-b", priority: 0, weight: 1},
	)
	aff := newFakeAffinity(map[string]uint{"k1": endpointIDOf(t, r, "ep-b")})
	r.WithAffinity(aff) // resolver 构造后注入（或经 NewEndpointResolver 参数扩展）

	got, err := r.ResolveCandidatesWithAffinity(context.Background(), 1, "gpt-4", "k1", nil)
	if err != nil {
		t.Fatalf("ResolveCandidatesWithAffinity: %v", err)
	}
	if got[0].Endpoint.Name() != "ep-b" {
		t.Fatalf("affinity first = %s, want ep-b", got[0].Endpoint.Name())
	}

	// 亲和端点不在候选（matcher 过滤）时按序返回
	got, err = r.ResolveCandidatesWithAffinity(context.Background(), 1, "gpt-4", "k1", func(ep *aggregate.Endpoint) bool {
		return ep.Name() != "ep-b"
	})
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if got[0].Endpoint.Name() == "ep-b" {
		t.Fatal("被过滤的亲和端点不应置顶")
	}
}
```

（`newFakeAffinity` 实现 `EndpointAffinity` 接口 fake：`Get(ctx, userID, alias, key) (uint, bool)`，与 Task 9 `AffinityStore.Get` 同名同签名。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/unit/endpoint_resolver/ -run Affinity -v`
Expected: 编译失败

- [ ] **Step 3: 最小实现**

`resolver.go` 定义接口并注入（`NewEndpointResolver` 加参数或 setter，bootstrap `internal/bootstrap/modules/` 绑定 `usecase.NewAffinityStore` 到该接口）：

```go
// EndpointAffinity 端点亲和查询（application 层 AffinityStore 实现，domain 只依赖接口）。
// 方法签名与 usecase.AffinityStore.Get 一致（Put 供 usecase 成功后回写，不进 domain 接口）。
type EndpointAffinity interface {
	Get(ctx context.Context, userID uint, alias, key string) (uint, bool)
}

// ResolveCandidatesWithAffinity 在 ResolveCandidates 基础上把亲和命中的候选提到最前。
// key 为空串表示无亲和，等价 ResolveCandidates。
func (r *endpointResolver) ResolveCandidatesWithAffinity(ctx context.Context, userID uint, alias vo.EndpointAlias, key string, matcher func(*aggregate.Endpoint) bool) ([]Candidate, error) {
	cands, err := r.ResolveCandidates(ctx, userID, alias, matcher)
	if err != nil || key == "" || r.affinity == nil {
		return cands, err
	}
	pinnedID, ok := r.affinity.Get(ctx, userID, alias.String(), key)
	if !ok {
		return cands, nil
	}
	for i, c := range cands {
		if c.Endpoint.ID() == pinnedID {
			cands = append([]Candidate{c}, append(cands[:i], cands[i+1:]...)...)
			break
		}
	}
	return cands, nil
}
```

usecase 三入口改造（以 chat 为例）：候选解析处提取亲和键（首条 user 文本提取 helper `firstUserText(req)` 从 typed DTO 取 messages 中首条 role=user 的 text parts 拼接）：

```go
	affKey, _ := usecase.AffinityKey(ctx, req.Body.Model, firstUserText(req))
	candidates, err := u.resolver.ResolveCandidatesWithAffinity(ctx, userID, vo.EndpointAlias(req.Body.Model), affKey, matcher)
```

转发成功返回处（`fwdErr == nil` 分支）写入亲和：

```go
		if fwdErr == nil {
			if affKey != "" {
				u.affinity.Put(ctx, userID, req.Body.Model, affKey, cand.Endpoint.ID())
			}
			return result, nil
		}
```

（`u.affinity` 经 `NewOpenAIUseCase` 构造注入 `*usecase.AffinityStore`；anthropic 同构。若不想跨层，`AffinityStore` 放 `application/llmproxy/usecase` 包内直接引用，domain 用接口。）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./test/unit/endpoint_resolver/ ./test/unit/llmproxy_usecase/ ./test/unit/affinity/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/domain/llmproxy/service/resolver.go internal/application/llmproxy/usecase/ internal/bootstrap/ test/unit/endpoint_resolver/affinity_test.go
git commit -m "feat(affinity): 亲和端点置顶与成功回写"
```

---

## Task 11: Playground 后端

**Files:**
- Create: `internal/handler/playground.go`、`internal/router/playground.go`
- Modify: `internal/router/router.go`（注册 playground 组）
- Modify: `internal/dto/playground.go`（新建请求/响应 DTO）
- Modify: `internal/bootstrap/modules/`（handler 绑定）
- Test: `test/e2e/playground/`（新建）

**Interfaces:**
- Consumes: `openAIUseCase.CreateChatCompletion`（经 `port.OpenAIUseCase`）、`CtxKeySkipStore`（存储分流已存在：`openai_store.go:27/95`、`anthropic_store.go:25` 消费该 key 跳过 session/message/tool 沉淀，审计照常——**Playground 直接复用，不新增 store 逻辑**）、huma `jwtAuth`
- Produces: `POST /api/web/v1/playground/chat`（body=`dto.OpenAIChatCompletionRequest` 复用，response=非流式 JSON 或 SSE 流）。

- [ ] **Step 1: 写失败测试（E2E）**

`test/e2e/playground/playground_test.go`（env/e2eguard 模式同 `test/e2e/apikeys`）：

```go
package playground

// 断言：
// 1. POST /api/web/v1/playground/chat（USER_TOKEN，body fixtures/requests/chat.json）返回 200；
// 2. GET /api/web/v1/audit/model/log/list 最新一条 model 等于所选别名（审计有记录）；
// 3. GET /api/web/v1/session/list 同时段无新增会话（session 无记录）。
// demo token 调用返回 403。
```

具体实现按 `test/e2e/apikeys` 的 `doJSON`/env 模式展开，请求体放 `fixtures/requests/chat.json`：

```json
{
  "model": "REPLACE_WITH_TEST_ALIAS",
  "messages": [{"role": "user", "content": "hi"}],
  "stream": false
}
```

断言 helper：审计与会话列表接口均带时间范围参数（取测试开始时间戳）。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./test/e2e/playground/ -v`
Expected: 404（路由不存在）

- [ ] **Step 3: 最小实现**

`internal/dto/playground.go`：

```go
package dto

// PlaygroundChatReq Playground 调试请求：直接复用 OpenAI Chat 请求体。
type PlaygroundChatReq struct {
	Body *OpenAIChatCompletionRequest `json:"-" required:"true"`
}
```

`internal/handler/playground.go`（沿用 openai handler 的 Body 包装与流式透传模式，huma-dto-conventions skill 的 Body 包装模板）：

```go
// PlaygroundChat 处理 Playground 调试请求：注入 CtxKeySkipStore 后走 LLM 转发全链路。
// 存储分流：openai_store/anthropic_store 消费 CtxKeySkipStore 跳过 session/message/tool，
// 审计照常——复用既有机制，不新增分支。
func (h *playgroundHandler) PlaygroundChat(ctx context.Context, req *dto.PlaygroundChatReq) (*dto.OpenAIChatCompletionRsp, error) {
	ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
	return h.uc.CreateChatCompletion(ctx, req.Body)
}
```

（流式：若 huma 路由声明 `StreamResponse`，按 `internal/handler/openai.go` 的流式返回形态同构处理——Playground 前端首版用非流式，流式在 Task 12 前端经 SSE 渲染，handler 需同时支持：响应类型按 `req.Body.Stream` 分流的实现与 openai handler 一致。）

`internal/router/playground.go`：huma 注册 `POST /api/web/v1/playground/chat`，`Security: jwtAuth`，中间件链：权限 `PermissionUser`（demo/pending 拒绝——沿用现有权限中间件声明方式，参考 `router/model.go` 的写操作）、userID 令牌桶限流（复用 `TokenBucketRateLimiterMiddleware` 的 `WithPermissionFilter` 同模式，桶键 = userID）。`router.go` 注册 playground 组。

- [ ] **Step 4: E2E 通过 + 全量回归**

Run: `go test ./test/e2e/playground/ -v && go test ./internal/... ./test/unit/... -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/handler/playground.go internal/router/playground.go internal/router/router.go internal/dto/playground.go internal/bootstrap/ test/e2e/playground/
git commit -m "feat(playground): Web 端模型调试接口（审计留痕、会话不落地）"
```

---

## Task 12: Playground 前端页面

**Files:**
- Create: `web/src/app/(dashboard)/playground/page.tsx`
- Modify: 导航组件（`grep -rn "dataset\|trigger" web/src/components --include="*.tsx" -l` 定位 Nav，加 Playground 条目）
- Test: `web/src/app/(dashboard)/playground/__tests__/`（新建）

**Interfaces:**
- Consumes: `POST /api/web/v1/playground/chat`（Task 11）、`GET /api/web/v1/model/list`（别名下拉）、`src/lib` 的 fetch 封装（Bearer 注入 + 错误 toast）
- Produces: `/playground` 路由页（user/admin 可见；demo 不可见——Nav 按现有 `isDemo()` 守卫隐藏）。

- [ ] **Step 1: 写失败测试**

`web/src/app/(dashboard)/playground/__tests__/page.test.tsx`：

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import PlaygroundPage from "../page";

describe("Playground 页面", () => {
  it("渲染模型选择、输入区与发送按钮", () => {
    render(<PlaygroundPage />);
    expect(screen.getByLabelText("模型")).toBeTruthy();
    expect(screen.getByPlaceholderText(/输入消息/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "发送" })).toBeTruthy();
  });
});
```

（依赖 Provider（I18n/Auth）的挂载方式沿用现有页面测试的 wrapper；断言不变。）

- [ ] **Step 2: 运行测试确认失败**

Run: `cd web && npx vitest run src/app/\(dashboard\)/playground/__tests__/page.test.tsx`
Expected: FAIL（模块不存在）

- [ ] **Step 3: 最小实现**

页面功能（`page.tsx`，风格沿用现有 dashboard 页面：Tailwind 4 + Base UI/shadcn 组件）：

1. 模型下拉：加载 `GET /model/list` 仅 enabled 项，显示 `alias`。
2. 参数面板：stream 开关（默认开）、temperature（0~2）、max_tokens（可空）。
3. 多轮消息编辑：`[{role: 'user'|'assistant'|'system', content: string}]` 列表，可增删行。
4. 发送：`src/lib` fetch 封装 POST `/playground/chat`；非流式直接渲染响应消息；流式用 SSE 解析（复用 `src/lib` 既有的数据集 SSE 导出解析模式，逐 chunk 追加渲染）。
5. 错误展示：复用 `src/lib` 错误码 toast。
6. Nav 条目「Playground」，`isDemo()` 时隐藏。

- [ ] **Step 4: 测试与 lint 通过**

Run: `cd web && npx vitest run && npm run lint && npx tsc --noEmit`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add web/src/app/\(dashboard\)/playground/ web/src/components/
git commit -m "feat(web): Playground 模型调试页"
```

---

## 收尾（所有 Task 完成后）

- [ ] **全量回归**：`go test ./... -count=1`、`make lint`、`cd web && npm run build`
- [ ] **CONTEXT.md 更新**：TokenAccounting 词条补 5m/1h 子档说明；ModelPricing 词条补 `cache_creation_1h_price` 回落语义；新增 EndpointScheduling（priority/weight/候选/fallback）与 EndpointAffinity、Playground 三个词条
- [ ] **ponytail-review**：审查全量 diff 的过度工程
- [ ] **沉淀 memory**（Serena）：5m/1h 计价回落语义、ResolveCandidates 排序算法、CtxKeySkipStore 复用决策
- [ ] **询问用户**是否提交/合并（禁止擅自 push/合并）
