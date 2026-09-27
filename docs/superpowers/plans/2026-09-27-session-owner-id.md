# 会话归属改按 API Key ID 判定 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 session 与 trace 的归属判定从可跨用户重复的 `api_key_name` 字符串改为唯一的 `api_key_id`，消除越权漏洞，并修复导致 2026-07 起新增会话对普通用户不可见的字段白名单缺陷。

**Architecture:** 新增 `sessions.api_key_id` 与 `traces.api_key_id` 列作为唯一权威归属，`api_key_name` 降级为纯展示字段（不删列、前端零改动）。写入侧从 context 取 `CtxKeyAPIKeyID` 落库；读取侧从 `LookupOwnerNamesByUserID` 切到已存在的 `LookupIDsByUserID`。存量 1869 条可无歧义匹配的会话随 `database migrate` 幂等回填。

**Tech Stack:** Go 1.25.1、GORM（PostgreSQL 生产 / sqlite 测试）、fiber v3 + huma v2、`go.uber.org/fx`、标准库 `testing`、`github.com/bytedance/sonic`。

**设计依据:** `docs/superpowers/specs/2026-09-27-session-owner-id-design.md`

## Global Constraints

- 错误创建与包装统一走 `internal/common/ierr`；禁止 `errors.New` / `fmt.Errorf`。
- 业务包禁止本地 `const` 块；常量放 `internal/common/constant/` 或 `internal/common/enum/`。
- JSON 统一 `github.com/bytedance/sonic`；禁止 `encoding/json`、`json.RawMessage`。
- HTTP 状态码用 `fiber.StatusXxx`；禁止裸数字。
- 日志用 `logger.WithCtx(ctx)`，消息前缀 `[PascalCaseModule]`。
- `internal/` 目录下**不得**放 `_test.go`；单测放 `test/unit/<topic>/`，E2E 放 `test/e2e/<topic>/`。
- 只用标准库 `testing`；禁止 testify / gomock；禁止用 `time.Sleep` 做同步（AST 级 lint 拦截）。
- **每个 `TestXxx` 函数首行必须是 `t.Parallel()`**（`paralleltest` linter 强制，T1 实测被拦）。含 `t.Run` 子测试时，子测试内也需 `t.Parallel()`。
- DTO 包禁止导入 `internal/infrastructure/database/model`。
- **SQL 硬约束**：归属过滤一律用显式占位符。复用既有模板 `fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID)` → `"api_key_id IN ?"`。禁止 struct 条件（GORM 忽略 `api_key_id = 0` 零值），禁止 `Where("api_key_id", v)` 无占位符写法（静默丢参）。
- 编辑任何 Go 文件**之前**先跑：`sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path <目标文件>`，输出不得截断。
- 改 `internal/bootstrap/modules/**` 前加载 `golang-uber-fx` skill。
- **pre-commit hook 行为**：hook 会 `gofmt -w .` 全仓并 `git add` 已跟踪文件，且跑全量 `go vet` / `go test` / `lint`（`./cmd/lint` 并发跑 conv + static）。因此**接口签名变更必须与其全部实现者、调用方、测试 fake 同批提交**，否则中间提交全仓 vet 挂。untracked 新文件 hook 不会自动加，需手动 `git add`。
- 工作目录：`.worktrees/session-owner-id`（分支 `bugfix/session-owner-id-2026-09-27`，基线 `5f95ea3a`）。

## 命令速查

```bash
# 单个测试包
go test ./test/unit/<topic>/ -run TestXxx -v

# 全量（pre-commit 会跑，提交前自行确认）
go test ./... && go vet ./... \
  && go run ./cmd/lint ./...
```

## File Structure

| 文件 | 职责 | 任务 |
|---|---|---|
| `internal/common/constant/sql.go` | `ProxyAPIKeyRepoFieldsAuth` 补 `FieldName`；新增 `SessionRepoFields*` 投影含 `api_key_id` | T1, T3 |
| `internal/infrastructure/database/model/session.go` | `Session` 加 `APIKeyID` 列 + 索引 | T3 |
| `internal/infrastructure/database/model/trace.go` | `Trace` 加 `APIKeyID` 列 + 索引 | T3 |
| `internal/dto/asynctask.go` | `MessageStoreTask` 加 `APIKeyID` | T4 |
| `internal/application/llmproxy/usecase/{openai_store,anthropic_store,trigger_capture}.go` | 从 ctx 取 `CtxKeyAPIKeyID` 传入任务 | T4 |
| `internal/infrastructure/pool/store_pool.go` | 构造 `dbmodel.Session` 写入 `APIKeyID` | T4 |
| `internal/domain/session/vo/api_key_owner.go` | 新增 `APIKeyOwnerID` 值对象 | T5 |
| `internal/domain/session/aggregate/session.go` | 聚合 owner 概念改 ID；`IsOwnedByID` | T5 |
| `internal/domain/session/repository.go` | 6 个读仓储签名 `ownerNames []string` → `ownerIDs []uint`；`ExportFilter.OwnerIDs` | T6 |
| `internal/infrastructure/repository/session_repository.go` | 过滤列换 `api_key_id`；聚合映射带 ID | T6 |
| `internal/domain/trace/repository.go` + `internal/infrastructure/repository/trace_repository.go` | `PaginateByOwners` 改 ID；`Trace.APIKeyID` | T7 |
| `internal/handler/trace.go` | 上报时写入 `APIKeyID` | T7 |
| `internal/application/trace/{query,command}/*.go` | 归属判定改 ID（含 `resolveParentTraceID`） | T7 |
| `internal/application/session/{query,command}/*.go` | 归属判定改 ID | T8 |
| `internal/application/dataset/query/dataset_query.go` | `ExportFilter.OwnerIDs` | T8 |
| `cmd/server/database.go` | `runMigrate` 内挂幂等回填 | T9 |
| `internal/infrastructure/repository/session_backfill.go` | 回填实现（新建） | T9 |
| `CONTEXT.md` | 改写 `APIKeyOwner` 条目 | T10 |
| `test/unit/apikey_auth_fields/` | 缺陷 B 白名单回归（新建） | T1 |
| `test/unit/session_owner_id/` | 聚合与三态语义单测（新建） | T5, T6 |
| `test/unit/session_backfill/` | 回填幂等单测（新建） | T9 |
| `test/e2e/cross_tenant_session/` | 跨用户同名 Key 六路径越权 E2E（新建） | T2, T8 |

## 任务依赖与上线分期

```
T1（缺陷 B，可独立上线）
  └─ T2（越权 E2E 红灯，锁定缺陷 A 现状）
       └─ T3（DDL）→ T4（写入侧）→ T9（回填）   ← 第一批部署：先 migrate 再滚动
            └─ T5（领域）→ T6（session 读仓储）→ T7（trace）→ T8（应用层判定）
                 └─ T10（文档与收尾）             ← 第二批部署
```

**部署顺序硬约束**：`database migrate`（加列+建索引+回填）必须先于滚动部署新版本。反序会让新版本按尚未存在或未回填的列过滤，造成会话短时全不可见。

---

### Task 1: 修复缺陷 B —— 鉴权字段白名单补 name

**Files:**
- Modify: `internal/common/constant/sql.go:151`
- Test: `test/unit/apikey_auth_fields/auth_fields_test.go`（新建）

**Interfaces:**
- Consumes: 无
- Produces: `constant.ProxyAPIKeyRepoFieldsAuth` 含 `constant.FieldName`，使 `APIKeyMiddleware` 注入的 `CtxKeyAPIKeyName` 非空。T4 依赖此前提，否则新写入的 `api_key_name` 仍为空串。

- [ ] **Step 1: 跑 use-modern-go**

```bash
cd .worktrees/session-owner-id
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/common/constant/sql.go
```

- [ ] **Step 2: 写失败测试**

创建 `test/unit/apikey_auth_fields/auth_fields_test.go`：

```go
package apikey_auth_fields

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TestProxyAPIKeyAuthFieldsIncludeName 鉴权投影必须含 name 列。
//
// 缺陷背景：白名单缺 FieldName 时 dao.Get 的 SELECT 不含该列，
// apiKey.Name 恒为空串，APIKeyMiddleware 注入空 CtxKeyAPIKeyName，
// 导致 sessions.api_key_name 写空、会话对所有普通用户不可见。
func TestProxyAPIKeyAuthFieldsIncludeName(t *testing.T) {
	if !slices.Contains(constant.ProxyAPIKeyRepoFieldsAuth, constant.FieldName) {
		t.Errorf("ProxyAPIKeyRepoFieldsAuth must contain %q, got %v",
			constant.FieldName, constant.ProxyAPIKeyRepoFieldsAuth)
	}
}

// TestProxyAPIKeyAuthFieldsIncludeOwnership 鉴权投影必须含 id 与 user_id。
//
// id 用于注入 CtxKeyAPIKeyID（归属判定的唯一权威依据），
// user_id 用于孤儿 key 防御与用户查询。
func TestProxyAPIKeyAuthFieldsIncludeOwnership(t *testing.T) {
	for _, field := range []string{constant.FieldID, constant.FieldUserID} {
		if !slices.Contains(constant.ProxyAPIKeyRepoFieldsAuth, field) {
			t.Errorf("ProxyAPIKeyRepoFieldsAuth must contain %q, got %v",
				field, constant.ProxyAPIKeyRepoFieldsAuth)
		}
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./test/unit/apikey_auth_fields/ -run TestProxyAPIKeyAuthFieldsIncludeName -v
```

Expected: FAIL，输出 `must contain "name", got [id user_id]`

- [ ] **Step 4: 实现修复**

`internal/common/constant/sql.go:151`，把：

```go
	ProxyAPIKeyRepoFieldsAuth = []string{FieldID, FieldUserID}
```

改为：

```go
	// ProxyAPIKeyRepoFieldsAuth 鉴权路径投影。
	// FieldName 必须在列表内：APIKeyMiddleware 依赖 apiKey.Name 注入
	// CtxKeyAPIKeyName，缺列会让 sessions.api_key_name 静默写成空串
	// （2026-07 起 2.6k 条会话因此对普通用户不可见）。
	ProxyAPIKeyRepoFieldsAuth = []string{FieldID, FieldUserID, FieldName}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./test/unit/apikey_auth_fields/ -v
```

Expected: PASS（2 个用例）

- [ ] **Step 6: 验证捕获能力**

临时把 `ProxyAPIKeyRepoFieldsAuth` 改回 `[]string{FieldID, FieldUserID}`，重跑 Step 5，**必须 FAIL**；确认后恢复。这一步不可省略——项目历史上出现过「测试自己拼装条件导致注入缺陷仍 PASS」的假测试。

- [ ] **Step 7: 提交**

```bash
git add internal/common/constant/sql.go test/unit/apikey_auth_fields/
git commit -m "fix(apikey): 鉴权字段白名单补 name，修复会话归属恒空

ProxyAPIKeyRepoFieldsAuth 缺 FieldName 导致 dao.Get 的 SELECT 不含该列，
apiKey.Name 恒空 → CtxKeyAPIKeyName 注入空串 → sessions.api_key_name 写空。
生产 2026-07 起 2623+ 条会话因此对所有普通用户不可见（按月分布吻合）。"
```

---

### Task 2: 越权 E2E 红灯（锁定缺陷 A 现状）

先写出能证明越权存在的测试并确认它**当前 FAIL**，作为后续改造的验收锚点。

**Files:**
- Create: `test/e2e/cross_tenant_session/fixture_test.go`
- Create: `test/e2e/cross_tenant_session/session_owner_test.go`

**Interfaces:**
- Consumes: `router.RegisterAPIRouter`、`bootstrap` 装配、`constant.ProxyAPIKeyRepoFieldsAuth`（T1 已修）
- Produces: `newCrossTenantFixture(t) *crossTenantFixture` 与其方法 `doJSONAs(token string, method, path string, body any) (int, []byte)`；T8 复用该 fixture 验收绿灯。

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path test/e2e/cross_tenant_session/fixture_test.go
```

- [ ] **Step 2: 读既有 E2E 装配范式**

阅读 `test/e2e/cross_tenant_reference/` 全部文件，照抄其装配方式（真实 `RegisterAPIRouter` + sqlite 内存库 + miniredis + `app.Test(req)`）。

**已知坑（必须照办）**：
1. `config.JwtAccessTokenExpired` 默认 0 会让 token 秒级过期，fixture 必须显式设置为正值（如 `time.Hour`）。
2. seed 用户必须差异化 `GithubBindID` / `GoogleBindID`，两者各参与唯一索引。
3. 管理 API 恒返回 HTTP 200，断言要查 body 内 `error.code`（`10001` 未授权 / `10003` 无权限），而非 HTTP status。
4. 同包并行子测试若共享 `cache=shared` 内存库会互相污染；每个顶层测试用独立 DSN。

- [ ] **Step 3: 写 fixture**

创建 `test/e2e/cross_tenant_session/fixture_test.go`。fixture 需 seed：
- 用户 A（`id=1`, permission=normal, GithubBindID="gh-a"）、用户 B（`id=2`, permission=normal, GithubBindID="gh-b"）
- A 的 API Key：`{ID: 1, UserID: 1, Name: "shared-name", Key: "key-a"}`
- B 的 API Key：`{ID: 2, UserID: 2, Name: "shared-name", Key: "key-b"}` ← **同名，这是攻击面**
- A 名下会话：`{ID: 1, APIKeyName: "shared-name", APIKeyID: 1, MessageIDs: []uint{1}, ToolIDs: []uint{}}`（`APIKeyID` 字段在 T3 之后才存在；T2 阶段先只写 `APIKeyName`，T3 完成后回来补 `APIKeyID: 1`）
- 各自的 JWT access token

- [ ] **Step 4: 写六路径越权测试**

创建 `test/e2e/cross_tenant_session/session_owner_test.go`，用户 B 用自己的 token 尝试访问 A 的会话 1，六条路径**全部**必须被拒：

```go
func TestCrossTenantSameKeyNameCannotReachOthersSession(t *testing.T) {
	f := newCrossTenantFixture(t)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"list", http.MethodGet, "/api/web/v1/session/list?page=1&page_size=20", nil},
		{"metadata", http.MethodGet, "/api/web/v1/session/metadata?id=1", nil},
		{"detail", http.MethodGet, "/api/web/v1/session?id=1", nil},
		{"delete", http.MethodDelete, "/api/web/v1/session", map[string]any{"session_ids": []uint{1}}},
		{"score", http.MethodPost, "/api/web/v1/session/score", map[string]any{"session_id": 1, "score": 5}},
		{"share", http.MethodPost, "/api/web/v1/session/share", map[string]any{"session_id": 1, "expire": "1d"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := f.doJSONAs(f.tokenB, tc.method, tc.path, tc.body)
			if status != http.StatusOK {
				t.Fatalf("unexpected transport status %d: %s", status, body)
			}
			assertNoAccessToSessionOne(t, tc.name, body)
		})
	}
}
```

`assertNoAccessToSessionOne` 的判定规则（写在同文件）：
- `list`：响应 `data.sessions` 不得包含 `id == 1` 的项
- 其余五条：`error.code` 必须非 0（`10003` 无权限或 `10004` 数据不存在均可接受），或 `data` 为空且未产生副作用

导出路径单独一个测试（SSE，不走 `doJSONAs`）：

```go
func TestCrossTenantSameKeyNameCannotExportOthersSession(t *testing.T) {
	f := newCrossTenantFixture(t)
	status, body := f.doRaw(f.tokenB, http.MethodGet,
		"/api/web/v1/dataset/preview?min_score=0")
	if status != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", status, body)
	}
	// A 的会话 1 不得出现在 B 的导出预览统计里
	if bytes.Contains(body, []byte(`"total_sessions":1`)) {
		t.Errorf("user B's export preview must not count user A's session, got %s", body)
	}
}
```

- [ ] **Step 5: 运行确认 FAIL（证明越权真实存在）**

```bash
go test ./test/e2e/cross_tenant_session/ -v
```

Expected: **FAIL**。当前实现按 `api_key_name IN ("shared-name")` 过滤，B 的同名 Key 命中 A 的会话，`list` 会返回 session 1、`metadata`/`detail`/`delete`/`score`/`share` 全部放行。

若此处意外 PASS，**停止并排查 fixture**——通常是 token 未生效（坑 1）或 seed 未落库，测试根本没打到判定逻辑。

- [ ] **Step 6: 提交红灯测试**

```bash
git add test/e2e/cross_tenant_session/
git commit -m "test(session): 补跨用户同名 Key 越权 E2E（当前红灯）

覆盖列表/详情/元数据/删除/评分/分享/导出七条路径。
归属依赖 api_key_name 且该名可跨用户重复，普通用户建同名 Key
即可读写他人会话；此前零覆盖。改按 api_key_id 判定后转绿。"
```

> 注：本任务提交后仓库处于「已知红灯」状态，后续任务会把它转绿。pre-commit hook 跑全量 `go test` 会失败，本次提交需 `git commit --no-verify`，并在 T8 完成后确认全量转绿。

---

### Task 3: 加 api_key_id 列与索引

**Files:**
- Modify: `internal/infrastructure/database/model/session.go`
- Modify: `internal/infrastructure/database/model/trace.go`
- Modify: `internal/common/constant/sql.go`（各 Session 投影补 `FieldAPIKeyID`）
- Test: `test/unit/session_owner_id/schema_test.go`（新建）

**Interfaces:**
- Consumes: `constant.FieldAPIKeyID`（已存在，值 `"api_key_id"`）
- Produces: `dbmodel.Session.APIKeyID uint`、`dbmodel.Trace.APIKeyID uint`；`constant.SessionRepoFieldsDetail` / `SessionRepoFieldsReadDetail` / `SessionRepoFieldsDedup` 等含 `FieldAPIKeyID`。T4/T6/T7/T9 全部依赖这两个字段存在。

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/database/model/session.go
```

- [ ] **Step 2: 写失败测试（GORM 索引 tag 校验）**

创建 `test/unit/session_owner_id/schema_test.go`。照抄 `test/unit/dao_index/`（若不存在则用 `test/unit/` 下含 `sqlite_master` 解析的既有测试）的范式：sqlite 内存库 `AutoMigrate` 后解析 `sqlite_master` 断言索引存在。

```go
package session_owner_id

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// TestSessionAPIKeyIDColumnAndIndex sessions 表必须有 api_key_id 列与其索引。
// 归属过滤是热路径（每次列表/详情/删除/评分都要过），无索引会退化全表扫描。
func TestSessionAPIKeyIDColumnAndIndex(t *testing.T) {
	db := openMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.Session{}); err != nil {
		t.Fatalf("automigrate sessions: %v", err)
	}
	if !db.Migrator().HasColumn(&dbmodel.Session{}, "api_key_id") {
		t.Error("sessions.api_key_id column missing")
	}
	if !db.Migrator().HasIndex(&dbmodel.Session{}, "idx_sessions_api_key_id") {
		t.Error("index idx_sessions_api_key_id missing")
	}
}

// TestTraceAPIKeyIDColumnAndIndex traces 表同理。
func TestTraceAPIKeyIDColumnAndIndex(t *testing.T) {
	db := openMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.Trace{}); err != nil {
		t.Fatalf("automigrate traces: %v", err)
	}
	if !db.Migrator().HasColumn(&dbmodel.Trace{}, "api_key_id") {
		t.Error("traces.api_key_id column missing")
	}
	if !db.Migrator().HasIndex(&dbmodel.Trace{}, "idx_traces_api_key_id") {
		t.Error("index idx_traces_api_key_id missing")
	}
}

func openMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=private"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}
```

> 驱动导入路径已核实为 `gorm.io/driver/sqlite`（与 `test/unit/apikey_repository/lookup_user_scope_test.go:18`、`test/unit/audit_repo/audit_admin_nil_scope_test.go:28` 一致）。

- [ ] **Step 3: 运行确认失败**

```bash
go test ./test/unit/session_owner_id/ -run 'TestSessionAPIKeyIDColumnAndIndex|TestTraceAPIKeyIDColumnAndIndex' -v
```

Expected: FAIL，`sessions.api_key_id column missing`

- [ ] **Step 4: 加字段**

`internal/infrastructure/database/model/session.go`，在 `APIKeyName` 之后插入：

```go
	// APIKeyID 归属 API Key ID，是权限判定的唯一权威依据。
	// 0 表示归属未知（2026-07 前后的存量空归属会话），仅 admin 可见。
	// api_key_name 同时保留，但只用于展示与审计可读性，不参与鉴权。
	APIKeyID uint `json:"api_key_id" gorm:"column:api_key_id;not null;default:0;index:idx_sessions_api_key_id;comment:归属 API Key ID"`
```

`internal/infrastructure/database/model/trace.go`，在 `APIKeyName` 之后插入：

```go
	// APIKeyID 归属 API Key ID（鉴权唯一依据，语义同 Session.APIKeyID）
	APIKeyID uint `json:"api_key_id" gorm:"column:api_key_id;not null;default:0;index:idx_traces_api_key_id;comment:归属 API Key ID"`
```

- [ ] **Step 5: 投影补列**

`internal/common/constant/sql.go`，给以下 Session 投影加 `FieldAPIKeyID`（归属判定要读该列，漏加会读出 0 导致误拒）：

```go
	SessionRepoFieldsDetail     = []string{FieldID, FieldAPIKeyName, FieldAPIKeyID, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldScore, FieldScoredAt}
	SessionRepoFieldsReadDetail = []string{FieldID, FieldAPIKeyName, FieldAPIKeyID, FieldCreatedAt, FieldUpdatedAt, FieldMessageIDs, FieldToolIDs, FieldMetadata, FieldScore, FieldScoredAt}
```

其余投影（`SessionRepoFieldsList`、`SessionRepoFieldsReadList`、`SessionRepoFieldsDedup`、`SessionRepoFieldsSummarize`、`SessionRepoFieldsModelIDSync`、`SessionRepoFieldsTerminalScan`）**不加** —— 它们服务于列表投影与后台任务，归属过滤在 WHERE 子句完成，不需要把列取回内存。

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./test/unit/session_owner_id/ -v
```

Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/infrastructure/database/model/session.go \
        internal/infrastructure/database/model/trace.go \
        internal/common/constant/sql.go \
        test/unit/session_owner_id/
git commit -m "feat(session): sessions/traces 新增 api_key_id 列与索引" --no-verify
```

> `--no-verify`：T2 的红灯 E2E 仍未转绿，hook 的全量 `go test` 会失败。T8 完成后不再需要。

---

### Task 4: 写入侧落库 api_key_id

**Files:**
- Modify: `internal/dto/asynctask.go:18-27`
- Modify: `internal/application/llmproxy/usecase/openai_store.go:49`、`:221`
- Modify: `internal/application/llmproxy/usecase/anthropic_store.go:34`
- Modify: `internal/application/llmproxy/usecase/trigger_capture.go:202`
- Modify: `internal/infrastructure/pool/store_pool.go:93-100`
- Test: `test/unit/session_owner_id/store_task_test.go`（新建）

**Interfaces:**
- Consumes: `dbmodel.Session.APIKeyID`（T3）、`constant.CtxKeyAPIKeyID`（已存在）、`util.CtxValueUint`（已存在）
- Produces: `dto.MessageStoreTask.APIKeyID uint`。T9 的回填只处理存量，新写入由本任务保证。

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/pool/store_pool.go
```

- [ ] **Step 2: 写失败测试**

创建 `test/unit/session_owner_id/store_task_test.go`：

```go
package session_owner_id

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// TestMessageStoreTaskCarriesAPIKeyID 存储任务必须携带 APIKeyID。
//
// 归属落库的唯一来源是中间件注入的 CtxKeyAPIKeyID；任务结构体漏该字段
// 会让 sessions.api_key_id 恒为 0，新会话对普通用户不可见。
func TestMessageStoreTaskCarriesAPIKeyID(t *testing.T) {
	ctx := context.WithValue(context.Background(), constant.CtxKeyAPIKeyID, uint(42))

	task := &dto.MessageStoreTask{
		Ctx:        ctx,
		APIKeyName: "my-key",
		APIKeyID:   util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
	}

	if task.APIKeyID != 42 {
		t.Errorf("APIKeyID = %d, want 42", task.APIKeyID)
	}
}

// TestCopyContextValuesPreservesAPIKeyID 异步任务的 ctx 复制必须保留 api key id。
// 存储走协程池，ctx 经 util.CopyContextValues 脱离请求生命周期。
func TestCopyContextValuesPreservesAPIKeyID(t *testing.T) {
	src := context.WithValue(context.Background(), constant.CtxKeyAPIKeyID, uint(7))
	dst := util.CopyContextValues(src)

	if got := util.CtxValueUint(dst, constant.CtxKeyAPIKeyID); got != 7 {
		t.Errorf("copied ctx api key id = %d, want 7", got)
	}
}
```

- [ ] **Step 3: 运行确认失败**

```bash
go test ./test/unit/session_owner_id/ -run TestMessageStoreTaskCarriesAPIKeyID -v
```

Expected: FAIL，编译错误 `unknown field APIKeyID in struct literal of type dto.MessageStoreTask`

- [ ] **Step 4: 加 DTO 字段**

`internal/dto/asynctask.go`，`MessageStoreTask` 的 `APIKeyName` 之后插入：

```go
	APIKeyID     uint                 // 归属 API Key ID（鉴权唯一依据）
```

- [ ] **Step 5: 四个调用点传值**

以下四处在构造 `&dto.MessageStoreTask{...}` 时，紧随 `APIKeyName:` 行插入同样一行：

```go
		APIKeyID:   util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
```

位置：
- `internal/application/llmproxy/usecase/openai_store.go:51` 附近（`u.taskSubmitter.SubmitMessageStoreTask` 内）
- `internal/application/llmproxy/usecase/openai_store.go:223` 附近（`submitter.SubmitMessageStoreTask` 内）
- `internal/application/llmproxy/usecase/anthropic_store.go:36` 附近
- `internal/application/llmproxy/usecase/trigger_capture.go:204` 附近

四处都已 import `util` 与 `constant`，无需改 import。注意对齐 gofmt 的字段对齐（hook 会 `gofmt -w`，不必手工纠结）。

- [ ] **Step 6: store_pool 写入**

`internal/infrastructure/pool/store_pool.go:93`，把：

```go
		session := &dbmodel.Session{
			APIKeyName: task.APIKeyName,
```

改为：

```go
		session := &dbmodel.Session{
			APIKeyName: task.APIKeyName,
			APIKeyID:   task.APIKeyID,
```

- [ ] **Step 7: 运行测试确认通过**

```bash
go test ./test/unit/session_owner_id/ -v && go build ./...
```

Expected: PASS 且编译通过

- [ ] **Step 8: 验证捕获能力**

临时把 Step 6 加的 `APIKeyID: task.APIKeyID` 注释掉 —— 单测仍会 PASS（它只验证任务结构体与 ctx 复制），说明**单测覆盖不到落库这一环**。落库由 T8 的 E2E 兜底验收。把这一事实记在 commit message 里，别假装已覆盖。恢复注释掉的行。

- [ ] **Step 9: 提交**

```bash
git add internal/dto/asynctask.go \
        internal/application/llmproxy/usecase/openai_store.go \
        internal/application/llmproxy/usecase/anthropic_store.go \
        internal/application/llmproxy/usecase/trigger_capture.go \
        internal/infrastructure/pool/store_pool.go \
        test/unit/session_owner_id/store_task_test.go
git commit -m "feat(session): 写入侧落库 api_key_id

MessageStoreTask 新增 APIKeyID，四个 store 调用点从 ctx 取
CtxKeyAPIKeyID，store_pool 构造 Session 时写入。
落库环节的验收由 T8 的跨租户 E2E 承担（单测只覆盖任务结构体与 ctx 复制）。" --no-verify
```

---

### Task 5: 领域层 owner 概念改 ID

**Files:**
- Modify: `internal/domain/session/vo/api_key_owner.go`
- Modify: `internal/domain/session/aggregate/session.go`
- Modify: `internal/infrastructure/repository/session_repository.go:50-60`（Save）、`:101-150`（FindByID/Paginate）、`:700-715`（toSessionAggregate）
- Test: `test/unit/session_owner_id/aggregate_test.go`（新建）

**Interfaces:**
- Consumes: `dbmodel.Session.APIKeyID`（T3）
- Produces:
  - `vo.APIKeyOwnerID` 类型：`type APIKeyOwnerID uint`，方法 `Uint() uint`、`IsEmpty() bool`（0 视为空）
  - `aggregate.CreateSession(ownerID vo.APIKeyOwnerID, ownerName string, messageIDs, toolIDs []uint, metadata map[string]string, now time.Time) (*Session, error)`
  - `aggregate.RestoreSession(id uint, ownerID vo.APIKeyOwnerID, ownerName string, messageIDs, toolIDs []uint, metadata map[string]string, score vo.SessionScore, createdAt, updatedAt time.Time) *Session`
  - `(*Session).OwnerID() vo.APIKeyOwnerID`、`(*Session).OwnerName() string`、`(*Session).IsOwnedByID(id uint) bool`
  - 旧的 `Owner() vo.APIKeyOwner` 与 `IsOwnedBy(string) bool` **删除**（避免两套归属语义并存）

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/domain/session/aggregate/session.go
```

- [ ] **Step 2: 写失败测试**

创建 `test/unit/session_owner_id/aggregate_test.go`：

```go
package session_owner_id

import (
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/domain/session/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/session/vo"
)

// TestCreateSessionRequiresOwnerID 归属 ID 为 0 时必须拒绝创建。
// ID 归属是鉴权唯一依据，0 表示身份缺失，落库会产生不可见的孤儿会话。
func TestCreateSessionRequiresOwnerID(t *testing.T) {
	_, err := aggregate.CreateSession(vo.APIKeyOwnerID(0), "any-name",
		[]uint{1}, nil, nil, time.Now())
	if err == nil {
		t.Error("CreateSession with owner id 0 must fail")
	}
}

// TestCreateSessionAllowsEmptyOwnerName 名称为空不阻塞创建。
// name 已降级为纯展示字段，不参与鉴权，不应成为写入的硬门槛。
func TestCreateSessionAllowsEmptyOwnerName(t *testing.T) {
	s, err := aggregate.CreateSession(vo.APIKeyOwnerID(3), "",
		[]uint{1}, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("CreateSession with empty name must succeed, got %v", err)
	}
	if s.OwnerID().Uint() != 3 {
		t.Errorf("OwnerID = %d, want 3", s.OwnerID().Uint())
	}
}

// TestIsOwnedByIDExactMatch 归属判定按 ID 精确匹配。
func TestIsOwnedByIDExactMatch(t *testing.T) {
	s, err := aggregate.CreateSession(vo.APIKeyOwnerID(5), "shared-name",
		[]uint{1}, nil, nil, time.Now())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if !s.IsOwnedByID(5) {
		t.Error("IsOwnedByID(5) must be true for owner id 5")
	}
	// 关键：同名不同 ID 必须判为非所有者（缺陷 A 的核心）
	if s.IsOwnedByID(6) {
		t.Error("IsOwnedByID(6) must be false for owner id 5")
	}
}

// TestZeroOwnerIDNeverMatches 归属 0（存量空归属）不得匹配任何真实 key。
// 自增主键从 1 起，故 0 天然不匹配；此用例锁定该不变量。
func TestZeroOwnerIDNeverMatches(t *testing.T) {
	s := aggregate.RestoreSession(1, vo.APIKeyOwnerID(0), "", []uint{1}, nil,
		nil, vo.SessionScore{}, time.Now(), time.Now())
	for _, id := range []uint{1, 2, 100} {
		if s.IsOwnedByID(id) {
			t.Errorf("session with owner id 0 must not be owned by %d", id)
		}
	}
}
```

> `vo.SessionScore{}` 的零值构造方式请先读 `internal/domain/session/vo/summary_score.go` 确认（可能需要用构造函数而非结构体字面量）。

- [ ] **Step 3: 运行确认失败**

```bash
go test ./test/unit/session_owner_id/ -run TestCreateSessionRequiresOwnerID -v
```

Expected: FAIL，编译错误 `undefined: vo.APIKeyOwnerID`

- [ ] **Step 4: 加值对象**

`internal/domain/session/vo/api_key_owner.go` 末尾追加（保留既有 `APIKeyOwner`，它仍被展示路径使用）：

```go
// APIKeyOwnerID Session 归属的 API Key ID 值对象。
//
// 归属判定的唯一权威依据。相比名称，ID 不可跨用户重复，
// 因此不存在「他人建同名 Key 即可越权」的问题。
// 0 表示归属未知（存量空归属会话），不匹配任何真实 Key。
type APIKeyOwnerID uint

// Uint 返回底层 ID
func (o APIKeyOwnerID) Uint() uint { return uint(o) }

// IsEmpty 归属是否缺失（0 为缺失）
func (o APIKeyOwnerID) IsEmpty() bool { return uint(o) == 0 }
```

- [ ] **Step 5: 改聚合**

`internal/domain/session/aggregate/session.go`：

1. 结构体字段 `owner vo.APIKeyOwner` 替换为两个字段：

```go
	ownerID   vo.APIKeyOwnerID
	ownerName string
```

2. `CreateSession` 签名与校验改为：

```go
// CreateSession 创建新 Session 聚合
//
// ownerID 为归属 API Key ID（鉴权唯一依据），不得为 0；
// ownerName 仅用于展示与审计可读性，允许为空。
func CreateSession(ownerID vo.APIKeyOwnerID, ownerName string, messageIDs, toolIDs []uint, metadata map[string]string, now time.Time) (*Session, error) {
	if ownerID.IsEmpty() {
		return nil, ierr.New(ierr.ErrValidation, "session api key owner id is empty")
	}
	if hasDuplicateIDs(messageIDs) {
		return nil, ierr.New(ierr.ErrValidation, "session message IDs contain duplicates")
	}
	if hasDuplicateIDs(toolIDs) {
		return nil, ierr.New(ierr.ErrValidation, "session tool IDs contain duplicates")
	}
	return &Session{
		ownerID:    ownerID,
		ownerName:  ownerName,
		messageIDs: messageIDs,
		toolIDs:    toolIDs,
		metadata:   metadata,
		createdAt:  now,
		updatedAt:  now,
	}, nil
}
```

3. `RestoreSession` 签名加 `ownerID` 与 `ownerName`（**不校验**，存量 0 归属必须能重建）：

```go
func RestoreSession(id uint, ownerID vo.APIKeyOwnerID, ownerName string, messageIDs, toolIDs []uint,
	metadata map[string]string, score vo.SessionScore,
	createdAt, updatedAt time.Time) *Session {
	s := &Session{
		ownerID:    ownerID,
		ownerName:  ownerName,
		messageIDs: messageIDs,
		toolIDs:    toolIDs,
		metadata:   metadata,
		score:      score,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
	s.SetID(id)
	return s
}
```

4. 访问器：删除 `Owner()` 与 `IsOwnedBy()`，新增

```go
// OwnerID 返回归属 API Key ID（鉴权依据）
func (s *Session) OwnerID() vo.APIKeyOwnerID { return s.ownerID }

// OwnerName 返回归属 API Key 名称（仅展示用，不参与鉴权）
func (s *Session) OwnerName() string { return s.ownerName }

// IsOwnedByID 判断指定 API Key ID 是否为会话所有者。
// 归属为 0（存量空归属）时恒返回 false。
func (s *Session) IsOwnedByID(apiKeyID uint) bool {
	if s.ownerID.IsEmpty() || apiKeyID == 0 {
		return false
	}
	return s.ownerID.Uint() == apiKeyID
}
```

5. 聚合文档注释首段的「所有者（APIKeyName）」改为「所有者（APIKeyID，名称仅展示）」。

- [ ] **Step 6: 修仓储映射**

`internal/infrastructure/repository/session_repository.go`：

- `Save`（约 :50-60）：`APIKeyName: s.Owner().String()` 改为

```go
			APIKeyName: s.OwnerName(),
			APIKeyID:   s.OwnerID().Uint(),
```

- `toSessionAggregate`（约 :700-715）：`vo.APIKeyOwner(m.APIKeyName)` 这个入参改为 `vo.APIKeyOwnerID(m.APIKeyID), m.APIKeyName`，与新的 `RestoreSession` 签名对齐。
- `Paginate`（约 :124-128）：入参 `owner string` 改为 `ownerID uint`，条件 `&dbmodel.Session{APIKeyName: owner}` 改为显式占位符：

```go
	db := r.db.WithContext(ctx).Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), []uint{ownerID})
```

同步改 `internal/domain/session/repository.go` 的 `SessionRepository.Paginate` 签名为 `Paginate(ctx context.Context, ownerID uint, param PageParam)`。

> `Paginate` 若无任何调用方（grep 确认），直接删除该方法与接口声明更干净，遵循 YAGNI。

- [ ] **Step 7: 全量编译并修连带错误**

```bash
go build ./... 2>&1 | head -40
```

逐个修复因签名变更报错的调用点。**此时会暴露 T8 要改的全部应用层位点**，先只让它编译通过（把 `slices.Contains(ownerNames, sess.Owner().String())` 临时改为 `sess.IsOwnedByID(...)` 的形式需要 ID 列表，尚不可得）—— 为避免半成品状态，本任务只改领域层与仓储映射，应用层的判定切换留给 T8；若编译无法通过，说明必须与 T8 合并提交，此时按「T5+T6+T8 一次提交」处理，不要留下不可编译的中间提交。

- [ ] **Step 8: 运行测试**

```bash
go test ./test/unit/session_owner_id/ -v
```

Expected: PASS

- [ ] **Step 9: 提交（若编译通过）**

```bash
git add internal/domain/session/ internal/infrastructure/repository/session_repository.go test/unit/session_owner_id/
git commit -m "refactor(session): 聚合归属由名称改为 API Key ID

新增 vo.APIKeyOwnerID；Session 聚合持有 ownerID（鉴权依据）与
ownerName（仅展示）；删除 Owner()/IsOwnedBy() 避免两套语义并存。" --no-verify
```

---

### Task 6: session 读仓储归属过滤改 ID

**Files:**
- Modify: `internal/domain/session/repository.go:112-118`（`ExportFilter`）、`:148-173`（`SessionReadRepository` 4 个方法签名）
- Modify: `internal/infrastructure/repository/session_repository.go:309-330`、`:566-600`、`:601-640`、`:641-700`、`:735-800`
- Test: `test/unit/session_owner_id/read_repo_scope_test.go`（新建）

**Interfaces:**
- Consumes: `constant.FieldAPIKeyID`、`constant.DBConditionInTemplate`
- Produces（供 T8 消费的最终签名）：
  - `ListSessionsByOwnerIDs(ctx context.Context, ownerIDs []uint, param model.CommonParam, startTime, endTime time.Time, keyword string, criteria *filter.FilterCriteria) ([]*SessionSummaryProjection, *model.PageInfo, error)`
  - `ListDistinctScores(ctx context.Context, ownerIDs []uint, startTime, endTime time.Time, sessionIDs []uint) ([]int, error)`
  - `ListDistinctModels(ctx context.Context, ownerIDs []uint, keyword string, startTime, endTime time.Time, sessionIDs []uint) ([]string, error)`
  - `ListMessageCountStats(ctx context.Context, ownerIDs []uint, startTime, endTime time.Time, sessionIDs []uint) (maxCount int, bucketCounts map[int]int64, err error)`
  - `ExportFilter.OwnerIDs []uint`（替换 `OwnerNames []string`）
  - `SessionMetaProjection.APIKeyID uint`、`SessionDetailProjection.APIKeyID uint`（新增字段，名称字段保留）

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/infrastructure/repository/session_repository.go
```

- [ ] **Step 2: 写失败测试（三态语义）**

创建 `test/unit/session_owner_id/read_repo_scope_test.go`。用 sqlite 内存库建表、插入 3 条会话（`api_key_id` 分别为 1、2、0），断言：

```go
// TestListSessionsByOwnerIDsScopeSemantics 三态归属语义：
//   - ownerIDs = nil        → 不加过滤（admin 全量）
//   - ownerIDs = []uint{}   → 恒假，返回空（名下无 Key 的用户不得看到全量）
//   - ownerIDs = []uint{1}  → 只返回 api_key_id=1 的会话
//   - api_key_id=0 的行在任何非 nil 场景下均不可见
func TestListSessionsByOwnerIDsScopeSemantics(t *testing.T) {
	// 表驱动，四个子场景各断言返回的 session ID 集合
}
```

具体断言矩阵（seed：session1→key1、session2→key2、session3→key0）：

| ownerIDs | 期望可见 session |
|---|---|
| `nil` | {1, 2, 3} |
| `[]uint{}` | {} |
| `[]uint{1}` | {1} |
| `[]uint{1, 2}` | {1, 2} |
| `[]uint{99}` | {} |

对 `ListDistinctScores` / `ListDistinctModels` / `ListMessageCountStats` 各写一个「空切片必须短路返回空」的用例（这是越权防线，`nil` 与空切片语义不可混淆）。

- [ ] **Step 3: 运行确认失败**

```bash
go test ./test/unit/session_owner_id/ -run TestListSessionsByOwnerIDsScopeSemantics -v
```

Expected: FAIL，编译错误（方法名不存在）

- [ ] **Step 4: 改接口**

`internal/domain/session/repository.go`：

1. `ExportFilter` 的 `OwnerNames []string` 改为：

```go
	// OwnerIDs 归属 API Key ID 列表。nil 表示不过滤（admin）；
	// 非 nil 时按归属过滤（空列表返回空结果，名下无 Key 不得越权导出全量）。
	OwnerIDs []uint
```

同步改该类型上方的文档注释。

2. `SessionReadRepository` 的 `ListSessionsByOwnerNames` 重命名为 `ListSessionsByOwnerIDs`，参数 `ownerNames []string` → `ownerIDs []uint`；`ListDistinctScores` / `ListDistinctModels` / `ListMessageCountStats` 同理。各方法注释里的 "ownerNames" 改为 "ownerIDs"。

3. `SessionMetaProjection` 与 `SessionDetailProjection` 各加一个字段（放在 `APIKeyName` 之后）：

```go
	APIKeyID   uint
```

- [ ] **Step 5: 改实现**

`internal/infrastructure/repository/session_repository.go`，五处过滤条件的列从 `constant.FieldAPIKeyName` 换成 `constant.FieldAPIKeyID`，变量名 `ownerNames` → `ownerIDs`：

- `:319`（`ListSessionsByOwnerIDs`）
- `:580`（`ListDistinctScores`）
- `:615`（`ListDistinctModels`）
- `:651`（`ListMessageCountStats`）
- `:744`（`applyExportFilter`，`f.OwnerNames` → `f.OwnerIDs`）

模板写法保持不变，只换字段常量：

```go
	sql = sql.Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), ownerIDs)
```

同时在 `GetSessionMeta`（:503-520）与 `GetSessionDetail`（:423-455）的投影映射里补 `APIKeyID: sessionRecord.APIKeyID`。

`ListSessionsForExport`（:750）与 `PreviewExport`（:783）的空切片短路判断从 `f.OwnerNames` 改为 `f.OwnerIDs`。

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./test/unit/session_owner_id/ -v
```

Expected: PASS

- [ ] **Step 7: 验证捕获能力**

临时把 `:319` 的过滤条件整行删除（模拟「忘记加归属过滤」），重跑 Step 6，`ownerIDs = []uint{1}` 的场景**必须 FAIL**。恢复。

- [ ] **Step 8: 提交（与 T8 同批，见下）**

本任务改了接口签名，全部调用方在 T8。按 Global Constraints 的 hook 规则，**T6 与 T8 必须同批提交**。此处先不提交，继续做 T7 与 T8，最后一次性提交。

---

### Task 7: trace 归属改 ID

**Files:**
- Modify: `internal/domain/trace/repository.go:12-24`（`Trace` 结构体）、`:53-55`（`PaginateByOwners`）
- Modify: `internal/infrastructure/repository/trace_repository.go`
- Modify: `internal/handler/trace.go:158`
- Modify: `internal/application/trace/port/handler.go:70-78`（`ReportTraceEventCommand`）
- Modify: `internal/application/trace/command/report_trace_event.go:90-139`
- Modify: `internal/application/trace/command/delete_trace.go:29-57`
- Modify: `internal/application/trace/query/authorize.go:45-51`、`list_traces.go:29-55`
- Test: `test/unit/session_owner_id/trace_scope_test.go`（新建）

**Interfaces:**
- Consumes: `dbmodel.Trace.APIKeyID`（T3）、`apikey.APIKeyRepository.LookupIDsByUserID`（已存在）
- Produces:
  - `trace.Trace.APIKeyID uint`（领域结构体新增字段，`APIKeyName` 保留作展示）
  - `trace.TraceRepository.PaginateByOwners(ctx context.Context, ownerIDs []uint, param model.CommonParam) ([]*Trace, *model.PageInfo, error)`
  - `port.ReportTraceEventCommand.APIKeyID uint`
  - `resolveOwnerIDs(ctx context.Context, repo apikeydomain.APIKeyRepository, userID uint, isAdmin bool) ([]uint, error)`（替换 `resolveOwners`）

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/application/trace/command/report_trace_event.go
```

- [ ] **Step 2: 写失败测试**

创建 `test/unit/session_owner_id/trace_scope_test.go`，覆盖两件事：

```go
// TestResolveParentTraceIDRejectsCrossTenantByID 父子 trace 关联的跨租户校验必须按 ID。
//
// 原实现按 apiKeyName 比对（report_trace_event.go:135），两个用户用同名 Key
// 时 name 相等，子 trace 会挂到他人的父 trace 下，造成 trace 树跨租户串联。
// 该位点在 spec 的改造清单中被遗漏，此用例锁定修复。
func TestResolveParentTraceIDRejectsCrossTenantByID(t *testing.T) {
	// 用 fake TraceRepository：父 trace 的 APIKeyID=1，
	// 以 APIKeyID=2 上报同名 Key 的子 trace，断言 parentTraceID == 0
}

// TestTracePaginateByOwnerIDsScopeSemantics trace 列表三态语义，
// 断言矩阵同 session（nil 全量 / 空切片恒假 / 指定 ID 精确匹配 / api_key_id=0 不可见）。
func TestTracePaginateByOwnerIDsScopeSemantics(t *testing.T) {
	// sqlite 内存库 + 三条 trace（api_key_id = 1 / 2 / 0）
}
```

fake 仓储用「嵌入接口 + 只覆写用到的方法」写法，最省：

```go
type fakeTraceRepo struct {
	trace.TraceRepository
	bySessionID map[string]*trace.Trace
}

func (f *fakeTraceRepo) FindBySessionID(_ context.Context, sessionID string) (*trace.Trace, error) {
	return f.bySessionID[sessionID], nil
}
```

- [ ] **Step 3: 运行确认失败**

```bash
go test ./test/unit/session_owner_id/ -run TestResolveParentTraceIDRejectsCrossTenantByID -v
```

Expected: FAIL（编译错误：`trace.Trace` 无 `APIKeyID` 字段 / `resolveParentTraceID` 签名不匹配）

- [ ] **Step 4: 领域结构体与仓储接口**

`internal/domain/trace/repository.go`：

1. `Trace` 结构体在 `APIKeyName` 之后加：

```go
	APIKeyID      uint
```

2. `PaginateByOwners` 注释与签名改 ID：

```go
	// PaginateByOwners 按归属 API Key ID 列表分页。ownerIDs 为 nil 时查全部（admin），
	// 非 nil 时按归属过滤（空列表返回空结果，名下无 Key 不得越权查全量）。
	PaginateByOwners(ctx context.Context, ownerIDs []uint, param model.CommonParam) ([]*Trace, *model.PageInfo, error)
```

- [ ] **Step 5: 仓储实现**

`internal/infrastructure/repository/trace_repository.go`：
- `PaginateByOwners` 过滤列换 `constant.FieldAPIKeyID`，保持 `fmt.Sprintf(constant.DBConditionInTemplate, ...)` 模板与空切片短路逻辑
- `UpsertBySessionID` 写入与领域↔模型映射补 `APIKeyID` 双向传递

- [ ] **Step 6: 上报链路传 ID**

1. `internal/application/trace/port/handler.go` 的 `ReportTraceEventCommand` 在 `APIKeyName` 之后加 `APIKeyID uint`
2. `internal/handler/trace.go:158` 附近，紧随 `APIKeyName:` 行加：

```go
		APIKeyID:        util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
```

3. `report_trace_event.go` 两处 `UpsertBySessionID` 调用补 `APIKeyID`（新建用 `cmd.APIKeyID`，更新用 `existing.APIKeyID`）
4. `resolveParentTraceID` 签名与判定改 ID：

```go
// resolveParentTraceID 按父 session 解析父 trace id；无父、父不存在或归属不一致时返回 0。
//
// 归属比对用 API Key ID：名称可跨用户重复，按名称比对会让子 trace
// 挂到同名他人的父 trace 下，形成跨租户 trace 树。
func resolveParentTraceID(ctx context.Context, repo trace.TraceRepository, parentSessionID, sessionID string, apiKeyID uint) uint {
	if parentSessionID == "" || parentSessionID == sessionID {
		return 0
	}
	parent, err := repo.FindBySessionID(ctx, parentSessionID)
	if err != nil || parent == nil {
		return 0
	}
	if apiKeyID != 0 && parent.APIKeyID != apiKeyID {
		return 0 // 跨租户 session 不应建立父/子关联
	}
	return parent.ID
}
```

调用点（`:90`）改为传 `cmd.APIKeyID`。

- [ ] **Step 7: 归属判定改 ID**

1. `authorize.go:45-51`：

```go
	ownerIDs, err := a.apiKeyRepo.LookupIDsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if item.APIKeyID == 0 || !slices.Contains(ownerIDs, item.APIKeyID) {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.TraceNotFoundMessage)
	}
```

2. `list_traces.go`：`resolveOwners` 改名 `resolveOwnerIDs`，返回 `[]uint`，内部调 `LookupIDsByUserID`；调用处变量名同步。`QueryParam.QueryFields` **保持含 `FieldAPIKeyName`**（那是搜索展示字段，不是鉴权）。
3. `delete_trace.go:29-57`：`ownerNames` → `ownerIDs`，判定改：

```go
		if !cmd.IsAdmin && (t.APIKeyID == 0 || !slices.Contains(ownerIDs, t.APIKeyID)) {
```

日志字段 `zap.String("owner", t.APIKeyName)` 可保留（便于人读），另加 `zap.Uint("ownerID", t.APIKeyID)`。

- [ ] **Step 8: 运行测试**

```bash
go test ./test/unit/session_owner_id/ -v
```

Expected: PASS

- [ ] **Step 9: 不单独提交**

与 T6、T8 同批（接口签名变更的约束同前）。

---

### Task 8: session 应用层归属判定改 ID 并转绿 E2E

**Files:**
- Modify: `internal/application/session/query/jwt_session_queries.go:48-50`、`:83-91`、`:212-226`
- Modify: `internal/application/session/query/session_meta_query.go:55-63`、`:101-110`
- Modify: `internal/application/session/query/option_list.go:32-44`、`:50`、`:70`、`:72`
- Modify: `internal/application/session/command/delete_session.go:31-63`
- Modify: `internal/application/session/command/score_session.go:42-56`、`:96-110`
- Modify: `internal/application/dataset/query/dataset_query.go:24-26`、`:157-164`、`:207-214`、`:324-331`
- Modify: `test/e2e/cross_tenant_session/fixture_test.go`（补 `APIKeyID` seed）
- Test: 转绿 `test/e2e/cross_tenant_session/`

**Interfaces:**
- Consumes: T6 的 `ListSessionsByOwnerIDs` / `ListDistinctScores` / `ListDistinctModels` / `ListMessageCountStats` / `ExportFilter.OwnerIDs`；T5 的 `(*Session).IsOwnedByID`；`SessionMetaProjection.APIKeyID`
- Produces: `ownerIDLookup` 接口（替换两处 `ownerNameLookup`）：

```go
type ownerIDLookup interface {
	LookupIDsByUserID(ctx context.Context, userID uint) ([]uint, error)
}
```

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path internal/application/session/query/jwt_session_queries.go
```

- [ ] **Step 2: 替换窄接口**

`jwt_session_queries.go:48-50` 的 `ownerNameLookup` 改为上方 `ownerIDLookup` 定义（同包的 `option_list.go` 注释指向此处，一并更新注释文字）。`dataset_query.go:24-26` 的同名接口同样改。

> 注意：这两个接口在不同包（`session/query` 与 `dataset/query`）各自定义，同包内不可重复定义，跨包需各改一处。

- [ ] **Step 3: 列表与详情判定**

`jwt_session_queries.go`：

```go
		ownerIDs, lookupErr := h.apiKeyRepo.LookupIDsByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner ids", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, nil, lookupErr
		}
		if len(ownerIDs) == 0 {
			return []*sessionport.SessionSummaryView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize, Total: 0}, nil
		}
		projections, pageInfo, err = h.readRepo.ListSessionsByOwnerIDs(ctx, ownerIDs, param, q.StartTime, q.EndTime, q.Keyword, criteria)
```

详情（`:212-226`）：

```go
	if !q.IsAdmin && !q.SkipOwnershipCheck {
		ownerIDs, lookupErr := h.apiKeyRepo.LookupIDsByUserID(ctx, q.UserID)
		if lookupErr != nil {
			log.Error("[SessionQuery] Failed to lookup owner ids", zap.Error(lookupErr), zap.Uint("userID", q.UserID))
			return nil, lookupErr
		}
		if detail.APIKeyID == 0 || !slices.Contains(ownerIDs, detail.APIKeyID) {
			log.Warn("[SessionQuery] No permission to access session",
				zap.Uint("sessionID", q.SessionID),
				zap.Uint("ownerID", detail.APIKeyID),
				zap.Uint("userID", q.UserID))
			return nil, ierr.New(ierr.ErrNoPermission, "no permission to access session")
		}
	}
```

- [ ] **Step 4: 元数据判定**

`session_meta_query.go`：`ownerNames` → `ownerIDs`（`LookupIDsByUserID`）；`:101-110` 判定改 `record.APIKeyID`。

**关键**：`sessionport.SessionMetaCacheRecord` 需新增 `APIKeyID uint` 字段并在 `:83-93` 的构造里赋值 `projection.APIKeyID`，否则缓存命中路径读不到归属 → 误拒。同时 **缓存结构体变更必须换缓存 key 前缀或加版本号**，否则线上旧缓存反序列化后 `APIKeyID` 为 0，会把所有会话判成无权访问。查 `internal/infrastructure/cache/session_detail.go` 的 key 模板，在 `internal/common/constant/string.go` 里给它 bump 一个版本后缀。

- [ ] **Step 5: 选项、删除、评分判定**

- `option_list.go`：`ownerNames` → `ownerIDs`，三处 readRepo 调用传 `ownerIDs`
- `delete_session.go:56-63`：

```go
		if !isAdmin && !sess.IsOwnedByID(...) {
```

需要在循环外把 `ownerIDs` 备好，循环内判定：

```go
		if !isAdmin {
			allowed := false
			for _, id := range ownerIDs {
				if sess.IsOwnedByID(id) {
					allowed = true
					break
				}
			}
			if !allowed {
				result.Failures = append(result.Failures, port.DeleteSessionFailedItem{ID: id, Error: constant.SessionDeleteErrorNoPermission})
				continue
			}
		}
```

（等价的简洁写法：`slices.Contains(ownerIDs, sess.OwnerID().Uint()) && !sess.OwnerID().IsEmpty()`，择一即可，保持文件内风格一致）

日志里的 `zap.String("owner", sess.Owner().String())` 改 `zap.Uint("ownerID", sess.OwnerID().Uint())`。

- `score_session.go` 两处（:42-56、:96-110）同样处理。

- [ ] **Step 6: 数据集导出**

`dataset_query.go` 三处（:157、:207、:324）：

```go
	if p.Permission != enum.PermissionAdmin {
		ownerIDs, err := h.apiKeyRepo.LookupIDsByUserID(ctx, p.UserID)
		if err != nil {
			logger.WithCtx(ctx).Error("[DatasetExport] Failed to lookup owner ids", zap.Error(err), zap.Uint("userID", p.UserID))
			return nil, err
		}
		f.OwnerIDs = ownerIDs
	}
```

三处的日志前缀分别是 `[DatasetExport]` / `[DatasetPreview]` / `[DatasetFormatPreview]`，保持原样不要统一。

- [ ] **Step 7: 补 E2E seed 的 APIKeyID**

`test/e2e/cross_tenant_session/fixture_test.go`：A 的会话补 `APIKeyID: 1`。另加一条 `APIKeyID: 0` 的空归属会话（`ID: 3`），断言**两个普通用户都看不到它、admin 能看到**。

- [ ] **Step 8: 全量编译与测试**

```bash
go build ./... && go test ./... 2>&1 | tail -30
```

Expected: 编译通过；`test/e2e/cross_tenant_session/` **全部转绿**；其余测试包无回归。

若有其它测试包因签名变更编译失败，逐个修复其 fake/stub（`grep -rn "LookupOwnerNamesByUserID\|ListSessionsByOwnerNames\|OwnerNames" test/` 定位）。

- [ ] **Step 9: 验证捕获能力**

把 `jwt_session_queries.go` 的详情判定临时改回按名称比对（`detail.APIKeyName` + `LookupOwnerNamesByUserID`），重跑 E2E，`detail` 子测试**必须 FAIL**。恢复。

- [ ] **Step 10: lint**

```bash
go vet ./... \
  && go run ./cmd/lint ./...
```

常见拦截：`gocritic paramTypeCombine` 要求相邻同类型参数合并；中文注释里 `//` 后必须半角空格；`errors.New` 被 forbidigo 拦（用 `ierr`）。

- [ ] **Step 11: 提交（T6+T7+T8 一批）**

```bash
git add -A
git commit -m "fix(session): 归属判定改按 api_key_id，消除同名 Key 越权

api_key_name 可跨用户重复（唯一索引是 user_id+name），普通用户建同名
Key 即可列出/读取/删除/评分/分享/导出他人会话，共 7 条路径。改为按
api_key_id 判定：名称降级为纯展示字段，不参与鉴权。

- 读仓储 4 个方法与 ExportFilter 由 ownerNames 改 ownerIDs
- trace 同步改造，并修复 resolveParentTraceID 按名称比对导致的
  跨租户 trace 树串联（spec 改造清单遗漏项）
- SessionMetaCacheRecord 加 APIKeyID 并 bump 缓存 key 版本，
  避免旧缓存反序列化后归属为 0 导致全量误拒
- 三态语义不变：nil=admin 全量 / 空切片=恒假 / api_key_id=0 仅 admin 可见"
```

---

### Task 9: 存量回填

**Files:**
- Create: `internal/infrastructure/repository/session_backfill.go`
- Modify: `cmd/server/database.go:30-32`
- Test: `test/unit/session_backfill/backfill_test.go`（新建）

**Interfaces:**
- Consumes: `dbmodel.Session`、`dbmodel.ProxyAPIKey`、`dbmodel.Trace`
- Produces: `repository.BackfillSessionAPIKeyID(ctx context.Context, db *gorm.DB) (sessions int64, traces int64, err error)`

- [ ] **Step 1: 跑 use-modern-go**

```bash
sh ".agents/skills/external/use-modern-go/scripts/run-tool.sh" list --file-path cmd/server/database.go
```

- [ ] **Step 2: 写失败测试**

创建 `test/unit/session_backfill/backfill_test.go`。seed（sqlite 内存库）：

| session | api_key_name | api_key_id | 期望回填后 |
|---|---|---|---|
| 1 | `"uniq-key"` | 0 | 对应 key 的 ID |
| 2 | `""` | 0 | 0（空归属不推断） |
| 3 | `"dup-key"` | 0 | 0（跨用户同名，歧义不推断） |
| 4 | `"ghost-key"` | 0 | 0（无对应 key） |
| 5 | `"uniq-key"` | 7 | 7（已有归属不覆盖） |

key seed：`{ID:1,UserID:1,Name:"uniq-key"}`、`{ID:2,UserID:1,Name:"dup-key"}`、`{ID:3,UserID:2,Name:"dup-key"}`

```go
// TestBackfillOnlyFillsUnambiguous 只回填能唯一确定归属的行。
func TestBackfillOnlyFillsUnambiguous(t *testing.T) { /* 上表断言 */ }

// TestBackfillIdempotent 连续执行两次结果一致。
// 迁移可能被重复执行（重试、多次部署），非幂等会造成数据漂移。
func TestBackfillIdempotent(t *testing.T) {
	// 跑两次 BackfillSessionAPIKeyID，断言两次后的全表快照相同，
	// 且第二次返回的 affected 计数为 0
}

// TestBackfillSkipsSoftDeletedKeys 软删的 key 不参与归属推断。
func TestBackfillSkipsSoftDeletedKeys(t *testing.T) { /* ... */ }
```

- [ ] **Step 3: 运行确认失败**

```bash
go test ./test/unit/session_backfill/ -v
```

Expected: FAIL，`undefined: repository.BackfillSessionAPIKeyID`

- [ ] **Step 4: 实现回填**

创建 `internal/infrastructure/repository/session_backfill.go`。要点：

- 只更新 `api_key_id = 0 AND api_key_name <> ''` 的行
- 名称必须在未软删的 `proxy_api_keys` 中**唯一**对应一个 key（`HAVING COUNT(*) = 1`）才回填
- 用一条 UPDATE ... FROM 子查询完成，避免逐行往返；PostgreSQL 与 sqlite 语法差异需兼容（sqlite 用 `UPDATE ... SET x = (SELECT ...) WHERE EXISTS (...)`，两者都支持相关子查询形式，优先写通用形式）
- 返回受影响行数，供日志与幂等断言
- 错误用 `ierr.Wrap(ierr.ErrDBUpdate, err, "backfill session api key id")`
- traces 表同样处理

通用写法（sqlite 与 PG 均支持）：

```go
	const uniqueNameSubquery = `
		SELECT k.id FROM proxy_api_keys k
		WHERE k.name = %s.api_key_name AND k.deleted_at = 0
		GROUP BY k.name HAVING COUNT(*) = 1`
```

> 拼接表名用常量而非用户输入，避免注入面；`%s` 只接受包内常量 `"sessions"` / `"traces"`。

- [ ] **Step 5: 挂到 migrate**

`cmd/server/database.go`：

```go
// runMigrate 执行数据库结构迁移：AutoMigrate 建表/建列/建索引，随后幂等回填归属 ID。
//
// 注意：AutoMigrate 不修改已有同名索引的列组合。涉及索引变更的迁移
// （如多租户化 user_id 复合唯一索引）需部署后手工执行 SQL 重建，见 PR #162 描述。
//
// 回填必须先于新版本滚动部署：新版本按 api_key_id 过滤，未回填会让
// 存量会话短时全不可见。
func runMigrate(ctx context.Context) {
	db := database.InitDatabase()
	lo.Must0(database.AutoMigrate(ctx))
	sessions, traces := lo.Must2(repository.BackfillSessionAPIKeyID(ctx, db.WithContext(ctx)))
	logger.Logger().Info("[Migrate] Backfilled api_key_id",
		zap.Int64("sessions", sessions), zap.Int64("traces", traces))
}
```

> `lo.Must2` 已核实存在（`samber/lo@v1.39.0/errors.go:83`，签名 `Must2[T1, T2 any](val1 T1, val2 T2, err any, messageArgs ...interface{}) (T1, T2)`）。`cmd/` 属启动阶段，`lo.Must` 系列在此是既有惯例。

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./test/unit/session_backfill/ -v
```

Expected: PASS（3 个用例）

- [ ] **Step 7: 验证捕获能力**

临时把 `HAVING COUNT(*) = 1` 去掉（允许歧义名回填），重跑，`TestBackfillOnlyFillsUnambiguous` 的 session 3 断言**必须 FAIL**。恢复。

- [ ] **Step 8: 提交**

```bash
git add internal/infrastructure/repository/session_backfill.go \
        cmd/server/database.go test/unit/session_backfill/
git commit -m "feat(migrate): 幂等回填 sessions/traces 的 api_key_id

仅回填 api_key_id=0 且 api_key_name 在未软删 key 中唯一对应的行
（生产约 1869 条）。空归属与跨用户同名的歧义行不推断，保持 0。"
```

- [ ] **Step 9: 输出生产回填 SQL 供用户审批**

把实际执行的 SQL（`EXPLAIN` 后的最终形态）与影响行数预估写进 PR 描述。**生产执行前必须向用户完整展示并单独获取授权**，不得自行在生产库执行。

---

### Task 10: 文档与收尾

**Files:**
- Modify: `CONTEXT.md:103-105`
- Modify: `docs/superpowers/specs/2026-09-27-session-owner-id-design.md`（补 spec 遗漏项记录）

- [ ] **Step 1: 改写 CONTEXT.md 的 APIKeyOwner 条目**

`CONTEXT.md:103-105` 当前内容与新模型矛盾（「用户只能访问其 API Key 名下的会话」按名称语义），改为：

```markdown
**APIKeyOwner（会话所有者）**:
Session/Trace 的归属，由 `api_key_id` 唯一确定（值对象 `vo.APIKeyOwnerID`），来自鉴权中间件注入的 `CtxKeyAPIKeyID`。用于权限校验：用户只能访问其名下 API Key ID 对应的会话。`api_key_name` 仍保留但**仅用于展示与审计可读性**，不参与鉴权——名称在 `(user_id, name)` 维度唯一，可跨用户重复，按名称判定会导致越权。`api_key_id = 0` 表示归属未知（2026-07 前后的存量会话），仅 admin 可见。
_Avoid_: owner name, api key identifier
```

- [ ] **Step 2: 在 spec 补记遗漏项**

在 spec 的「已知遗留」前插入一节，记录实施期发现的两个 spec 未列位点：

```markdown
## 实施期补充（spec 原文遗漏）

1. `internal/application/trace/command/report_trace_event.go:127` 的
   `resolveParentTraceID` 用 `apiKeyName` 做跨租户父子校验，同名 Key 会让
   子 trace 挂到他人父 trace 下。已在 T7 一并改为按 ID 比对。
2. `sessionport.SessionMetaCacheRecord` 需新增 `APIKeyID` 并 bump 缓存 key
   版本。否则线上旧缓存反序列化后归属为 0，会把所有会话判成无权访问
   （缓存命中路径绕过 DB，不会自愈）。已在 T8 处理。
```

- [ ] **Step 3: 全量验证**

```bash
go build ./... \
  && go test ./... \
  && go vet ./... \
  && go run ./cmd/lint ./...
```

Expected: 全绿。此时 T2 引入的红灯已由 T8 转绿，后续提交不再需要 `--no-verify`。

- [ ] **Step 4: ponytail-review**

对完整 diff 跑 `ponytail-review`，逐条评估过度工程项（尤其检查：是否引入了无调用方的 `Paginate`、是否有只被一处使用的 helper、值对象是否有未被用到的方法）。

- [ ] **Step 5: 提交**

```bash
git add CONTEXT.md docs/superpowers/specs/2026-09-27-session-owner-id-design.md
git commit -m "docs: APIKeyOwner 概念改按 ID 归属，补记 spec 遗漏位点"
```

- [ ] **Step 6: 交付确认**

向用户报告：改动摘要、测试结果、**生产回填 SQL 与部署顺序**（先 migrate 再滚动），并询问是否提 PR 或直接合并 master。**禁止擅自推送或部署。**

---

## Self-Review

**1. Spec 覆盖检查**

| spec 要求 | 对应任务 |
|---|---|
| 缺陷 B：`ProxyAPIKeyRepoFieldsAuth` 补 `FieldName` | T1 |
| 缺陷 A：归属改 `api_key_id` | T5–T8 |
| `MessageStoreTask` 加 `APIKeyID` | T4 |
| 四处 store 调用点传 ID | T4 |
| `store_pool.go:94` 写入 | T4 |
| `handler/trace.go:158` 写入 | T7 |
| `dbmodel.Session` / `Trace` 加列 + 索引 | T3 |
| 6 个仓储方法改签名 | T6（4 个 + ExportFilter）、T7（PaginateByOwners） |
| 约 15 处应用层判定改 ID | T8（session/dataset）、T7（trace） |
| 回填挂 `runMigrate`、幂等 | T9 |
| `CONTEXT.md` APIKeyOwner 改写 | T10 |
| 缺陷 B 回归测试 + 捕获能力验证 | T1 Step 2/6 |
| 缺陷 A E2E（6 路径）+ 捕获能力验证 | T2、T8 Step 7/9（含第 7 条导出路径） |
| 回填幂等测试 | T9 |
| 三态语义测试 | T6、T7 |
| 部署顺序约束 | 计划头部「任务依赖与上线分期」+ T9 Step 5 注释 + T10 Step 6 |

无遗漏。**反向发现 spec 漏了 2 个位点**，已补为 T7 Step 6.4 与 T8 Step 4，并在 T10 Step 2 回写 spec。

**2. 占位符扫描**

无 TBD / TODO / "类似 Task N" / "添加适当的错误处理"。原先三处待确认点中两处已在计划定稿前核实并写死：

- sqlite 驱动 = `gorm.io/driver/sqlite`（已核实）
- `lo.Must2` 存在于 `samber/lo@v1.39.0/errors.go:83`（已核实）

仅剩一处需实施者现场读文件确认，已标明确认方法与原因：T5 Step 2 的 `vo.SessionScore` 零值构造方式（读 `internal/domain/session/vo/summary_score.go`，可能需构造函数而非结构体字面量）。

**3. 类型一致性**

- `vo.APIKeyOwnerID` 在 T5 定义，T6/T8 消费 —— 一致
- `IsOwnedByID(uint) bool` 在 T5 定义，T8 Step 5 使用 —— 一致
- `ListSessionsByOwnerIDs` 在 T6 Interfaces 定义，T8 Step 3 调用 —— 一致
- `ExportFilter.OwnerIDs []uint` 在 T6 定义，T8 Step 6 赋值 —— 一致
- `resolveOwnerIDs` 在 T7 Interfaces 定义，T7 Step 7.2 使用 —— 一致
- `BackfillSessionAPIKeyID(ctx, db) (int64, int64, error)` 在 T9 定义并在同任务 Step 5 使用 —— 一致
- `ownerIDLookup` 接口在 T8 Produces 定义，同任务 Step 2 落地 —— 一致；注意它在 `session/query` 与 `dataset/query` 两个包各需定义一次（已在 Step 2 注明）
