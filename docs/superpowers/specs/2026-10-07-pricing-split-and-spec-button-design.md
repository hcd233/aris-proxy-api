# 模型定价拆分 + 规格/定价显式导入按钮设计

> 状态：已实施（2026-10-07）
> 取代：`2026-10-07-model-spec-prefill-design.md` 的「自动触发 + 定价折叠卡」部分（规格接口与模态五值等其余内容仍有效）

## 背景

上一版把 models.dev 导入做成「上游模型名防抖 600ms 自动触发」，四件套（上下文/输出/模态/定价）写进同一个模型弹窗，定价区是弹窗内的折叠卡。三个问题：

1. 自动触发对用户不可见也不可控——查询时机由输入节奏决定，命中后静默改字段，只能靠一行内联提示察觉；
2. 定价与规格耦合在同一个弹窗：定价有独立语义（计费口径、分档、时段），却只能从「编辑模型」进去，列表上也看不到一个模型值多少钱；
3. 弹窗过长，定价折叠卡的摘要行承担了本应由列表承担的展示职责。

## 目标与非目标

### 目标

1. 模型弹窗加「从 models.dev 获取规格」按钮：显式点击，填上下文 / 最大输出 / 输入模态三件套
2. 定价拆为与「编辑模型配置」平级的操作：列表操作列新增定价按钮，打开独立定价弹窗，内含「从 models.dev 获取定价」按钮
3. 模型列表新增定价列，展示代表档单价，悬停展开全部档位

### 非目标

- 后端接口改动（`GET /model/spec/prefill` 与 `PATCH /model` 的 `pricing` 语义已足够）
- 新建模型时配置定价（定价弹窗依赖已保存的模型 ID；新建后到列表再配置）
- 输出模态、模糊匹配、自动改价、静态预设档位移除（沿用上一版决策）

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| 1 | 触发方式 | 显式按钮取代防抖自动触发 | 用户可控：点一下才覆盖，不会在输入过程中悄悄改字段 |
| 2 | dirty 保护 | 删除 | 显式点击即「我要覆盖」，保留 dirty 会让按钮行为取决于历史编辑，反直觉 |
| 3 | 规格覆盖范围 | 上下文 / 最大输出 / 输入模态；缺 `text` 时补上 | text 是保存校验的必选模态，不补会把表单推进保存失败态 |
| 4 | 定价的所属层级 | 列表操作列独立按钮 + 独立弹窗 | 定价是模型的一个属性维度而非「编辑」的子步骤；列表能直接看到价 |
| 5 | 定价保存负载 | `PATCH /model` 只发 `pricing` | 其余字段缺省 = 不修改，避免用行内旧快照覆盖并发的规格编辑 |
| 6 | 新建时的定价 | 不在创建弹窗内配置，创建后再配 | 定价弹窗按模型 ID 保存；创建弹窗塞回定价会让「平级按钮」的拆分失效 |
| 7 | 列表定价列取值 | 代表档 = 无条件默认规则（无则首条规则），多档显示 `+N` | 列宽只能承担一对数字；分档结构放 Tooltip |
| 8 | 两处导入的失败反馈 | 内联提示（「未命中或无数据，可手动填写」），零 toast | 与上一版一致；显式点击下提示紧邻按钮，不会漏看 |
| 9 | 导入按钮图标 | `Download`（沿用被移除的旧导入按钮） | 沿用仓库内既有「从 models.dev 导入」视觉语言 |
| 10 | 折叠卡 | 删除（摘要改为列表列） | 摘要行与列表列职责重复；弹窗本身就是容器，不再套一层壳 |

## 前端（web/）

### 模型弹窗（`model-dialog.tsx`）

- 删除：防抖 effect、`dirty`/`dirtyRef`/`formRef`/`lastQueried`、`PricingEditor` 引入与 `pricing` 表单字段
- 新增：`upstream_model` 字段下方一行「从 models.dev 获取规格」按钮（`variant="outline" size="sm"`，`Download` 图标，请求中换 `Loader2` spinner），禁用条件 = 上游真名为空或请求进行中
- 命中：覆盖 `contextLength` / `maxOutputTokens`（响应该字段为 0 时保留原值）/ `capabilities`（缺 text 补 text）；未命中、无数据、接口报错统一内联提示
- 提示绑定触发时的上游真名（`autofillHint.name === form.upstreamModel.trim()` 才显示），改名即失效——沿用上一版免清理模式
- `ModelForm` 去掉 `pricing` 字段；`emptyModelForm` 同步

### 定价弹窗（`pricing-dialog.tsx`，新增）

- 与「编辑模型配置」平级：列表操作列新增 `Coins` 图标按钮（aria-label = 定价）
- 正文 = `PricingEditor`（去掉折叠壳与摘要）+ 顶部「从 models.dev 获取定价」按钮
- 导入命中（`found && pricing.rules.length > 0`）→ 整体替换规则表为 `{currency:"USD", rules}`；未命中/无 cost/报错 → 内联提示，表单不动
- 保存 → `PATCH /model?id=` 只带 `pricing`；成功 toast + `refreshAll()`（两条数据链路同步刷新）
- `DialogContent` 加 `max-h-[85dvh] overflow-y-auto`：规则条数多时正文可滚，避免超出视口
- 弹窗按 `key={open-id}` 重挂载重置导入提示

### 列表（`grouped-view.tsx` / `flat-view.tsx` / `shared.tsx`）

- 新增「定价」列（`upstream.col_pricing`）：分组视图插在能力与状态之间（组头 colSpan 8→9），平铺视图同样位置（不参与排序——分档规则无法映射到单个 SQL 列）
- 单元格 `PricingInline`：未计价 `—`；已计价 `$1 / $5` + 多档 `+N`，Tooltip 列出「档位 × 输入/输出」（表头 `档位` / `输入 / 输出` + 单位行）
- `ModelActionsCell` 增加定价按钮（编辑 / 定价 / 删除三个平级操作）；三个视图分支（分组桌面、分组移动、平铺桌面、平铺移动）全部同步，demo 只读账户同锁
- 移动端在同一行徽标区追加 `PricingInline`（与 `SpecBadges` / `CapabilityBadges` 同排）
- `isDefaultRule` 从 `pricing-editor.tsx` 提到 `shared.tsx`（定价列与编辑器共用同一判定）

### i18n

新增 10 键 × 3 语（`models.spec_fetch`、`upstream.col_pricing`、`upstream.pricing.dialog_desc/fetch/fetch_filled/fetch_miss/saved/save_error/cell_tier/cell_inout`）；删除随折叠卡失效的 3 键（`upstream.pricing.summary_off/summary_on/summary_default`）。

## 错误处理

- 导入失败（未命中 / 无定价数据 / 接口报错）：内联提示，表单不动，不弹全局错误
- 保存失败：`showErrorToast`，弹窗保持打开（用户可重试）
- 并发：定价保存只发 `pricing`，不覆盖他人对规格字段的改动
- 保存校验不变：capabilities 必含 text（规格导入自动补齐后成为纯兜底）

## 测试

- `npm run lint`、`npm run test`（50 passed）、`npm run format:check`、`npm run build`
- 运行时验证（弹窗交互、列展示）未执行：按用户约定不擅自起本地 dev server
