# 游标分页改造设计文档

> 日期：2026-07-27
> 分支：feature/cursor-pagination-2026-07-27
> 状态：设计已确认，待实现

## 1. 背景与目标

当前所有分页 list 接口使用 offset 分页（`page`/`pageSize` + `Count`），由 `dao.baseDAO.Paginate` 统一实现。深翻页时 offset 扫描与 Count 查询在大表（audit/traces/messages）上开销显著。

**目标**：将全部 15 个分页 list 接口改造为游标分页（cursor-based / keyset pagination），前端同步改造为无限滚动交互。

**决策记录**（brainstorming 澄清结论）：

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 前端交互形态 | 无限滚动/加载更多，去掉页码跳转 |
| 2 | 排序能力 | 保留现有全部排序能力（任意 `sortField` + `asc/desc`） |
| 3 | total 计数 | 完全去掉，只返回 `hasMore`，不再执行 `Count` |
| 4 | 兼容性 | 直接 breaking change，前后端 + E2E 同一 PR 发布 |
| 5 | 游标编码 | 方案 A：不透明复合游标 token（base64url(JSON)） |

## 2. 后端设计

### 2.1 参数与响应结构

替换 `internal/common/model/param.go` 与 `internal/infrastructure/database/dao/param.go` 中的 `PageInfo`/`PageParam`：

```go
// 请求（query 绑定）
type CursorParam struct {
    Cursor string `query:"cursor" maxLength:"1024"` // 空 = 首页
    Limit  int    `query:"limit" required:"true" minimum:"1" maximum:"500"`
}
// CommonParam = { CursorParam, QueryParam, SortParam }

// 响应（json）
type CursorPageInfo struct {
    Limit      int    `json:"limit"`
    NextCursor string `json:"nextCursor,omitempty"` // hasMore=false 时为空
    HasMore    bool   `json:"hasMore"`
}
```

### 2.2 游标编解码（新增 `dao/cursor.go`）

- 载荷：`{f: sortField, v: sortValue(JSON 原生类型或 null), id: lastID, d: "asc"|"desc"}` → JSON → base64url（RawURLEncoding）
- 解码校验：`f` 必须等于本次请求的 `sortField`、`d` 必须等于本次 `sort`，不匹配返回 huma 400（i18n 错误信息），防止翻页中途改排序导致数据错乱
- `v` 类型还原：通过 gorm `schema.Parse(ModelT)` 按列名找到字段 Go 类型，将 JSON 值转换回 `uint/int64/string/time.Time/bool/float64` 后值绑定

### 2.3 `dao.CursorPaginate`（替换 `baseDAO.Paginate`）

1. 默认 `sort=desc`、`sortField=id`（管理类列表最新在前；会话消息/工具列表保持 asc）
2. 有游标时追加 keyset 条件（DESC、`v` 非 NULL）：`WHERE (col < ? OR (col = ? AND id < ?) OR col IS NULL)`；`v` 为 NULL：`WHERE (col IS NULL AND id < ?)`；ASC 镜像
3. `ORDER BY col DESC, id DESC`（id 为 tiebreaker；MySQL DESC 下 NULL 排最后，与条件自洽）
4. 查 `limit+1` 条判断 `hasMore`，截断后从最后一行提取排序列值 + id 生成 `nextCursor`
5. 不执行 `Count`
6. `sortField` 沿用 `util.SafeSortField` 字符白名单防注入，排序值全部值绑定

### 2.4 接口清单（15 个）

| 模块 | 接口 | 备注 |
|------|------|------|
| apikey | `ListAPIKeys` | |
| audit | `ListAuditLogs`（含 admin 视角） | 手写分页收敛到 CursorPaginate |
| blocked | `ListBlocked` | |
| cron | `ListCronJobs` | |
| cronaudit | `ListCronCallAudits` | 手写 applySort 收敛 |
| endpoint | `ListEndpoints` | |
| model | `ListModels` | |
| session | `ListSessions` | domain `Paginate` 改游标 |
| session | `ListSessionMessages` | 保持 id asc |
| session | `ListSessionTools` | 保持 id asc |
| session_share | `ListSessionShares` | |
| session_share | `ListShareMessages` | 保持 id asc |
| session_share | `ListShareTools` | 保持 id asc |
| trace | `ListTraces`、`ListTraceEvents`（conversation 复用 CommonParam 一并改） | |

DTO 改造：Rsp 的 `PageInfo *model.PageInfo` → `*model.CursorPageInfo`；Req 的 `Page/PageSize`/`PageParam` → `CursorParam`。domain `session.Repository.Paginate` 签名同步改。

`tracecli` 不调用 list 接口，无需改动。

## 3. 前端设计

- `types.ts`：`PageInfo` → `{ limit, nextCursor?, hasMore }`
- `api-client.ts`：13 个 list 方法签名改 `(params: { cursor?: string; limit: number; query?; sort?; sortField? })`
- 删除 `pagination-bar.tsx`，新增 `infinite-scroll-list.tsx`：IntersectionObserver 哨兵 + 「加载更多」按钮 fallback，底部三态（加载中 / 没有更多 / 空列表）
- 重写 `use-infinite-list.ts`：fetcher 改 `(cursor, limit) => { items, nextCursor, hasMore }`，行为契约（enabled/reset/并发去重/失败静默）不变
- 每页大小选择器保留（20/50/100/200/500，切换时 reset）；总数显示移除；新增文案补 en/zh/ja；遵守 i18n 布局稳定契约（`min-w-[7.5rem]`）
- 12 处页面适配：管理表格页累积列表 + 排序切换 reset；session-detail messages/tools、share 页适配新 fetcher

## 4. 测试计划

- 单元（`test/unit/` 新增）：游标编解码往返、asc/desc、NULL 值、排序不匹配 400、`limit+1` 截断
- E2E 改造（5 个文件）：`session_list_keyword`、`session_list_filter_model`、`session_detail_perf`、`session_list_batch_perf`、`session_dto`（unit）——参数改 cursor/limit，断言改 hasMore/nextCursor 驱动
- `CONTEXT.md` 增补"游标分页"领域术语
- 前端：`npm run lint && npm run build`；chrome mcp 抽查 2-3 个页面滚动加载

## 5. 性能基线（生产环境实测）

测量方法：call-api 对生产 `https://api.lvlvko.top` 逐接口测量（JWT 认证，pageSize=20），每接口分首页/中间页/深页三档，每档 5 次取中位数，串行 + 250ms 间隔避免触发限流。测量时间：2026-07-27。改造部署后同法复测对比。

| 接口 | total | 首页 | 中间页 | 深页 |
|------|------:|-----:|-------:|-----:|
| session/list | 2350 | 0.184s | 0.182s (p59) | 0.229s (p118) |
| session/message/list | 340 | 0.165s | 0.149s (p8) | 0.153s (p17) |
| session/tool/list | 60 | 0.205s | - | 0.148s (p3) |
| session/share/list | 1 | 0.126s | - | - |
| session/share/message/list | 295 | 0.173s | - | - |
| session/share/tool/list | 70 | 0.128s | - | - |
| apikey/list | 2 | 0.107s | - | - |
| endpoint/list | 7 | 0.134s | - | - |
| model/list | 13 | 0.139s | - | - |
| audit/model/log/list | 39370 | 0.145s | 0.154s (p984) | 0.139s (p1969) |
| audit/cron/log/list | 2191 | 0.123s | 0.149s (p55) | 0.121s (p110) |
| cron/list | 4 | 0.130s | - | - |
| block/list | 2 | 0.119s | - | - |
| trace/list | 11 | 0.117s | - | - |
| trace/event/list | 2 | 0.110s | - | - |

**基线结论**：
- 当前生产最大表为 audit/model/log（约 4 万行），深页 offset 劣化尚不明显（0.139s vs 首页 0.145s），Count 查询在该量级也可接受
- session/list 深页已出现可观测劣化（0.229s vs 首页 0.184s，+24%），其响应还叠加 summary 批量装配
- 改造收益定位：**消除 Count 查询与深 offset 扫描，使翻页耗时与页深无关（O(1)），为数据量增长到十万/百万级提供保护**；同时统一前端为无限滚动交互。部署后复测重点对比 session/list 与 audit 两个大表接口

## 6. 风险与边界

- **排序列 NULL 值**：按「NULL 排最后、可翻页」规则处理；非预期排序列若大量 NULL 可能导致该排序下翻页内容分布不均，属文档声明行为
- **翻页中改排序**：返回 400，前端通过切排序 reset 规避
- **深页性能**：keyset 条件依赖排序列索引；`id`/`created_at` 已有索引，其他列排序时深页性能取决于该列索引情况
- **数据实时变动**：无限滚动期间新增数据会导致后续页轻微偏移（游标分页固有特性，优于 offset 的重复/遗漏问题）
- **破坏性变更**：旧 `page`/`pageSize` 参数直接移除，前后端同 PR 发布；无其他已知调用方（tracecli 已确认不涉及）
