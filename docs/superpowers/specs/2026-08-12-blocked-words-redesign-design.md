# Blocked Words 页面列名与交互重设计文档

> 日期：2026-08-12
> 状态：已评审
> 分支：`feature/blocked-words-redesign-2026-08-12`

## 背景

管理后台 Blocked Words 页面（`web/src/app/(dashboard)/blocked/page.tsx`）当前形态：

- **列**：ID / 词汇 / 动作 / 命中次数 / 创建时间 / 操作
- **交互**：搜索；新增（对话框：单个词输入 + 动作单选）；切换动作（RefreshCw 图标按钮）；删除（每行删除按钮 + 确认对话框）；分页；移动端卡片布局

痛点：

1. **列名语义模糊**：「词汇」与页面 subtitle「敏感词黑名单」及 CONTEXT.md 领域术语不一致；「动作」未点明是"命中后的处理动作"。
2. **切换动作不直观**：`RefreshCw` 图标按钮无状态可见性，用户需 hover 才能猜测含义。
3. **新增流程重**：添加一个词需要打开对话框、输入、选动作、确认，与"快速录入黑名单"的使用场景不匹配。
4. **无批量删除**：黑名单可能积累大量词条，逐条删除低效。

## 目标

1. 列名术语统一，消除歧义。
2. 切换动作改为**点击徽章直接切换**（deny⇄allow），状态即控件。
3. 新增改为**行内快速添加**，回车即加（默认 deny），移除对话框。
4. 删除保留每行确认框，新增**批量删除**（交互对齐 sessions 页面），后端支持 ids 批量删除。

## 领域语义（沿用 CONTEXT.md 词汇表）

- **Blocked（敏感词）**：管理员配置的敏感词黑名单条目，含 `word`、`action`（`deny` 拦截 / `allow` 放行）、`hitCount`。
- **deny**：命中时 LLM 代理请求返回 403 Forbidden 并记录审计。
- **allow**：命中时请求照常转发，但不落库 session/message/tool（审计照常，命中计数照常递增）。
- 混合命中时 deny 优先。

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| D1 | 列名改动范围 | 「词汇」→「敏感词」、「动作」→「处理动作」；**「创建时间」保留不改**（用户指定）；ID / 命中次数 / 操作保留 | 与领域术语统一；创建时间语义无歧义 |
| D2 | 切换动作交互 | **点击徽章直接切换** deny⇄allow，带 title 提示与切换中禁用（用户指定） | 状态即控件，无需额外操作入口 |
| D3 | 新增交互 | **行内输入框 + 回车添加，默认 deny**，移除对话框（用户指定） | 黑名单录入是高频重复动作，最短路径；需要 allow 的词添加后点徽章一键切换 |
| D4 | 删除交互 | 保留每行删除按钮 + 确认对话框；**新增批量删除**，交互对齐 sessions（勾选 + 「删除 N」destructive 按钮 + 确认框）（用户指定） | 批量清理能力，模式复用 |
| D5 | 批量删除后端实现 | **后端 DELETE /api/v1/block 支持 `?ids=1,2,3` 逗号分隔**（用户指定） | 对齐 session 的 `?ids=` 模式，单请求原子性好；前端单删/批量共用同一接口 |

## 前端设计

### 列名（三语 en/zh/ja 同步）

| 列 | 现值（zh） | 新值（zh） | 说明 |
|----|-----------|-----------|------|
| ID | ID | ID | 保留 |
| 词汇 | 词汇 | **敏感词** | 对齐领域术语 |
| 动作 | 动作 | **处理动作** | 点明"命中后如何处理" |
| 命中次数 | 命中次数 | 命中次数 | 保留 |
| 创建时间 | 创建时间 | 创建时间 | 保留（用户指定） |
| 操作 | 操作 | 操作 | 保留 |

locale keys：`blocked.word` → 新文案；新增 `blocked.action` 文案改为「处理动作」（沿用既有 key，值改文案即可）。

### 切换动作（ActionBadge → 可点击）

- `ActionBadge` 改为 `<button>`：点击调 `handleToggleAction(item)`，deny⇄allow 切换。
- 视觉：保持现有 deny/allow 配色（destructive / emerald），hover 提亮 + `cursor-pointer`。
- `title={t("blocked.action_switch_hint")}`（如「点击切换为拦截/放行」）。
- 切换中禁用（局部 `togglingId` state），防止连点并发。
- 移动端卡片中的徽章同样可点击。

### 行内快速添加

- 卡片头部（搜索框所在行）新增输入框 + 回车提交；与搜索框共享一行（桌面）或纵向堆叠（移动）。
- placeholder：`blocked.inline_add_placeholder`（如「输入敏感词，回车添加（默认拦截）」）。
- 回车：`createBlocked({ word, action: "deny" })`；成功 → 清空输入框 + `toast.success`；失败 → `showErrorToast`。
- 空词 / 纯空白不提交（disabled 或直接 return）。
- 移除新增 Dialog 及其 `dialogOpen` / `form` / `saving` state 与 RadioGroup。

### 批量删除（对齐 sessions）

- 表格新增首列 checkbox（表头全选 + 行勾选），复用 sessions 的 `role="checkbox"` 自绘模式（`Check` 图标 + `aria-checked`）。
- 移动端卡片同样渲染勾选框（卡片右上角）。
- `selected: Set<number>` + `toggleSelect` / `toggleSelectAll`；翻页时清空选中（对齐 sessions 行为）。
- 选中 > 0 时工具栏出现 `variant="destructive"` 的「删除 N」按钮 → 批量确认对话框 → `batchDeleteBlocked(ids)` → 成功 toast（含删除数量，若后端返回）+ 清空选中 + 刷新列表。
- 单删保留：每行 DeleteButton + `DeleteConfirmDialog`（`deleteConfirm` hook 不变）。

### 前端 API（`web/src/lib/api-client.ts`）

```ts
// 单删与批量共用 DELETE /api/v1/block?ids=
async deleteBlocked(id: number): Promise<CommonRsp> {
  return this.request<CommonRsp>(`/api/v1/block?ids=${id}`, { method: "DELETE" });
}

async batchDeleteBlocked(ids: number[]): Promise<DeleteBlockedRsp> {
  return this.request<DeleteBlockedRsp>(`/api/v1/block?ids=${ids.join(",")}`, { method: "DELETE" });
}
```

`types.ts` 新增 `DeleteBlockedRsp`（对齐后端返回，含 `deletedCount`）。

## 后端设计

### DTO（`internal/dto/blocked.go`）

```go
// 现状
type DeleteBlockedReq struct {
    ID uint `query:"id" required:"true" minimum:"1" doc:"Blocked ID"`
}

// 改为（对齐 session DeleteSessionReq 的 ids 模式）
type DeleteBlockedReq struct {
    IDs string `query:"ids" required:"true" minLength:"1" doc:"Blocked ID 列表，逗号分隔，如 1 或 1,2,3"`
}
```

响应：`DeleteBlockedRsp` 新增 `DeletedCount uint`（对齐 session 删除响应风格，供前端 toast 显示）。

### Port（`internal/application/blocked/port/handler.go`）

```go
type DeleteBlockedCommand struct {
    BlockedIDs []uint
}
```

### Command（`internal/application/blocked/command/delete_blocked.go`）

- 解析 `[]uint`；空列表直接返回（防御）。
- 调用 repo 批量删除；成功后 `rebuildNotify` 一次（AC 自动机重建，与现状一致）。

### Repository（`internal/infrastructure/repository/blocked_repository.go`）

```go
func (r *blockedRepository) DeleteBatch(ctx context.Context, ids []uint) error
```

实现：`db.Where("id IN ?", ids).Delete(...)`（软删，dao.Delete 风格）或循环单删；优先批量 SQL。`BlockedRepository` 接口同步新增方法。

### Handler（`internal/handler/blocked.go`）

`HandleDeleteBlocked`：解析 `req.IDs` 逗号分隔 → `[]uint` → 转 `DeleteBlockedCommand`。

### 路由（`internal/router/blocked.go`）

无结构改动（参数在 dto 中），仅确认 OpenAPI 文档随 dto 自动更新。

## 边界与假设

- **翻页即清空选中**：对齐 sessions 现有行为，不跨页保持勾选。
- **重复词创建**：后端 create 无唯一校验（现状），行内重复添加由后端错误 toast 兜底；不新增前端预校验（YAGNI）。
- **不新增编辑词内容能力**：现状只能改 action，不改 word；需求未要求，保持。
- **批量删除原子性**：`DeleteBatch` 使用单条 `WHERE id IN (?)` 软删 SQL，单语句原子，不存在部分成功；`deletedCount` 取 RowsAffected。
- 创建时间列名不改，i18n 三语均保留「创建时间 / Created / 作成日時」。

## 验证

1. 后端：`make lint`；`go test -count=1 ./internal/...`（blocked 相关单测 + 新增 DeleteBatch / ids 解析用例）。
2. 前端：`cd web && npm run lint && npm run build`。
3. 浏览器（Chrome MCP）：登录管理后台 → Blocked Words 页，实测：
   - 列名显示为「敏感词 / 处理动作 / 命中次数 / 创建时间」；
   - 行内输入回车新增成功（默认 deny 徽章）；
   - 点击徽章切换 deny⇄allow 成功；
   - 勾选多行 → 「删除 N」→ 确认 → 列表刷新、选中清空；
   - 单删按钮 + 确认框仍可用；
   - 移动端视口下新增输入框、勾选、徽章切换可用。
