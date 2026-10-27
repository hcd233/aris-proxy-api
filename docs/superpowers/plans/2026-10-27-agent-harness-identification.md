# Agent Harness 识别 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让会话列表/详情能显示并按「来源 Agent Harness」筛选，Dashboard 增加按 harness 的调用量走势图；规则存库、运行时可维护，改规则后历史数据自动重标且不需要回填。

**Architecture:** 原始 UA 是唯一持久化事实（`sessions.client_ua`，创建时写入，不参与合并前滚），harness 是每次查询现算的派生物：Go 侧 `agentharness.Match` 负责展示，SQL 侧用「规则表标量子查询」负责筛选与走势图分组。规则表 `agent_harness_rules`（有序前缀 → 名称）由 admin 接口维护，不缓存。

**Tech Stack:** Go 1.25 / huma v2 / GORM（PostgreSQL）/ samber lo / Next.js 16 + React。

**Spec:** `docs/superpowers/specs/2026-10-27-agent-harness-identification-design.md`

## Global Constraints

- Go 1.25.1；单元测试一律 `go test -count=1`；提交前 `make lint` 必须通过。
- 测试只放 `test/unit/<topic>/` 与 `test/e2e/<topic>/`，`internal/` 下不新增 `_test.go`。
- 业务错误统一用 `internal/common/ierr`；DTO 绑定规则遵循 `huma-dto-conventions` skill。
- 不新增第三方依赖（前端同样）。
- 每条新增可筛选字段必须同步三处：DTO huma 枚举、应用层 switch、常量定义（历史 422 教训）。
- 前端验证：`npm run lint`、`npm run test`、`npx next build --webpack`（worktree 下 node_modules 是软链，必须 `--webpack`）。
- 分支 `feature/agent-harness-ident-2026-10-27`，worktree `.worktrees/agent-harness-ident`。

## Review Focus

这些是设计隐含、但没有哪条任务测试天然覆盖的输入，各自在本计划指定的任务里补测试：

1. UA 为空串 / 只有空白 → 必须落 `Other`，不能 panic、不能命中任何规则。（Task 1）
2. UA 大小写与前后空格混杂（`ZCode/3.15.3`、` deepseek-harness/0.2.1 `）→ 必须命中，pattern 一律小写比较。（Task 1）
3. 规则 pattern 含 `%` 或 `_` → 写入必须被拒（否则 LIKE 通配符放大命中面）。（Task 1）
4. `Other` 作为规则名（任意大小写）→ 写入必须被拒。（Task 1）
5. 存量会话 `client_ua = ''`：筛选 `harness:Other` 必须命中，筛选具体 harness 必须不命中。（Task 5）
6. 规则被改名/删除后，走势图缓存必须立刻失效（不能等 5 分钟）。（Task 7）
7. `harness:!X` 的语义是「映射结果不是 X」，必须包含 `Other` 与空 UA。（Task 5）

---

### Task 1: `agentharness` 域核心（匹配与校验）

**Files:**
- Create: `internal/domain/agentharness/rule.go`
- Test: `test/unit/agent_harness/match_test.go`

**Interfaces:**
- Produces:
  - `type Rule struct { ID uint; Pattern string; Name string; Sort int }`
  - `const Other = "Other"`
  - `func Match(ua string, rules []Rule) string` —— `rules` 必须已按 `(Sort, ID)` 升序排好，返回首个前缀命中项的 `Name`，无命中或 `ua` 为空返回 `Other`
  - `func NormalizePattern(raw string) (string, error)` —— `strings.TrimSpace` + `strings.ToLower`；空、长度 > 200、含 `%` `_` 返回 `ierr.ErrValidation`
  - `func NormalizeName(raw string) (string, error)` —— `strings.TrimSpace`（不改大小写）；空、长度 > 64、`strings.EqualFold(name, Other)` 返回 `ierr.ErrValidation`

- [ ] **Step 1: 写失败测试**

`test/unit/agent_harness/match_test.go`，表驱动，至少这些用例：

```go
rules := []agentharness.Rule{
    {ID: 2, Pattern: "zcode/", Name: "ZCode", Sort: 10},
    {ID: 1, Pattern: "deepseek-harness/", Name: "DSH", Sort: 20},
    {ID: 3, Pattern: "deepseek-harness/0.2.1-rc.2", Name: "DSH-RC2", Sort: 5},
}
// 断言：
// Match("deepseek-harness/0.2.1-rc.2 (+https://…)", rules) == "DSH-RC2"  // 先命中 sort 小的
// Match("ZCode/3.15.3 ai-sdk/…", rules)                  == "ZCode"      // 大小写不敏感
// Match("  zcode/3.14.5", rules)                         == "ZCode"      // 容忍前后空格
// Match("OpenAI/Python 2.24.0", rules)                   == "Other"
// Match("", rules)                                       == "Other"
// Match("   ", rules)                                    == "Other"
// NormalizePattern("  ZCode/ ") == "zcode/"
// NormalizePattern("a%b") / ("a_b") / ("") / strings.Repeat("x", 201) 均返回 error
// NormalizeName("Codex") == "Codex"（保留大小写）；NormalizeName("other") 返回 error
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestMatch -v`
Expected: FAIL（包不存在 / 符号未定义）

- [ ] **Step 3: 实现 `internal/domain/agentharness/rule.go`**

`Match` 用 `strings.HasPrefix(strings.ToLower(ua), rule.Pattern)` 顺序扫描，命中即返回；空白 UA 先 `TrimSpace` 再判空。校验函数用 `ierr.New(ierr.ErrValidation, …)`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/agent_harness/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/domain/agentharness test/unit/agent_harness
git commit -m "feat(agentharness): UA 前缀匹配与规则校验"
```

---

### Task 2: 规则表、仓储与种子

**Files:**
- Create: `internal/infrastructure/database/model/agent_harness_rule.go`
- Create: `internal/domain/agentharness/repository.go`
- Create: `internal/infrastructure/repository/agent_harness_rule_repository.go`
- Modify: `internal/infrastructure/database/model/base.go`（`Models` 追加 `&AgentHarnessRule{}`）
- Modify: `internal/infrastructure/database/postgresql.go`（AutoMigrate 后 `seedAgentHarnessRules`）
- Modify: `internal/bootstrap/modules/repository.go`（导出 `NewAgentHarnessRuleRepository`）
- Test: `test/unit/agent_harness/repository_test.go`

**Interfaces:**
- Consumes: `agentharness.Rule`、`agentharness.NormalizePattern/NormalizeName`（Task 1）
- Produces:
  - `internal/domain/agentharness/repository.go`:
    ```go
    type RuleRepository interface {
        List(ctx context.Context) ([]Rule, error)                // ORDER BY sort, id
        Get(ctx context.Context, id uint) (*Rule, error)         // 未找到返回 (nil, nil)
        Create(ctx context.Context, r Rule) (*Rule, error)       // 冲突返回 ierr.ErrConflict 或 ErrValidation
        Update(ctx context.Context, r Rule) error
        Delete(ctx context.Context, id uint) error               // 硬删除
        Fingerprint(ctx context.Context) (string, error)         // 见 Task 7
    }
    ```
  - `repository.NewAgentHarnessRuleRepository(db *gorm.DB) agentharness.RuleRepository`（不做任何缓存，每次直查小表）

- [ ] **Step 1: 写失败测试（GORM DryRun 钉 SQL 形态 + 校验行为）**

`test/unit/agent_harness/repository_test.go`：
- 用 `postgres.New(postgres.Config{Conn: nil, DriverName: "postgres", DSN: "host=127.0.0.1 user=x dbname=x sslmode=disable"})` + `gorm.Config{DryRun: true}` 构造 db（参照 `test/unit/session_list_split`）。
- 断言 `List` 生成 `SELECT * FROM "agent_harness_rules" ORDER BY sort, id`（用 `db.Session(&gorm.Session{DryRun:true}).ToSQL` 或捕获 callback 的 SQL 字符串）。
- 断言 `Create` 前 `Pattern` 已被 `NormalizePattern` 规范化（传 `"  ZCode/ "` 时 SQL 参数为 `zcode/`）。
- 断言 `Create` 传非法 pattern 时不生成 SQL、直接返回 `ierr.ErrValidation`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestRuleRepository -v`
Expected: FAIL（符号未定义）

- [ ] **Step 3: 实现模型与仓储**

模型字段：`Pattern string`（`uniqueIndex:idx_harness_rules_pattern`）、`Name string`（`index:idx_harness_rules_name`）、`Sort int default 0`；`BaseModel` 已提供 `ID/CreatedAt/UpdatedAt/DeletedAt`。
`Delete` 用 `db.Delete(&dbmodel.AgentHarnessRule{}, id)`（`DeletedAt` 是 int64，GORM 默认硬删除）。
`Fingerprint` 实现为单条聚合查询 `SELECT count(*), COALESCE(max(updated_at), 'epoch') FROM agent_harness_rules`，返回 `"<count>:<maxUpdated UTC UnixNano>"`；不做任何缓存（供 Task 7 的走势图缓存键使用）。
仓储层只做映射，规范化与校验放在应用层命令中（Task 6）——但 `Create/Update` 仍 defensively 校验一次，防止绕过。

- [ ] **Step 4: 加种子数据**

`postgresql.go` 的 `AutoMigrate` 末尾调用：

```go
func seedAgentHarnessRules(ctx context.Context, db *gorm.DB) error
```

表为空时插入 7 条：`deepseek-harness/`→`DSH`(10)、`zcode/`→`ZCode`(20)、`pi (`→`Pi`(30)、`codex_cli_rs/`→`Codex`(40)、`Codex Desktop/`→`Codex`(41)、`claude-cli/`→`Claude Code`(50)、`opencode/`→`OpenCode`(60)。用 `INSERT ... ON CONFLICT (pattern) DO NOTHING`，失败只记日志不阻塞启动。

- [ ] **Step 5: 跑测试 + 编译**

Run: `go test -count=1 ./test/unit/agent_harness/ -v && go build ./...`
Expected: PASS 且编译通过

- [ ] **Step 6: 提交**

```bash
git add internal/domain/agentharness internal/infrastructure internal/bootstrap test/unit/agent_harness
git commit -m "feat(agentharness): 规则表、仓储与种子数据"
```

---

### Task 3: `sessions.client_ua` 写路径

**Files:**
- Modify: `internal/infrastructure/database/model/session.go`（新增 `ClientUA`）
- Modify: `internal/common/constant/sql.go`（`FieldClientUA = "client_ua"`）
- Modify: `internal/infrastructure/pool/store_pool.go:94`（创建会话行时写 UA）
- Test: `test/unit/agent_harness/session_write_test.go`

**Interfaces:**
- Consumes: `constant.CtxKeyClient`（apikey/playground 中间件注入的原始 UA）
- Produces: `sessions.client_ua` 列；`dbmodel.Session.ClientUA string`

- [ ] **Step 1: 写失败测试**

`test/unit/agent_harness/session_write_test.go`：用 `sqlmock`/DryRun 捕获 `store_pool` 创建会话行的 INSERT，断言 `client_ua` 参数等于 ctx 中的 `CtxKeyClient`；ctx 无该键时为空串。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestSessionClientUA -v`
Expected: FAIL

- [ ] **Step 3: 实现**

模型加
```go
ClientUA string `json:"client_ua" gorm:"column:client_ua;not null;default:'';comment:创建会话的客户端 User-Agent"`
```
`store_pool.go` 创建 `dbmodel.Session{...}` 时加 `ClientUA: util.CtxValueString(task.Ctx, constant.CtxKeyClient)`。
**不要**把 `client_ua` 加进 `writeBackSnapshot`（`internal/infrastructure/repository/session_dedup.go`）的前滚字段。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestSessionClientUA -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/infrastructure/database/model/session.go internal/common/constant/sql.go internal/infrastructure/pool/store_pool.go test/unit/agent_harness
git commit -m "feat(session): 会话创建时固化原始 UA"
```

---

### Task 4: 读路径投影 + 展示（列表 / 详情 / 元数据）

**Files:**
- Modify: `internal/domain/session/repository.go`（两个 projection 加 `ClientUA`）
- Modify: `internal/infrastructure/repository/session_repository.go`（`sessionSummaryRow`、`SessionSummarySelect`、`paginateSessions`、`GetSessionDetail`、`GetSessionMeta`）
- Modify: `internal/common/constant/sql.go`（`SessionRepoFields*` 与 `SessionSummarySelect` 补 `client_ua`）
- Modify: `internal/application/session/port/handler.go`（`SessionSummaryView` / `SessionMetaView` / `SessionDetailView` 加 `AgentHarness`，meta 视图加 `ClientUA`）
- Modify: `internal/application/session/query/jwt_session_queries.go`（列表 + 详情两个 handler，注入 `agentharness.RuleRepository`，填充 `AgentHarness`）
- Modify: `internal/application/session/query/session_meta_query.go`（元数据 handler，填 `AgentHarness` + `ClientUA`）
- Modify: `internal/dto/session.go`（`SessionSummary`/`SessionDetail` 加 `AgentHarness`；`SessionMetadata` 加 `AgentHarness`、`ClientUA`）
- Modify: `internal/handler/session.go`（三处 handler 把字段搬进 DTO）
- Modify: `internal/bootstrap/modules/{application.go,handler.go}`（装配）
- Test: `test/unit/agent_harness/session_view_test.go`

**Interfaces:**
- Consumes: `agentharness.Match`、`agentharness.RuleRepository.List`、`RuleRepository`（Task 1/2）
- Produces:
  - `SessionSummaryView.AgentHarness string`、`SessionMetaView.AgentHarness / ClientUA string`、`SessionDetailView.AgentHarness string`
  - DTO 字段：`agentHarness`（三处）、`clientUA`（仅 `SessionMetadata`）

- [ ] **Step 1: 写失败测试**

`test/unit/agent_harness/session_view_test.go`：
- fake `RuleRepository` 返回 `[{Pattern:"zcode/",Name:"ZCode"}]`。
- 断言列表 handler 对两条 projection（`client_ua="ZCode/3.15.3 …"` 与 `""`）分别产出 `AgentHarness="ZCode"` 与 `"Other"`。
- 断言 meta handler 对同一 projection 返回 `ClientUA` 原值与 `AgentHarness`。
- 规则仓储报错时：列表仍返回数据，`AgentHarness` 全部为 `Other`，并记录 warn（fail-open，不 500）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestSessionView -v`
Expected: FAIL

- [ ] **Step 3: 实现投影与展示**

`SessionSummarySelect` 追加 `client_ua`；`paginateSessions` 的 `sessionSummaryRow` 加 `ClientUA string`；`GetSessionDetail`/`GetSessionMeta` 从 `SessionRepoFields*` 读到的记录填 `ClientUA`。
三个查询 handler 在拿到 projections 后调用一次 `rules.List(ctx)`，用 `agentharness.Match(p.ClientUA, rules)` 填 view；仓储错误按 fail-open 处理（打 warn，`AgentHarness = Other`）。

- [ ] **Step 4: 跑测试 + 全量单测**

Run: `go test -count=1 ./test/unit/... && go build ./...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/domain/session internal/infrastructure/repository/session_repository.go internal/common/constant/sql.go internal/application/session internal/dto/session.go internal/handler/session.go internal/bootstrap test/unit/agent_harness
git commit -m "feat(session): 会话列表/详情展示来源 harness"
```

---

### Task 5: `harness` 筛选（filter 钩子 + facet + 选项）

**Files:**
- Modify: `internal/common/filter/parser.go`（`FieldConfig.Custom`）
- Modify: `internal/common/constant/sql.go`（`SessionFilterFieldHarness = "harness"`、`SessionHarnessResolveSQL`）
- Modify: `internal/application/session/query/jwt_session_queries.go`（`sessionFieldConfigs` 加 harness）
- Modify: `internal/application/session/query/option_list.go`（返回规则名去重 + `Other`）
- Modify: `internal/dto/option.go`（`Field` 枚举加 `harness`）
- Test: `test/unit/agent_harness/filter_sql_test.go`

**Interfaces:**
- Consumes: `agent_harness_rules` 表（Task 2）
- Produces:
  - `filter.FieldConfig.Custom func(f Filter) (sql string, args []any, err error)`
  - SQL 片段常量：
    ```
    SessionHarnessResolveSQL = "(SELECT r.name FROM agent_harness_rules r WHERE lower(client_ua) LIKE r.pattern || '%' ORDER BY r.sort, r.id LIMIT 1)"
    ```

- [ ] **Step 1: 写失败测试**

`test/unit/agent_harness/filter_sql_test.go`（GORM DryRun 取 SQL）：
- `harness:ZCode` → SQL 含 `= "ZCode"` 的子查询条件，`args == ["ZCode"]`。
- `harness:Other` → SQL 以 `IS NULL` 结尾，无参数。
- `harness:!ZCode` → `((...) IS NULL OR (...) <> "ZCode")` 形态，包含 Other 与空 UA。
- `harness:>1` → 返回 `ierr.ErrValidation`（不支持比较符）。
- 存量 `client_ua = ''` 时 `harness:Other` 命中、`harness:ZCode` 不命中（用 sqlite/内存库或直接断言 SQL 语义注释 + e2e 兜底，二选一）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestHarnessFilterSQL -v`
Expected: FAIL

- [ ] **Step 3: 实现**

1. `FieldConfig` 加 `Custom` 字段；`buildCondition` 第一行 `if config.Custom != nil { return config.Custom(f) }`。
2. `sessionFieldConfigs[constant.SessionFilterFieldHarness] = filter.FieldConfig{Custom: buildHarnessCondition}`。
3. `buildHarnessCondition` 支持 `:` 与 `:!`，其余操作符返回 `ierr.ErrValidation`；`Other` 大小写不敏感匹配。
4. `option_list` 的 `harness` 分支：`rules.List` 取 `Name` 去重（保持 sort 顺序）后追加 `Other`；`Keyword` 过滤沿用现有 `strings.Contains` 逻辑。
5. 三处同步：DTO 枚举（`score,model,messageCount,harness`）、`sessionFieldConfigs`、常量。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/agent_harness/ -v && go test -count=1 ./test/unit/filter/ ./test/unit/session_option_list/`
Expected: PASS（既有 filter/option 测试不回归）

- [ ] **Step 5: 提交**

```bash
git add internal/common/filter internal/common/constant/sql.go internal/application/session internal/dto/option.go test/unit/agent_harness
git commit -m "feat(session): 支持按 agent harness 筛选会话"
```

---

### Task 6: 规则管理 admin API（含 UA 预览）

**Files:**
- Create: `internal/application/agentharness/port/handler.go`
- Create: `internal/application/agentharness/command/{create_rule.go,update_rule.go,delete_rule.go}.go`
- Create: `internal/application/agentharness/query/{list_rules.go,preview.go}.go`
- Create: `internal/handler/harness.go`
- Create: `internal/dto/harness.go`
- Create: `internal/router/harness.go`
- Modify: `internal/router/router.go`（注册分组）、`internal/bootstrap/modules/{application.go,handler.go}`
- Test: `test/unit/agent_harness/rule_command_test.go`

**Interfaces:**
- Consumes: `RuleRepository`（Task 2）、`NormalizePattern/NormalizeName/Match`（Task 1）
- Produces:
  - `port.ListHarnessRulesHandler`、`port.CreateHarnessRuleHandler`、`port.UpdateHarnessRuleHandler`、`port.DeleteHarnessRuleHandler`、`port.PreviewHarnessHandler`
  - 路由（`PermissionAdmin`）：`GET /harness/rule/list`、`POST /harness/rule`、`PATCH /harness/rule`、`DELETE /harness/rule`、`POST /harness/rule/preview`
  - `CreateHarnessRuleCommand{ Pattern, Name string; Sort int }`、`UpdateHarnessRuleCommand{ ID uint; Pattern, Name string; Sort int }`
  - `PreviewHarnessQuery{ UA string }` → `PreviewResult{ Harness string; Rule *agentharness.Rule }`

- [ ] **Step 1: 写失败测试**

`test/unit/agent_harness/rule_command_test.go`：
- `Create` 传 `"  ZCode/ "` → 仓储收到的 `Pattern == "zcode/"`。
- `Create` 传 `Name: "other"` → `ierr.ErrValidation`，仓储未被调用。
- `Update` 传不存在的 ID → `ierr.ErrDataNotExists`。
- `Preview{UA: "ZCode/3.15.3 …"}`（规则 `zcode/`→`ZCode`）→ `Harness == "ZCode"`、`Rule.Pattern == "zcode/"`；`Preview{UA: ""}` → `Harness == "Other"`、`Rule == nil`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestHarnessRuleCommand -v`
Expected: FAIL

- [ ] **Step 3: 实现命令/查询 + DTO + handler + router**

DTO 按 `huma-dto-conventions` 的 Body 模板（`Body struct{...}` 包装）；删除用 `DELETE` + body `{id}`；错误用 `ierr`；路由分组沿用 `router.go` 里 web v1 分组，操作 ID：`listHarnessRules`/`createHarnessRule`/`updateHarnessRule`/`deleteHarnessRule`/`previewHarnessRule`。

- [ ] **Step 4: 跑测试 + 编译**

Run: `go test -count=1 ./test/unit/agent_harness/ -v && go build ./...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/application/agentharness internal/handler/harness.go internal/dto/harness.go internal/router internal/bootstrap test/unit/agent_harness
git commit -m "feat(harness): 规则管理 admin API 与 UA 预览"
```

---

### Task 7: 审计 harness 走势图

**Files:**
- Create: `internal/domain/modelcall/vo/harness_trend.go`（`type HarnessTrendPoint struct { Harness string; Time time.Time; Count int }`）
- Create: `internal/application/audit/query/harness_trend.go`
- Modify: `internal/application/audit/port/handler.go`（`HarnessTrendHandler`）
- Modify: `internal/application/audit/query/service.go`（注入 + 转发）
- Modify: `internal/infrastructure/repository/audit_repository.go`（`QueryHarnessTrend` + `loadHarnessTrend`）
- Modify: `internal/handler/audit.go`、`internal/dto/audit.go`、`internal/router/audit.go`
- Modify: `internal/bootstrap/modules/application.go`
- Test: `test/unit/agent_harness/audit_trend_test.go`

**Interfaces:**
- Consumes: `RuleRepository.Fingerprint(ctx)`（Task 2）
- Produces:
  - `port.HarnessTrend(ctx, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*vo.HarnessTrendPoint, error)`
  - 仓储：`QueryHarnessTrend(ctx, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity, rulesFingerprint string)`
  - 路由：`GET /audit/stats/harness/trend?startTime&endTime&granularity`，权限同 `queryModelTrend`
  - DTO：`HarnessTrendRsp{ Series []*HarnessTrendSeries }`，`HarnessTrendSeries{ Harness string; Points []*HarnessTrendItem }`，`HarnessTrendItem{ Time time.Time; Count int }`，`Other` 排最后，时间桶零值补齐

- [ ] **Step 1: 写失败测试**

`test/unit/agent_harness/audit_trend_test.go`（GORM DryRun）：
- 断言 SQL 含 `COALESCE((SELECT r.name FROM agent_harness_rules r WHERE lower(user_agent) LIKE r.pattern || '%' ORDER BY r.sort, r.id LIMIT 1), 'Other')` 与 `GROUP BY`。
- 断言 `apiKeyIDs` 为 nil 时不加 `IN` 条件、为空非 nil 时短路返回空（与 `loadModelTrend` 同规则）。
- 断言缓存键包含 fingerprint：同一请求换 fingerprint 得到不同 `readCacheKey(...)`（导出 helper 或断言两次 `cachedSlice` 的 key 字符串）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestHarnessTrend -v`
Expected: FAIL

- [ ] **Step 3: 实现**

`loadHarnessTrend` 只统计 `created_at` 区间 + `deleted_at = 0`（**不加** upstream 成功条件）；`QueryHarnessTrend` 用
`readCacheKey(constant.ReadCachePurposeAuditStatsHarness, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(startTime), cacheTimeBucket(endTime), granularity, rulesFingerprint)`。
新增常量 `ReadCachePurposeAuditStatsHarness = "audit:stats:harness"`。
handler 侧按 `Other` 最后、harness 名升序整理并零值补齐（参照 `port.FillTrendSeries` 的写法）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/agent_harness/ -v && go build ./...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/domain/modelcall internal/application/audit internal/infrastructure/repository/audit_repository.go internal/handler/audit.go internal/dto/audit.go internal/router/audit.go internal/common/constant internal/bootstrap test/unit/agent_harness
git commit -m "feat(audit): 按 harness 的调用量走势统计"
```

---

### Task 8: 修复 demo 视角的会话筛选

**Files:**
- Modify: `internal/domain/session/repository.go`（`ListSessionsByIDs` 签名）
- Modify: `internal/infrastructure/repository/session_repository.go`（抽 `applySessionListFilters`，两个方法共用）
- Modify: `internal/application/session/query/jwt_session_queries.go`（demo 分支传条件）
- Test: `test/unit/agent_harness/demo_filter_test.go`

**Interfaces:**
- Produces:
  - `applySessionListFilters(db *gorm.DB, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) (*gorm.DB, error)`
  - `ListSessionsByIDs(ctx, ids []uint, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria)`

- [ ] **Step 1: 写失败测试**

`test/unit/agent_harness/demo_filter_test.go`（GORM DryRun）：
- `ListSessionsByIDs(ids, param, start, end, "kw", criteria{harness:ZCode})` 生成的 SQL 同时含 `id IN (...)`、`created_at >= ?`、keyword 子查询、harness 子查询。
- 既有 `ListSessionsByOwnerIDs` 生成的 SQL 与改动前逐字一致（回归钉死）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/agent_harness/ -run TestDemoSessionFilter -v`
Expected: FAIL

- [ ] **Step 3: 实现**

从 `ListSessionsByOwnerIDs` 提取 WHERE 构造为 `applySessionListFilters`（scope 由调用方先加），两个方法都调用；handler 的 demo 分支补传 `q.StartTime/q.EndTime/q.Keyword/criteria`。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/agent_harness/ -v && go test -count=1 ./test/unit/session_...`
Expected: PASS（`session_list_split`、`session_keyword_filter` 等既有用例不回归）

- [ ] **Step 5: 提交**

```bash
git add internal/domain/session internal/infrastructure/repository/session_repository.go internal/application/session test/unit/agent_harness
git commit -m "fix(session): demo 视角的会话筛选生效"
```

---

### Task 9: Web 会话列表（列 + facet）

**Files:**
- Modify: `web/src/lib/types.ts`（`SessionSummary.agentHarness?`）
- Modify: `web/src/lib/api-client.ts`（无新接口，确认 filter 透传）
- Modify: `web/src/app/(dashboard)/sessions/page.tsx`（facet + 列 + 移动端卡片）
- Modify: `web/src/locales/{zh,en,ja}.json`（`sessions.filter_harness`、`sessions.col_harness`）
- Test: `web/src/components/shadcn-native.test.ts`（追加断言）

**Interfaces:**
- Consumes: 会话列表响应 `agentHarness`、option list `field=harness`

- [ ] **Step 1: 写失败测试**

在 `shadcn-native.test.ts` 追加：sessions 页源码必须包含 `key: "harness"` 的 facet 定义与 `agentHarness` 的渲染（源码级断言，沿用该文件既有风格）。

- [ ] **Step 2: 跑测试确认失败**

Run: `npm run test -- --run shadcn-native`
Expected: FAIL

- [ ] **Step 3: 实现**

facet：`{ key: "harness", label: t("sessions.filter_harness"), options: fetchOptionsFor("harness") }`。
列：表头加「Agent」，单元格渲染 `<Badge variant={h === "Other" ? "outline" : "secondary"}>{h}</Badge>`；移动端 Item 里同样渲染。

- [ ] **Step 4: 跑测试 + lint**

Run: `npm run test -- --run && npm run lint`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add web/src
git commit -m "feat(web): 会话列表展示并筛选 agent harness"
```

---

### Task 10: Web 会话详情（徽标 + UA tooltip）

**Files:**
- Modify: `web/src/components/session-detail/session-detail-client.tsx`
- Modify: `web/src/lib/types.ts`（`SessionMetadata.agentHarness?` / `clientUA?`）
- Modify: `web/src/locales/{zh,en,ja}.json`
- Test: `web/src/components/shadcn-native.test.ts`

- [ ] **Step 1: 写失败测试**：断言详情组件源码包含 `agentHarness` 与 `clientUA` 的 tooltip 渲染。
- [ ] **Step 2: 跑测试确认失败**：`npm run test -- --run shadcn-native` → FAIL
- [ ] **Step 3: 实现**：头部徽标 + `<Tooltip>` 显示 `clientUA || t("sessions.ua_missing")`；`Other` 用 outline 样式。
- [ ] **Step 4: 跑测试 + lint**：PASS
- [ ] **Step 5: 提交**：`git commit -m "feat(web): 会话详情展示 harness 与原始 UA"`

---

### Task 11: Web Dashboard 走势图

**Files:**
- Create: `web/src/components/charts/harness-trend-chart.tsx`
- Modify: `web/src/app/(dashboard)/page.tsx`（加入卡片）
- Modify: `web/src/lib/api-client.ts`（`getHarnessTrend`）
- Modify: `web/src/lib/types.ts`、`web/src/locales/{zh,en,ja}.json`
- Test: `web/src/components/shadcn-native.test.ts`

- [ ] **Step 1: 写失败测试**：断言 dashboard 页引用 `HarnessTrendChart` 且该组件使用堆叠面积（`stackId`）与固定 `Other` 颜色。
- [ ] **Step 2: 跑测试确认失败**：FAIL
- [ ] **Step 3: 实现**：沿用 `model-trend-chart.tsx` 的时间桶/粒度/刷新逻辑，`type="monotone"` + `stackId="harness"`；图例顺序按接口返回顺序（`Other` 最后）。
- [ ] **Step 4: 跑测试 + lint + build**：`npm run test -- --run && npm run lint && npx next build --webpack` → PASS
- [ ] **Step 5: 提交**：`git commit -m "feat(web): dashboard 增加 harness 调用量走势图"`

---

### Task 12: Web 规则管理页

**Files:**
- Create: `web/src/app/(dashboard)/harness/page.tsx`
- Modify: `web/src/lib/api-client.ts`（list/create/update/delete/preview）
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/components/dashboard-nav*`（导航项，admin only）
- Modify: `web/src/locales/{zh,en,ja}.json`
- Test: `web/src/components/shadcn-native.test.ts`

- [ ] **Step 1: 写失败测试**：断言页面存在预览输入框与（pattern / name / sort）表格列，且导航项 `admin` 可见性判断。
- [ ] **Step 2: 跑测试确认失败**：FAIL
- [ ] **Step 3: 实现**：表格 + 新增/编辑弹窗（`sort` 数字输入）+ 删除二次确认 + 顶部「UA 预览」（调用 `POST /harness/rule/preview`，展示命中规则与最终 harness）。
- [ ] **Step 4: 跑测试 + lint + build**：PASS
- [ ] **Step 5: 提交**：`git commit -m "feat(web): harness 规则管理页"`

---

### Task 13: E2E

**Files:**
- Create: `test/e2e/agent_harness/agent_harness_test.go`
- Create: `test/e2e/session_demo_filter/session_demo_filter_test.go`

**Interfaces:**
- Consumes: 全部 HTTP 接口；参照 `test/e2e/client_models/client_models_test.go` 的登录/建 Key/发请求骨架。

- [ ] **Step 1: 写用例**

`agent_harness`：
1. admin 新建规则 `harness-e2e/`→`E2EHarness`；带 `User-Agent: harness-e2e/1.0` 发一次模型调用；会话列表出现该会话且 `agentHarness == "E2EHarness"`；`filter=harness:E2EHarness` 命中。
2. 把规则改名 `E2EHarness2` → **不发版、不重启**，同一会话 `agentHarness` 变为 `E2EHarness2`（历史自动重标）。
3. 删除规则 → 该会话回到 `Other`。
4. 走势图接口返回 `E2EHarness` 序列；改规则后同一 `[start,end]` 立即反映（缓存指纹）。
5. 普通用户调规则管理接口 → 403。

`session_demo_filter`：
6. demo 账户 `filter=harness:*` / `score` / 时间范围 / `keyword` 均能缩小结果；结果仍落在 `id % K == 0` 抽样集合内。

- [ ] **Step 2: 跑用例确认失败**

Run: `go test -count=1 ./test/e2e/agent_harness/ ./test/e2e/session_demo_filter/ -v`
Expected: 在未部署新代码的环境失败（记录基线）

- [ ] **Step 3: 跑通**

部署到测试环境后重跑，Expected: PASS。

- [ ] **Step 4: 提交**

```bash
git add test/e2e/agent_harness test/e2e/session_demo_filter
git commit -m "test(e2e): agent harness 识别与 demo 筛选"
```

---

### Task 14: 文档、审查与收尾

**Files:**
- Modify: `CONTEXT.md`（新增词条 `AgentHarness`、`HarnessRule`；在 Session 词条补 `client_ua` 语义）
- Modify: `docs/superpowers/specs/2026-10-27-agent-harness-identification-design.md`（如有实施期偏差）

- [ ] **Step 1: 更新 CONTEXT.md**（中文词条，`_Avoid_` 行按现有风格）
- [ ] **Step 2: 全量验证**

Run: `make lint && go test -count=1 ./test/unit/...`
Run: `cd web && npm run lint && npm run test -- --run && npx next build --webpack`
Expected: 全绿（存量 warning 不算失败）

- [ ] **Step 3: `ponytail-review` 审查 diff**，逐条处理过度工程项
- [ ] **Step 4: 用 Serena 写本次工程经验 memory**（UA 派生值设计、SQL 子查询口径、demo 筛选既有缺陷）
- [ ] **Step 5: 提交**

```bash
git add CONTEXT.md docs
git commit -m "docs: 补充 agent harness 领域词条与实施偏差"
```

---

## 交付后待办（不在本计划内）

- 发布说明需点明：demo 视角的评分/模型/消息数/时间范围/关键词筛选开始生效。
- 生产 `sessions.client_ua` 由 AutoMigrate 加列，无回填；存量会话永久 `Other`。
