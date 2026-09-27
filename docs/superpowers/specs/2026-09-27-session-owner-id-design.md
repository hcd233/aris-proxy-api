# 会话归属改按 API Key ID 判定

> 状态：设计已确认，待实施
> 分支：`bugfix/session-owner-id-2026-09-27`
> 基线：`5f95ea3a`
> 日期：2026-09-27

## 背景

全库代码评审发现两个相互关联的缺陷，根因同为「会话归属依赖 API Key 名称字符串」。

### 缺陷 A：归属名称可跨用户重复，导致越权（安全）

`sessions` 表没有 `user_id` / `api_key_id` 列，归属的唯一依据是 `api_key_name`：

```go
// internal/infrastructure/database/model/session.go:18
APIKeyName string `gorm:"column:api_key_name;not null;default:''"`
```

而 `proxy_api_keys` 的唯一索引是 `(user_id, name, deleted_at)`，`issue_api_key.go` 只校验用户存在性，**不禁止跨用户同名**。`LookupOwnerNamesByUserID`（`api_key_repository.go:230`）按 user_id 取名字列表，不剔除与他人冲突的名字，仓储侧过滤为 `api_key_name IN (?)`。

后果：任意普通用户创建一个与他人同名的 API Key，即可列出、读取全文、删除、评分、**公开分享**、导出他人会话。受影响路径共 6 类（列表 / 详情 / 删除 / 评分 / 分享 / 数据集导出预览采样）。

同类问题在 model 域已修过（`model_repository.go:412-422` 的冲突名剔除），但主路径 `LookupOwnerNamesByUserID` 未推广该防护。

### 缺陷 B：鉴权字段白名单漏列 name，归属恒为空（可用性，正在持续产生）

```go
// internal/common/constant/sql.go:151
ProxyAPIKeyRepoFieldsAuth = []string{FieldID, FieldUserID}
```

`APIKeyMiddleware`（`apikey.go:49`）用该白名单查 key，白名单**不含 `FieldName`** → `apiKey.Name` 恒为 `""` → `CtxKeyAPIKeyName` 注入空串 → `store_pool.go:94` 把 `api_key_name` 写成空串。

对照：`UserRepoFieldsAuth = {ID, Name, Permission}` 含 Name，所以 `CtxKeyUserName` 一直正常；只有 API Key 名漏了。

生产数据（2026-09-27 查询时点）完全吻合按月分布：

| 月份 | 空归属 | 有归属 |
|---|---|---|
| 2026-09 | 1767 | 0 |
| 2026-08 | 422 | 0 |
| 2026-07 | 237 | 0 |
| 2026-06 | 187 | 518 |
| 2026-05 | 0 | 932 |
| 2026-04 | 23 | 456 |
| 2026-03 | 0 | 2 |

后果：2026-07 起产生的会话 `api_key_name=''`，而过滤是 `api_key_name IN (ownerNames)`，空串永不匹配任何 key 名（`vo.APIKeyName` 校验非空）。这批会话对**所有普通用户彻底不可见**，仅 admin 可见。`traces` 表 10 条同理全空（`handler/trace.go:158` 读同一个 ctx 键）。

### 对照证据：ID 归属从未出问题

`model_call_audits` 共 68687 行，`api_key_id = 0` 的有 **0 行**——因为 `FieldID` 在白名单内，`CtxKeyAPIKeyID` 一直正常。名称归属的脆弱性已在生产实际暴露，ID 归属则始终可靠。这是本设计选择 ID 归属的直接依据。

## 生产现状（2026-09-27 只读查询）

| 项 | 值 |
|---|---|
| `proxy_api_keys` | 5 条 / 3 个用户 |
| 跨用户同名 key | **0**（缺陷 A 尚未被利用） |
| `sessions` 总数 | 约 4531（查询时点，持续增长） |
| 空归属会话 | 约 2623–2633（持续增长） |
| 有归属会话 | 1908 = 1869 可唯一匹配 + 39 匹配不到现存 key |
| 归属名歧义（同名匹配多用户） | 0 |
| `traces` | 10 条，全部空归属 |

## 目标与非目标

**目标**

1. 归属判定改由 `api_key_id` 承担，消除缺陷 A。
2. 修复缺陷 B，恢复 `api_key_name` 的数据完整性（展示与审计用途）。
3. 1869 条可无歧义确定归属的存量会话零回退。

**非目标**

1. 不回填 `api_key_name` 为空的存量会话（约 2.6k 条）。归属信息已永久丢失，不做推断。这批会话保持仅 admin 可见。
2. 不回填 39 条 `api_key_name` 匹配不到现存 key 的会话（key 已删除或改名），同样保持仅 admin 可见。
3. 不改变 `BatchGet` 的软删过滤行为（见「已知遗留」）。
4. 不处理本次评审发现的其他问题（SSRF 重定向、分享接口冒充 admin、令牌存储等），另开分支。

## 设计

### 核心：职责分离，保留 name 列

不删除 `api_key_name`。改为职责分离：

| 列 | 职责 | 参与鉴权 |
|---|---|---|
| `api_key_id`（新增） | 归属判定、权限过滤 | 是，唯一权威 |
| `api_key_name`（保留） | 展示、审计可读性 | **否** |

理由：

- `SessionDetailView.APIKeyName`、数据集 owner 标注、admin 按 owner 过滤都在消费 name。删列需连带改前端，不在本轮目标内（YAGNI）。
- ID 不可重复，缺陷 A 被根除。
- 前端零改动。

`api_key_id = 0` 的行（空归属 + unmatched）天然不匹配任何真实 ID（自增主键从 1 起），无需额外守卫。该语义与既有三态过滤（`nil` = admin 全量 / 非 nil 空切片 = 恒假空结果）正交叠加，互不干扰。

### 领域模型调整

`aggregate.Session` 的 owner 概念由名称换为 ID。`vo.APIKeyOwner`（名称值对象）不再表达归属语义，仅在需要展示名称处保留。

`CONTEXT.md` 的 **APIKeyOwner** 条目须改写：当前定义为「Session 所属的 API Key 名称值对象……用户只能访问其 API Key 名下的会话」，与新模型不符。

### 写入侧改造

| 位置 | 改动 |
|---|---|
| `internal/common/constant/sql.go:151` | `ProxyAPIKeyRepoFieldsAuth` 补 `FieldName` |
| `internal/dto/asynctask.go:18` 的 `MessageStoreTask` | 新增 `APIKeyID` 字段 |
| `openai_store.go:51`、`openai_store.go:223`、`anthropic_store.go:36`、`trigger_capture.go:204` | 取 `util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID)` 一并传入 |
| `internal/infrastructure/pool/store_pool.go:94` | 构造 `dbmodel.Session` 时写入 `APIKeyID` |
| `internal/handler/trace.go:158` | 同步写入 `APIKeyID` |

`util.CopyContextValues` 已复制 `CtxKeyAPIKeyID`（`context.go:23`），异步任务链路无需改动。

### 归属判定侧改造

应用层从 `LookupOwnerNamesByUserID` 切换到**已存在**的 `LookupIDsByUserID`（`api_key_repository.go:245`，audit 域在用），无需新增仓储方法。

需改签名的仓储方法（6 个）：

1. `SessionReadRepository.ListSessionsByOwnerNames` → 按 ID
2. `SessionReadRepository.ListDistinctScores`
3. `SessionReadRepository.ListDistinctModels`
4. `SessionReadRepository.ListMessageCountStats`
5. `session.ExportFilter.OwnerNames` → `OwnerIDs`
6. `trace.TraceRepository.PaginateByOwners`

需改判定的应用层（`slices.Contains` 比对对象换成 ID）：

- `session/query/jwt_session_queries.go:83`（列表）、`:213`（详情归属校验）
- `session/query/session_meta_query.go:57,102`、`session/query/option_list.go:36,50,70`
- `session/command/delete_session.go:33,58`、`score_session.go:43,97`
- `trace/query/authorize.go:45`、`trace/query/list_traces.go:54`、`trace/command/delete_trace.go:31`
- `dataset/query/dataset_query.go:158,208,325`

**SQL 硬约束**：一律使用 `Where("api_key_id IN (?)", ids)` 显式占位符。禁止 struct 条件——GORM 会忽略 `api_key_id = 0` 这类零值条件，且 `Where("api_key_id", v)`（无占位符）会静默丢弃参数。

### 数据库迁移

`dbmodel.Session` 与 `dbmodel.Trace` 各新增：

```go
APIKeyID uint `json:"api_key_id" gorm:"column:api_key_id;not null;default:0;index:idx_sessions_api_key_id;comment:归属 API Key ID"`
```

归属过滤是热路径，必须建索引。

回填挂在 `runMigrate`（`cmd/server/database.go:30`）内 `AutoMigrate` 之后，不新增 cobra 命令（遵守 `docs/agents/commands.md` 的硬约束）。

回填语义（幂等）：仅当 `api_key_id = 0` 且 `api_key_name <> ''` 且该 name 在 `proxy_api_keys` 中唯一对应一个 key 时，填入该 key 的 ID。目标 1869 条；空归属与歧义名不受影响。

反复执行安全：第二次运行时目标行 `api_key_id` 已非 0，不再命中。

**生产执行前必须向用户完整展示回填 SQL 并单独获取授权。**

### 部署顺序（硬约束）

```
① database migrate   （加列 + 建索引 + 回填 1869 条）
② 滚动部署新版本
```

反序会让新版本按尚未存在或未回填的列过滤，造成会话短时全不可见。

## 测试策略

### 缺陷 B 回归

断言 `ProxyAPIKeyRepoFieldsAuth` 包含 `FieldName`，且经真实中间件后 `CtxKeyAPIKeyName` 非空。

**捕获能力验证**：临时从白名单移除 `FieldName`，该用例必须 FAIL；确认后复原。

### 缺陷 A 回归（此前零覆盖）

现有 `test/unit/apikey_repository/lookup_user_scope_test.go` 只覆盖 `userID=0` 与空列表短路，`test/e2e/cross_tenant_reference` 只有引用完整性守卫，**跨用户同名场景无任何覆盖**。

新增 e2e：用户 A 与 B 各创建同名 API Key，A 名下会话对 B 必须全部不可达，覆盖 6 条路径（列表 / 详情 / 删除 / 评分 / 分享 / 数据集导出）。

**捕获能力验证**：在 name 归属的旧实现下该用例必须 FAIL。

测试须走真实 `router.RegisterAPIRouter` 装配，不得自行拼接路由组——自拼路径的测试无法捕获装配层缺陷。

### 回填幂等

sqlite 内存库连续执行两次，结果一致；空归属行与歧义名行的 `api_key_id` 保持 0。

### 三态语义

`ownerIDs = nil`（admin）→ 不加过滤；`ownerIDs = []`（无 key 用户）→ 恒假空结果；`api_key_id = 0` 的行在任何非 nil 场景下均不可见。

## 已知遗留（本轮不修，记录备查）

1. **软删 key 导致历史会话不可见**：`BatchGet`（`dao/base.go:122`）带 `DBConditionDeletedAtZero`，故 `LookupIDsByUserID` / `LookupOwnerNamesByUserID` 只返回未软删的 key。用户删除 key 后其名下历史会话立即不可见。这是既有行为，改 ID 归属后原样保留，不扩大也不收窄。
2. **约 2.6k 条空归属会话**永久仅 admin 可见（归属信息已丢失，不推断）。
3. **39 条 unmatched 会话**同上（对应 key 已删除或改名）。
4. 本次评审的其余问题（上游重定向 SSRF、分享接口以 `IsAdmin: true` 跳过校验、API Key 明文入库、令牌存 localStorage 等）另开分支。
