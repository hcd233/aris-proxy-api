# Upstream MaaS 单页重构设计（endpoints + models 合并）

- 日期：2026-08-27
- 状态：已确认（brainstorming 完成，待用户终审）
- 前置：#162「model/endpoint 配置用户级多租户化」已合入 master（0e850d09）
- 视觉形态确认：浏览器 mockup 对比（方案 A 表格内分组行），会话记录见 `.superpowers/brainstorm/`

## 背景与目标

#162 之后 endpoints/models 已下放为全体用户的个人配置，但 Web 端仍是两个割裂的平铺列表：

1. 两页重复渲染同一批信息（models 页逐行重复 endpoint 名）；
2. 归属用户只有扁平 `username` 字段、无头像，且前端表格根本没渲染；
3. 与 apikeys 页（70d5d3f8）确立的嵌套 `user{id,name,avatar}` 展示惯例不一致。

目标：将 endpoints/models 合并为单个 **Upstream** 页面（导航名 "Upstream MaaS"），按 endpoint 分组浏览配置；增删改交互收敛到同一页面；接口层随之收敛为单一读路径，并统一为嵌套 user 对象。

## 信息架构变更

- 侧边栏删除 `Endpoints`、`Models` 两项，新增单项 **Upstream**；新路由 `/upstream`，删除 `web/src/app/(dashboard)/endpoints/`、`models/` 两个目录。
- 导出弹窗 ×4（ExportDialog / ExportClaudecodeDialog / ExportCodexDialog / ExportPiDialog）与 `TraceInstallPopover` 迁至新页。
- 权限模块标识由 `module="endpoints"` / `module="models"` 统一为 `module="upstream"`：
  - `PermissionGuard` 的 `module` 取值与 `isModuleOpen()` 白名单（`web/src/app/(dashboard)/page.tsx` 的 dashboard 统计探测）；
  - 后端 demo 模块白名单（`DemoModuleAccessor` 相关枚举）同步排查，保持 demo 账户现有 locked 模式（删除按钮锁定、脱敏逻辑不变）。

## 接口变更

### 新增：`GET /api/v1/upstream/list`（JWT 鉴权，Web 管理端分组视图）

请求沿用 `model.CommonParam`（`page`/`pageSize`/自由检索参数按既有 DTO 结构）+ #162 的 `username` admin 过滤，query 参数结构与既有列表一致。

```jsonc
{
  "groups": [
    {
      "endpoint": { "id": 1, "name": "OpenRouter 主力", "...": "现有 EndpointItem 全部字段",
                    "user": { "id": 3, "name": "centonhuang", "avatar": "https://…" } },
      "models": [
        { "id": 11, "alias": "claude-sonnet", "...": "现有 ModelItem 字段",
          "user": { "id": 3, "name": "centonhuang", "avatar": "https://…" } }
      ],
      "modelCount": 3,
      "truncated": false
    }
  ],
  "pageInfo": { "page": 1, "pageSize": 10, "total": "<当前筛选下的 endpoint 总数>" },
  "modelTotal": 42
}
```

要点：

| 决策点 | 结论 |
|---|---|
| 分页语义 | `page/pageSize` 对 **endpoint 组** 分页；默认 pageSize=10，可选 10/20/50。组内模型随组全量返回，不分页 |
| 组内上限 | 单组最多返回 200 个模型，超出截断并在**该组对象**的 `truncated,omitempty` 布尔字段标记。防御性兜底，`// ponytail: 上限写死200；需要真分页时再引入组内游标` |
| `modelTotal` | 当前筛选范围内模型总数；dashboard 统计卡一次调用同时取端点数（`pageInfo.total`）与模型数（`modelTotal`），替代原先 `listEndpoints(1,1)` + `listModels(1,1)` 两次探测 |
| 归属用户 | endpoint/models 均**嵌套 `user{id,name,avatar}` 对象**，对齐 apikeys 页惯例（明确否决 #162 在旧接口上的扁平 `username`；旧接口删除后无历史包袱）。用户缺失/软删时整字段缺省（omitempty），前端显示 `—` |
| ModelItem | 移除其中的 `endpoint` 嵌套字段——组结构本身就是归属关系 |
| keyword 检索 | 自由文本命中 endpoint 名称**或**其下任一模型字段 → 该组整组返回（不做事后组内过滤）；admin 的 `username` 过滤沿用 #162 逻辑 |

**实现路径**（application 层新 query handler，仓储复用 #162 已有查询）：

```
scope 解析（ScopeUserID==0 且有 username → FindByName 回填，用户不存在→空结果，均沿用 #162）
① keyword → 匹配的 endpointID 集合（endpoint.name LIKE ∪ 其下 model 字段 LIKE，去重）
② endpointID 集合 ∩ scope → 对 endpoint 分组分页（repo.Paginate，total=组数）
③ 按本页 endpointIDs 批量拉取 models（repo 批查，同一批做 GROUP BY 得各组 modelCount 与 modelTotal）
④ 内存按 EndpointID 分桶组装 views；批量 loadUsers 回填嵌套 user
```

### 删除：管理端两条旧 list 路由

- `GET /api/v1/endpoint/list`、`GET /api/v1/model/list` 及各自 handler/port/DTO 全部删除（仓储层查询保留，被 upstream 组装逻辑复用）。UI 是唯一消费方（消费方清查见下），不设并行过渡期。

| 原 `/endpoint/list`、`/model/list` 消费方 | 本次处置 |
|---|---|
| endpoints 页表格 | 页面整体删除 |
| models 页新建/编辑对话框 endpoint 下拉 | 选择器取消（见"绑定不可移动"决策） |
| dashboard 统计卡 | 改调 `upstream/list` 一次取双数 |
| unit/e2e（`endpoint_query`、`user_scope_config`、`model_capabilities`、`model_id` 等） | 断言随重设计更新 |
| cmd/client 四平台客户端 | 无调用（已核实） |

### 客户端分发路由改名：`GET /api/v1/client/list` → `GET /api/v1/model/list`

面向 aris 客户端的 API Key 模型分发接口（#161 引入）路径更名为 `/api/v1/model/list`，鉴权方式（API Key）、响应结构、OperationID 语义均不变。旧的 JWT 管理 `/model/list` 本就被本次删除，路径腾出无冲突。

同步点（消费方清查结论，共 3 处，全部仓库内）：

1. `internal/router/client.go` 注册路径 `constant.RoutePathList` → `/model/list`；
2. `internal/common/constant/traceclient.go` `ClientModelsListPath` 常量值更新（唯一引用方 `internal/client/api/client.go:70` 随之生效）；
3. `test/e2e/client_models/client_models_test.go` 三处硬编码路径。

注：对外生成的安装/配置脚本未硬编码该路径，无线上脚本失效风险。

## 页面交互设计（形态 A：表格内分组行）

桌面（非移动端）：

```
┌ PageHeader: Upstream MaaS                [TraceInstall] [+ 新建端点] ┐
│ FilterBar:  🔍搜端点/模型   [admin: username▾]                        │
│ ┌───────────────────────────────────────────────────────────┐       │
│ │▓ ◉ centonhuang · OpenRouter 主力  [Chat][Resp] · 3 · ✎🗑[+模型]│ ← 组头(bg-muted)
│ │   ├ alias|modelId|upstream|limits|caps|enabled|created|操作 │      │
│ │   └ …                                                      │       │
│ │▓ ◉ lvlvko · Anthropic 官方 …                                │       │
│ └───────────────────────────────────────────────────────────┘       │
│ PaginationBar: 每 N 个端点（10/20/50）                                │
└─────────────────────────────────────────────────────────────────────┘
```

- **组头行**：Avatar+用户名、endpoint 名、协议 badges（ProviderIcon 现样式）、`modelCount`、操作区 `✎ 编辑端点` `🗑 删除端点` `[+ 模型]`。
- **模型行**：alias（保留点击复制）、modelId、upstream、limits、capabilities、enabled 开关、created、`✎/🗑` —— 比现 models 表少 endpoint 列。
- **模型 Dialog**：从组头发起创建、从行内发起编辑；**无 endpoint 选择器**（绑定不可移动）。endpoint 字段由所在组上下文注入请求。
- **端点 Dialog**：新建沿用现有表单 + admin 增加 ownerUserID 代建选择（`CreateEndpointReqBody.OwnerUserID` 后端 #162 已支持，前端首次落地）；编辑表单不变（后端 `UpdateEndpointReqBody` 本就无归属变更能力）。
- **端点删除确认**：文案提示"将同时影响其下 N 个模型"（N=modelCount；级联行为以现有 DELETE /endpoint 实现为准，实施时验证并在 E2E 固化）。
- **Mobile**：每个 endpoint 组一张卡，卡内纵向列出模型摘要行（对齐 sessions/apikeys 卡片模式），组内操作同桌面。
- **排序**：groups 内 endpoint 按现有列表默认序；组内 models 保持现有模型排序约定。

Dashboard 页统计卡 `isModuleOpen("endpoints"/"models")` 双探测合并为一次 `upstream/list` 调用。

## 前端契约同步

- `web/src/lib/types.ts` 新增 `UpstreamGroup` / `UpstreamUser` / `ListUpstreamRsp` 等类型（嵌套 `user{id,name,avatar}` 对齐 `APIKeyUser` 惯例），删除不再使用的旧 list 响应类型。
- `api-client.ts` 新增 `api.listUpstream(...)`，移除 `listEndpoints` / `listModels`；调用点仅剩新 upstream 页与 dashboard 统计。
- `locales/en|ja|zh.json` 同步新增导航项 "Upstream MaaS"、组头操作、删除确认（含影响 N 个模型）、截断提示等文案；顺带清理 endpoints/models 两页废弃文案。

## 边界情况

| 场景 | 行为 |
|---|---|
| `UserID==0` 或归属用户已软删 | `user` 字段缺省，组头显示 `—`（仅用户名位置降级，不影响其余展示） |
| demo 权限 | 现状不变：baseURL/APIKey/upstreamModel 脱敏照旧，删除类按钮 locked |
| 单组模型 >200 | 截断返回 + `truncated` 标记，UI 显示"已截断"提示 |
| keyword 命中的组不在当前页 | 正常落在后续页（按组分页天然保证同组不跨页断裂） |
| 非 admin 查他人 username | 后端忽略该参数（#162 约定），前端 facets 仅对 admin 渲染（现状保留） |
| 空结果 | 沿用 `ListEmptyState`；组存在但组内 0 模型允许存在（模型可删空），组头计数显示 0 |
| BatchFindByIDs 失败 | fail-fast 整请求失败（与 apikeys/loadEndpoints 惯例一致） |

## 测试计划

- **E2E 新增 `test/e2e/upstream_list/`**：分页 total=组数与 `modelTotal` 正确性、同组不跨页、keyword 整组聚合、嵌套 user 回填（含软删用户缺省）、普通用户 scope 隔离、admin 代建与 username 过滤。
- **E2E 更新**：`user_scope_config`、`model_capabilities`、`model_id` 中对旧 list 的断言改为走新接口/CRUD；`client_models` 改新路径。
- **单元测试**：application 层新 handler 的 keyword→ID 集合、分桶组装、截断标记逻辑。
- **回归**：DELETE /endpoint 的级联行为用例固化；PATCH /model 不接受归属变更不被破坏。
- **前端**：`cd web && npm run lint && npm run build`；本地起后端 + chrome mcp 实测导航、分组渲染、四类 CRUD 弹窗、PaginationBar 10 档位。

## 方案决策记录（讨论过程否决项）

| 议题 | 否决 | 采用理由 |
|---|---|---|
| models 分组形态 | B 仅增强单元格（改动最小）/ C 手风琴 | A 组头即分组语义本身，滚动浏览直观；C 多一次点击 |
| 分页协议 | 二级游标分页 | 组内数量级小，游标复杂度不值；一级沿用 CommonParam/PaginationBar 生态 |
| `/endpoint/list` 存废 | 保留双读 | 单一读路径避免契约漂移；消费方已全部清查可迁 |
| 新接口命名 | `GET /model/list` 复用 | 语义已是"上游配置聚合视图"，挂 model 资源下名不副实 |
| 归属用户载体 | 扁平 `username`/`userAvatar`（#162 临时风格） | 回归 70d5d3f8 嵌套惯例，头像+用户名一次到位 |
| 模型可移动性 | 编辑框保留 endpoint 选择器 | "绑定不可移动"简化心智：模型属于所在组，换组即重建 |
| 客户端接口 | 保持 `/client/list` 原名 | 用户明确要求更名 `/model/list`；仓库内消费方可控 |
