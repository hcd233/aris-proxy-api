# 模型表单 models.dev 规格自动填充 + 定价折叠卡 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 模型弹窗在填入上游模型名后自动从 models.dev 填充上下文/最大输出/输入模态/定价四件套（无按钮、无 toast、不吞手填值），输入模态扩为五值，定价区改折叠卡。

**Architecture:** 后端把 `api.json` 单文档解析从「仅定价」扩为「规格+定价」（`Quote`→`Describe`），prefill 端点重定位为 `GET /model/spec/prefill` 并返回规格字段；前端 `model-dialog` 以 600ms 防抖自动调用该端点、按 dirty 集合选择性回填，`pricing-editor` 外层加受控折叠卡。数据模型无改动（`capabilities` 为 JSON 列）。

**Tech Stack:** Go (huma v2 / fx / GORM / sonic)、Next.js + React (shadcn 风格组件)、Redis 缓存 models.dev 文档 24h。

**Spec:** `docs/superpowers/specs/2026-10-07-model-spec-prefill-design.md`（已获用户批准）

## Global Constraints

- 命令一律 `rtk` 前缀（`rtk go test` / `rtk git commit` / `rtk npm run lint`）；Go 测试用 `make test`（全量）或 `go test -count=1 -run <Name> ./test/unit/<topic>/`（聚焦）
- 写 Go 代码前先跑 `use-modern-go` 的 `list` 并加载 `golang-samber-lo` / `golang-samber-mo` skill；项目硬约束（`docs/agents/go-backend.md`）优先
- models.dev 模型 ID **精确匹配、区分大小写**；未命中/拉取失败一律 `found=false`，**永不阻塞录入、永不自动改价**
- 自动填充路径**零 toast**：只用字段下方内联提示；接口报错按「未命中」处理
- dirty 语义：`contextLength` / `maxOutputTokens` / `capabilities` / `pricing` 四组字段，本弹窗内手动编辑过的**不被自动回填覆盖**
- 币种恒 USD（#187 语义）：已计价 ⇔ `currency:"USD"` 且 rules 非空；`text` 模态必含（UI 锁定 + 保存校验兜底）
- wire 层金额为展示单位浮点；query 参数 camelCase（`upstreamModel`、`contextLength`）
- i18n 三语同步：`web/src/locales/en.json` / `ja.json` / `zh.json`，插值用 `{n}` 占位 + `String.prototype.replace`
- fx DI：接口/构造器改名必须同步 `internal/bootstrap/modules/application.go` 与 `handler.go`（漏改是运行时 nil panic，单测/build 不报）
- web 改动验证三件套：`cd web && npm run lint && npm run build && npm run format:check`（prettier 在 pre-commit 里，格式不净提交会被拦）
- 响应 DTO 字段命名 camelCase：`contextLength` / `maxOutputTokens` / `capabilities` / `pricing`（spec 中的 snake_case 示例以此为准，Task 8 顺带修正 spec 示例）
- 每个 Task 独立提交；提交信息用中文 conventional 前缀（`feat(web):` / `refactor(pricing):` / `test:` / `docs:`）

---

## 文件结构

| 文件 | 职责 | 变化 |
|---|---|---|
| `internal/infrastructure/modelsdev/client.go` | models.dev 文档拉取 + 解析（规格+定价） | Modify（Task 1） |
| `internal/application/model/port/provider.go` | 规格来源端口 `ModelSpecProvider` | Modify（Task 1） |
| `internal/common/enum/input_modality.go` | 输入模态枚举五值 | Modify（Task 2） |
| `internal/application/model/query/prefill_spec.go` | 规格导入用例（原 `prefill_pricing.go`） | Rename+Modify（Task 3） |
| `internal/dto/model.go` | `ModelSpecPrefillReq/Rsp` + capability 校验值 | Modify（Task 2/3） |
| `internal/handler/model.go` / `internal/router/model.go` | `HandlePrefillSpec` + `/model/spec/prefill` 路由 | Modify（Task 3） |
| `internal/bootstrap/modules/{application,handler}.go` | fx 装配改名同步 | Modify（Task 3） |
| `test/unit/{modelsdev,prefill_spec,input_modality}/` | Go 单测 | Modify/Rename/Create（Task 1/2/3） |
| `test/e2e/model_pricing/spec_prefill_test.go` | 端点 E2E（env-gated） | Create（Task 3） |
| `web/src/lib/types.ts` / `web/src/lib/api-client.ts` | DTO 镜像 + `prefillModelSpec` | Modify（Task 4） |
| `web/src/app/(dashboard)/upstream/shared.tsx` | `ModelForm.capabilities`、`MODEL_CAPABILITIES`、徽标图标 | Modify（Task 5） |
| `web/src/app/(dashboard)/upstream/model-dialog.tsx` | 模态 chips、自动填充、内联提示、折叠卡状态 | Modify（Task 5/6/7） |
| `web/src/app/(dashboard)/upstream/pricing-editor.tsx` | 移除导入按钮、外层折叠卡 | Modify（Task 6/7） |
| `web/src/app/(dashboard)/upstream/page.tsx` | 保存/加载映射、筛选器五值 | Modify（Task 5） |
| `web/src/locales/{en,ja,zh}.json` | 三语文案 | Modify（Task 5/6/7） |
| `CONTEXT.md` / `web/CONTEXT.md` | 领域词汇同步 | Modify（Task 8） |

---

### Task 0: 基线收口——提交并行 WIP（星期按钮 UX + 定价导入上下文边界）

**⚠️ 协调前提**：这些改动由**并行会话**产出（`git pull --autostash` 冲突恢复 + 上下文边界特性），可能仍在编辑。**开工前与用户确认工作区已移交**；执行中若 `git status` 出现本任务清单之外的新改动，停下来问用户，不要提交别人的半成品。

**Files:**
- Modify（既有 WIP，全部纳入提交）: `web/src/app/(dashboard)/upstream/pricing-editor.tsx`、`web/src/app/(dashboard)/upstream/model-dialog.tsx`、`web/src/lib/api-client.ts`、`web/src/locales/en.json`、`web/src/locales/ja.json`、`web/src/locales/zh.json`、`internal/application/model/port/handler.go`、`internal/application/model/query/prefill_pricing.go`、`internal/dto/model.go`、`internal/handler/model.go`、`test/unit/prefill_pricing/prefill_pricing_test.go`

**Interfaces:**
- Consumes: 无（基线收口）
- Produces: 可提交的干净基线。后续 Task 的前提事实：`PricingEditor` props 为 `{value, onChange, upstreamModel, contextLength}`；`api.prefillModelPricing(upstreamModel, contextLength?)`；`tiersToRules(tiers, contextLength)` 末档边界语义（多档且 `contextLength > 末档起点` 时末档 `[min, contextLength)` + 同价默认兜底规则）；`ModelPricingPrefillReq` 带 `contextLength` query 参数。**这些语义本特性全部保留**，仅在 Task 3/6 做改名与入口迁移。

- [ ] **Step 1: 核对工作区改动清单**

Run: `git status --short`
Expected: 仅上列 11 个文件（+ 可能存在的 `docs/` 提交痕迹与未跟踪 `web/src/app/harness/`，均不理会）。若出现其他已跟踪文件的改动 → 停下问用户。

- [ ] **Step 2: 验证基线自洽**

```bash
go test -count=1 ./test/unit/prefill_pricing/ ./test/unit/modelsdev/ ./test/unit/pricing/ && make lint
cd web && npm run lint && npm run build
```
Expected: 测试全绿、lint 无错误、build 成功（此时 `pricing-editor.tsx` 的 import 冲突已被并行会话解决，`Badge`/`cn`/类型导入与正文一致）。

- [ ] **Step 3: 提交基线（一个 commit，双特性）**

```bash
rtk git add "web/src/app/(dashboard)/upstream/pricing-editor.tsx" "web/src/app/(dashboard)/upstream/model-dialog.tsx" web/src/lib/api-client.ts web/src/locales/en.json web/src/locales/ja.json web/src/locales/zh.json internal/application/model/port/handler.go internal/application/model/query/prefill_pricing.go internal/dto/model.go internal/handler/model.go test/unit/prefill_pricing/prefill_pricing_test.go
rtk git commit -m "feat(pricing): 定价导入末档按上下文长度边界 + 星期按钮组 UX（并行 WIP 收口）"
```
Expected: pre-commit 钩子通过，工作区干净（仅剩未跟踪 harness 目录）。

---

### Task 1: models.dev Describe 扩展（规格解析 + 端口改名）

**Files:**
- Modify: `internal/application/model/port/provider.go`
- Modify: `internal/infrastructure/modelsdev/client.go`
- Modify: `internal/application/model/query/prefill_pricing.go`（仅改调用点 `Quote`→`Describe`，改名在 Task 3）
- Modify: fx 绑定 `PricingQuoteProvider` 的位置（用 `rtk grep -rn "PricingQuoteProvider" internal/` 定位，通常在 `internal/bootstrap/modules/`）
- Test: `test/unit/modelsdev/modelsdev_test.go`

**Interfaces:**
- Consumes: Task 0 基线
- Produces:
  - `port.ModelSpec{ContextLength int64; MaxOutputTokens int64; InputModalities []enum.InputModality; Quote PricingQuote}`
  - `port.ModelSpecProvider` 接口：`Describe(ctx context.Context, modelID string) (ModelSpec, bool, error)`（原 `PricingQuoteProvider.Quote`）
  - `mapModalities` 语义：models.dev `modalities.input` 过滤到枚举五值、**未知值丢弃**、输出按 `enum.InputModalities` 顺序

- [ ] **Step 1: 写失败测试（新用例 + 旧用例改名）**

在 `test/unit/modelsdev/modelsdev_test.go` 中将现有 `TestQuote*` 全部改名为 `TestDescribe*`（内部 `c.Quote(` 调用改 `c.Describe(`），并追加：

```go
func TestDescribeLimitAndModalities(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"anthropic":{"models":{"claude-sonnet-4-5":{
			"limit":{"context":200000,"output":64000},
			"modalities":{"input":["text","image","pdf","hologram"],"output":["text"]},
			"cost":{"input":1,"output":5}}}}}`))
	}))
	t.Cleanup(srv.Close)
	c := modelsdev.NewClient(srv.Client(), nil)
	c.Source = srv.URL
	spec, ok, err := c.Describe(t.Context(), "claude-sonnet-4-5")
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	if spec.ContextLength != 200000 || spec.MaxOutputTokens != 64000 {
		t.Fatalf("limit = %+v", spec)
	}
	// 未知模态 hologram 静默丢弃；输出按枚举序 text/image/pdf
	want := []enum.InputModality{enum.InputModalityText, enum.InputModalityImage, enum.InputModalityPDF}
	if !slices.Equal(spec.InputModalities, want) {
		t.Fatalf("modalities = %v", spec.InputModalities)
	}
	if spec.Quote.Tiers[0].Input != 1 {
		t.Fatalf("quote lost: %+v", spec.Quote)
	}
}

func TestDescribeMissingLimitAndModalities(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"openai":{"models":{"m":{"cost":{"input":3}}}}}`))
	}))
	t.Cleanup(srv.Close)
	c := modelsdev.NewClient(srv.Client(), nil)
	c.Source = srv.URL
	spec, ok, err := c.Describe(t.Context(), "m")
	if err != nil || !ok {
		t.Fatalf("describe: ok=%v err=%v", ok, err)
	}
	if spec.ContextLength != 0 || spec.MaxOutputTokens != 0 || len(spec.InputModalities) != 0 {
		t.Fatalf("missing fields must be zero: %+v", spec)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/modelsdev/`
Expected: FAIL（编译错误：`Describe` / `ModelSpec` 未定义）

- [ ] **Step 3: 实现规格解析**

`internal/application/model/port/provider.go` 整体替换为：

```go
package port

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// PricingTier 公开定价的一档（USD/1M tokens）：ContextMin 起（含）的 prompt 量适用本档
type PricingTier struct {
	ContextMin    int64
	Input         float64
	Output        float64
	CacheCreation float64
	CacheRead     float64
}

// PricingQuote 公开定价（USD/1M tokens）：按上下文区间分档，Tiers 按 ContextMin 升序且首档为 0
type PricingQuote struct {
	Tiers []PricingTier
}

// ModelSpec models.dev 公开模型规格 + 分档报价
type ModelSpec struct {
	ContextLength   int64                // 上下文窗口（tokens），无数据为 0
	MaxOutputTokens int64                // 最大输出（tokens），无数据为 0
	InputModalities []enum.InputModality // 输入模态（枚举序，未知值已丢弃），无数据为 nil
	Quote           PricingQuote         // 分档报价
}

// ModelSpecProvider 公开模型规格来源（由 infrastructure/modelsdev 实现）
type ModelSpecProvider interface {
	// Describe 按模型 ID 精确匹配查询；未命中 ok=false；拉取失败返回 err
	Describe(ctx context.Context, modelID string) (ModelSpec, bool, error)
}
```

`internal/infrastructure/modelsdev/client.go`：`modelsDevModel` 扩展两个字段并新增结构体：

```go
type modelsDevModel struct {
	Cost       modelsDevCost       `json:"cost"`
	Limit      modelsDevLimit      `json:"limit"`
	Modalities modelsDevModalities `json:"modalities"`
}

// modelsDevLimit 模型规格上限（tokens）
type modelsDevLimit struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output"`
}

// modelsDevModalities 模态集合（仅 input 对本项目有消费方）
type modelsDevModalities struct {
	Input []string `json:"input"`
}
```

`Quote` 方法改名 `Describe`，返回值换 `port.ModelSpec`，`buildQuote` 调用结果填入 `spec.Quote`，并在构造 spec 时填：

```go
spec := port.ModelSpec{
	ContextLength:   entry.Limit.Context,
	MaxOutputTokens: entry.Limit.Output,
	InputModalities: mapModalities(entry.Modalities.Input),
}
```

文件底部新增（`slices`/`enum` 已在 import 中或按需补）：

```go
// mapModalities 把 models.dev 输入模态映射为枚举值：未知值静默丢弃，输出按枚举序
func mapModalities(input []string) []enum.InputModality {
	out := make([]enum.InputModality, 0, len(input))
	for _, m := range enum.InputModalities {
		if slices.Contains(input, string(m)) {
			out = append(out, m)
		}
	}
	return out
}
```

同步改名所有 `PricingQuoteProvider` 引用（`rtk grep -rn "PricingQuoteProvider\|\.Quote(" internal/ test/`）为 `ModelSpecProvider` / `.Describe(`，包括 `internal/application/model/query/prefill_pricing.go` 的 `h.provider.Describe(ctx, name)` 与 `test/unit/prefill_pricing/prefill_pricing_test.go` 的 fake（方法 `Quote` 改名 `Describe`，字段不动）。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test -count=1 ./test/unit/modelsdev/ ./test/unit/prefill_pricing/ && make lint
```
Expected: PASS（prefill 用例行为不变，仅 fake 接口改名）

- [ ] **Step 5: Commit**

```bash
rtk git add internal/application/model/port/provider.go internal/infrastructure/modelsdev/client.go internal/application/model/query/prefill_pricing.go internal/bootstrap/modules test/unit/modelsdev/modelsdev_test.go test/unit/prefill_pricing/prefill_pricing_test.go
rtk git commit -m "refactor(pricing): models.dev 客户端 Quote→Describe，扩展 limit/modalities 规格解析"
```

---

### Task 2: InputModality 枚举扩五值

**Files:**
- Modify: `internal/common/enum/input_modality.go`
- Modify: `internal/dto/model.go`（合法值标签）
- Test: `test/unit/input_modality/input_modality_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 的 `mapModalities`（依赖枚举序）
- Produces: `enum.InputModalityPDF/Video/Audio` 三个常量；`enum.InputModalities = [text, image, pdf, video, audio]`（**枚举序即规范输出序**，Task 1/3 依赖）；DTO `capabilities`/`capability` 合法值扩为五值

- [ ] **Step 1: 写失败测试**

`test/unit/input_modality/input_modality_test.go`：

```go
// Package input_modality 输入模态枚举契约测试
package input_modality

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// TestInputModalitiesOrder 枚举序即规范输出序（models.dev 模态映射与前端 chips 均按此序）
func TestInputModalitiesOrder(t *testing.T) {
	t.Parallel()
	want := []enum.InputModality{"text", "image", "pdf", "video", "audio"}
	if !slices.Equal(enum.InputModalities, want) {
		t.Fatalf("InputModalities = %v", enum.InputModalities)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/input_modality/`
Expected: FAIL（`enum.InputModalities` 长度 2 ≠ 5）

- [ ] **Step 3: 扩枚举 + DTO 标签**

`internal/common/enum/input_modality.go` 在 `InputModalityImage` 后追加三个常量（注释风格对齐现有）：

```go
	// InputModalityPDF PDF 文件输入
	InputModalityPDF InputModality = "pdf"

	// InputModalityVideo 视频输入
	InputModalityVideo InputModality = "video"

	// InputModalityAudio 音频输入
	InputModalityAudio InputModality = "audio"
```

并把 `InputModalities` 换为：

```go
// InputModalities 全部合法输入模态（枚举序即规范输出序）
var InputModalities = []InputModality{InputModalityText, InputModalityImage, InputModalityPDF, InputModalityVideo, InputModalityAudio}
```

`internal/dto/model.go` 三处合法值同步（`rtk grep -n "text,image" internal/dto/model.go` 定位）：

- 创建/更新请求的 `Capabilities` doc 字符串：`合法值 text/image` → `合法值 text/image/pdf/video/audio`
- `ModelListReq.Capability`：`enum:"text,image"` → `enum:"text,image,pdf,video,audio"`（doc 同步）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test -count=1 ./test/unit/input_modality/ && make lint`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
rtk git add internal/common/enum/input_modality.go internal/dto/model.go test/unit/input_modality
rtk git commit -m "feat(model): 输入模态枚举扩为 text/image/pdf/video/audio 五值"
```

---

### Task 3: prefill 用例改 PrefillSpec + 端点重定位 /model/spec/prefill

**Files:**
- Rename+Modify: `internal/application/model/query/prefill_pricing.go` → `internal/application/model/query/prefill_spec.go`
- Modify: `internal/application/model/port/handler.go`（Query/Result/Handler 改名扩字段）
- Modify: `internal/dto/model.go`（`ModelSpecPrefillReq/Rsp`）
- Modify: `internal/handler/model.go`（`HandlePrefillSpec`）
- Modify: `internal/router/model.go`（路由）
- Modify: `internal/bootstrap/modules/application.go`、`internal/bootstrap/modules/handler.go`（fx 装配）
- Rename+Modify: `test/unit/prefill_pricing/` → `test/unit/prefill_spec/`
- Create: `test/e2e/model_pricing/spec_prefill_test.go`
- Modify: `docs/superpowers/specs/2026-10-07-model-spec-prefill-design.md`（响应示例字段名改 camelCase，见 Step 6）

**Interfaces:**
- Consumes: Task 1 `port.ModelSpec` / `ModelSpecProvider.Describe`；Task 2 枚举序
- Produces:
  - `port.PrefillSpecQuery{UpstreamModel string; ContextLength int64}`、`port.PrefillSpecResult{Found bool; ContextLength int64; MaxOutputTokens int64; InputModalities []enum.InputModality; Currency enum.Currency; Rules []dto.PricingRuleDTO}`、`port.PrefillSpecHandler`
  - `query.NewPrefillSpecHandler(provider port.ModelSpecProvider) port.PrefillSpecHandler`
  - `dto.ModelSpecPrefillReq{UpstreamModel string; ContextLength int64}` / `dto.ModelSpecPrefillRsp{Found bool; ContextLength int64; MaxOutputTokens int64; Capabilities []enum.InputModality; Pricing *PricingDTO}`（+ `CommonRsp`）
  - 路由 `GET /api/web/v1/model/spec/prefill`，OperationID `prefillModelSpec`
  - 供 Task 4 消费的 wire 形态：`{"found":true,"contextLength":200000,"maxOutputTokens":64000,"capabilities":["text","image","pdf"],"pricing":{"currency":"USD","rules":[...]}}`

- [ ] **Step 1: 写失败测试（用例集搬迁 + 新用例 + e2e）**

`test/unit/prefill_pricing/` 整目录改名 `test/unit/prefill_spec/`，包名 `prefill_pricing`→`prefill_spec`，`fakeQuoteProvider`→`fakeSpecProvider`（`Describe` 返回 `port.ModelSpec`，原 `quote` 字段移到 `spec.Quote`），`NewPrefillPricingHandler`→`NewPrefillSpecHandler`，`port.PrefillPricingQuery`→`port.PrefillSpecQuery`（`ContextLength` 字段语义不变）。现有 5 个行为用例（分档默认/末档边界/不越界/单档/降级）**断言一行不改**，仅适配构造。追加：

```go
func TestPrefillSpecFieldsAndModalities(t *testing.T) {
	t.Parallel()
	h := query.NewPrefillSpecHandler(&fakeSpecProvider{ok: true, spec: port.ModelSpec{
		ContextLength:   200000,
		MaxOutputTokens: 64000,
		InputModalities: []enum.InputModality{enum.InputModalityText, enum.InputModalityPDF},
		Quote:           port.PricingQuote{Tiers: []port.PricingTier{{ContextMin: 0, Input: 1, Output: 5}}},
	}})
	rsp, err := h.Handle(t.Context(), port.PrefillSpecQuery{UpstreamModel: "claude-sonnet-4-5"})
	if err != nil || !rsp.Found {
		t.Fatalf("found case: %+v err=%v", rsp, err)
	}
	if rsp.ContextLength != 200000 || rsp.MaxOutputTokens != 64000 {
		t.Fatalf("spec fields = %+v", rsp)
	}
	if len(rsp.InputModalities) != 2 || rsp.InputModalities[1] != enum.InputModalityPDF {
		t.Fatalf("modalities = %v", rsp.InputModalities)
	}
	if rsp.Currency != enum.CurrencyUSD || len(rsp.Rules) != 1 {
		t.Fatalf("pricing = %+v", rsp)
	}
}
```

新建 `test/e2e/model_pricing/spec_prefill_test.go`（复用同包 `mustEnv` / `getJSON` / `newE2EClient`）：

```go
// TestSpecPrefill 模型规格导入：命中返回规格+定价、未命中 found=false（需 BASE_URL/WEB_JWT；离线 skip）
func TestSpecPrefill(t *testing.T) {
	t.Parallel()
	env := mustEnv(t, "BASE_URL", "WEB_JWT")
	data := getJSON(t, env["BASE_URL"], env["WEB_JWT"],
		"/api/web/v1/model/spec/prefill?upstreamModel=claude-sonnet-4-5")
	d, ok := data["data"].(map[string]any)
	if !ok {
		t.Fatalf("prefill data missing: %v", data)
	}
	if d["found"] != true {
		t.Fatalf("claude-sonnet-4-5 must be found: %v", d)
	}
	if _, ok := d["contextLength"].(float64); !ok {
		t.Fatalf("contextLength missing: %v", d)
	}
	miss := getJSON(t, env["BASE_URL"], env["WEB_JWT"],
		"/api/web/v1/model/spec/prefill?upstreamModel=__no_such_model__")
	dm, _ := miss["data"].(map[string]any)
	if dm["found"] != false {
		t.Fatalf("unknown model must miss: %v", dm)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test -count=1 ./test/unit/prefill_spec/`
Expected: FAIL（编译错误：`PrefillSpec*` 未定义）

- [ ] **Step 3: 实现端口/用例/DTO/路由/装配**

`internal/application/model/port/handler.go` 的 prefill 三件套整体替换为（`Rules` 保留 `[]dto.PricingRuleDTO` 的既有形态）：

```go
// PrefillSpecQuery 模型规格导入查询
type PrefillSpecQuery struct {
	UpstreamModel string
	ContextLength int64 // 表单上下文长度；大于末档起点时末档上界取该值
}

// PrefillSpecResult 模型规格导入结果（found=false 时其余字段为零值；币种恒 USD）。
// Rules 为按上下文区间分档的定价规则，时段窗口 models.dev 无数据需手填。
type PrefillSpecResult struct {
	Found           bool
	ContextLength   int64
	MaxOutputTokens int64
	InputModalities []enum.InputModality
	Currency        enum.Currency
	Rules           []dto.PricingRuleDTO
}

// PrefillSpecHandler 模型规格导入处理器
type PrefillSpecHandler interface {
	Handle(ctx context.Context, q PrefillSpecQuery) (*PrefillSpecResult, error)
}
```

`prefill_pricing.go` 改名 `prefill_spec.go`，类型/构造器改名 `prefillSpecHandler` / `NewPrefillSpecHandler`（provider 字段类型换 `port.ModelSpecProvider`），`Handle` 主体替换为：

```go
	spec, ok, err := h.provider.Describe(ctx, name)
	// 拉取失败/未命中统一降级为未命中（录入不被阻塞），因此只走正向分支
	if err == nil && ok {
		return &port.PrefillSpecResult{
			Found:           true,
			ContextLength:   spec.ContextLength,
			MaxOutputTokens: spec.MaxOutputTokens,
			InputModalities: spec.InputModalities,
			Currency:        enum.CurrencyUSD,
			Rules:           tiersToRules(spec.Quote.Tiers, q.ContextLength),
		}, nil
	}
	return &port.PrefillSpecResult{}, nil
```

`tiersToRules` 函数（含末档边界语义）**原样保留**，仅随文件搬迁；空模型名短路分支保留。

`internal/dto/model.go`：`ModelPricingPrefillReq/Rsp` 改名替换为：

```go
// ModelSpecPrefillReq 模型规格导入请求（表单自动填充用）
type ModelSpecPrefillReq struct {
	UpstreamModel string `query:"upstreamModel" required:"true" maxLength:"200" doc:"上游模型名（与 models.dev 模型 ID 精确匹配）"`
	ContextLength int64  `query:"contextLength,omitempty" minimum:"0" doc:"模型上下文长度（tokens）；大于末档起点时末档上界取该值"`
}

// ModelSpecPrefillRsp 模型规格导入响应（found=false 表示未命中/上游不可达，其余字段零值）。
// 仅用于填充表单，永不自动改价；pricing 为完整定价（含上下文区间规则），币种恒 USD。
type ModelSpecPrefillRsp struct {
	CommonRsp
	Found           bool                 `json:"found" doc:"是否命中 models.dev"`
	ContextLength   int64                `json:"contextLength,omitempty" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int64                `json:"maxOutputTokens,omitempty" doc:"最大输出（tokens）"`
	Capabilities    []enum.InputModality `json:"capabilities,omitempty" doc:"输入模态（枚举序）"`
	Pricing         *PricingDTO          `json:"pricing,omitempty" doc:"导入的定价（含上下文区间规则）"`
}
```

`internal/handler/model.go`：接口方法与实现改名 `HandlePrefillSpec`，注释改为「models.dev 模型规格导入（仅填充表单，未命中/失败降级为 found=false）」，主体：

```go
func (h *modelHandler) HandlePrefillSpec(ctx context.Context, req *dto.ModelSpecPrefillReq) (*dto.HTTPResponse[*dto.ModelSpecPrefillRsp], error) {
	rsp := &dto.ModelSpecPrefillRsp{}
	res, err := h.prefill.Handle(ctx, port.PrefillSpecQuery{UpstreamModel: req.UpstreamModel, ContextLength: req.ContextLength})
	// 未命中/失败统一降级为 found=false（录入不被阻塞），因此只走正向分支
	if err == nil && res != nil && res.Found {
		rsp.Found = true
		rsp.ContextLength = res.ContextLength
		rsp.MaxOutputTokens = res.MaxOutputTokens
		rsp.Capabilities = res.InputModalities
		rsp.Pricing = &dto.PricingDTO{Currency: res.Currency, Rules: res.Rules}
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
```

（`ModelDependencies.Prefill` 与 `modelHandler.prefill` 字段类型换 `port.PrefillSpecHandler`，字段名不动。）

`internal/router/model.go`：注册块替换为 `OperationID: "prefillModelSpec"`、`Path: "/spec/prefill"`、`Summary: "PrefillModelSpec"`、`Description: "Fetch model spec and public pricing from models.dev (form auto-fill only)"`、中间件键 `"prefillModelSpec"`、handler `modelHandler.HandlePrefillSpec`。

`internal/bootstrap/modules/application.go`：`NewPrefillPricingHandler`→`NewPrefillSpecHandler(provider modelport.ModelSpecProvider) modelport.PrefillSpecHandler`（返回 `modelquery.NewPrefillSpecHandler(provider)`），`fx.Provide` 列表同步；`internal/bootstrap/modules/handler.go` 的 `NewModelDependencies` 参数类型换 `modelport.PrefillSpecHandler`。最后 `rtk grep -rn "PrefillPricing\|ModelPricingPrefill" internal/ test/` 确认零残留。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test -count=1 ./test/unit/prefill_spec/ ./test/unit/modelsdev/ ./test/unit/pricing/ && make lint && make test
```
Expected: 全绿（e2e 离线 skip 不计入）

- [ ] **Step 5: Commit（后端）**

```bash
rtk git add -A internal/ test/unit/
rtk git commit -m "refactor(model): prefill 端点重定位 /model/spec/prefill，响应扩展规格四件套"
```

- [ ] **Step 6: 修正 spec 响应示例命名并提交**

`docs/superpowers/specs/2026-10-07-model-spec-prefill-design.md` 的响应 JSON 示例中 `"context_length"`→`"contextLength"`、`"max_output_tokens"`→`"maxOutputTokens"`（对齐本计划的 camelCase 契约），随后：

```bash
rtk git add docs/superpowers/specs/2026-10-07-model-spec-prefill-design.md
rtk git commit -m "docs: spec 响应示例字段名对齐 camelCase 契约"
```

---

### Task 4: web 类型镜像 + api-client 改名

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/api-client.ts`

**Interfaces:**
- Consumes: Task 3 wire 形态
- Produces: `ModelCapability = "text" | "image" | "pdf" | "video" | "audio"`；`ModelSpecPrefillRsp`；`api.prefillModelSpec(upstreamModel: string, contextLength?: number): Promise<ModelSpecPrefillRsp>`（供 Task 6 调用）

- [ ] **Step 1: types.ts 替换**

`ModelCapability`（约 451 行）替换为：

```ts
export type ModelCapability = "text" | "image" | "pdf" | "video" | "audio";
```

`ModelPricingPrefillRsp`（约 484-489 行，含 JSDoc）整体替换为：

```ts
/**
 * 模型规格导入响应（found=false 表示未命中/上游不可达）。
 * pricing 为完整定价（含上下文区间规则），时段窗口需手填。 */
export interface ModelSpecPrefillRsp {
  found: boolean;
  contextLength?: number;
  maxOutputTokens?: number;
  capabilities?: ModelCapability[];
  pricing?: PricingDTO;
}
```

- [ ] **Step 2: api-client.ts 替换方法**

`prefillModelPricing` 方法（约 634-646 行）整体替换为：

```ts
  /** 模型规格导入：按上游模型名查 models.dev（自动填充用，未命中 found=false） */
  async prefillModelSpec(
    upstreamModel: string,
    contextLength?: number,
  ): Promise<ModelSpecPrefillRsp> {
    const cl =
      contextLength && contextLength > 0 ? `&contextLength=${Math.floor(contextLength)}` : "";
    return this.request<ModelSpecPrefillRsp>(
      `${API_PREFIX}/model/spec/prefill?upstreamModel=${encodeURIComponent(upstreamModel)}${cl}`,
    );
  }
```

同步改 import：`ModelPricingPrefillRsp`→`ModelSpecPrefillRsp`。`pricing-editor.tsx` 对旧方法的调用在 Task 6 移除——本 Task 结束时 `npm run build` 会失败是预期中间态，**验证以 `npm run lint` 通过为准**（lint 允许未使用/不匹配？——不允许，见 Step 3：本 Task 必须同时把 `pricing-editor.tsx` 的调用行改名保持编译通过）。

- [ ] **Step 3: 保持编译的最小过渡改名**

`web/src/app/(dashboard)/upstream/pricing-editor.tsx` 中 `api.prefillModelPricing(` → `api.prefillModelSpec(`（仅一行调用改名，导入按钮本体 Task 6 才删）。然后：

Run: `cd web && npm run lint && npm run build`
Expected: 全绿

- [ ] **Step 4: Commit**

```bash
rtk git add web/src/lib/types.ts web/src/lib/api-client.ts "web/src/app/(dashboard)/upstream/pricing-editor.tsx"
rtk git commit -m "feat(web): 类型镜像与 api-client 对齐 model/spec/prefill 契约"
```

---

### Task 5: 模态 chips + ModelForm 重构 + 筛选/徽标

**Files:**
- Modify: `web/src/app/(dashboard)/upstream/shared.tsx`
- Modify: `web/src/app/(dashboard)/upstream/model-dialog.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`
- Modify: `web/src/locales/en.json`、`ja.json`、`zh.json`

**Interfaces:**
- Consumes: Task 4 `ModelCapability` 五值
- Produces: `shared.tsx` 导出 `MODEL_CAPABILITIES: ModelCapability[]`（枚举序）；`ModelForm.capabilities: ModelCapability[]`（替代 `supportText`/`supportImage`）；`emptyModelForm.capabilities = ["text"]`。Task 6 依赖 `ModelForm` 新形态。

- [ ] **Step 1: shared.tsx 改造**

1. `ModelForm` 接口：删 `supportText` / `supportImage` 两行，加 `capabilities: ModelCapability[];`（保留 `pricing` JSDoc）。import 补 `type ModelCapability`。
2. `emptyModelForm`：删两布尔，加 `capabilities: ["text"],`。
3. 新增导出常量（放 `formatTokens` 附近）：

```ts
// 输入模态全集（枚举序，与后端 enum.InputModalities 一致）
export const MODEL_CAPABILITIES: ModelCapability[] = ["text", "image", "pdf", "video", "audio"];
```

4. `CapabilityBadges` 图标映射扩展：import 增 `FileText, Video, AudioLines`（lucide-react）与 `type LucideIcon`；徽标渲染处把 `cap === "image" ? <ImageIcon .../> : <Type .../>` 替换为查表：

```tsx
const CAPABILITY_ICONS: Record<string, LucideIcon> = {
  text: Type,
  image: ImageIcon,
  pdf: FileText,
  video: Video,
  audio: AudioLines,
};
```

渲染：`const Icon = CAPABILITY_ICONS[cap] ?? Type;` 再 `<Icon className="size-3 text-muted-foreground" />`（徽标外壳/tooltip 不动）。

- [ ] **Step 2: model-dialog.tsx chips 替换开关**

删除「能力」区两个 Switch 卡片（`supportText`/`supportImage` 相关 JSX），换成 chips 组；import 增 `FileText, Video, AudioLines`、`type LucideIcon`、`cn`、`MODEL_CAPABILITIES`：

```tsx
const CAPABILITY_CHIPS: { value: ModelCapability; labelKey: string; Icon: LucideIcon }[] = [
  { value: "text", labelKey: "models.capability_text", Icon: Type },
  { value: "image", labelKey: "models.capability_image", Icon: ImageIcon },
  { value: "pdf", labelKey: "models.capability_pdf", Icon: FileText },
  { value: "video", labelKey: "models.capability_video", Icon: Video },
  { value: "audio", labelKey: "models.capability_audio", Icon: AudioLines },
];
```

```tsx
<div className="space-y-1">
  <Label>{t("models.capabilities")}</Label>
  <div className="flex flex-wrap gap-1.5">
    {CAPABILITY_CHIPS.map(({ value, labelKey, Icon }) => {
      const checked = form.capabilities.includes(value);
      // text 为必选模态（保存校验的 UI 呈现），不可取消
      const locked = value === "text";
      return (
        <Button
          key={value}
          type="button"
          size="sm"
          variant="outline"
          aria-pressed={checked}
          disabled={locked}
          className={cn(
            checked &&
              "border-primary/40 bg-primary/10 text-primary hover:bg-primary/15",
          )}
          onClick={() =>
            setForm((f) => ({
              ...f,
              capabilities: checked
                ? f.capabilities.filter((c) => c !== value)
                : [...f.capabilities, value],
            }))
          }
        >
          <Icon className="size-3.5" />
          {t(labelKey)}
        </Button>
      );
    })}
  </div>
</div>
```

- [ ] **Step 3: page.tsx 保存/加载/筛选映射**

1. 保存校验（原 `!modelForm.supportText` 块）换：

```ts
    if (!modelForm.capabilities.includes("text")) {
      toast.error(t("models.capabilities_require_text"));
      return;
    }
```

2. `capabilities` 构造（原三行 spread）换为按枚举序归一：

```ts
    const capabilities = MODEL_CAPABILITIES.filter((c) => modelForm.capabilities.includes(c));
```

3. 两处表单装载（`handleFlatEdit` 与平铺/分组编辑入口，搜 `supportText:`）换：

```ts
      capabilities: m.capabilities?.length ? [...m.capabilities] : ["text"],
```

4. 筛选器（约 148 行）换五值 + 动态标签：

```ts
      {
        key: "capability",
        label: t("upstream.filter_capability"),
        options: ["text", "image", "pdf", "video", "audio"],
        formatValue: (v) => t(`models.capability_${v}`),
        target: "param",
        single: true,
      },
```

5. `rtk grep -rn "supportText\|supportImage" web/src` 清零残留（含 `__tests__` 里的表单夹具同步改 `capabilities: ["text"]`）。

- [ ] **Step 4: i18n 三语**

`en.json` / `ja.json` / `zh.json` 各加 3 键（紧邻 `models.capability_image`）：

| key | zh | en | ja |
|---|---|---|---|
| `models.capability_pdf` | PDF 输入 | PDF input | PDF入力 |
| `models.capability_video` | 视频输入 | Video input | 動画入力 |
| `models.capability_audio` | 音频输入 | Audio input | 音声入力 |

- [ ] **Step 5: 验证 + Commit**

```bash
cd web && npm run lint && npm run build && npm run format:check
```
Expected: 全绿

```bash
rtk git add web/src "web/src/app/(dashboard)/upstream" web/src/locales
rtk git commit -m "feat(web): 模型表单模态改五值 chips，ModelForm 与筛选/徽标同步"
```

---

### Task 6: 自动填充（防抖/dirty/内联提示/竞态）+ 移除导入按钮

**Files:**
- Modify: `web/src/app/(dashboard)/upstream/model-dialog.tsx`
- Modify: `web/src/app/(dashboard)/upstream/pricing-editor.tsx`（仅删导入按钮与 props）
- Modify: `web/src/locales/en.json`、`ja.json`、`zh.json`（删 3 键、加 2 键）

**Interfaces:**
- Consumes: Task 4 `api.prefillModelSpec`；Task 5 `ModelForm.capabilities`
- Produces: `markDirty(field: "contextLength" | "maxOutputTokens" | "capabilities" | "pricing")`（model-dialog 内部）；内联提示态 `autofillHint: "" | "filled" | "miss"`。Task 7 依赖 `dirtyRef` / `formRef` / 自动填充 effect 的结构（在其内加一行自动展开）。

- [ ] **Step 1: model-dialog.tsx 自动填充逻辑**

组件顶部 import 调整：`useState`→`useEffect, useRef, useState`，加 `api`、`type ModelSpecPrefillRsp` 不需要（直接内联推断）。组件体内（`const t = useT();` 之后）加入：

```tsx
  const [dirty, setDirty] = useState<ReadonlySet<string>>(new Set());
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;
  const [autofillHint, setAutofillHint] = useState<"" | "filled" | "miss">("");
  const formRef = useRef(form);
  formRef.current = form;
  const lastQueried = useRef("");

  const markDirty = (field: string) => setDirty((d) => new Set(d).add(field));

  // 弹窗打开重置 dirty 与提示（新建/编辑同一语义：仅本弹窗内手改过的字段受保护）
  useEffect(() => {
    if (open) {
      setDirty(new Set());
      setAutofillHint("");
      lastQueried.current = "";
    }
  }, [open]);

  // 上游模型名防抖 600ms 自动填充：只填未手改字段；同值短路；旧响应丢弃；零 toast
  useEffect(() => {
    const name = form.upstreamModel.trim();
    setAutofillHint("");
    if (!name || name === lastQueried.current) return;
    let cancelled = false;
    const timer = setTimeout(async () => {
      lastQueried.current = name;
      try {
        const rsp = await api.prefillModelSpec(name, formRef.current.contextLength || undefined);
        if (cancelled || formRef.current.upstreamModel.trim() !== name) return; // 竞态守卫
        if (!rsp.found) {
          setAutofillHint("miss");
          return;
        }
        setForm((f) => {
          const next = { ...f };
          const d = dirtyRef.current;
          if (!d.has("contextLength") && rsp.contextLength) next.contextLength = rsp.contextLength;
          if (!d.has("maxOutputTokens") && rsp.maxOutputTokens)
            next.maxOutputTokens = rsp.maxOutputTokens;
          if (!d.has("capabilities") && rsp.capabilities?.length)
            next.capabilities = rsp.capabilities;
          if (!d.has("pricing") && rsp.pricing?.rules?.length)
            next.pricing = { currency: "USD", rules: rsp.pricing.rules };
          return next;
        });
        setAutofillHint("filled");
      } catch {
        // 自动触发路径静默降级：接口报错按未命中处理
        if (!cancelled && formRef.current.upstreamModel.trim() === name) setAutofillHint("miss");
      }
    }, 600);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [form.upstreamModel, setForm]);
```

内联提示渲染在「上游模型」输入框 hint 位置（`models.model_id_hint` 下方同款式小字）：

```tsx
  {autofillHint === "filled" && (
    <p className="text-[11px] text-muted-foreground">{t("models.autofill_filled")}</p>
  )}
  {autofillHint === "miss" && (
    <p className="text-[11px] text-muted-foreground">{t("models.autofill_miss")}</p>
  )}
```

四组 dirty 标记接线（在对应 `setForm` 调用前加一行 `markDirty(...)`）：

- `contextLength` 输入框 `onChange` → `markDirty("contextLength")`
- `maxOutputTokens` 输入框 `onChange` → `markDirty("maxOutputTokens")`
- 模态 chips `onClick`（Task 5 代码块内）→ `markDirty("capabilities")`
- `PricingEditor` 的 `onChange` 回调 → `markDirty("pricing")`

- [ ] **Step 2: pricing-editor.tsx 移除导入按钮**

1. 删 `importing` state、`handlePrefill` 函数、导入 `<Button>` 块（含 `Download` 图标按钮与所在 flex 行中的条件渲染）。
2. 删 props `upstreamModel` / `contextLength`（接口与解构同步），组件 JSDoc 中「从 models.dev 导入」行改为「导入由模型表单自动触发（models.dev 规格自动填充）」。
3. 删随之无用的 import：`toast`、`api`、`Download`（`rtk grep -n "toast\|api\.\|Download" pricing-editor.tsx` 确认零残留再删）。
4. `model-dialog.tsx` 的 `<PricingEditor ...>` 调用点删 `upstreamModel={...}` 与 `contextLength={...}` 两个 prop。

- [ ] **Step 3: i18n 三语**

删除 3 键：`upstream.pricing.prefill`、`upstream.pricing.prefill.miss`、`upstream.pricing.prefill.hint`。新增 2 键（放 `models.model_id_hint` 附近）：

| key | zh | en | ja |
|---|---|---|---|
| `models.autofill_filled` | 已从 models.dev 填充规格，可手动调整 | Filled from models.dev — adjust as needed | models.dev から仕様を入力しました（自由に調整できます） |
| `models.autofill_miss` | models.dev 未命中，可手动填写 | Not found on models.dev — fill in manually | models.dev で見つかりませんでした。手動で入力してください |

- [ ] **Step 4: 验证 + Commit**

```bash
cd web && npm run lint && npm run build && npm run format:check
rtk grep -rn "prefillModelPricing\|upstream.pricing.prefill" web/src   # 期望零输出
```

```bash
rtk git add web/src "web/src/app/(dashboard)/upstream" web/src/locales
rtk git commit -m "feat(web): 上游模型名防抖自动填充规格与定价（dirty 保护 + 内联提示 + 竞态守卫）"
```

---

### Task 7: 定价折叠卡（受控展开 + 摘要行 + 自动展开联动）

**Files:**
- Modify: `web/src/app/(dashboard)/upstream/pricing-editor.tsx`
- Modify: `web/src/app/(dashboard)/upstream/model-dialog.tsx`
- Modify: `web/src/locales/en.json`、`ja.json`、`zh.json`

**Interfaces:**
- Consumes: Task 6 自动填充 effect 结构（`dirtyRef`、setForm 块）
- Produces: `PricingEditor` props `{value, onChange, open, onOpenChange}`；摘要行文案键 `upstream.pricing.summary_off/summary_on/summary_default`

- [ ] **Step 1: pricing-editor.tsx 折叠卡外壳**

Props 接口替换为：

```ts
export interface PricingEditorProps {
  value?: PricingDTO;
  onChange: (p: PricingDTO) => void;
  /** 折叠卡展开态（受控；自动填充定价后由上层置 true） */
  open: boolean;
  onOpenChange: (v: boolean) => void;
}
```

组件签名加 `open, onOpenChange` 解构；import 增 `Coins, ChevronDown, ChevronUp`（lucide-react）与 `formatCost`（`@/lib/money`）。组件 return 外层替换为折叠卡（原内容整体挪进 `{open && ...}` 的 `div`，缩进一层，**内部 JSX 一行不改**）：

```tsx
  const rules = value?.rules ?? [];
  const priced = rules.length > 0;
  const defaultRule = rules.find(isDefaultRule) ?? rules[0];
  const summary = priced
    ? `${t("upstream.pricing.summary_on").replace("{n}", String(rules.length))}${
        defaultRule
          ? ` · ${t("upstream.pricing.summary_default")} ${formatCost(defaultRule.input_price, "USD")}/${formatCost(defaultRule.output_price, "USD")}`
          : ""
      }`
    : t("upstream.pricing.summary_off");

  return (
    <div className="rounded-lg border">
      <button
        type="button"
        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm"
        aria-expanded={open}
        onClick={() => onOpenChange(!open)}
      >
        <Coins className="size-4 shrink-0 text-muted-foreground" />
        <span className="font-medium">{t("upstream.pricing.title")}</span>
        <span className="truncate text-xs text-muted-foreground">{summary}</span>
        {open ? (
          <ChevronUp className="ml-auto size-4 shrink-0" />
        ) : (
          <ChevronDown className="ml-auto size-4 shrink-0" />
        )}
      </button>
      {open && <div className="space-y-3 border-t px-3 py-2.5">{/* 原有内容：计价开关行 + 规则列表 + 添加按钮 */}</div>}
    </div>
  );
```

注意：`rules` 变量原组件内已有声明，避免重复声明（合并即可）；`value?.rules ?? []` 的取值口径保持一致。

- [ ] **Step 2: model-dialog.tsx 状态与联动**

1. 组件体内加 `const [pricingOpen, setPricingOpen] = useState(false);`，并在 Task 6 的「弹窗打开重置」effect 内追加 `setPricingOpen(false);`。
2. 自动填充 effect 中，`setForm((f) => {...})` **之前**加：

```tsx
        const willFillPricing = !dirtyRef.current.has("pricing") && !!rsp.pricing?.rules?.length;
```

`setAutofillHint("filled");` **之后**加：

```tsx
        if (willFillPricing) setPricingOpen(true); // 定价自动回填后展开折叠卡供核对
```

3. `<PricingEditor ...>` 调用点换受控 props：`open={pricingOpen}`、`onOpenChange={setPricingOpen}`；删除弹窗外层的 `Coins` 图标 + `Label`（`upstream.pricing.title`）标题行（标题移入折叠卡头部），`Coins` import 若无他用则删除。

- [ ] **Step 3: i18n 三语**

| key | zh | en | ja |
|---|---|---|---|
| `upstream.pricing.summary_off` | 计价关闭 | Pricing off | 課金オフ |
| `upstream.pricing.summary_on` | 计价开启 · {n} 条规则 | Pricing on · {n} rules | 課金オン · {n} ルール |
| `upstream.pricing.summary_default` | 默认档 | default tier | デフォルト档位 |

（`{n}` 由代码 `.replace("{n}", ...)` 注入，三语同键。）

- [ ] **Step 4: 验证 + Commit**

```bash
cd web && npm run lint && npm run build && npm run format:check
```

```bash
rtk git add web/src "web/src/app/(dashboard)/upstream" web/src/locales
rtk git commit -m "feat(web): 定价区改折叠卡——摘要行 + 受控展开 + 自动填充后自动展开"
```

---

### Task 8: 领域文档同步 + 全量验证

**Files:**
- Modify: `CONTEXT.md`
- Modify: `web/CONTEXT.md`

**Interfaces:**
- Consumes: Task 1-7 全部落地
- Produces: 词汇表与代码一致；全量验证证据

- [ ] **Step 1: CONTEXT.md 词条同步**

1. **InputModality** 词条：合法值改为 `text/image/pdf/video/audio` 五值，注明「枚举序即规范输出序；models.dev 未知模态静默丢弃」。
2. **PricingPrefill（定价导入辅助）** 词条改写为 **SpecPrefill（模型规格导入）**：`GET /model/spec/prefill` 按上游模型名精确匹配 models.dev，返回上下文/最大输出/输入模态/定价四件套；前端**自动触发**（上游模型名防抖 600ms）、只填本弹窗未手动编辑的字段（dirty 语义）、内联提示、永不自动改价。
3. Model 词条补充：输入模态五值、text 必含。

- [ ] **Step 2: web/CONTEXT.md 同步**

补前端术语：**自动填充**（防抖触发/dirty 保护/竞态守卫/内联提示）、**定价折叠卡**（摘要行「计价关闭 / 计价开启 · N 条规则 · 默认档 $x/$y」、受控展开）、模态 chips（text 锁定）。

- [ ] **Step 3: 全量验证**

```bash
make lint && make test
cd web && npm run lint && npm run build && npm run format:check
```
Expected: 全绿。随后按 `next-dev-loop` skill 做运行时验证，逐项确认：

1. 新建模型：输入上游模型名（如 `claude-sonnet-4-5`）停顿 1s → 上下文/最大输出/模态/定价自动回填，字段下方出现「已从 models.dev 填充」
2. 手改上下文为 128000 后换一个模型名 → 上下文保持 128000，其余字段刷新
3. 输入不存在的模型名 → 「models.dev 未命中」内联提示，无 toast
4. 模态 chips：text 不可取消，pdf/video/audio 可切换
5. 定价折叠卡：默认折叠显示摘要；自动填充定价后自动展开；手动收起/展开正常；计价开关关 = 摘要变「计价关闭」
6. 列表 capability 筛选出现五值

- [ ] **Step 4: 提交文档**

```bash
rtk git add CONTEXT.md web/CONTEXT.md
rtk git commit -m "docs: 领域词汇同步——SpecPrefill/模态五值/自动填充与折叠卡术语"
```

---

## 自检记录（writing-plans Self-Review）

1. **Spec 覆盖**：自动填充四件套（Task 3 响应 + Task 6 填充）✓；模态五值（Task 2/5）✓；定价折叠卡（Task 7）✓；端点重定位与旧端点删除（Task 3）✓；dirty/竞态/零 toast（Task 6）✓；text 锁定（Task 5）✓；CONTEXT 同步（Task 8）✓；测试三层（Go 单测 Task 1-3、e2e Task 3、web 运行时 Task 8）✓。
2. **占位符**：无 TBD/TODO；所有代码块给出可落盘内容；「grep 定位」类步骤均已给出精确命令与目标。
3. **类型一致性**：`ModelSpecProvider.Describe` / `port.ModelSpec` / `PrefillSpecQuery` / `PrefillSpecResult` / `ModelSpecPrefillRsp` / `prefillModelSpec` / `ModelForm.capabilities` / `MODEL_CAPABILITIES` 全链命名一致；`tiersToRules(tiers, contextLength)` 边界语义在 Task 0 声明、Task 3 原样保留。
