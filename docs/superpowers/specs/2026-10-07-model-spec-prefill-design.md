# 模型表单 models.dev 规格自动填充 + 定价折叠卡设计

> 状态：设计已确认（v2：自动触发替代显式按钮），待实施
> 日期：2026-10-07
> 前置：`2026-10-05-model-pricing-cost-design.md`（定价规则表 + prefill）、`2026-10-07-pricing-usd-only-design.md`（#187：启用计价开关 + 币种固定 USD，已上线）

## 背景

定价 prefill（876351d5）已支持从 models.dev 导入定价，#187 又把定价交互改为「启用计价」开关（币种固定 USD，导入按钮两态可点、命中后自动开计价）。仍存三个体验断点：

1. 导入入口是定价编辑器内部的显式按钮，只覆盖定价；上下文/最大输出/模态仍要手动填；
2. models.dev 同一条目里的 `limit.context`、`limit.output`、`modalities.input` **没有被利用**，上下文/输出靠静态预设手填，模态只有 text/image 两个开关（Claude 有 pdf、Gemini 有 video，信息丢失）；
3. 弹窗表单 + 定价规则编辑器全程平铺，弹窗很长。

本设计把导入升级为**模型表单级自动填充**（上下文/输出/模态/定价四件套），并把定价区改为折叠卡。

## 目标与非目标

### 目标

1. 填写上游模型名时**自动**调用 prefill 接口，将 models.dev 的上下文、最大输出、输入模态、定价（币种+分档规则）填入表单；不设显式按钮
2. 输入模态枚举扩展为 `text/image/pdf/video/audio`，与 models.dev `modalities.input` 一一对应
3. 定价区改折叠卡：默认一行摘要，展开为现有规则编辑器
4. 自动填充不得吞掉用户手动编辑过的字段；输入过程零 toast 打扰

### 非目标

- **输出模态建模**（`modalities.output` 当前无任何消费方，YAGNI）
- **模糊模型名匹配**（维持精确匹配、区分大小写，沿用 2026-10-05 决策 36）
- **自动改价/同步已保存数据**（仅填充弹窗表单，保存仍由用户显式触发）
- **别名/模型 ID 自动命名**（用户语义，不动）
- **定价规则编辑器重做**（876351d5 勾选式交互 + #187 计价开关语义均保持；另有进行中的星期按钮 UX 修复 WIP，见「实施注意事项」）
- **models.json/catalog.json 双数据源**（`api.json` 单文档已含 cost+limit+modalities）
- **静态预设档位移除**（未命中 models.dev 时仍是快捷输入，保留）

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| 1 | 导入交互 | 上游模型名输入防抖 ~600ms 自动调 prefill，无按钮 | 用户选定；录入即得，零额外点击 |
| 2 | 触发字段 | `upstream_model`（上游模型） | 与 models.dev 精确匹配的字段；「模型 ID」是对外暴露名可自定义，alias 同步还会带出噪声 |
| 3 | 覆盖策略 | 只填**本次弹窗中未手动编辑过**的字段（dirty 跟踪） | 自动行为不得吞掉手填值；新建/编辑弹窗同一套语义 |
| 4 | 反馈方式 | 全程无 toast，字段下方内联轻提示 | 输入过程中逐键触发查询，toast 会轰炸；内联提示随输入消失 |
| 5 | 模态范围 | 仅扩展输入模态五值，输出模态不建模 | 输入枚举与 models.dev 1:1；输出模态无消费方 |
| 6 | text 约束 | text 为必选模态，chips 中不可取消 | 与保存校验「必含 text」一致，消除一个错误路径 |
| 7 | 端点形态 | `GET /api/web/v1/model/spec/prefill` 替换 `GET /api/web/v1/model/pricing/prefill`（旧端点删除） | 旧端点唯一消费者（定价编辑器导入按钮）本次移除；web 内部 API 同仓同发布，无外部兼容负担 |
| 8 | 定价区呈现 | 折叠卡（默认折叠摘要行，自动填充定价后自动展开） | 弹窗降高；未计价用户看不到复杂编辑器；摘要「计价关闭」/「计价开启 · N 条规则 · 默认档 $1/$5」 |
| 9 | 部分命中 | 有啥填啥（found=true 且逐字段缺省） | api.json 条目字段可能缺失；逐字段按「有值 ∧ 未 dirty」写入 |

## 后端（Go）

### models.dev 客户端扩展（`internal/infrastructure/modelsdev/client.go`）

数据源不变：继续只拉 `api.json`（Redis 缓存 key `pricing:modelsdev:doc`、TTL 24h、16MB 上限均复用）——同一条模型条目内 `cost` 与 `limit`、`modalities` 并存。

- `modelsDevModel` 解析结构扩展：`Limit {Context, Output int64}`（`json:"limit"`）、`Modalities {Input []string}`（`json:"modalities"`）
- `Quote()` 改名 **`Describe()`**：一次返回规格 + 分档报价：

```go
// port.PricingQuoteProvider（internal/application/model/port/provider.go）同步改名
type ModelSpec struct {
    ContextLength    int64
    MaxOutputTokens  int64
    InputModalities  []enum.InputModality
    Quote            PricingQuote // 现有分档报价，逻辑不动
}
Describe(ctx context.Context, modelID string) (ModelSpec, bool, error)
```

- 模态映射：`modalities.input` 逐一映射到枚举（`text/image/pdf/video/audio`），**未知值静默丢弃**（models.dev 未来新增模态不致报错）；输出顺序固定为枚举序
- 匹配逻辑（`pickProvider` 官方 provider 优先）不动

### 枚举扩展（`internal/common/enum/input_modality.go`）

- `InputModalities` 增加 `pdf` / `video` / `audio` 三个值（类型与存储形态不变，`capabilities` 为 JSON 列，**无表结构改动**）
- 合法值同步点：`internal/dto/model.go` 的 `Capabilities` doc/校验、`ModelListReq.Capability` 的 `enum:"text,image"` 扩为五值

### 端点重定位

- `internal/router/model.go`：`GET /pricing/prefill` → **`GET /spec/prefill`**，OperationID `prefillModelSpec`，权限沿用 `PermissionUser`
- `internal/dto/model.go`：`ModelPricingPrefillReq/Rsp` 改名 `ModelSpecPrefillReq/Rsp`，响应扩为：

```json
{
  "found": true,
  "contextLength": 200000,
  "maxOutputTokens": 64000,
  "capabilities": ["text", "image", "pdf"],
  "pricing": { "currency": "USD", "rules": [ {"time_windows": [], "context_min": 0, "context_max": 200000, "input_price": 1, "output_price": 5, "cache_creation_price": 1.25, "cache_read_price": 0.1} ] }
}
```

  - `found=false` 时其余字段零值（沿用）；`found=true` 时逐字段缺省表示 models.dev 无该数据；`pricing` 缺省表示该条目无 cost；`currency` 恒为 `USD`（#187 后币种仅 USD/空）
- `internal/application/model/query/prefill_pricing.go` → `prefill_spec.go`：`tiersToRules` 分档→区间规则逻辑**原样保留**，`Handle` 填充扩展为同时输出规格三件套
- handler/端口/类型全链改名（`PrefillPricing*` → `PrefillSpec*`）

## 前端（web/）

### 自动填充（`model-dialog.tsx`）

- **触发**：`upstream_model` 输入 onChange 后防抖 600ms 调 `api.prefillModelSpec(name)`；同值短路（记录上次查询值，不重复请求）；输入为空不请求；**仅输入变化触发**（打开/重开弹窗不主动查询，dirty 标记随弹窗打开重置）
- **竞态守卫**：请求发出时捕获 name，响应回来时若表单 `upstreamModel` 已不等于该 name 则丢弃结果（快速输入不被旧响应覆盖）
- **dirty 跟踪**：`contextLength` / `maxOutputTokens` / `capabilities` / `pricing` 四组字段的「本弹窗内手动编辑过」标记；对应输入框 onChange、模态 chip 点击、`PricingEditor.onChange` 时置位。自动回填**只写未 dirty 的字段**（逐字段判断，支持部分命中）；`alias` / `modelId` / `upstreamModel` 永不回填
- **内联提示**（`upstream_model` 字段下方，替换现有 hint 行位置）：
  - 命中且发生回填 → 「已从 models.dev 填充」（muted 小字）
  - 未命中/上游不可达 → 「models.dev 未命中，可手动填写」
  - `upstreamModel` 再次变化 → 提示清空回到 idle，直到新响应回来
  - **零 toast**（与显式按钮版的关键差异）
- **定价联动**：定价字段被自动回填时自动进入已计价（等价于 #187 的「命中后自动开计价」：`{ currency: "USD", rules }` 非空即开），且定价折叠卡自动展开供核对

### 模态区改 chips（`model-dialog.tsx` + `shared.tsx`）

- 两个 Switch 改为五个多选 chips：`text / image / pdf / video / audio`
- text chip 常亮且**不可取消**（决策 6）；其余点击切换
- `ModelForm` 的 `supportText` / `supportImage` 两个布尔改为 `capabilities: ModelCapability[]`（与 DTO 同形），保存映射逻辑简化为直传
- `ModelCapability` type 扩为五值；`CapabilityBadges`（`shared.tsx`）图标映射扩展：text→Type、image→ImageIcon、pdf→FileText、video→Video、audio→AudioLines（lucide-react）
- 列表筛选器（`page.tsx:148` `capability` filter）选项扩为五值，i18n label 同步

### 定价折叠卡（`pricing-editor.tsx`）

- 组件外层加折叠卡：默认**折叠**为一行摘要，点击展开/收起
- 摘要行内容（术语对齐 #187）：
  - 未计价（`currency == ""`）→ 「计价关闭」
  - 已计价 → 「计价开启 · {N} 条规则 · 默认档 {input}/{output}」（金额复用 `web/src/lib/money.ts` 既有格式化：去尾零、小数位上限 6）
- 展开后为现有 `PricingEditor` 内容（**「启用计价」开关 + `USD / 1M tokens` 单位提示 + 规则列表 + 勾选式时段/区间 + 增删排序**），**仅移除内部「从 models.dev 导入」按钮**（入口统一为自动触发）
- 计价开关语义沿用 #187：关 = `{ currency: "", rules: [] }`（清空定价）；开且规则为空时自动补一条无条件默认规则

### 类型与 i18n

- `web/src/lib/types.ts` DTO 镜像同步（prefill 响应、ModelCapability）
- `en.json` / `ja.json` / `zh.json` 三语同步（内联提示、chips 标签、折叠摘要、筛选项）

## 错误处理

- 未命中/上游不可达：内联轻提示，表单不动（不阻塞录入）
- 前端接口报错/网络失败：按「未命中」处理（同一内联提示），**不抛全局错误弹窗**（自动触发路径静默降级）
- 自动填充永不自动改价：只写弹窗表单态，保存仍显式
- 竞态：旧响应丢弃（决策见上）
- 保存校验不变：capabilities 必含 text（text 不可取消后该校验成为纯兜底）
- 手动编辑后再次自动触发：dirty 字段不被覆盖（决策 3）

## 测试

### Go 单测

- `modelsdev.Describe`：limit/modalities 解析、未知模态丢弃、价格标量 string/number 两形态、分档归并（沿用现有用例改造）、条目缺字段的零值行为
- prefill 用例（`test/unit/prefill_pricing/` → `prefill_spec/`）：规格+定价同时返回、部分命中、未命中、trim、缓存命中
- enum 五值校验、DTO 校验合法值扩展

### E2E

- `spec/prefill` 响应断言（mock models.dev）：四件套字段 + found=false 降级（沿用既有 prefill 用例改造）

### Web

- `npm run lint && npm run build`（含 prettier --check）
- `next-dev-loop` 运行时验证：输入上游模型名防抖后字段回填、内联提示出现/消失、手填字段不被覆盖、模态 chips 切换与 text 锁定、定价折叠卡自动展开与手动收起、筛选器五值

## 文档维护

- `CONTEXT.md`：**InputModality** 词条扩五值；**PricingPrefill** 词条改写为 **SpecPrefill**（规格导入：上下文/输出/模态/定价自动填充、dirty 语义、无自动改价）
- `web/CONTEXT.md` 同步前端术语（自动填充、dirty、折叠卡摘要）

## 实施注意事项

0. **并行 WIP（星期按钮 UX 修复）**：工作区有一份未提交改动（星期按钮 outline 样式/「每天」徽标/lastDay 禁点 + locale 键，来自 `git pull --autostash` 冲突恢复），落地本设计时**必须基于其合并后结果**（定价折叠卡包住含星期按钮修复的编辑器），不得回退该 WIP；实施前先确认它已提交或随本特性一并提交
1. **fx DI 装配**：改名涉及 `bootstrap/router.go` 的 `routeParams` 字段与赋值同步，否则运行时 nil panic（单测/build 不报）
2. **huma wire 平铺**：请求 `query:"upstreamModel"` 参数名不变
3. **竞态守卫先例**：上游 CR 修复（2026-08-29）有序号守卫模式可参考；前端用「响应时比对当前值」即可，不必引入请求序号
4. **web prettier 进 pre-commit**：`web/` 改动需 `prettier --check` 通过；git pathspec `web/*.ts` 不匹配嵌套路径
5. **改名清理**：`PrefillPricing*` 旧符号全链删除，不留兼容别名（内部 API）；`web/src/lib/api-client` 的 `prefillModelPricing` 同步改名
6. **验证**：`make lint` + `make test`；web `lint && build`
