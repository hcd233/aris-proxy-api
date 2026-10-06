# 定价交互改造：币种固定 USD 设计

> 状态：设计已确认，待实施
> 分支：`refactor/pricing-usd-only-2026-10-07`
> 基线：`1ca0c6f0`
> 日期：2026-10-07
> 修订对象：`2026-10-05-model-pricing-cost-design.md`（决策 4「币种」与前端定价区交互被本设计取代，其余部分继续有效）

## 背景

2026-10-05 的计费设计引入「Model 级币种（CNY/USD，空 = 未计价）」并落地为 Web 端一个币种下拉。实际使用中暴露两个问题：

1. **币种选择是伪选择**：所有上游供应商与 models.dev 公开报价均为 USD，CNY 分支从未被真实使用。
2. **币种下拉承载了两个职责**：既选币种，又充当「是否计价」的开关（`""` = 未计价 → 规则区隐藏、审计记 `NULL`）。职责耦合导致交互笨重，且下拉与规则区的联动逻辑（切币种时自动补规则行、切「未计价」时清空）难以理解。

本次改造：**保留「是否计价」的开关语义，把币种从「可选择」降为「固定 USD」**，交互上币种下拉替换为一个计价开关；规则卡片、时段窗口、上下文区间、四价等编辑能力全部保留（不降级为只读）。

### 生产数据证据（2026-10-07 只读查询 `api.lvlvko.top`）

| 查询 | 结果 |
|---|---|
| `SELECT pricing_currency, count(*) FROM models GROUP BY 1` | 37 行，全部 `''`（未计价）；CNY/USD 各 0 行 |
| `SELECT count(*) FROM models WHERE pricing_currency <> ''` | `0` |
| `SELECT pricing_currency, count(*) FROM model_call_audits GROUP BY 1` | 69,859 行，全部 `''` |

结论：**生产环境从未配置过任何定价，不存在 `CNY` 存量数据**，因此删除 CNY 枚举无需数据迁移，也不破坏历史审计快照的读取。

## 目标与非目标

### 目标

1. 移除币种选择，币种固定为 `USD`；`currency` 字段保留（继续承担「未计价 vs 已计价」语义）
2. 用「计价」开关替代原币种下拉，承担「未计价 ⇄ 已计价」的状态切换
3. 「从 models.dev 导入」在两种状态下都可点，命中后自动进入已计价并填充规则
4. 保留全部规则编辑能力（四价、时段窗口、上下文区间、增删行、上下移）
5. 修复「编辑时无法清空定价」缺陷（原 `pricing: undefined` 被 `JSON.stringify` 丢弃，后端视为「不修改」）

### 非目标（明确不做）

- 彻底移除币种概念（不删 DB 列 `pricing_currency`、不删 wire 的 `currency` 字段、不改成本聚合的按币种分组）
- 单币种特化成本面板（`Cost Panel` 的按币种分行/拆线保留通用实现，实际只有 USD 一条）
- 存量数据迁移（无 CNY 数据；37 行未计价模型语义不变）
- 只读化规则卡片（用户明确选择保留全编辑能力）

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| 1 | 币种支持范围 | 仅 `USD`（+ 空值 = 未计价） | 真实上游报价全为 USD；CNY 是伪选择 |
| 2 | 币种字段去留 | **保留** wire 与 DB 的 `currency` 字段，值固定 `USD` | 继续区分「未计价（NULL）」与「免费模型（0）」；成本聚合无需改造；未来加币种零成本 |
| 3 | 「是否计价」入口 | 新增「计价」Switch | 原下拉承担该职责，删下拉必须补回，否则未计价态无法进入已计价态 |
| 4 | 未计价 → 已计价 | 打开开关即补一条无条件默认规则 | 一条无条件规则是 `NewPricing` 的硬校验要求，自动补齐让开关直接可用 |
| 5 | 导入按钮可见性 | 两种状态下都可见可点，命中后自动开计价并填充 | 导入是最常用入口，不应要求用户先开开关再导入 |
| 6 | 规则卡片能力 | 全保留（四价/时段/区间/增删/上下移） | 保住「models.dev 未命中的自建模型手填价格」与「时段窗口需手填」（models.dev 无时段数据） |
| 7 | 清空定价 | 关闭开关 → 前端发送 `{currency:"", rules:[]}`（非 nil） | 修掉原缺陷；同时消除「nil = 不修改」的 wire 歧义 |
| 8 | `money.ts` 的 `¥` 分支 | 删除 CNY 特判，保留「未知币种 → 空符号」兜底 | CNY 不再是合法币种，该分支成死代码 |

## 前端设计（`web/src/app/(dashboard)/upstream/`）

### 状态模型

定价仍是两态，由 `currency` 表达：

- `""`（未计价）：不产生费用估算，审计 `cost_micro = NULL`
- `"USD"`（已计价）：规则表生效；四价全 0 = 免费模型，审计 `cost_micro = 0`

`PricingCurrency` 类型收紧为 `"" | "USD"`。

### 布局

```
[计价 ⬤]  USD / 1M tokens          [ 从 models.dev 导入 ]
          ↑ 仅已计价时显示
┌ 规则卡片（仅已计价时渲染）───────────────────────┐
│ #1 [默认]                          ↑ ↓ 🗑        │
│ [⬤]时段限制  [ ]上下文区间                        │
│ (条件行：时段窗口组 / 上下文 min-max)             │
│ 输入单价 输出单价 缓存创建单价 缓存读取单价        │
└─────────────────────────────────────────────────┘
[ + 添加规则 ]
```

### 交互规则

1. **未计价态**：显示「计价」开关（关）+ 一行说明文案（「未计价模型不产生费用估算；可手动配置或从 models.dev 导入」）+「从 models.dev 导入」按钮；规则卡片区整体不渲染。
2. **打开开关** → `currency = "USD"`，`rules` 为空时自动补一条 `emptyRule()`（无条件默认规则）→ 卡片展开。
3. **关闭开关** → `{ currency: "", rules: [] }`，即清空定价。
4. **「从 models.dev 导入」**：
   - `upstreamModel` 为空 → `toast.error("未找到，可手动填写")`，状态不变
   - `found=false`（未命中 / 上游不可达，后端统一降级）→ 同上 toast，状态不变
   - 命中 → 写入 `{ currency: "USD", rules: rsp.pricing.rules }`，**自动打开计价开关**，`toast.success`
5. **规则卡片内部逻辑一行不动**：`isDefaultRule` 徽章、时段窗口开关与默认值、上下文区间开关与 `200_000` 默认值、星期按钮组、时区下拉（`UTC` / `Asia/Shanghai`）、四价输入（`step=0.000001`）、增删与上下移全部保留。
6. **单位提示**：开关右侧静态文案 `USD / 1M tokens`（仅已计价时显示），替代原下拉的位置感，让用户知道四价单位。

### 删除项

- 币种 `Select` 及其三个 `SelectItem`（「未计价」/`CNY`/`USD`）
- `emit(nextCurrency, nextRules)` 的币种参数：改为 `emit(nextRules)`，币种由规则表推导 —— `currency = len(rules) > 0 ? "USD" : ""`。这让「未计价 ⇔ 规则为空」的不变量在前端也天然成立，并消除所有「切币种」分支：`emit` 内不再需要 `rules.length > 0 ? rules : [emptyRule()]` 的补行逻辑，补行改由开关的 `onCheckedChange` 独占负责

`PricingEditorProps.value` 保持可选（组件容忍 `undefined`，视为未计价）；必填约束只加在 `ModelForm.pricing` 上。

### 顺带修复：清空定价不生效

**缺陷**：编辑模式下把币种切到「未计价」保存后，定价没被清掉。原因是 `PricingEditor.emit` 在空态调用 `onChange(undefined)`，`ModelForm.pricing` 变为 `undefined`，`JSON.stringify` 丢弃该字段 → 后端 `req.Body.Pricing == nil` → `PricingSet = false` → 走「不修改」分支。

**修法**：前端**永远发送具体的 `pricing` 对象**，不在 wire 层用 `nil` 表达「未计价」：

- `ModelForm.pricing` 由 `PricingDTO?` 改为**必填** `PricingDTO`
- `emptyModelForm.pricing = { currency: "", rules: [] }`
- `openEditModel` / `handleFlatEdit` 回填 `model.pricing ?? { currency: "", rules: [] }`
- create/update body 均传非 nil 的 `pricing`
- 后端行为自然正确：`PricingFromDTO({currency:"",rules:[]})` → `NewPricing("", [])` → 未计价 → `PricingSet = true` → 清空

收益：消除「nil = 不修改」的隐含契约，`UpdateModelReqBody.pricing` 的注释（「currency 与 rules 均置空 = 清空为未计价」）从「文档承诺、实现失效」变为真实成立。

## 后端改动

| 文件 | 改动 |
|---|---|
| `internal/common/enum/currency.go` | 删除 `CurrencyCNY`；`Valid()` 只认 `CurrencyNone` / `CurrencyUSD` |
| `internal/dto/pricing.go` | huma 标签 `enum:",CNY,USD"` → `enum:",USD"`（CNY 请求 → 422，不静默接受）；doc 更新为「空 = 未计价 / USD = 已计价」 |
| `internal/infrastructure/database/model/model.go` | 列注释 `''/CNY/USD` → `''/USD`（AutoMigrate 自动应用） |
| `internal/infrastructure/database/model/model_call_audit.go` | 列注释 `''/CNY/USD` → `''/USD` |
| `internal/domain/llmproxy/vo/pricing.go` | **无需改动**：`NewPricing` 通过 `currency.Valid()` 判断，随枚举自动收紧 |
| `internal/handler/model.go` | **无需改动**：`if req.Body.Pricing != nil { ... PricingSet = true }` 逻辑在新契约下自然正确 |

### 明确不改

- `vo.Pricing` / `vo.PricingRule` / `vo.TimeWindow` 结构
- DB 列 `models.pricing_rules`、`models.pricing_currency`、`model_call_audits.cost_micro`、`model_call_audits.pricing_currency`
- 成本聚合（`cost/summary`、`cost/distribution`）的按币种分组逻辑与 SQL
- `prefill_pricing.go` 的 `tiersToRules` 分档映射（已是含上下文区间规则的完整规则表）
- Cost Panel / Dashboard 成本卡的按币种分行与拆线（实际只有 USD 一条）

### 未计价模型的回填边界

`PricingToDTO` 对未计价返回 `nil` → wire 中无 `pricing` 字段 → 前端行数据 `model.pricing` 为 `undefined`。前端只在回填处兜底 `?? { currency: "", rules: [] }`；`ModelForm.pricing` 类型收紧为必填后，其余消费点由 TypeScript 编译器保证非空。

## 测试

### 单测（改币种 + 补回归）

- `test/unit/pricing/pricing_test.go`：`CurrencyCNY` → `CurrencyUSD`；「计价但无规则」用例改用 USD
- `test/unit/pricing_dto/pricing_dto_test.go`：同上
- `test/unit/model_pricing_agg/model_pricing_agg_test.go`：同上（同时覆盖 `Model.Pricing()` 装配）
- `test/unit/model_call_pricing/model_call_pricing_test.go`：同上（覆盖 `cost_micro` 与 `pricing_currency` 快照）
- **新增**（`test/unit/pricing/`）：`currency: "CNY"` 被 `Valid()` 拒绝的回归用例

### E2E（`test/e2e/model_pricing/`）

1. 更新注释与断言中的 CNY → USD
2. 回归「清空定价」路径：创建 USD 定价模型 → 审计 `cost_micro` 非 NULL → 关闭计价（发送 `{currency:"",rules:[]}`）→ 审计 `cost_micro` 回到 NULL
3. 保留原有断言：未计价模型 `cost_micro` 为 NULL；`cost/summary` 与 `cost/distribution` 返回正确

### Web

`npm run lint && npm run test && npm run build`，再按 `next-dev-loop` 做运行时验证：

- 未计价态下开关关闭、卡片不渲染
- 开开关 → 出现一条「默认」规则卡片
- 关开关 → 卡片消失，保存后重开弹窗仍为未计价（验证清空生效）
- 导入命中 → 卡片填充且开关自动打开；导入未命中 → toast 且状态不变

## 文档维护

- `CONTEXT.md`：`ModelPricing` 词条币种改为「USD，空 = 未计价」；`PricingRule` 词条内的币种说明同步
- `web/CONTEXT.md`：`PricingEditor` 词条重写（去掉币种下拉、补计价开关与导入按钮的自动开计价行为）
- **修正既有漂移**：`web/CONTEXT.md` 现写「『从 models.dev 导入』仅填充默认规则行四价」，与实现不符（实际是整体替换为含上下文分档的完整规则表，`tiersToRules`），一并改正
- i18n（`en` / `zh` / `ja` 三份同步）：
  - **新增**：`upstream.pricing.enabled`（计价开关 label）、`upstream.pricing.unit`（`USD / 1M tokens`，三语同值）、`upstream.pricing.disabled_hint`（未计价态说明）
  - **删除**：`upstream.pricing.currency`、`upstream.pricing.currency.none`（下拉取消后无引用）
  - **保留**：`upstream.pricing.title` / `prefill` / `prefill.miss` / `prefill.hint` / `rule.*` / `window.add` / `context*` / 四价 label

## 实施注意事项

1. **Go skill 清单**：改 `internal/common/enum`、`internal/dto` 前按 `docs/agents/workflow.md` 加载 `use-modern-go`（先跑 `list`）、`golang-naming`、`golang-code-style`、`golang-samber-lo`、`golang-samber-mo`，以及 `huma-dto-conventions`（动 `internal/dto/**`）
2. **lint conv**：`go run ./cmd/lint ./...` 必须全量跑（按包过滤会漏报）
3. **worktree**：`web/node_modules` 用 `cp -al` 造硬链接树（Turbopack 拒绝软链，见 memory `pricing/cost-model-impl-2026-10-05`）；Go 构建需补 `internal/web/dist/index.html` 占位
4. **验证**：`make lint` + `make test`；E2E 需先准备 `internal/web/dist`
5. **提交前**：用 `ponytail-review` 审查 diff 的过度工程，并用 `serena_write_memory` 沉淀本次经验
