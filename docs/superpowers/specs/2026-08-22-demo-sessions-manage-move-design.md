# Demo sessions 管理入口迁移到 sessions 页

> 日期：2026-08-22
> 状态：待评审

## 1. 背景与目标

demo 演示账户的会话白名单目前由 admin 在 **demo tab** 管理：`DemoSessionsManager` 组件包含上半「已选列表」（查看/批量移除已加入的 sessions）与下半「候选选择器」（搜索全部会话并勾选批量加入）。该交互的候选选择器与 sessions 列表页功能高度重复（同样的搜索/筛选/分页/勾选）。

本次改动目标：

1. **demo tab 删除候选选择器**：保留「已选列表」卡片（查看/移除能力不变），移除下半部分会话选择器。
2. **sessions 列表页与详情页新增「添加到 demo」按钮**：把添加能力迁移到 sessions 页，逐条添加/移除，行内直接操作。
3. **按钮显隐条件**：仅当 demo 登录开启（`loginEnabled`）且当前用户为 admin 时显示。

## 2. 现状梳理（前端）

- `web/src/app/(dashboard)/demo/page.tsx`：`<PermissionGuard adminOnly>` 下渲染 `DemoConfigCard` + `DemoSessionsManager`。
- `web/src/components/demo-sessions-manager.tsx`：上半已选列表（`listDemoSessions` + 批量 `removeDemoSessions` + `DeleteConfirmDialog`），下半候选选择器（`listSessions` + 复用 `FilterBar`/`TimeRangePicker` + 勾选批量 `addDemoSessions`）。
- `web/src/lib/api-client.ts`：`getDemoConfig`（登录用户可读，返回 `loginEnabled`/`modules`）、`listDemoSessions(page, pageSize)`（admin，单页上限 100）、`addDemoSessions({sessionIds})`（admin）、`removeDemoSessions(ids)`（admin）。
- `web/src/lib/auth-context.tsx`：`isAdmin()` / `isDemo()`；`demoModules` 仅 demo 用户加载，loginEnabled 未暴露给非 demo 用户。
- sessions 列表页 `web/src/app/(dashboard)/sessions/page.tsx`：桌面 Table 末列 actions（`w-16`，删除图标按钮）、移动端卡片右上角操作区（评分/徽章/删除）。
- 详情页 `web/src/components/session-detail/session-detail-client.tsx`：header 操作区顺序为 返回 | 标题 | 评分 | 分享 | 删除 | tools。
- 后端接口已齐备（`internal/router/demo.go`：`addDemoSessions`/`removeDemoSessions`/`listDemoSessions` 均 admin only，`getDemoConfig` 登录用户可读），**本次后端零改动**。

## 3. 设计决策

| 决策点 | 结论 |
|--------|------|
| demo tab 删除范围 | 仅删下半「候选选择器」；「已选列表」卡片保留 |
| 按钮可见性 | `isAdmin() && loginEnabled`。admin 限定依据：add/remove/list 接口均为 admin only；`loginEnabled` 经 `getDemoConfig` 读取（登录用户可读） |
| 按钮交互 | **toggle**：未添加显示添加态，点击 `addDemoSessions({sessionIds:[id]})`；已添加显示已加入态，点击 `removeDemoSessions([id])` 移出。已选列表卡片保留在 demo tab（批量移除入口仍在），按钮提供 sessions 页行内的单条添加/移除能力 |
| 列表页按钮形态 | 每行一个图标按钮（与删除按钮并列），不做批量勾选添加 |
| 已添加状态数据源 | 进入页面 fetch 一次 `listDemoSessions` 全量（pageSize=100 循环至 total），构建 ID 集合；白名单通常远小于 100 条 |

## 4. 前端改动明细

| 文件 | 改动 |
|------|------|
| `web/src/components/demo-sessions-manager.tsx` | 删除候选选择器部分（candidates state、filterBar、TimeRangePicker、`handleAdd`、`SessionListTable` 中仅候选使用的分支、`add_*` 文案引用），保留已选列表 |
| `web/src/app/(dashboard)/demo/page.tsx` | 无结构变化（组件内部瘦身即可） |
| `web/src/hooks/use-demo-whitelist.ts` | **新增** hook：加载 `loginEnabled` + 全量白名单 ID 集合；暴露 `loginEnabled`、`isInDemo(id)`、`toggle(id)`（内部调用 add/remove 成功后同步本地集合）、`pending` |
| `web/src/components/demo-add-button.tsx` | **新增**展示组件：props `sessionId / inDemo / pending / onToggle`；`!isAdmin() \|\| !loginEnabled` 时不渲染；未添加=`BadgePlus` 图标+tooltip「加入演示账户」，已添加=勾选态（primary 色）+tooltip「已加入，点击移出」 |
| `web/src/app/(dashboard)/sessions/page.tsx` | 页面级调用 `useDemoWhitelist`；桌面 Table actions 列加宽（`w-16`→容纳两图标）并排渲染删除+添加按钮；移动端卡片操作区同步加按钮 |
| `web/src/components/session-detail/session-detail-client.tsx` | 页面级调用 `useDemoWhitelist`；header 操作区在分享与删除按钮之间加同一按钮 |
| `web/src/locales/{zh,en,ja}.json` | 新增 `demo.add_tooltip` / `demo.in_demo_tooltip` / `demo.added` / `demo.removed`；删除候选选择器专属 key（`selector_title` / `selector_desc` / `add_selected` / `search_placeholder` / `no_candidates` / `candidates_load_error` / `add_success` / `add_error`）；`selected_*` / `remove_*` 等保留 |

## 5. 验证

1. `cd web && npm run lint && npm run build`。
2. 浏览器（chrome MCP）admin 登录：
   - demo 开关开：sessions 列表/详情出现按钮；添加后按钮变已加入态，demo 账户登录可见该会话；再点移出后恢复。
   - demo 开关关：按钮不渲染。
   - demo 账户 / 普通 user 登录：按钮不渲染。
3. demo tab：只剩配置卡片 + 已选列表（移除仍可用）。
4. 后端接口未改动，`test/e2e/demo/demo_account_test.go` 无需变更；部署后跑 E2E 回归。

## 6. 不做的事（YAGNI）

- 不做列表页批量勾选「添加到 demo」。
- 不在 sessions 响应 DTO 增加 `isInDemo` 字段（前端全量拉白名单即可，后端零改动）。
- 不清理 `listDemoSessions`/`addDemoSessions`/`removeDemoSessions` 接口与 api-client 方法（按钮与已选列表仍在用）。
