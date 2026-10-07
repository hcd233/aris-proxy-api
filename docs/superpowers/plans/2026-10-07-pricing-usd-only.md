# 定价交互改造（币种固定 USD）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 Model 定价的「币种下拉」替换为「启用计价」开关，币种固定 USD，并修掉编辑时无法清空定价的缺陷。

**Architecture:** 前端 `PricingEditor` 用「规则表是否为空」推导币种（非空 ⇔ `USD`），开关只负责补/清空规则表；因此「未计价 ⇔ 规则为空」的不变量在前端也成立，wire 层不再用 `nil` 表达未计价。后端只收紧 `enum.Currency.Valid()`（删 `CurrencyCNY`），`vo.Pricing` 语义与成本聚合链路零改动。

**Tech Stack:** Go 1.25.1（huma v2 + GORM + ierr）、Next.js 16.3.3 + React 19 + Tailwind v4 + shadcn/ui、vitest 3。

**设计文档：** `docs/superpowers/specs/2026-10-07-pricing-usd-only-design.md`

## Global Constraints

- **币种固定 USD**：`enum.Currency` 合法值只有 `""`（未计价）与 `"USD"`；`CurrencyCNY` 常量删除，请求带 `"CNY"` 必须被拒（422），不得静默接受。
- **wire 金额单位**：展示单位（货币单位 / 1M tokens）浮点；微单位只在后端与 DB，换算固定走 `dto.PriceMicroFromDisplay` / `dto.PriceDisplayFromMicro`。
- **Go 硬约束**：错误统一用 `internal/common/ierr`（禁 `errors.New` / `fmt.Errorf`）；业务包禁本地 `const`（归 `constant`/`enum`）；测试文件只能放 `test/unit/<topic>/` 或 `test/e2e/<topic>/`，不得放 `internal/`；每个测试函数必须 `t.Parallel()`；测试禁 `time.Sleep`（用 `ticker` / `time.After`）。
- **huma DTO**：`internal/dto/**` 改动前按 `huma-dto-conventions` skill 执行；请求 DTO 字段必须有 `query`/`body` 等来源标签。
- **前端硬约束**：所有 HTTP 走 `src/lib/api-client.ts`；不得手拼 `/web` 前缀；禁内联 `style`；文案必须三份 i18n 同步（`web/src/locales/{en,zh,ja}.json`）；`truncate` 元素必须在 `TooltipTrigger` 子树内。
- **lint conv 必须全量跑**：`go run ./cmd/lint ./...`（按包过滤会漏报）。
- **文档语言**：所有生成的文档与注释使用中文。

---

## 前置：worktree 环境准备

工作目录固定为 `.worktrees/refactor-pricing-usd-only-2026-10-07`，分支 `refactor/pricing-usd-only-2026-10-07`（已创建，设计文档已提交）。

- [ ] **Step 1: 补 Go embed 占位（`//go:embed all:dist` 不跟随软链，必须放真实文件）**

```bash
cd .worktrees/refactor-pricing-usd-only-2026-10-07
mkdir -p internal/web/dist
echo '<!doctype html>' > internal/web/dist/index.html
```

- [ ] **Step 2: 造 `web/node_modules` 硬链接树（Turbopack 拒绝软链，`cp -al` 同盘零拷贝）**

```bash
cd .worktrees/refactor-pricing-usd-only-2026-10-07/web
cp -al ../../../web/node_modules node_modules
```

- [ ] **Step 3: 验证前端工具链可用**

Run: `cd .worktrees/refactor-pricing-usd-only-2026-10-07/web && npm run test`
Expected: vitest 跑完 3 个既有测试文件，全部 PASS（证明 node_modules 链接有效）

---

## Task 1: 后端币种收紧为 USD

**Files:**
- Modify: `internal/common/enum/currency.go`
- Modify: `internal/dto/pricing.go:13`
- Modify: `internal/infrastructure/database/model/model.go:20`
- Modify: `internal/infrastructure/database/model/model_call_audit.go:41`
- Test: `test/unit/pricing/pricing_test.go`
- Test: `test/unit/pricing_dto/pricing_dto_test.go`
- Test: `test/unit/model_pricing_agg/model_pricing_agg_test.go`
- Test: `test/unit/model_call_pricing/model_call_pricing_test.go`
- Test: `test/e2e/model_pricing/pricing_test.go`（仅文件头注释）

**Interfaces:**
- Consumes: 无（本任务是链路起点）
- Produces: `enum.Currency` 合法集合 = `{enum.CurrencyNone, enum.CurrencyUSD}`；`enum.CurrencyCNY` 常量不再存在（后续任何任务不得引用它）

- [ ] **Step 1: 先写失败的回归测试**

在 `test/unit/pricing/pricing_test.go` 的 `cases` 切片中，"非法币种" 那条后面插入一条：

```go
		{"非法币种", enum.Currency("JPY"), []vo.PricingRule{defaultRule}, true},
		{"已废弃币种CNY", enum.Currency("CNY"), []vo.PricingRule{defaultRule}, true},
```

（`enum.Currency("JPY")` 这个同形态字面量已存在于该文件并通过 lint，所以 `enum.Currency("CNY")` 同样不会触发 magic-string 规则。）

在 `test/unit/pricing_dto/pricing_dto_test.go` 的 `TestPricingFromDTO` 末尾追加：

```go
	// 非法：已废弃币种
	if _, err := port.PricingFromDTO(&dto.PricingDTO{
		Currency: enum.Currency("CNY"),
		Rules:    []dto.PricingRuleDTO{{InputPrice: 1}},
	}); err == nil {
		t.Fatal("deprecated CNY currency should be rejected")
	}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 -run 'TestNewPricingValidation|TestPricingFromDTO' ./test/unit/pricing/ ./test/unit/pricing_dto/`
Expected: FAIL — `已废弃币种CNY` 与 `deprecated CNY currency should be rejected` 两条失败（当前 `Valid()` 仍接受 `CNY`）

- [ ] **Step 3: 收紧 `Valid()`**

`internal/common/enum/currency.go` 改为：

```go
package enum

// Currency 计价币种；空值表示未计价。当前仅支持 USD。
type Currency string

const (
	CurrencyNone Currency = ""
	CurrencyUSD  Currency = "USD"
)

// Valid 币种是否合法（含空值=未计价）。
//
//	@receiver c Currency
//	@return bool
//	@author centonhuang
//	@update 2026-10-07 10:00:00
func (c Currency) Valid() bool {
	switch c {
	case CurrencyNone, CurrencyUSD:
		return true
	default:
		return false
	}
}
```

注意：本步只删 `CurrencyCNY` 常量，**暂不改**其余引用（编译会失败，Step 5 统一改）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 -run 'TestNewPricingValidation|TestPricingFromDTO' ./test/unit/pricing/ ./test/unit/pricing_dto/`
Expected: 编译失败 —— `undefined: enum.CurrencyCNY`（`pricing_test.go:33` 与 `pricing_dto_test.go` 仍在引用），进入 Step 5 修复引用

- [ ] **Step 5: 把全部 `enum.CurrencyCNY` 引用改为 `enum.CurrencyUSD`**

共 8 处：

| 文件 | 位置 |
|---|---|
| `test/unit/pricing/pricing_test.go` | `{"计价但无规则", enum.CurrencyCNY, nil, true}` → `enum.CurrencyUSD` |
| `test/unit/pricing/pricing_test.go` | `TestPricingMatchTimeWindow` 的 `mustPricing(t, enum.CurrencyCNY, ...)` → `enum.CurrencyUSD` |
| `test/unit/pricing_dto/pricing_dto_test.go` | `Currency: enum.CurrencyCNY` → `enum.CurrencyUSD` |
| `test/unit/pricing_dto/pricing_dto_test.go` | `got.Currency != enum.CurrencyCNY` → `enum.CurrencyUSD` |
| `test/unit/model_pricing_agg/model_pricing_agg_test.go` | `vo.NewPricing(enum.CurrencyCNY, ...)` → `enum.CurrencyUSD` |
| `test/unit/model_pricing_agg/model_pricing_agg_test.go` | `got != enum.CurrencyCNY` → `enum.CurrencyUSD` |
| `test/unit/model_call_pricing/model_call_pricing_test.go` | `vo.NewPricing(enum.CurrencyCNY, ...)` → `enum.CurrencyUSD` |
| `test/unit/model_call_pricing/model_call_pricing_test.go` | `task3.PricingCurrency != string(enum.CurrencyCNY)` → `string(enum.CurrencyUSD)` |

- [ ] **Step 6: 跑 4 个单测包，确认全绿**

Run: `go test -count=1 ./test/unit/pricing/ ./test/unit/pricing_dto/ ./test/unit/model_pricing_agg/ ./test/unit/model_call_pricing/`
Expected: 全部 `ok`

- [ ] **Step 7: 收紧 huma enum 标签**

`internal/dto/pricing.go` 的 `PricingDTO` 改为：

```go
// PricingDTO 模型定价（wire 展示单位：货币单位每 1M tokens）。
// currency=="" ⇔ 未计价；currency=="USD" 时 rules 须恰好含一条无条件默认规则
// （time_windows 空 + context_min=0 + context_max=0），数组顺序=匹配优先级。
type PricingDTO struct {
	Currency enum.Currency    `json:"currency,omitempty" enum:",USD" doc:"计价币种；空=未计价，USD=已计价"`
	Rules    []PricingRuleDTO `json:"rules,omitempty" doc:"定价规则（数组顺序=匹配优先级）"`
}
```

- [ ] **Step 8: 更新两处 DB 列注释**

`internal/infrastructure/database/model/model.go` 第 20 行：

```go
	PricingCurrency string             `json:"pricing_currency" gorm:"column:pricing_currency;not null;default:'';comment:计价币种(''/USD,空=未计价)"`
```

`internal/infrastructure/database/model/model_call_audit.go` 第 41 行：

```go
	PricingCurrency          string    `json:"pricing_currency" gorm:"column:pricing_currency;not null;default:'';comment:计价币种快照(''/USD)"`
```

- [ ] **Step 9: 更新 E2E 文件头注释里的 CNY**

`test/e2e/model_pricing/pricing_test.go` 文件头改为：

```go
// Package model_pricing 模型计费 E2E。
//
// 前置：目标环境已配置带定价的模型别名 MODEL_ALIAS（currency=USD、
// input=1/1M、output=2/1M）与未计价别名 MODEL_ALIAS_UNPRICED；
// WEB_JWT 提供 web 管理视角（缺省跳过成本断言）。默认离线 skip，不打生产。
```

- [ ] **Step 10: 全量 lint + 编译检查**

Run: `go run ./cmd/lint ./... && go build ./cmd/server`
Expected: 无输出（lint 通过）；`go build` 成功

- [ ] **Step 11: 提交**

```bash
git add internal/common/enum/currency.go internal/dto/pricing.go \
  internal/infrastructure/database/model/model.go internal/infrastructure/database/model/model_call_audit.go \
  test/unit/pricing/pricing_test.go test/unit/pricing_dto/pricing_dto_test.go \
  test/unit/model_pricing_agg/model_pricing_agg_test.go test/unit/model_call_pricing/model_call_pricing_test.go \
  test/e2e/model_pricing/pricing_test.go
git commit -m "refactor(pricing): 币种收紧为仅 USD，删除 CNY 枚举

- enum.Currency.Valid() 只认空值与 USD，CurrencyCNY 常量删除
- huma enum 标签收紧为 \",USD\"，CNY 请求 422 拒绝
- 同步 DB 列注释与 4 个单测包，补 CNY 被拒回归用例"
```

---

## Task 2: 前端定价编辑器改造 + 清空定价契约修复

**Files:**
- Modify: `web/src/locales/en.json`、`web/src/locales/zh.json`、`web/src/locales/ja.json`（各 `upstream.pricing.*` 段）
- Modify: `web/src/app/(dashboard)/upstream/pricing-editor.tsx`
- Modify: `web/src/app/(dashboard)/upstream/shared.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`（`openEditModel` / `handleFlatEdit` 回填）
- Modify: `web/src/lib/types.ts`（`PricingCurrency`）
- Modify: `web/src/lib/money.ts`

**Interfaces:**
- Consumes: Task 1 的 `enum.Currency`（wire 只接受 `""` / `"USD"`）
- Produces:
  - `PricingEditorProps = { value?: PricingDTO; onChange: (p: PricingDTO) => void; upstreamModel: string }` —— `onChange` 不再接受 `undefined`
  - `ModelForm.pricing: PricingDTO`（必填；`{ currency: "", rules: [] }` 表示未计价）
  - 不变量：`pricing.rules.length > 0 ⇔ pricing.currency === "USD"`（前端与后端一致）

- [ ] **Step 1: 收窄前端币种类型**

`web/src/lib/types.ts`：

```ts
export type PricingCurrency = "" | "USD";
```

- [ ] **Step 2: 清理 `money.ts` 的 CNY 分支**

`web/src/lib/money.ts` 的 `formatCost` 改为：

```ts
export function formatCost(v: number | null | undefined, currency?: string): string {
  if (v === null || v === undefined) return "\u2014";
  const symbol = currency === "USD" ? "$" : "";
  const s = v.toFixed(6).replace(/\.?0+$/, "");
  return `${symbol}${s === "" ? "0" : s}`;
}
```

- [ ] **Step 3: 三份 i18n 同步增删**

三份文件（`web/src/locales/{en,zh,ja}.json`）的 `upstream.pricing.*` 段都在第 823-849 行，按下列内容替换。

**删除**这两行（原 824、825 行，三份文件形态一致，仅值不同 —— 下面是 en 的实际值）：

```json
"upstream.pricing.currency": "Currency",
"upstream.pricing.currency.none": "Unpriced",
```

（zh 对应值为 `"计价币种"` / `"未计价"`，ja 为 `"通貨"` / `"価格未設定"`。删除时连同行尾逗号一起删，保持 JSON 合法。）

**在 `"upstream.pricing.title"` 那一行之后插入**三个新 key。

en.json：

```json
"upstream.pricing.title": "Pricing",
"upstream.pricing.enabled": "Enable pricing",
"upstream.pricing.unit": "USD / 1M tokens",
"upstream.pricing.disabled_hint": "Unpriced models produce no cost estimates. Configure manually or import from models.dev.",
```

zh.json：

```json
"upstream.pricing.title": "定价",
"upstream.pricing.enabled": "启用计价",
"upstream.pricing.unit": "USD / 1M tokens",
"upstream.pricing.disabled_hint": "未计价模型不产生费用估算；可手动配置或从 models.dev 导入",
```

ja.json：

```json
"upstream.pricing.title": "価格設定",
"upstream.pricing.enabled": "価格設定を有効化",
"upstream.pricing.unit": "USD / 1M tokens",
"upstream.pricing.disabled_hint": "価格未設定のモデルはコスト見積もりを生成しません。手動設定または models.dev からの取り込みが可能です。",
```

- [ ] **Step 4: 改造 `PricingEditorProps` 与组件状态逻辑**

`web/src/app/(dashboard)/upstream/pricing-editor.tsx` 的 props 接口改为（去掉 `Currency` 语义，`onChange` 不再接受 `undefined`）：

```tsx
export interface PricingEditorProps {
  value?: PricingDTO;
  onChange: (p: PricingDTO) => void;
  /** 用于导入的上游模型名（表单 upstreamModel 字段） */
  upstreamModel: string;
}
```

组件开头的状态与操作函数替换为（`currency` 变量删除，改由规则表推导）：

```tsx
export function PricingEditor({ value, onChange, upstreamModel }: PricingEditorProps) {
  const t = useT();
  const [importing, setImporting] = useState(false);
  const rules = value?.rules ?? [];
  // 币种固定 USD：规则表非空 ⇔ 已计价
  const priced = rules.length > 0;

  const emit = (nextRules: PricingRuleDTO[]) => {
    const next: PricingDTO =
      nextRules.length > 0
        ? { currency: "USD", rules: nextRules }
        : { currency: "", rules: [] };
    onChange(next);
  };

  const patchRule = (idx: number, patch: Partial<PricingRuleDTO>) => {
    emit(rules.map((r, i) => (i === idx ? { ...r, ...patch } : r)));
  };

  const moveRule = (idx: number, delta: number) => {
    const j = idx + delta;
    if (j < 0 || j >= rules.length) return;
    const next = [...rules];
    [next[idx], next[j]] = [next[j], next[idx]];
    emit(next);
  };

  const handlePrefill = async () => {
    if (!upstreamModel.trim()) {
      toast.error(t("upstream.pricing.prefill.miss"));
      return;
    }
    setImporting(true);
    try {
      const rsp = await api.prefillModelPricing(upstreamModel.trim());
      const imported = rsp.pricing?.rules ?? [];
      if (!rsp.found || imported.length === 0) {
        toast.error(t("upstream.pricing.prefill.miss"));
        return;
      }
      // 非空规则 ⇒ emit 推导为 USD，等价于「自动打开计价开关」
      emit(imported);
      toast.success(t("upstream.pricing.prefill.hint"));
    } finally {
      setImporting(false);
    }
  };
```

同时把文件顶部 `import type { ... } from "@/lib/types";` 中的 `PricingCurrency` 删掉（新代码不再使用；`Select` 系列仍被时区下拉使用，**保留**）。

- [ ] **Step 5: 替换 JSX 头部（币种下拉 → 计价开关）**

把原「币种 `Label` + `Select` + 导入按钮」整段（原 **第 118-153 行**，即 `<div className="flex flex-wrap items-center gap-2">` 到其闭合 `</div>`）替换为：

```tsx
      {/* flex-wrap：计价开关 + 单位说明 + 导入按钮的 min-content 之和超过弹窗正文宽度时
          换行；否则会把弹窗 grid 轨道撑宽，定价卡片整体溢出弹窗右边界 */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="flex items-center gap-2">
          <Switch
            id="pricing-enabled"
            size="sm"
            checked={priced}
            onCheckedChange={(checked) => emit(checked ? [emptyRule()] : [])}
          />
          <Label htmlFor="pricing-enabled">{t("upstream.pricing.enabled")}</Label>
        </div>
        {priced && (
          <span className="text-xs text-muted-foreground">{t("upstream.pricing.unit")}</span>
        )}
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="ml-auto"
          onClick={handlePrefill}
          disabled={importing}
        >
          <Download className="size-4" />
          {t("upstream.pricing.prefill")}
        </Button>
      </div>

      {!priced && (
        <p className="text-[11px] text-muted-foreground">{t("upstream.pricing.disabled_hint")}</p>
      )}
```

同时删除该块上方原有的 2 行 `flex-wrap` 说明注释（原第 116-117 行），它已并入上面代码块；`flex-wrap` 类必须保留（最近一次修复 `1ca0c6f0` 就是为窄屏溢出加的）。

- [ ] **Step 6: 把规则区的 `{currency !== "" && (` 改为 `{priced && (`**

原规则区条件渲染是第 155 行的 `{currency !== "" && (<div className="space-y-2">...`，把条件改为：

```tsx
      {priced && (
        <div className="space-y-2">
```

规则卡片内部 JSX（`#序号` / 「默认」徽章 / 上下移 / 删除 / 时段开关 / 星期按钮 / 上下限 / 四价 / 添加时段窗口）**逐字保持原样**，只改两处 `emit` 调用点：

- 删除规则行按钮（原 `emit(currency, rules.filter((_, i) => i !== idx))`）→

```tsx
                    onClick={() => emit(rules.filter((_, i) => i !== idx))}
```

- 「添加规则」按钮（原 `onClick={() => emit(currency, [...rules, emptyRule()])}`）→

```tsx
            onClick={() => emit([...rules, emptyRule()])}
```

（副作用收益：原实现在「删光所有规则行」时会发送 `{currency:"USD",rules:[]}`，被后端 `NewPricing` 以 "pricing rules cannot be empty" 422 拒绝；新实现自动降级为未计价，不再报错。）

- [ ] **Step 7: `ModelForm.pricing` 改为必填**

`web/src/app/(dashboard)/upstream/shared.tsx`：

```tsx
export interface ModelForm {
  alias: string;
  modelId: string;
  upstreamModel: string;
  contextLength: number;
  maxOutputTokens: number;
  supportText: boolean;
  supportImage: boolean;
  /** 定价（currency="" ⇔ rules 为空 ⇔ 未计价） */
  pricing: PricingDTO;
}
```

```tsx
export const emptyModelForm: ModelForm = {
  alias: "",
  modelId: "",
  upstreamModel: "",
  contextLength: DEFAULT_CONTEXT_LENGTH,
  maxOutputTokens: DEFAULT_MAX_OUTPUT,
  supportText: true,
  supportImage: false,
  pricing: { currency: "", rules: [] },
};
```

- [ ] **Step 8: 两处编辑回填加未计价兜底**

`web/src/app/(dashboard)/upstream/page.tsx` 的 `openEditModel`：

```tsx
      pricing: model.pricing ?? { currency: "", rules: [] },
```

`handleFlatEdit`：

```tsx
      pricing: m.pricing ?? { currency: "", rules: [] },
```

（`handleSaveModel` 里的 `pricing: modelForm.pricing` 两处**不需要改**：类型收紧后它就是非 nil 对象，这正是「关闭计价也能清空」的关键。）

- [ ] **Step 9: 类型检查 + lint + 既有测试**

Run: `cd web && npm run lint && npm run test && npm run build`
Expected: 三条命令均成功；`npm run build` 输出静态导出成功（无 TS 错误）

- [ ] **Step 10: 运行时验证（`next-dev-loop`）**

先起后端与前端 dev server：

```bash
go run ./cmd/server server start --host localhost --port 8080
cd web && NEXT_PUBLIC_API_BASE_URL=http://localhost:8080 npx next dev --port 3100
```

按 `next-dev-loop` skill 的 `--scope worktree` 隔离 session，用 `agent-browser` 断言以下 5 项（入口 `http://localhost:3100/web/upstream/`）：

1. 打开某模型的编辑弹窗，未计价模型的「启用计价」开关为关，规则卡片不渲染，未计价说明文案可见
2. 打开开关 → 出现一条标注「默认」的规则卡片，开关右侧出现 `USD / 1M tokens`
3. 关闭开关 → 规则卡片消失；保存成功后重开弹窗，**仍是未计价**（验证清空定价生效）
4. 未计价态点「从 models.dev 导入」且上游模型名可命中 → 卡片被填充且开关自动打开
5. 已计价态点「添加规则」→ 出现第二条规则卡片，可上移/下移/删除

- [ ] **Step 11: 提交**

```bash
git add web/src/locales/en.json web/src/locales/zh.json web/src/locales/ja.json \
  "web/src/app/(dashboard)/upstream/pricing-editor.tsx" \
  "web/src/app/(dashboard)/upstream/shared.tsx" \
  "web/src/app/(dashboard)/upstream/page.tsx" \
  web/src/lib/types.ts web/src/lib/money.ts
git commit -m "refactor(web): 定价改为启用开关 + 币种固定 USD

- 币种下拉替换为「启用计价」开关，币种由规则表推导（非空 ⇔ USD）
- 导入按钮两种状态下均可点，命中后自动进入已计价
- pricing 不再传 undefined，修掉编辑时清空定价不生效
- 删除 money.ts 的 CNY 符号分支，i18n 增删三个 key"
```

---

## Task 3: 领域文档同步

**Files:**
- Modify: `CONTEXT.md`（`ModelPricing` 词条、`PricingPrefill` 词条）
- Modify: `web/CONTEXT.md`（`PricingEditor` 词条）

**Interfaces:**
- Consumes: Task 1、Task 2 的最终行为（币种固定 USD、导入整体替换规则表）
- Produces: 无代码接口；词汇表与实现对齐

- [ ] **Step 1: 更新 `CONTEXT.md` 的 `ModelPricing` 词条**

把该词条第 1 句的「币种（CNY/USD，空=未计价）」改为「币种（固定 USD，空=未计价）」，其余不变：

```markdown
**ModelPricing（模型定价）**:
挂在 Model 上的计费配置：币种（固定 USD，空=未计价）+ 定价规则表。价格单位为「每 1M tokens 的微单位」（1e-6 货币单位，int64 入账，禁浮点）。`currency` 为空 ⇔ 规则为空 ⇔ 未计价（审计 cost 记 NULL）；`currency` 非空且四价全 0 ⇔ 免费模型（审计 cost 记 0）。计价只覆盖 LLM 代理成功调用（失败不计费）。
_Avoid_: price config, billing config, rate card
```

- [ ] **Step 2: 修正 `CONTEXT.md` 的 `PricingPrefill` 词条漂移**

该词条现写「仅用于填充录入表单的默认规则行」，与实现不符（实际整体替换为含上下文分档的规则表）。改为：

```markdown
**PricingPrefill（定价导入辅助）**:
从 models.dev 公开定价按 `upstream_model` 精确匹配（trim 后、区分大小写）查询 USD 单价，**整体替换**录入表单的规则表（含按上下文分档的规则；models.dev 无时段窗口数据，需手填），**永不自动改价**；未命中/上游不可达一律降级为「未找到，可手填」。
_Avoid_: price import, auto pricing
```

- [ ] **Step 3: 重写 `web/CONTEXT.md` 的 `PricingEditor` 词条**

```markdown
**PricingEditor（定价规则编辑器）**:
Model 编辑弹窗内的定价录入区：「启用计价」开关（关 = 未计价，币种固定 USD）+ 规则行列表（时段窗口组、上下文区间、四价），行可增删与上下移（数组顺序 = 匹配优先级），无条件默认规则行标注「默认」。「从 models.dev 导入」在两种状态下都可点，命中后整体替换规则表（含上下文分档）并自动进入已计价；models.dev 无时段窗口数据，需手填。
_Avoid_: price form, billing editor
```

- [ ] **Step 4: 确认无残留的「币种下拉 / CNY」表述**

Run: `rg -n "币种下拉|CNY" CONTEXT.md web/CONTEXT.md`
Expected: 无输出。若 `web/CONTEXT.md` 的 Cost Panel 等词条提到「按币种分行/拆线」，**保留不改**（通用实现继续有效，仅是实际只有 USD 一条）。

- [ ] **Step 5: 提交**

```bash
git add CONTEXT.md web/CONTEXT.md
git commit -m "docs: 同步定价币种固定 USD 的领域术语，修正导入描述漂移"
```

---

## Task 4: 端到端回归与收尾

**Files:**
- Modify: `test/e2e/model_pricing/pricing_test.go`（新增 CNY 拒绝回归 + helper）
- Modify: 全局验证（无文件改动）

**Interfaces:**
- Consumes: Task 1 的 422 行为、Task 2 的前端交互
- Produces: 无

> **⚠️ 与设计文档的一处偏差（需确认）：** 设计文档的 E2E 清单要求「清空定价路径」的端到端回归。实施时评估后放弃该用例，理由是：清空后需要判定「`MODEL_ALIAS` 的最新审计行 cost 为 NULL」，而同 alias 的既有已计价历史行会与新行竞争「最新行」，必须改 `latestAuditCost` 的轮询语义（引入 flake 风险），且该用例必须改写真实模型的定价、无法并行（违反项目 `paralleltest` 强制规则）。改用两处覆盖等价路径：
> 1. 后端语义：`test/unit/pricing_dto/pricing_dto_test.go` 已有「未计价（显式清空）」用例覆盖 `PricingFromDTO({currency:"",rules:[]})`；
> 2. 前端交互：Task 2 Step 10 的运行时验证第 3 项「关开关 → 保存 → 重开弹窗仍为未计价」。
>
> E2E 改为验证本次改动的后端契约收紧（`CNY` 必须 422）。

- [ ] **Step 1: 写 E2E 回归用例**

在 `test/e2e/model_pricing/pricing_test.go` 末尾追加：

```go
// findModelID 按 alias 在平铺模型列表定位模型主键，并返回其当前 pricing 对象（未计价为 nil）。
func findModelID(t *testing.T, baseURL, jwt, alias string) (uint, any) {
	t.Helper()
	obj := getJSON(t, baseURL, jwt, "/api/web/v1/model/list?page=1&pageSize=50&query="+alias)
	items, _ := obj["items"].([]any)
	for _, it := range items {
		row, _ := it.(map[string]any)
		if row["alias"] != alias {
			continue
		}
		id, ok := row["id"].(float64)
		if !ok {
			t.Fatalf("model %s has non-numeric id: %v", alias, row["id"])
		}
		return uint(id), row["pricing"]
	}
	t.Fatalf("model %s not found in flat list", alias)
	return 0, nil
}

// patchPricingStatus 提交一次定价更新，返回 HTTP 状态码（不校验业务错误信封）。
func patchPricingStatus(t *testing.T, baseURL, jwt string, id uint, pricing map[string]any) int {
	t.Helper()
	body, err := sonic.Marshal(map[string]any{"pricing": pricing})
	if err != nil {
		t.Fatalf("marshal pricing: %v", err)
	}
	url := fmt.Sprintf("%s/api/web/v1/model?id=%d", strings.TrimRight(baseURL, "/"), id)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")
	resp, err := newE2EClient().Do(req)
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// TestPricing_DeprecatedCurrencyRejected 已废弃的 CNY 币种必须被 422 拒绝（请求校验阶段拦截，不写库）。
func TestPricing_DeprecatedCurrencyRejected(t *testing.T) {
	t.Parallel()
	env := mustEnv(t, "BASE_URL", "WEB_JWT", "MODEL_ALIAS")
	id, _ := findModelID(t, env["BASE_URL"], env["WEB_JWT"], env["MODEL_ALIAS"])
	status := patchPricingStatus(t, env["BASE_URL"], env["WEB_JWT"], id, map[string]any{
		"currency": "CNY",
		"rules":    []any{map[string]any{"input_price": 1}},
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("CNY pricing must be rejected with 422, got %d", status)
	}
}
```

- [ ] **Step 2: 编译并跑 E2E（无环境变量时应全部 skip）**

Run: `go test -count=1 ./test/e2e/model_pricing/`
Expected: `ok`（本机无 `BASE_URL`/`WEB_JWT` 时全部 `SKIP`，证明编译与 helper 无误）

- [ ] **Step 3: 全量 lint**

Run: `go run ./cmd/lint ./...`
Expected: 无输出

- [ ] **Step 4: 全量 Go 测试**

Run: `cd .worktrees/refactor-pricing-usd-only-2026-10-07 && make test`
Expected: 全绿。已知无关 flake：`test/unit/session_owner_id/TestStorePoolPersistsAPIKeyID` 在全量并发下偶发 `no such table: sessions`，重跑即过（见 memory `pricing/cost-model-impl-2026-10-05`）。

- [ ] **Step 5: 前端三件套复跑**

Run: `cd web && npm run lint && npm run test && npm run build`
Expected: 全部成功

- [ ] **Step 6: 过度工程审查**

按 `ponytail-review` skill 审查本次全部分支 diff（`git diff master...HEAD`），逐条确认无投机抽象、无重复造轮子、无死代码。若发现，删除后回到 Step 3-5 复验。

- [ ] **Step 7: 沉淀工程经验**

用 `serena_write_memory` 记录（memory 名建议 `pricing/usd-only-and-clearing-2026-10-07`）：

- `enum.Currency` 收紧到只认 `""`/`USD` 时，`NewPricing` 与 handler 无需改动（`Valid()` 是唯一开关）
- 生产库无任何定价存量（37 模型 / 69,859 审计全为 `''`），故本次无数据迁移
- 前端「规则表非空 ⇔ 币种」推导同时修掉两个 bug：清空定价不生效、删光规则行导致 422
- E2E 未加「清空定价」用例的理由（最新行竞争 + 无法并行）

- [ ] **Step 8: 收尾清理**

```bash
rm -rf web/.next web/out
```

- [ ] **Step 9: 提交**

```bash
git add test/e2e/model_pricing/pricing_test.go
git commit -m "test(e2e): 补已废弃 CNY 币种被 422 拒绝的回归用例"
```

- [ ] **Step 10: 询问用户后续动作**

按项目规范，**不得擅自**合并或推送。向用户报告：分支 `refactor/pricing-usd-only-2026-10-07` 已完成 + 验证证据，询问「提 MR / 直接合并 master / 暂不合并」。

---

## 自检记录

**Spec 覆盖检查**

| 设计文档条目 | 落地任务 |
|---|---|
| 决策 1、2（仅 USD、保留 currency 字段） | Task 1 Step 3/7（只收紧 `Valid()` 与 huma 标签，DB 列保留） |
| 决策 3、4（计价开关、开开关补默认规则） | Task 2 Step 5（`emit(checked ? [emptyRule()] : [])`） |
| 决策 5（导入两态可点 + 自动开计价） | Task 2 Step 5/4（按钮去掉 `currency !== ""` 条件；`emit(imported)` 自动推导 USD） |
| 决策 6（卡片能力全保留） | Task 2 Step 6（规则卡片内部逐字保留，仅改 2 处 `emit` 调用） |
| 决策 7（关闭开关发送非 nil 空对象） | Task 2 Step 4/7/8（`emit` 恒返回 `PricingDTO`；`ModelForm.pricing` 必填） |
| 决策 8（清理 `money.ts` 的 `¥`） | Task 2 Step 2 |
| 后端改动表 5 个文件 | Task 1 Step 3/7/8 |
| 单测改币种 + CNY 拒绝回归 | Task 1 Step 1/5 |
| E2E 更新 | Task 1 Step 9 + Task 4 Step 1 |
| 文档维护（含修正 2 处漂移） | Task 3 Step 1/2/3 |
| Web 运行时验证清单 | Task 2 Step 10（5 项断言） |

**偏差记录**：E2E「清空定价路径」改为前端运行时验证 + 既有后端单测覆盖（理由见 Task 4 开头的偏差说明，待用户确认）。

**类型一致性**：`onChange: (p: PricingDTO) => void`（Task 2 Step 4）与 `ModelForm.pricing: PricingDTO`（Task 2 Step 7）、`emit` 返回 `PricingDTO` 三处签名一致；`enum.CurrencyUSD` 在 Task 1 全量替换后无 `CurrencyCNY` 残留。
