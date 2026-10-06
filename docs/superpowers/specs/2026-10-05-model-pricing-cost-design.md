# 模型计费（成本估算）设计

> 状态：设计已确认（v2，含定价规则表），待实施
> 分支：`feature/model-pricing-cost-2026-10-05`
> 基线：`9de0f2fe`
> 日期：2026-10-05

## 背景

调研 cc-switch（github.com/farion1231/cc-switch，MIT）后确定两个可迁移能力：Gemini 协议支持与模型计费。经范围拆分，本轮先做**模型计费**（Gemini 协议另行设计）。

参照 cc-switch 的用量与成本能力（`model_pricing.rs` 的 models.dev 导入、`usage/calculator.rs` 的四类 token 计价）：按"模型单价 × 四类 token"估算每次调用费用，在审计与统计中展示成本维度。aris 现状已有 `ModelCallAudit` 四类 token 审计（Input/Output/CacheCreation/CacheRead）与趋势/吞吐统计，但**没有任何价格与费用概念**。

评审阶段补充了 cc-switch 未覆盖的两个真实计费场景：**时段价格**（峰谷价，如 DeepSeek/Moonshot 夜间谷价）与**上下文区间价格**（如 Gemini 2.5 Pro 的 ≤200k / >200k 分档）。因此定价模型设计为**定价规则表**，而非单一四价。

## 目标与非目标

### 目标

1. Model（用户级）可配置定价规则表：每条规则含可选时段条件 + 可选上下文区间条件 + 四类单价；币种在 Model 级（CNY/USD）
2. 录入以手工为主，支持从 models.dev 一键导入填充默认规则（平价四价）
3. LLM 代理调用审计时按（调用时刻, prompt 总 token）匹配规则、按当时价格计算估算费用并落库（历史费用不随改价/时段流逝而漂移）
4. Web 展示：upstream 页定价规则录入、audit 页费用列与成本合计/趋势、Dashboard 本期成本卡、按用户/API Key/模型的成本分布

### 非目标（明确不做）

- 预算/限额告警、余额、扣费、账单结算
- 汇率换算（按币种分组展示，不折算）
- 价格历史版本与审计规则/单价快照（审计只存费用结果 + 币种快照）
- 存量审计回算（`cost_micro` 为 NULL 即"未计价"，运维可自行跑一次性 SQL 回填）
- 失败调用计费（仅成功调用计价）
- **累进分段计价**（税阶式分段求和；口径为整段跳档）
- **生效日期区间规则**（"11 月 1 日起调价"由人工改价 + 请求时落库 + 不回算天然覆盖）
- **节假日等特殊日历规则**
- Gemini 协议（下一轮；计费按 token 四维设计，与协议无关，届时接入即可）
- 模糊模型名归一化匹配（cc-switch 的清洗规则是为日志猜名服务的；aris 的 `upstream_model` 是用户显式填写的真实模型名，精确匹配足够）

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| 1 | 计费口径 | 纯成本估算 | 不涉及钱的转移，"看得见花了多少" |
| 2 | 价格归属 | 挂在 Model（用户级） | 同名 alias 各用户上游与合同价不同，与多租户语义一致 |
| 3 | 计算时机 | 请求时算好落库 | 历史费用不漂移，聚合是现成 SQL SUM |
| 4 | 币种 | Model 级币种（CNY/USD），聚合分币种展示 | 不失真、无外部汇率依赖；同一模型不同时段/档位不同币种是荒谬的，币种不下放到规则 |
| 5 | 录入方式 | 手工 + models.dev 导入辅助（仅填充表单） | 降低录入成本，不自动改价 |
| 6 | 存量数据 | 不回算，NULL=未计价 | 语义干净，避免"历史按今天价算"的失真口径 |
| 7 | 展示范围 | upstream + audit + Dashboard + 成本分布（按用户/API Key/模型） | 全量成本视图 |
| 8 | 钱的精度 | int64 微单位（1e-6 货币单位） | 零新依赖、SQL 聚合精确，符合 `metrics/float-rounding-guard` 约定（禁止浮点入账） |
| 9 | 价格结构 | 定价规则表，**第一命中**（顺序敏感）+ **强制恰好一条无条件默认规则** | 覆盖时段/上下文区间场景；匹配确定可预测（类似防火墙规则），默认规则保证计价永不落空 |
| 10 | 区间计价口径 | **整段跳档**，区间按 **prompt 总 token**（input+cacheCreation+cacheRead）判定 | Gemini/Claude 官方口径；不做累进分段 |

## 领域模型

### 定价值对象（新增 `internal/domain/llmproxy/vo/pricing.go`）

```go
// Pricing 模型定价：币种 + 规则表
type Pricing struct {
    currency enum.Currency // 空值 = 未计价
    rules    []PricingRule
}

// PricingRule 定价规则（四价单位：微单位/1M tokens）
type PricingRule struct {
    timeWindows      []TimeWindow // 空 = 全时段
    contextMin       int64        // prompt token 下界（含），0 = 不限
    contextMax       int64        // 上界（不含），0 = 无上限
    inputMicro       int64
    outputMicro      int64
    cacheCreateMicro int64
    cacheReadMicro   int64
}

// TimeWindow 时段窗口（循环生效）
type TimeWindow struct {
    days     []int    // 周几 1=周一…7=周日，空 = 每天
    start    string   // "HH:MM"，含
    end      string   // "HH:MM"，不含；end < start 表示跨午夜
    timezone string   // IANA 时区，默认 "UTC"
}
```

语义约定：

- `enum.Currency`（`internal/common/enum`）：`CurrencyNone = ""` / `CurrencyCNY = "CNY"` / `CurrencyUSD = "USD"`
- **未计价 vs 免费**：`currency == ""` ⇔ `rules` 为空 ⇔ 未计价（审计记 NULL）；`currency != ""` 且命中规则四价全 0 ⇔ 免费模型（审计记 0）
- 构造校验 `NewPricing(currency, rules) (Pricing, error)`（`ierr.ErrValidation`）：
  - `currency == ""` 时 rules 必须为空；`currency != ""` 时 rules 非空
  - **恰好一条无条件规则**（`timeWindows` 空 ∧ `contextMin == 0` ∧ `contextMax == 0`），作为兜底默认规则（位置无关）
  - 单价 ≥ 0 且 ≤ 1e12 微单位（= 1e6 货币单位/1M tokens，防 int64 乘法溢出）；缓存两价允许 0（按 0 计，不做隐式比例推算）
  - 区间：`contextMin ≥ 0`；`contextMax == 0` 或 `contextMax > contextMin`
  - 时段窗口：`HH:MM` 可解析；`days` 成员 ∈ [1,7]；`timezone` 可 `time.LoadLocation`；`start != end`
- **匹配 `Match(at time.Time, promptTokens int64) PricingRule`**：按 rules 数组顺序逐条判定「时段命中 ∧ 区间命中」，**第一条全中者生效**；无条件默认规则保证总有命中（条件规则全不中时返回它）
  - 时段命中：窗口列表为空 ⇒ 全时段；否则任一窗口命中即算（OR）。窗口命中 = `days` 含调用时刻在该时区下的周几（`days` 空按每天）∧ 时刻落在 `[start, end)` 半开区间；`end < start` 视为跨午夜的连续段
  - 区间命中：`contextMin ≤ promptTokens` 且（`contextMax == 0` 或 `promptTokens < contextMax`）
  - `promptTokens = input + cacheCreation + cacheRead`（整段跳档：落哪档就全部 token 按该档四价）
- **`ComputeCost(rule PricingRule, input, output, cacheCreate, cacheRead int64) int64`**：四项分别 `round(price × tokens / 1_000_000)` 后求和。**用 `math.Round`，禁止 `int64(v*scale+0.5)` 截断**（负数错舍/溢出，见 `metrics/float-rounding-guard`）
- token 计数入参用 4 个 int64 而非 `modelcall/vo.TokenBreakdown`，避免 llmproxy 域反向依赖 modelcall 域

### Model 聚合（`internal/domain/llmproxy/aggregate/model.go`）

- 新增私有字段 `pricing vo.Pricing`
- `CreateModel` 入参**追加 `pricing vo.Pricing` 参数**（不引入附加构造器），校验委托 `vo.NewPricing`；同步更新全部调用点
- 新增 `UpdatePricing(pricing vo.Pricing) error`（价格可独立修改，与改名乐观锁无关）
- Getter：`Pricing() vo.Pricing`

### ModelCallAudit 聚合（`internal/domain/modelcall/aggregate/audit.go`）

- 新增字段：`costMicro *int64`、`pricingCurrency enum.Currency`
- `RecordCallInput` 扩展同名两项；`GetCostMicro() *int64`、`GetPricingCurrency() enum.Currency`
- 语义：`costMicro == nil` ⇔ 未计价（模型未配置币种，或调用失败）；`*costMicro == 0` ⇔ 免费模型
- 审计**不快照命中的规则**，只存费用结果 + 币种快照（决策 6 的延伸）

## 数据模型

GORM AutoMigrate 自动迁移（`database migrate` job），仅加列、无新表、无新索引（成本查询复用 `idx_mca_*` 既有索引；按用户维度沿用 `user → api_key_ids` 查询模式）。

### `models` 表加 2 列

| 列 | 类型 | 语义 |
|---|---|---|
| `pricing_rules` | JSON（`serializer:json`，复用 `capabilities` 既有模式） | 定价规则数组（微单位单价）；`[]` = 未计价 |
| `pricing_currency` | `varchar(8) not null default ''` | 计价币种（`''`/`CNY`/`USD`） |

规则数组示例（DB 存储形态，微单位）：

```json
[{"time_windows":[{"days":[1,2,3,4,5],"start":"00:30","end":"08:30","timezone":"UTC"}],
  "context_min":0,"context_max":200000,
  "input_price_micro":1000000,"output_price_micro":4000000,
  "cache_creation_price_micro":0,"cache_read_price_micro":100000},
 {"time_windows":[],"context_min":0,"context_max":0,
  "input_price_micro":2000000,"output_price_micro":8000000,
  "cache_creation_price_micro":2500000,"cache_read_price_micro":200000}]
```

选择 JSON 列而非独立规则表的理由：规则只在转发路径内存匹配与表单展示用，**无 SQL 查询需求**，JSON 列比"新表 + join"少一张表一层装配（ponytail）。

### `model_call_audits` 表加 2 列

| 列 | 类型 | 语义 |
|---|---|---|
| `cost_micro` | `bigint`（可空） | 估算费用（微单位），NULL=未计价 |
| `pricing_currency` | `varchar(8) not null default ''` | 计价币种**快照**（事后改币种不影响历史聚合） |

## 计算链路

复用审计收尾 seam `recordModelCall`（`internal/application/llmproxy/usecase/recorder.go`）：

1. 转发解析出的 `aggregate.Model` 经仓储装配携带 `Pricing`（含规则表反序列化）
2. 构造 `RecordCallInput` 时按下列规则填 `CostMicro` / `PricingCurrency`：
   - `!m.Pricing().IsPriced()` → `CostMicro = nil`
   - 调用失败（`CallStatus.UpstreamStatusCode != 200`）→ `CostMicro = nil`（失败不计费）
   - 成功且已计价 → `rule := m.Pricing().Match(createdAt, promptTokens)`，`costMicro := m.Pricing().ComputeCost(rule, ...)`，`PricingCurrency = m.Pricing().Currency()`（含免费模型的 0）
3. 时点取审计 `createdAt`（调用时间），保证时段规则语义稳定
4. 经现有 pond 任务异步落库，不增加请求路径延迟；7 条转发路径共用该 seam，无路径特判

## API 契约（Huma，`/api/web/v1`）

DTO 惯例遵循 `huma-dto-conventions` skill：huma v2 请求 wire 格式平铺。**wire 层金额一律为展示单位浮点（`cost`、`*_price`）+ `currency`**，微单位（`*_micro`）只存在于 DB 与 domain；DTO↔domain 边界集中换算，聚合在 SQL 整数域完成、输出时换算一次。

### Model CRUD 扩展（现有 endpoint/model 路由 DTO 加嵌套对象）

请求/响应均加 `pricing` 对象：

```json
"pricing": {
  "currency": "CNY",
  "rules": [
    {"time_windows": [{"days": [1,2,3,4,5], "start": "00:30", "end": "08:30", "timezone": "UTC"}],
     "context_min": 0, "context_max": 200000,
     "input_price": 1.0, "output_price": 4.0, "cache_creation_price": 0, "cache_read_price": 0.1},
    {"time_windows": [], "context_min": 0, "context_max": 0,
     "input_price": 2.0, "output_price": 8.0, "cache_creation_price": 2.5, "cache_read_price": 0.2}
  ]
}
```

校验（与 `vo.NewPricing` 同源，422）：

- `currency` ∈ `""`/`CNY`/`USD`；`currency == ""` ⇔ `rules` 为空
- `currency != ""` 时 rules 非空且**恰含一条无条件规则**（`time_windows` 空 + `context_min=0` + `context_max=0`）
- 时段窗口：`HH:MM` 可解析、`days` ∈ [1,7]、`timezone` 可加载、`start != end`
- 区间：`context_max == 0` 或 `context_max > context_min`；价格 ≥ 0 且 ≤ 1_000_000（展示单位 = 微单位 ≤ 1e12）
- 换算集中在一个工具对：`priceMicroFromDisplay(float64) int64`（`math.Round(v*1e6)`）/ `priceDisplayFromMicro(int64) float64`，其余层不感知微单位
- 未填定价（`currency == ""`、`rules: []`）为默认行为，向后兼容存量创建请求

### models.dev 导入辅助（新端点）

`GET /api/web/v1/model/pricing/prefill?upstream_model=<name>`（JWT，user/admin 均可用）

响应：

```json
{ "found": true, "currency": "USD", "input_price": 0.8, "output_price": 4,
  "cache_creation_price": 1, "cache_read_price": 0.08 }
```

- 数据源：models.dev 公开定价（api.json），后端拉取后写 Redis 缓存（key `pricing:modelsdev:doc`，TTL 24h），进程内解析
- 匹配规则：`upstream_model` trim 后与模型 ID **精确匹配**（区分大小写），未命中 `found=false`
- 返回的是平价四价，前端填入**默认规则**行；时段/区间规则由用户手工添加
- 仅用于填充表单，**永不自动改价**；上游不可达返回 `found=false`，不报错阻塞
- 实现位置：`internal/application/model/query`（`PrefillPricing` 用例）+ 基础设施侧 models.dev 拉取服务（复用 `internal/infrastructure/httpclient`）

### 审计查询扩展

- `list_audit_logs` 行加 `cost`（number，展示单位，NULL=未计价时为 null）与 `currency`
- 新增 `GET /api/web/v1/audit/cost/summary?from&to&granularity`：
  响应 `{ "totals": [{"currency":"CNY","cost":0.123}], "series": [{"bucket_time":"2026-10-05T00:00:00Z","currency":"CNY","cost":0.045}] }`
  按币种分组；粒度沿用 `Granularity`（minute/hour/day/week）；`bucket_time` 为 RFC3339 UTC；时间范围与现有 audit 统计一致
- 新增 `GET /api/web/v1/audit/cost/distribution?group_by=user|api_key|model&from&to&limit`：
  响应 `{ "group_by":"model", "items": [{"id":"...","name":"...","currency":"CNY","cost":0.123}] }`
  （每行 = 分组 × 币种）；`limit` 默认 10、上限 100，按币种内费用降序
- 权限与脱敏：user 视角只允许 `group_by=api_key|model` 且限定自身 `api_key_ids`；`group_by=user` 仅 admin/demo 全量视角；demo 的用户身份脱敏沿用 `option_list` 的三态模式
- 实现位置：`internal/application/audit/query`（新 handler + service 注册），仓储聚合走现有 DAO + SQL `SUM(cost_micro) ... GROUP BY currency, ...`，NULL 行不计入（口径注明"仅统计已配置价格的调用"）

## 前端（web/）

| 页面 | 改动 |
|---|---|
| `upstream` | Model 编辑弹窗加"定价"区：币种下拉 + **规则列表编辑器**（每行：时段窗口组（可多组：周几多选 + 起止时刻 + 时区）+ 上下文区间（min/max，max 空=无上限）+ 四价；可增删行、拖拽排序（顺序 = 匹配优先级）；无条件默认规则行标注"默认"）+ 「从 models.dev 导入」按钮（填充默认规则行，可改后保存） |
| `audit` | 列表加"费用"列（`$0.0012` / `¥0.008`，NULL 显示 `—`）；统计区加成本合计卡 + 成本趋势线（按币种拆线）+ 成本分布块（分组切换：模型/API Key，admin 多"用户"） |
| `dashboard` 首页 | 本期成本卡（按币种分行，点击跳 audit） |
| 公共 | `src/lib/types` DTO 镜像同步；金额格式化工具（float → 展示字符串，去尾零，小数位上限 6，避免浮点尾差直出，如 `0.30000000000000004`）；i18n 中英 |

## 错误处理

- prefill 未命中/上游不可达：`found=false`，前端提示"未找到，可手动填写"，录入不被阻塞
- 未计价调用：`cost_micro` NULL，列表显示 `—`，统计口径注明"仅统计已配置价格的调用"
- DTO 校验失败：422（币种非法、价格负数/超上限、`currency` 与 `rules` 不一致、无条件规则缺失或不唯一、时段窗口非法、区间非法、时区不可加载）
- 计算溢出防护：价格上限 1e12 微单位 × token 数在 int64 内安全（1e12 × 1e6/1e6 = 1e12）；舍入一律 `math.Round`
- 成本聚合 SQL 不做浮点运算，浮点尾差防护沿用 `metrics/float-rounding-guard` 约定（输出层 `math.Round`）
- **tzdata**：`time.LoadLocation` 依赖 zoneinfo，生产镜像若缺失则非 UTC 时区规则会构造失败——在服务端入口 `import _ "time/tzdata"` 内嵌（约 450KB）作为确定性兜底

## 测试

### 单测

- `vo.Pricing` 构造校验：非法币种、`currency`/`rules` 不一致、无条件规则缺失/重复、负价、超上限、区间非法、时段窗口非法（HH:MM / days / 时区 / start==end）
- `Match`：第一命中顺序敏感、默认兜底、时段窗口 OR、跨午夜（22:00–02:00）、非 UTC 时区（含一个 DST 案例）、days 过滤、区间半开边界（恰等于 `context_max` 落下一档、恰等于 `context_min` 命中）、时段 ∧ 区间双条件
- `ComputeCost`：四价组合、免费 0、舍入边界 `.5`、大 token 数不溢出、缓存价 0
- DTO 换算工具：展示值 ↔ 微单位往返、`math.Round` 边界
- prefill：精确匹配、trim、未命中、缓存命中
- audit 查询：分币种聚合、NULL 排除、分组三态权限

### E2E（`test/e2e/model_pricing/`）

1. 建带定价规则的 Model（含时段/区间规则）→ 经 mock 上游发起 LLM 调用 → 断言 `model_call_audits.cost_micro` 按命中规则计算正确、`pricing_currency` 快照正确
2. 未配置币种的 Model 调用 → `cost_micro` 为 NULL
3. `cost/summary` 与 `cost/distribution` 分币种/分组返回正确（含 admin `group_by=user` 与 user 越权 403）
4. prefill 端点（mock models.dev 响应或录制）

### Web

`npm run lint && npm run build` + 按 `next-dev-loop` 做运行时验证（规则编辑器增删排序、导入填充、费用列展示、成本图表）。

## 文档维护

- `CONTEXT.md` 新增词条：**ModelPricing（模型定价）**、**PricingRule（定价规则）**、**TimeWindow（时段窗口）**、**ContextTier（上下文区间）**、**EstimatedCost（估算费用）**、**PricingCurrency（计价币种）**、**PricingPrefill（定价导入辅助）**；并在 Model 词条补充定价语义（未计价 vs 免费、第一命中、整段跳档）
- 前端术语同步 `web/CONTEXT.md`

## 实施注意事项（来自 Serena 历史经验）

1. **fx DI 装配**：router 依赖加字段后，`bootstrap/router.go` 的 `routeParams` 结构与赋值必须同步，否则运行时 nil panic（单测/build 不报）
2. **huma wire 格式平铺**：DTO `Body *XxxReqBody` 请求方向会被解包；E2E 写 wrapped 格式会静默 422
3. **禁浮点入账**：金额一律 int64 微单位，换算用 `math.Round(v*scale)/scale`，禁 `int64(v*scale+0.5)`
4. **tzdata 内嵌**：入口 `import _ "time/tzdata"`，避免生产镜像缺 zoneinfo 导致非 UTC 时区规则构造失败
5. **demo 账号**：模块白名单与脱敏视角需覆盖新增查询（成本分布的用户维度）
6. **本地 E2E 环境**：连续 404 会触发 scanner 封 IP（`redis-cli DEL scanner:ban:127.0.0.1`）；管理接口限流 20 req/min，E2E 并行注意 429
7. **验证**：`make lint`（含 `lint conv`）+ `make test`；E2E 需先 `cp -r web/out internal/web/dist` 或放 embed 占位
