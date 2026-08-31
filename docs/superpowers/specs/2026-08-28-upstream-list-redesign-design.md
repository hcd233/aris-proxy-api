# Upstream MaaS 列表重设计：分组树表 + 平铺视图

- 日期：2026-08-28
- 状态：已确认（brainstorming 完成，待用户终审）
- 前置：`2026-08-27-upstream-maas-web-redesign-design.md`（endpoints+models 合并为单页 Upstream，已落地）
- 视觉形态确认：浏览器 mockup 两轮对比，会话记录见 `.superpowers/brainstorm/`

## 背景

上一轮重构把 endpoints/models 合并为 `/upstream` 单页，解决了"两页重复渲染同一批信息"的问题。但合并后的列表展示本身存在一批缺陷，且页面膨胀到 1359 行难以维护。

对 `web/src/app/(dashboard)/upstream/page.tsx` 的现状审查结论：

| # | 缺陷 | 证据 |
|---|---|---|
| 1 | **表头完全空白** | `<TableHead colSpan={9} className="bg-transparent" />` —— 9 列无一列名，用户靠猜 |
| 2 | 存在纯占位空列 | `<TableCell>{null}</TableCell>` |
| 3 | 端点连通信息在列表中不可见 | `openaiBaseURL`/`anthropicBaseURL`/`maskedAPIKey` 均在数据中，但组头只渲染名称+协议图标，要看 URL 必须打开编辑弹窗 |
| 4 | 三个相似字符串列零视觉区分 | `alias`（对外别名）/`modelId`（客户端 ID）/`upstreamModel`（上游真名）语义完全不同，却都是等宽小字并排 |
| 5 | 组头层级弱 | `bg-muted/60` 的 colSpan 行嵌在无表头的表里，"组"与"行"近乎同权重 |
| 6 | 停用模型无视觉降权 | `enabled:false` 的行与启用行外观一致 |
| 7 | 后端新字段未被消费 | `totalModelCount`（截断前口径）已在 DTO 中，UI 仍只显示一个干巴巴的"已截断" |
| 8 | 无折叠、无排序 | 端点模型多时无法收起；任何列都不能排序 |
| 9 | 桌面/移动两套 JSX 内联同一巨型文件 | 组头与模型行展示逻辑重复两遍，改一处要改两处 |

目标：重做列表展示，并补齐"模型编目"视角——用户在此页面的两类诉求（运维排障看端点连通配置 / 查复制模型别名）并重，需要可切换视角。

## 决策摘要

| 议题 | 结论 | 依据 |
|---|---|---|
| 核心场景 | 两者并重，默认按端点分组，可切「平铺」全部模型 | 用户选定 |
| 分组视图形态 | 带列名的缩进树表（mockup 方案 A） | 信息密度最高；表头终结"猜列"；树枝线让归属不依赖背景色 |
| 平铺视图数据源 | **新增后端模型分页接口**（真分页/真总数/SQL 级排序） | 用户明确选择完整语义而非前端派生 |
| 切换控件 | segmented control 与 FilterBar 同行，文案 **分组 / 平铺** | 不占额外垂直空间 |
| 能力范围 | 完整集：6 列排序 + 4 维筛选（含 capability） | 用户选定 |
| 切换状态 | 共享共有维度（keyword/username），各自保留特有维度，分页独立 | 用户选定 |

## 一、文件拆分

现状 1359 行单文件，加平铺视图必然突破 1800 行。按职责拆分：

```
web/src/app/(dashboard)/upstream/
  page.tsx              容器：视图切换 + 数据编排 + 弹窗挂载（目标 <300 行）
  grouped-view.tsx      分组树表（桌面）+ 移动卡片
  flat-view.tsx         平铺表（桌面）+ 移动卡片
  endpoint-dialog.tsx   端点新建/编辑弹窗
  model-dialog.tsx      模型新建/编辑弹窗 + TokenPresetPopover
  shared.tsx            OwnerCell / CapabilityBadges / SpecBadges / formatTokens
  use-upstream-list.ts  分组数据 hook
  use-model-list.ts     平铺数据 hook（含排序状态）
```

`shared.tsx` 消除当前桌面/移动两处重复的展示逻辑（缺陷 #9）。

## 二、后端新接口

### `GET /api/web/v1/model/list`（JWT 鉴权）

挂在既有 `modelGroup`（`internal/router/model.go`），已具备 JWT 中间件与 `modelManage` 限流。

**路径冲突已排除**：路由现为四分区架构，管理端在 `/api/web/v1/*`（JWT），客户端模型分发在 `/api/cli/v1/*`（API Key，`constant.ClientModelsListPath`）。两者不同前缀，物理隔离。前置 spec 中"删除 JWT 版 /model/list 以让路径给客户端"的约束在分区重构后已不适用。

请求参数：

| 参数 | 类型 | 说明 |
|---|---|---|
| `page` / `pageSize` | int | 复用 `model.PageParam`（必填，pageSize ≤ 500） |
| `query` | string | 关键词，命中 `alias` / `model_id` / `upstream_model` |
| `sort` / `sortField` | enum / string | 排序，见下方白名单 |
| `status` | string | `enabled` / `disabled`，空=全部 |
| `endpointID` | uint | 按端点精确过滤，0=不过滤 |
| `capability` | string | `text` / `image`，空=不过滤 |
| `username` | string | 仅 admin 生效（对齐 upstream 接口惯例） |

响应（每行嵌套 `endpoint` 与 `user`，沿用 API Keys 列表的嵌套模式）：

```jsonc
{
  "items": [
    {
      "id": 11, "alias": "deepseek-v3", "modelId": "dsv3",
      "upstreamModel": "deepseek-chat",
      "enabled": true, "contextLength": 128000, "maxOutputTokens": 8192,
      "capabilities": ["text"],
      "endpoint": { "id": 1, "name": "DeepSeek 官方" },
      "user": { "id": 3, "name": "centonhuang", "avatar": "https://…" },
      "createdAt": "2026-03-14T…", "updatedAt": "2026-03-14T…"
    }
  ],
  "pageInfo": { "page": 1, "pageSize": 20, "total": 247 }
}
```

### 安全约束（两条硬红线）

**1. 排序字段必须显式白名单。**

不得沿用 `util.SafeSortField` —— 它只校验字符集 `[a-zA-Z0-9_]`（`internal/util/string.go:77`），传 `api_key` 一样放行。白名单落 `internal/common/constant`：

```
alias / context_length / max_output_tokens / created_at / endpoint_id / enabled
```

非法值**回退默认排序**（`created_at desc`）而非报错——避免前端拼错参数导致整页 500。

**2. scope 用 `*uint` 三态，禁止零值兼作全量哨兵。**

- admin → `nil`（不过滤）
- 非 admin → 真实 userID
- `userID == 0` → 直接 401（`ierr.ErrUnauthorized`）

这是仓库既有约定（`internal/domain/llmproxy/repository.go:13-15` 注释明确禁止），也是 2026-08-25 那轮越权修复的核心教训：`len(scope) > 0` 式守卫会让认证缺失静默退化为全平台可见。

### 仓储层

新增 `ModelRepository.PaginateWithFilter(ctx, param, filter, scopeUserID *uint)`。

`capability` 筛选**不需要 JSON 谓词**：

```go
// capabilities 是 text 列 + serializer:json（internal/infrastructure/database/model/model.go:18），
// 不是 PG 原生 jsonb；且 enum.InputModalities 是封闭枚举（仅 text/image）。
// 因此 LIKE '%"image"%' 在 PG 与 sqlite 上语法一致、行为一致，带引号匹配无歧义。
db.Where(`capabilities LIKE ?`, `%"`+capability+`"%`)
```

代价是无法走索引。可接受：models 表规模千行量级（生产实测 tools 表约 1052 行，models 同量级），全表扫成本可忽略；这与既有 `messages.message` 用 text 存 JSON 是同一套权衡。

`endpoint` 嵌套字段用 `EndpointRepository.BatchFindByIDs` 批量回填（避免 N+1），`user` 用 `UserRepository.BatchFindByIDs`，`userID==0` 过滤在调用方做（对齐 `list_upstream.go:135` 的既有写法）。

demo 权限下 `upstreamModel` 走 `commonutil.MaskSecret`（与分组接口一致）。

### 前端类型契约（含死类型清理）

`web/src/lib/types.ts` 中残留四个上一轮删除旧接口后的**死类型**（经全量检索，除注释外零引用）：

| 死类型 | 处置 |
|---|---|
| `ModelItem`（扁平 `username` + 嵌套完整 `EndpointItem`） | 删除 |
| `ListModelsRsp`（字段名 `models`） | 删除 |
| `EndpointItem`（扁平 `username`） | 删除 |
| `ListEndpointsRsp` | 删除 |

**不得复用它们**：其扁平 `username` 口径与本页确立的嵌套 `user{id,name,avatar}` 惯例冲突，且 `ModelItem.endpoint` 嵌的是完整 `EndpointItem`（含 baseURL/Key），比新接口所需的 `{id,name}` 大得多。

新类型（列表项数组字段名统一为 `items`，对齐 API Keys 列表惯例）：

```ts
export interface ModelListEndpoint { id: number; name: string }

export interface ModelListItem {
  id: number; alias: string; modelId: string; upstreamModel: string;
  enabled: boolean; contextLength: number; maxOutputTokens: number;
  capabilities: ModelCapability[];
  endpoint?: ModelListEndpoint;   // 端点缺失时缺省
  user?: UpstreamUser;            // 复用既有嵌套用户类型
  createdAt: string; updatedAt: string;
}

export interface ListModelsPageRsp extends CommonRsp {
  items?: ModelListItem[];
  pageInfo?: PageInfo;
}
```

`api-client.ts` 新增 `api.listModelsPage(params)`（对象入参，对齐 faceted filter 迁移后的惯例）。

## 三、分组视图（方案 A 落地）

```
┌ 模型/端点 ─────────┬ 上游真名 ─┬ 规格 ─┬ 状态 ┬ 创建 ┬ 操作 ┐  ← 真表头
│▎▾ ◉ centonhuang  DeepSeek 官方  [Chat][Anthropic]  ⓘ连通详情   3 模型  ✎ 🗑 [+模型]
│  ├ deepseek-v3 ·id:dsv3   deepseek-chat      128K→8K   ●   03-14  ···
│  ├ deepseek-r1 ·id:dsr1   deepseek-reasoner   64K→32K  ●   03-14  ···
│  └ ~~deepseek-vl~~ 已停用  deepseek-vl2        32K→4K   ○   04-02  ···   ← opacity-45
│▎▾ ◉ lvlvko  Azure 东亚  [Resp]  ⓘ连通详情  [240 中显示 200]  ✎ 🗑 [+模型]
└──────────────────────────────────────────────────────────────┘
```

变更点逐条对应缺陷：

| 变更 | 修复 |
|---|---|
| 补真实表头，删除 `colSpan={9}` 空 `TableHead` 与 `<TableCell>{null}</TableCell>` | #1 #2 |
| 组头改白底 + 左侧 3px `--primary` 色条 | #5 |
| 组头新增 `ⓘ 连通详情` Popover：双 baseURL + Key 掩码 + 创建时间，可复制 | #3 |
| `alias` 加粗为主体、`modelId` 降级为同格内 `·id:` 后缀、`upstreamModel` 独立列用 mono + muted | #4 |
| 模型行左侧虚线树枝缩进 | #5 |
| 停用行 `opacity-45` + alias 删除线 | #6 |
| 截断徽标改消费 `totalModelCount`：`{total} 中显示 {shown}` | #7 |
| 组头可折叠（`usePersistentState` 记忆折叠的 endpointID 集合） | #8 |

「连通详情」放 Popover 而非直接铺在列表里，是因为 baseURL 长度不可控（`https://xxx.openai.azure.com/openai/deployments/...`），铺在行内必然截断；Popover 内可完整换行 + 一键复制。

## 四、平铺视图与切换

- **切换器**：segmented control「分组 / 平铺」，与 FilterBar 同行；当前视图存 `dashboard.upstream.view`
- **列头可点排序**，箭头指示当前排序列与方向
- **底部真分页 + 真总数**（来自新接口，非派生）
- 端点作为独立可排序列 → 免费获得"按端点聚类"能力

### 切换状态语义

用**两个 `useFilterBar` 实例**：`dashboard.upstream.grouped` 与 `dashboard.upstream.flat`。切换时单向同步 `keyword` 与 `username` 两个共有 token；平铺特有的 `status`/`endpoint`/`capability` 与排序状态存在自己的 key；两视图分页各自独立（分组第 3 页 ≠ 平铺第 3 页）。

**为何不用单实例 + 动态 facets**：`use-filter-bar.ts:146-150` 的 `serializeTokens` 对不在 facets 列表里的 token，因 `facetList.find(...)?.target !== "param"` 对 `undefined` 成立而**照样序列化进 `filter` 参数**。单实例方案下切回分组视图时，平铺遗留的 `status`/`capability` token 会被误发给分组接口。双实例天然隔离，无需修改共享 hook。

## 五、边界情况

| 场景 | 行为 |
|---|---|
| 平铺视图 `userID==0` | 401（非空列表），与分组接口一致 |
| `sortField` 传非白名单值（含 `api_key`） | 回退 `created_at desc`，不报错、不进 ORDER BY |
| `capability` 传未知值 | 视为不过滤（而非返回空），避免前端拼错导致空白页 |
| `endpointID` 指向他人端点（非 admin） | scope 过滤天然生效 → 空结果，不泄露存在性 |
| 端点已删但模型残留 | `endpoint` 嵌套字段缺省（omitempty），UI 显示 `—` |
| 归属用户软删 / `userID==0` | `user` 缺省，显示 `—`（沿用现状） |
| demo 权限 | `upstreamModel` 脱敏、删除按钮 locked（沿用现状） |
| 分组视图组内 0 模型 | 允许，组头计数显示 0（沿用现状） |
| 折叠状态下的组 | 仅隐藏模型行，组头计数与截断徽标仍显示 |

## 六、测试计划

### 后端

回归测试必须**能捕获缺陷** —— 写完后临时注入 buggy 版本确认 FAIL 再改回（仓库既有方法论，2026-08-19 教训）：

- **scope 三态**：admin `nil` 见全量 / 普通用户仅见自己 / `userID==0` → 401
- **排序白名单**：`sortField=api_key` 必须回退默认，断言 `api_key` 不出现在生成的 SQL 中
- **capability 筛选**：`capability=image` 仅返回含 image 的模型；`capability=` 与未知值均不过滤
- **endpoint/user 嵌套回填**：批量查询无 N+1；端点缺失时字段缺省
- **e2e**（`test/e2e/model_list/`）：走生产入口 `router.RegisterAPIRouter` 打真实路径，**不得自己拼 group 前缀**——这是 #165 路径脱节的直接教训（自拼路径的测试注入缺陷后仍 PASS）。预置可用凭据以区分 404（路由缺失）与 401（鉴权失败）。

### 前端

- vitest 覆盖排序参数映射、facet 同步纯函数
- `npm run lint` 须过 `truncate-requires-tooltip` 自定义规则（新增截断文案都要配 Tooltip）
- `npm run build` 通过
- 三主题（light / dark / moonshot）下核查组头色条、停用行降权、树枝线的对比度

### i18n

`locales/en|ja|zh.json` 同步新增：表头列名 ×7、分组/平铺切换、连通详情标题与字段名、截断新文案（含 `{total}`/`{shown}` 占位）、状态与能力筛选项、排序无障碍标签。

## 七、否决项记录

| 议题 | 否决方案 | 采用理由 |
|---|---|---|
| 分组视图形态 | B 端点卡片内嵌模型表 / C 详情藏悬浮层 | A 密度最高；B 展开后每端点吃约 200px 垂直空间，模型多时翻页累 |
| 平铺数据源 | 前端从分组数据派生 | 用户要真分页/真总数；派生方案分页语义是端点级，与"平铺"心智冲突 |
| 切换控件 | Tab 页签（多吃 40px，本项目列表页无先例）/ 右上角图标按钮（可发现性最差） | segmented control 省空间且语义直白 |
| 排序字段校验 | 复用 `util.SafeSortField` | 只校验字符集不校验列名，`api_key` 可注入 ORDER BY |
| capability 筛选实现 | SQL JSON 谓词（PG/sqlite 语法分叉） | text+serializer:json 存储 + 封闭枚举 → `LIKE '%"x"%"'` 跨库一致 |
| 切换状态 | 单 `useFilterBar` 实例 + 动态 facets | `serializeTokens` 会把未声明 facet 的 token 照样发出，导致跨视图参数污染 |
| 切换状态 | 完全隔离 / 不持久化 | 关键词共享符合"换个视角看同一批东西"的心智 |
