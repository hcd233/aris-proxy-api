# Blocked Words action 语义重构：Allow → Omit 设计文档

> 日期：2026-08-12
> 状态：已评审（方案 A 获用户确认）
> 分支：`refactor/blocked-action-omit-2026-08-12`

## 背景

Blocked Words（敏感词）的命中处理动作 `action` 当前枚举为 `deny` / `allow`。`allow` 的语义是「命中放行，但不记录 session/message/tool」——即忽略存储，而非一般意义的"允许"。将 `allow` 更名为 `omit`，语义更准确地表达「命中即忽略落库」，避免与"放行/允许"歧义。

## 领域语义（沿用 CONTEXT.md 词汇表）

- **Blocked（敏感词）**：管理员配置的敏感词黑名单条目，含 `word`、`action`（`deny` 拦截 / `omit` 忽略）、`hitCount`。
- **deny**：命中时 LLM 代理请求返回 403 Forbidden 并记录审计。
- **omit**：命中时请求照常转发，但不落库 session/message/tool（审计照常，命中计数照常递增）。
- 混合命中时 deny 优先。

## 决策记录

| # | 决策 | 选择 | 理由 |
|---|------|------|------|
| D1 | 枚举值 | `"allow"` → **`"omit"`**（常量 `BlockedActionAllow` → `BlockedActionOmit`） | 语义重构：值、常量名、前端类型、i18n 全链路同步（用户指定） |
| D2 | 存量数据 | **方案 A**：枚举值直接改 `"omit"`；部署后手动 SQL 迁移 `UPDATE blocked SET action='omit' WHERE action='allow'` | 沿用 `upstream-model` 迁移惯例；迁移前存量 `allow` 行因「非 deny 即放行」兜底逻辑行为不变 |
| D3 | 兜底逻辑 | `DenyIDs`（`action == "" || action == deny` → deny，其余放行）**保持不变** | 迁移前存量 `allow` 行仍正确放行，无需兼容代码 |
| D4 | 前端文案 | zh「忽略」/ en「Omit」/ ja「忽略」；i18n key `blocked.action_allow` → `blocked.action_omit` | 语义对齐（用户指定） |
| D5 | 历史设计文档 | `2026-08-11-blocked-word-action-design.md` / `2026-08-12-blocked-words-redesign-design.md` **不修改** | 评审历史记录，保持不可变 |

## 修改清单

### 后端

1. **枚举**（`internal/common/enum/blocked_action.go`）
   - `BlockedActionAllow` → `BlockedActionOmit = "omit"`，注释改「命中忽略，不记录 session/message/tool」

2. **DTO**（`internal/dto/blocked.go`）
   - `CreateBlockedReqBody.Action` / `UpdateBlockedReqBody.Action` 的 `enum:"deny,allow"` → `enum:"deny,omit"`，doc 同步

3. **Command**（`internal/application/blocked/command/update_blocked.go`）
   - 校验 `cmd.Action != enum.BlockedActionDeny && cmd.Action != enum.BlockedActionOmit`
   - 错误消息「must be deny or allow」→「must be deny or omit」

### 测试

4. `test/unit/blocked_command/update_blocked_test.go`：`enum.BlockedActionAllow` → `enum.BlockedActionOmit`（两处断言消息）
5. `test/unit/blocked_matcher/deny_ids_test.go`：`enum.BlockedActionAllow` → `enum.BlockedActionOmit`（四处 + 断言文案）
6. `test/e2e/blocked/blocked_test.go`：`"allow"` → `"omit"`（创建/更新调用、注释、变量名 `allowWord` → `omitWord`、`e2eallow` → `e2eomit` 等）

### 前端

7. `web/src/lib/types.ts`：`BlockedAction = "deny" | "allow"` → `"deny" | "omit"`
8. `web/src/app/(dashboard)/blocked/page.tsx`：
   - 切换逻辑 `item.action === "deny" ? "allow" : "deny"` → `"omit" : "deny"`
   - i18n key `blocked.action_allow` → `blocked.action_omit`；注释 `deny⇄allow` → `deny⇄omit`
9. `web/src/locales/zh.json` / `en.json` / `ja.json`：`blocked.action_allow` → `blocked.action_omit`（zh「忽略」/ en「Omit」/ ja「忽略」）

### 词汇表

10. `CONTEXT.md`：Blocked 术语 `action` 描述 `allow` → `omit`，语义「命中放行，但不落库」→「命中忽略，不落库」

### 数据迁移（部署后手动执行）

```sql
UPDATE blocked SET action = 'omit' WHERE action = 'allow';
```

## 验证

1. 后端：`make lint`；`go test -count=1 ./internal/... ./test/unit/...`（blocked 相关单测全绿）
2. 前端：`cd web && npm run lint`；`make web-build`
3. E2E（离线默认 skip，需显式环境变量）：`go test -count=1 ./test/e2e/blocked/`
4. 部署后：生产库执行 `UPDATE blocked SET action='omit' WHERE action='allow';` 并抽样验证无残留 `allow`
