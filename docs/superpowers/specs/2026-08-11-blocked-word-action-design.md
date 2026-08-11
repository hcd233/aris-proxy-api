# Blocked Words 敏感词 action 配置设计文档

> 日期：2026-08-11
> 状态：已评审
> 分支：`feature/blocked-word-action-2026-08-11`

## 背景

当前敏感词（Blocked）只有一种行为：LLM 代理请求内容命中时返回 403 Forbidden 并记录审计（`internal/application/llmproxy/usecase/openai.go` / `anthropic.go` 的 `checkContent` + `SendOpenAIContentBlockedError`）。

需求：为每个敏感词增加 `action` 配置：

1. `deny`：命中 → 禁止请求，返回 403（**现状逻辑，默认值**，向后兼容存量数据）
2. `allow`：命中 → 允许请求照常转发上游，但**不记录 session/message/tool**；audit 完全正常记录（不标记命中）

同时要求：web 端支持创建时配置 action、可修改已有配置项；API 端增加相应接口能力与字段。

## 领域语义（沿用 CONTEXT.md 词汇表）

- **Blocked**：管理员配置的敏感词黑名单条目，含 `word`、`hitCount`。
- **BlockedService**：管理 AC 自动机生命周期的领域服务，`Check(text) []uint` 返回所有命中词 ID。
- **ModelCallAudit**：每次 LLM 模型调用的完整审计记录，由异步任务 `ModelCallAuditTask` 经协程池写入，与消息存储（`MessageStoreTask`）相互独立。
- **Session/Message/Tool**：由 Proxy Capture 沉淀，通过 `SubmitMessageStoreTask` 异步落库。

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| D1 | action 枚举命名 | **`deny`（默认）/ `allow`** | API 字段 `action`，语义直白，与"允许/禁止"对应（用户指定） |
| D2 | 混合命中策略 | **deny 优先**：任一命中词为 deny → 403；全部为 allow → 放行 | 安全优先，allow 只在全部命中词均放行型时生效（用户指定） |
| D3 | 放行模式 hitCount | **照常递增**（拦截与放行都计数） | 管理员可见该词触达频率，与"正常记录 audit"口径一致（用户指定） |
| D4 | 放行模式 audit | **完全正常，不做标记** | 与普通请求一致（用户指定），由转发链路自动记录 |
| D5 | "放行不记录"实现 | **ctx 标记 + 3 个 store 基元入口检查**（方案 A） | store 调用点 14 处零改动，覆盖全部 7 条转发路径（native + 跨协议）；audit/限流/转发不受影响 |
| D6 | Update 接口 | **`PATCH /block?id=xxx`，body 仅 `action`（指针字段）**，不改 word | 参照 endpoint PATCH 风格；word 有唯一索引 `idx_word_deleted_at`，改词有冲突风险且需求未要求（YAGNI） |
| D7 | 存量数据兼容 | DB 列 `default 'deny'` + 代码层空值按 deny 兜底 | AutoMigrate 加列对存量行赋默认值；AC 自动机加载时空值防御 |

## 后端设计

### 枚举（`internal/common/enum/blocked_action.go`，新文件）

```go
// BlockedAction 敏感词命中处理动作
type BlockedAction = string

const (
    BlockedActionDeny  BlockedAction = "deny"  // 命中即拦截，返回 403
    BlockedActionAllow BlockedAction = "allow" // 命中放行，但不记录 session/message/tool
)
```

### DB Model（`internal/infrastructure/database/model/blocked.go`）

```go
Action string `json:"action" gorm:"column:action;type:varchar(16);not null;default:deny;comment:命中处理动作"`
```

GORM AutoMigrate 自动加列；存量行获得默认值 `deny`。

### 领域聚合（`internal/domain/blocked/aggregate/blocked.go`）

- 增加私有字段 `action string`
- `CreateBlocked(id uint, word string, action string)`：action 为空时归一化为 `deny`
- 新增 `Action() string` getter

### 领域服务（`internal/application/blocked/service.go`）

- `rebuild` 时同时维护 `actionByID map[uint]string`（空值按 deny 归一化）
- 新增 `DenyIDs(ids []uint) []uint`：过滤出 action=deny 的命中词 ID（`lo.Filter`）

### Repository（`internal/domain/blocked/repository.go` + `internal/infrastructure/repository/blocked_repository.go`）

- 接口新增 `UpdateAction(ctx context.Context, id uint, action string) error`
- `toBlockedAggregate` / `toBlockedDBModel` 透传 action
- 实现 `UpdateAction`：`db.Model(&dbmodel.Blocked{}).Where(id).UpdateColumn("action", action)`

### Port（`internal/application/blocked/port/handler.go`）

```go
type UpdateBlockedCommand struct {
    BlockedID uint
    Action    string
}
type UpdateBlockedHandler interface {
    Handle(ctx context.Context, cmd UpdateBlockedCommand) error
}
```

`CreateBlockedCommand` 增加 `Action string` 字段。

### Command（`internal/application/blocked/command/update_blocked.go`，新文件）

`updateBlockedHandler`：校验 action ∈ {deny, allow}（非法返回 `ierr.ErrValidation`）→ `repo.UpdateAction` → `rebuildNotify(ctx)` 重建 AC 自动机。

### DTO（`internal/dto/blocked.go`）

```go
// CreateBlockedReqBody 增加
Action string `json:"action,omitempty" enum:"deny,allow" doc:"命中处理动作（默认 deny）"`

// 新增 UpdateBlockedReq（PATCH /block?id=xxx）
type UpdateBlockedReq struct {
    ID   uint                   `query:"id" required:"true" minimum:"1" doc:"Blocked ID"`
    Body *UpdateBlockedReqBody  `json:"body" doc:"Request body"`
}
type UpdateBlockedReqBody struct {
    Action *string `json:"action,omitempty" enum:"deny,allow" doc:"命中处理动作"`
}

// BlockedItem 增加
Action string `json:"action" doc:"命中处理动作"`
```

### Handler（`internal/handler/blocked.go`）

- `BlockedHandler` 接口新增 `HandleUpdateBlocked`
- `BlockedDependencies` 新增 `Update port.UpdateBlockedHandler`
- 实现：校验 body 非空 → `update.Handle` → `EmptyRsp`

### Router（`internal/router/blocked.go`）

```go
huma.Register(group, huma.Operation{
    OperationID: "updateBlocked",
    Method:      http.MethodPatch,
    Path:        "",
    ...
}, handler.HandleUpdateBlocked)
```

### UseCase 分流（`internal/application/llmproxy/usecase/openai.go` / `anthropic.go` + `blocked_check.go`）

`CreateChatCompletion` / `CreateMessage` 中命中逻辑改为：

```go
if matched := u.checkContent(req); len(matched) > 0 {
    _ = u.blockedChecker.IncrementHits(ctx, matched) // 两种模式都计数（D3）
    if denyIDs := u.blockedChecker.DenyIDs(matched); len(denyIDs) > 0 {
        // ── 现有 403 逻辑原样保留（words 取 DenyIDs）──
        // 手动构造 ModelCallAuditTask(ErrorMessage=BlockedAuditRemarkTemplate) + SendXxxContentBlockedError
    }
    // 全部 allow：放行，注入跳过存储标记后继续正常转发（D5）
    ctx = context.WithValue(ctx, constant.CtxKeySkipStore, true)
}
```

### ctx key（`internal/common/constant/ctx.go`）

```go
// CtxKeySkipStore 命中 allow 型敏感词，跳过 session/message/tool 存储
CtxKeySkipStore enum.CtxKey = "skipStore"
```

读取工具：`util.CtxValueBool`（`internal/util/`，若无则新增，参照 `CtxValueString` / `CtxValueUint`）。

### Store 基元入口检查（`openai_store.go` / `anthropic_store.go`）

`storeOpenAIChatMessages` / `storeResponseFromRsp` / `storeAnthropicMessages` 入口：

```go
if util.CtxValueBool(ctx, constant.CtxKeySkipStore) {
    return
}
```

audit 记录走独立任务（`SubmitModelCallAuditTask`），不受影响（D4）。

## 前端设计

### 类型（`web/src/lib/types.ts`）

```ts
export interface CreateBlockedReqBody {
  word: string;
  action?: string; // deny | allow，默认 deny
}
export interface BlockedItem {
  id: number;
  word: string;
  action: string;
  hitCount: number;
  createdAt: string;
}
export interface UpdateBlockedReqBody {
  action: string;
}
```

### API 客户端（`web/src/lib/api-client.ts`）

- `createBlocked(body)` 透传 action
- 新增 `updateBlocked(id: number, action: string)` → `PATCH /api/v1/block?id=${id}`

### 页面（`web/src/app/(dashboard)/blocked/page.tsx`）

- 创建对话框：word 输入下方加 action 单选（`RadioGroup`，deny「拦截」/ allow「放行不记录」），默认 deny
- 桌面表格加「动作」列：deny 显示 Badge「拦截」，allow 显示「放行」；行操作新增「切换动作」按钮（点击后调 `updateBlocked` 切换为另一 action，附确认提示）
- 移动端卡片同步展示 action 与切换入口
- `emptyForm` 增加 `action: "deny"`

### 文案（`web/src/locales/zh.json` / `en.json` / `ja.json`）

新增：`blocked.action`（动作）、`blocked.action_deny`（拦截）、`blocked.action_allow`（放行不记录）、`blocked.action_switch`（切换动作）、`blocked.action_switch_confirm`（确认切换提示）、`blocked.action_updated`（更新成功）。

## 测试设计

### 单元测试（`test/unit/blocked_matcher/`）

- `BlockedService.DenyIDs`：混合 deny/allow、全 allow、全 deny、空输入
- 聚合 `CreateBlocked`：action 空 → deny 归一化
- `updateBlockedHandler`：非法 action → `ErrValidation`；合法 → 调用 repo 并触发 rebuildNotify（用 fake repo）

### E2E（`test/e2e/blocked/`，新主题）

1. **deny 回归**：创建 deny 词 → 带该词的请求返回 403（现有行为不变）
2. **allow 放行**：创建 allow 词 → 带该词的请求 200 且响应为上游真实内容 → 校验 session/message 未落库（列表查不到）、audit 有记录
3. **Update 接口**：PATCH 改 action 后行为切换（deny → allow → 放行）
4. **混合命中**：deny + allow 同时命中 → 403

## 验证命令

- 后端：`make lint`、`go test -count=1 ./internal/... ./test/...`
- 前端：`cd web && npm run lint`、`make web-build`
- E2E：`go test -count=1 ./test/e2e/blocked/`
