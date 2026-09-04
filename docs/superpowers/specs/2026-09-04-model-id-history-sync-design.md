# 模型 ID 历史记录一键同步（Model ID History Sync）设计

> 日期：2026-09-04
> 分支：`feature/model-id-history-sync-2026-09-04`
> 状态：已与用户确认设计

## 1. 背景与目标

`Model.modelId` 是业务模型 ID（创建默认 = alias，可更新）。当前在模型管理页修改 modelId 后，
三张历史表仍保留旧 ID 字符串，导致审计统计、会话筛选、趋势图出现"新旧 ID 断档"。

目标：在模型编辑流程中提供"同步更新历史记录"能力——当 modelId 变化时，
将该模型归属用户名下所有历史记录中的旧 model id 一键替换为新值。

## 2. 需求决策记录（已确认）

| 决策点 | 结论 |
|---|---|
| 入口与场景 | 模型管理页编辑弹窗联动（方案 A）：修改 modelId 时勾选"同步更新历史记录" |
| 数据范围 | `model_call_audit.model_id` + `session.model_ids` + `message.model_id` 三处全改（方案 B） |
| 隔离范围 | 严格按模型归属 user 隔离，只命中该 user 名下 API Key 产生的行（方案 A），防止误伤其他用户同名字符串 |
| 执行方式 | 同步批量 UPDATE，HTTP 请求内完成（方案 4A）；前端 loading 态 |
| 留痕 | 仅 zap 结构化日志一条（operator、old→new、三表影响行数），不建 DB 审计表（方案 5B 简化版） |
| 触发边界 | 仅当本次更新实际修改了 modelId 且用户勾选同步时才执行；未改 modelId 时 checkbox 不可用 |
| 前端形式 | 弹窗内 checkbox，一次请求提交（方案 A），避免"模型已改名但历史未改"的中间态 |

## 3. 数据事实（代码勘察结论）

| 表 | 字段 | 存储 | 归属限定 |
|---|---|---|---|
| `model_call_audits` | `model_id`（单值 string，有索引） | 列值 | `api_key_id` → `proxy_api_keys.user_id` |
| `sessions` | `model_ids`（JSONB 字符串数组） | `serializer:json` | `api_key_name` → `proxy_api_keys.name` + `user_id` |
| `messages` | `model_id`（单值 string） | 列值 | **无归属字段**，经所属 session 的 `message_ids` 限定 |

补充事实：
- 现有 UpdateModel 链路：`dto.UpdateModelReqBody` → `handler.HandleUpdateModel` → `port.UpdateModelCommand` → `updateModelHandler.Handle` → `Model.Update(...)` → `repo.Update`。
- `UpdateModelCommand` 已有 `ScopeUserID`（admin 全量视角用），但历史替换的 scope 取**模型归属 `m.UserID()`** 而非操作者，保证 admin 代管时只作用于模型 owner 的数据。
- `proxy_api_keys` 软删除（`deleted_at > 0`）的 key **不排除**——其历史会话/审计仍归属该 user。
- `messages` 行按 checksum 全局去重，跨会话（理论上跨用户）共享；共享行被替换后对另一用户可见，属共享行固有语义（见 §6）。

## 4. 总体流程

```
前端编辑弹窗（modelId 变化 → checkbox 可用；用户勾选）
  → PATCH /api/web/v1/model?id=<modelID>（现有更新接口，body 新增 syncHistory）
    → updateModelHandler.Handle：
        1. FindByID 取旧 m.ModelID()
        2. 领域 Update（含 modelId 变更）+ repo.Update（既有逻辑不变）
        3. 若 syncHistory && oldModelID != newModelID：
           repo.ReplaceHistoricalModelID(ctx, m.UserID(), old, new) —— 单事务 3 条 UPDATE
        4. 返回 {auditCount, sessionCount, messageCount}
    → 前端 toast 展示影响行数
```

## 5. 后端改动

### 5.1 DTO（`internal/dto/model.go`）

- `UpdateModelReqBody` 新增 `SyncHistory *bool`（`json:"syncHistory,omitempty"`，缺省 = 不同步）。
- 更新模型响应由 `*dto.EmptyRsp` 改为新增 `ModelUpdateRsp`（huma 响应包装遵循 `huma-dto-conventions`）：

```go
type ModelUpdateRsp struct {
    AuditCount   int `json:"auditCount" doc:"审计记录替换行数"`
    SessionCount int `json:"sessionCount" doc:"会话替换行数"`
    MessageCount int `json:"messageCount" doc:"消息替换行数"`
}
```

未勾选同步时三值均为 0。**注意**：响应类型变更对 OpenAPI 是 breaking change，前端同步更新。

### 5.2 Port（`internal/application/model/port/handler.go`）

- `UpdateModelCommand` 新增 `SyncHistory *bool`。

### 5.3 Command（`internal/application/model/command/update_model.go`）

- 调用 `m.Update(...)` 前记录 `oldModelID := m.ModelID()`；更新后取新 model id。
- 当 `cmd.SyncHistory != nil && *cmd.SyncHistory && oldModelID != newModelID` 时，
  调用 `repo.ReplaceHistoricalModelID(ctx, m.UserID(), oldModelID, newModelID)`。
- 替换失败则整体返回错误（模型本体更新不回滚——见 §7 错误处理）。
- 成功后 zap 结构化日志一条：operatorUserID、modelID（uint）、old、new、三表影响行数。

### 5.4 Repository（`llmproxy.ModelRepository` 接口 + 基础设施实现）

新增方法：

```go
type ModelIDSyncCounts struct {
    AuditCount   int64
    SessionCount int64
    MessageCount int64
}

ReplaceHistoricalModelID(ctx context.Context, userID uint, oldID, newID string) (ModelIDSyncCounts, error)
```

单事务（`db.Transaction`）内三条 UPDATE，全部以该 user 的 API Key 集合为界：

```sql
-- ① 审计：api_key_id 直接关联
UPDATE model_call_audits SET model_id = :new
WHERE model_id = :old
  AND api_key_id IN (SELECT id FROM proxy_api_keys WHERE user_id = :uid);

-- ② 会话：JSONB 数组逐元素替换
UPDATE sessions SET model_ids = (
  SELECT COALESCE(jsonb_agg(CASE WHEN v = :old THEN :new::jsonb ELSE to_jsonb(v) END), '[]'::jsonb)
  FROM jsonb_array_elements_text(model_ids::jsonb) AS v
)
WHERE model_ids::jsonb @> to_jsonb(:old)
  AND api_key_name IN (SELECT name FROM proxy_api_keys WHERE user_id = :uid);
-- （具体参数绑定形式以实现为准；:old 为带引号的 JSON 字符串）

-- ③ 消息：经该 user 的 session 引用限定
UPDATE messages SET model_id = :new
WHERE model_id = :old
  AND id IN (
    SELECT DISTINCT jsonb_array_elements(message_ids::jsonb)::text::bigint
    FROM sessions
    WHERE api_key_name IN (SELECT name FROM proxy_api_keys WHERE user_id = :uid)
  );
```

实现要点：
- 全程 PG 方言（jsonb 函数），repository 已直接面向 PG，无需跨方言兼容。
- 排除已删除的 API Key **不适用**：`proxy_api_keys` 不过滤 `deleted_at`，保证已删 key 的历史数据同样被同步。
- 会话表软删除行（若有）同样在 UPDATE 范围内（按 `api_key_name` 命中，不加 deleted 过滤）。
- 三条 UPDATE 原子性由单事务保证；行数统计用 `RowsAffected` 累加。

## 6. 已知 trade-off（明确接受）

**message 共享行跨用户可见**：message 行按 checksum 全局去重，两个 user 可能共享同一 message 行。
本设计的消息替换 scope 是"该 user 的 session 引用到的 message 行"，共享行被替换后，
另一 user 的会话详情里该条消息的 model_id 也会变为新值。
备选方案（跳过共享行）会留下"同一条消息在两边 model_id 不同"的不一致，更不可取，故不做。
此边界记录于本 spec，不做额外防护。

## 7. 错误处理

- 替换事务失败：返回 `ierr` 业务错误（统一走 `internal/common/ierr`），接口返回错误；
  模型本体已更新（模型改名成功、历史未同步），前端提示"历史同步失败，可重试或手动处理"。
- modelId 未实际变化却传 `syncHistory=true`：不执行替换，返回 0 行（幂等，不报错）。
- 请求超时风险：同步执行大表 UPDATE 可能耗时数秒；HTTP 超时沿用现有网关默认值，前端 loading 态等待。

## 8. 前端改动（`web/src/app/(dashboard)/upstream/`）

- `model-dialog.tsx`：
  - 编辑模式下，`modelId` 输入值相对打开弹窗时的原值变化 → 渲染「同步更新历史记录」checkbox；
    未变化或新建模式不渲染。
  - checkbox 勾选态随表单提交。
- `upstream/page.tsx`（或调用保存的组件）：
  - 更新请求带 `syncHistory`；
  - 成功 toast 展示三表影响行数（如「历史同步完成：审计 N 条、会话 N 条、消息 N 条」）；
    未同步时保持现有文案。
- `web/src/lib/types.ts`：`UpdateModelReqBody` 新增 `syncHistory?: boolean`；更新响应类型新增三字段。
- i18n：新增 checkbox 文案 + toast 文案（中英各 2-3 个 key）。

## 9. 权限

- 路由与现有模型更新一致（登录用户管理自己的模型；admin 全量）。
- 历史替换 scope 固定取模型归属 `m.UserID()`：普通用户改自己的模型只影响自己的数据；
  admin 代建/代管时同样只作用于模型 owner 名下数据。

## 10. 测试计划

- **E2E**（`test/e2e/model_id/`，沿用现有 harness 与真实 PG）：
  - `TestModelID_SyncHistory`：
    1. 用户 A 创建模型（modelId=X）；
    2. 造历史数据：A 的 audit（model_id=X）、A 的 session（model_ids 含 X）、A 引用的 message（model_id=X）；
    3. 用户 B 建同名 modelId=X 并造一条 audit（隔离对照）；
    4. A 更新模型 modelId=X→Y 且 syncHistory=true；
    5. 断言：A 三表全部变为 Y、影响行数返回正确；B 的 audit 仍为 X（隔离不误伤）。
  - `TestModelID_SyncHistoryNoChange`：modelId 未变 + syncHistory=true → 返回 0 行，无副作用。
  - `TestModelID_SyncHistoryUnchecked`：modelId 变化但 syncHistory 缺省 → 历史数据保持旧值。
- 单元测试：repo 层 SQL 为 PG 方言，单测环境无法覆盖 jsonb 函数的部分以 E2E 为准；
  command 层分支逻辑（是否调用替换）可用单测覆盖。
- 前端：`npx tsc --noEmit` + `npm run lint` + `npm run build`；运行时验证按 `next-dev-loop`。

## 11. 验收清单

- [ ] 模型改名 + 勾选同步 → 三表历史数据替换且影响行数返回前端展示
- [ ] 未勾选 / modelId 未变 → 历史数据不受影响
- [ ] 不同用户同名字符串互不影响（隔离）
- [ ] admin 代管时 scope 为模型 owner
- [ ] 替换留痕日志可查（operator、old→new、三表行数）
- [ ] `go test ./...`、`make lint`、前端三件套通过
