# Model / Endpoint 配置多租户化（用户级隔离）设计

> 日期：2026-08-26
> 状态：已评审通过（brainstorming 设计评审）
> 关联决策：完全私有模型池；存量数据划归 admin；admin 可全局视图 + 按用户名过滤

## 1. 背景与目标

当前 `endpoints` 与 `models` 是全局配置：所有用户的 LLM 请求共享同一套端点与模型别名。`ListAliases()` 全表查询所有 enabled 的 model，OpenAI / Anthropic 的 `/v1/models` 对所有用户返回相同列表。

**目标**：将 endpoint / model 配置下放到 user 级别，按多租户隔离：

- 每个用户拥有自己的 endpoint 和 model 集合，互不可见
- `/v1/models`（OpenAI / Anthropic）只返回当前 API Key 所属用户配置的 model
- 转发链路只在当前用户的配置范围内解析别名
- 管理后台对所有 `user` 级用户开放自管能力；admin 可查看全部用户配置并按用户名过滤

## 2. 数据模型与唯一索引

### 2.1 表结构变更

**`endpoints` 表**

- 新增列 `user_id uint NOT NULL`（逻辑外键 → `users.id`，不加 DB 物理外键，遵循项目现状）
- 唯一索引：`(name, deleted_at)` → `(user_id, name, deleted_at)`
  - 不同用户可有同名端点；同一用户内端点名唯一
  - 软删语义不变（`deleted_at = 0` 参与唯一索引）

**`models` 表**

- 新增列 `user_id uint NOT NULL`
- 唯一索引：`(alias, endpoint_id, deleted_at)` → `(user_id, alias, endpoint_id, deleted_at)`

### 2.2 归属一致性约束

model 的 `user_id` 必须等于其关联 endpoint 的 `user_id`：

- **应用层强制**：create model 时校验目标 endpoint 属于当前用户，否则 400；创建时 `user_id` 从 endpoint 带入而非客户端传入
- 不加 DB 级复合约束（PG 无跨行约束，应用层保证即可）

### 2.3 存量数据迁移

迁移放 `database migrate-data` 手动命令链路（不进自动 `database migrate`），参照 model-id-management 先例：

1. AutoMigrate 加 `user_id` 列（default 0）
2. Backfill：所有 `user_id = 0` 的存量 endpoint/model 划归主 admin（`permission = admin` 中 ID 最小者）；幂等——只填 `user_id = 0` 的行
3. 重建唯一索引：DropIndex 旧索引 → CreateIndex 新索引
   - 带 `HasIndex` 守卫防 PG DDL 自动提交导致的重跑卡死
   - PG 上 GORM AutoMigrate 改已有唯一索引不可靠，必须 manual migration 显式处理
4. 迁移顺序：AutoMigrate（加列）→ Backfill（填值）→ 索引重建（依赖非空 user_id 保证新索引语义正确）

## 3. API 与权限改造

### 3.1 管理后台 CRUD（`/api/v1/endpoint`、`/api/v1/model`）

- 权限中间件：`PermissionAdmin` → `PermissionUser`
- **普通用户视角**：
  - 所有接口（list/detail/create/update/delete）强制按 context 中 `CtxKeyUserID` 过滤，只能看到、操作自己的配置
  - 客户端传的任何归属参数被忽略
- **admin 视角**：
  - list 默认返回全部用户的配置（带分页），新增可选过滤参数 `username`（按用户名过滤，后端解析 username → userID 后按 `user_id` 过滤）；不传则全量
  - create/update/delete 按资源 ID 操作（现有模式不变），admin 天然可操作任意用户的资源
  - create 时可指定归属用户
- **demo 用户**：写接口按权限等级比较天然拒绝；demo 读走 DemoConfig 模块白名单逻辑不变

### 3.2 LLM 网关侧

- **List models**：
  - OpenAI `/v1/models`、Anthropic `/v1/models`：`ListAliases(ctx)` → `ListAliasesByUser(ctx, userID)`，只返回该用户 enabled 的 model alias
  - `APIKeyMiddleware` 已注入 `CtxKeyUserID`，handler 直接透传，无需新增鉴权逻辑
- **转发解析**：
  - `FindEndpointByAlias(ctx, alias)` → `FindEndpointByAlias(ctx, userID, alias)`，SQL 过滤加 `user_id = ?`
  - 随机选端点、跨协议转换、UpstreamCreds 组装、协议支持标记检查均不受影响（只依赖解析结果，不感知归属）

## 4. 错误处理

| 场景 | 行为 |
|---|---|
| 转发时用户未配置该别名 | 复用现有协议化 model not found 404 格式（见 memory `llmproxy/openai-model-not-found-error-format-2026-08-20`）；不区分"不存在"与"不属于你"，避免泄露其他用户配置的存在性 |
| 普通用户操作他人资源 | 404（同上原则） |
| create model 时 endpoint 不属于当前用户 | 400 校验错误 |
| 迁移重跑 | backfill 只填 `user_id=0` 幂等；索引重建带 HasIndex 守卫 |

## 5. 测试策略

- **单元测试**：
  - repository 层：user 隔离查询（A 用户查不到 B 的 model）、唯一索引冲突场景（同用户重名 / 异用户同名均合法）
  - usecase 层：list models 按 user 过滤、FindEndpointByAlias 带 userID 解析
  - 迁移函数：backfill 幂等、索引重建守卫
- **回归测试**：现有 endpoint/model CRUD、LLM 转发全路径测试更新为携带 userID
- **E2E**：沉淀到 `test/e2e/<topic>/`——双用户隔离场景（A 创建的 model B 在 list 与转发中均不可见）

## 6. 明确不做（YAGNI）

- 公共模板 + 私有覆盖的混合模型（已选完全私有）
- admin 代管的独立角色视图（复用现有 CRUD + username 过滤即可）
- audit 表按用户过滤改造（本次不动，后续独立需求）
- DB 物理外键与复合触发器校验
