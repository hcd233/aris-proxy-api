# 公开端点（Endpoint Visibility）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 admin 创建/维护的 endpoint 可以标记为 `public`，普通用户只读可见并可调用其模型，自己名下的端点仍然完全自管。

**Architecture:** 在 `endpoints` 表加 `visibility`（`private`/`public`）列；公开性只由 admin 写入，只读性由既有租户 scope 天然保证。网关解析新增「公开池」一级回退（自有租户 → 公开池 → 存量共享池），所有「用户能用的模型」读路径统一加 `includePublic` 并集。

**Tech Stack:** Go 1.2x + huma v2 + GORM（sqlite 单测/集成测试、PostgreSQL 生产）+ fx；前端 Next.js（`web/`）。

**Spec:** `docs/superpowers/specs/2026-10-21-public-endpoint-design.md`

## Global Constraints

- 工作区：`.worktrees/public-endpoint-visibility`，分支 `feature/public-endpoint-visibility-2026-10-21`。
- 沙箱禁止写 `~/Library/Caches`，**所有 go 命令必须带** `GOCACHE=/tmp/aris-go-cache`。
- 全量回归：`GOCACHE=/tmp/aris-go-cache go test -count=1 ./cmd/... ./internal/... ./test/...`。
- lint：`GOCACHE=/tmp/aris-go-cache make lint`，必须 0 error / 0 warning。
- 业务常量一律进 `internal/common/constant`，业务包禁止 local const。
- DTO 禁止 `any` / `interface{}` / `json.RawMessage` / `encoding/json`（用 sonic）。
- 测试函数必须 `t.Parallel()`（subtest 同理），除非共享 sqlite 内存库并写 `//nolint:paralleltest` 说明。
- 不新增数据库索引。
- 公开条件必须复用同一个 helper，禁止在 4 处 SQL 手写。
- 提交前跑 `ponytail-review` 审 diff 的过度工程。

## Review Focus

最容易在使用中咬人、且没有现成测试覆盖的 5 个点（每条都在下面某个任务里有对应测试）：

1. admin 把端点从 `public` 改回 `private` 后，普通用户的目录读与转发必须**立刻**失效——Task 3（列表）/Task 7（`/v1/models` 读缓存）。
2. 普通用户自建同名 alias 与公开 alias 并存时，必须稳定走自己的端点，不能落到 admin 的额度上——Task 5、Task 9。
3. 非 admin 在请求体里塞 `visibility` 必须得到明确的业务错误，而不是静默忽略或写库——Task 4、Task 9。
4. 多个 admin 公开同名 alias 时，每个公开模型都必须解析到**自己 owner** 的 endpoint，不能串到别人的端点——Task 5。
5. 用户把同名 alias 的模型禁用后，行为与「禁用后回退共享池」一致（回退公开池）——Task 5。

> 任务顺序说明：Task 3 先改 `EndpointRepository.FindIDsByScope` 签名并同步**全部**测试 stub，后续任务的测试才能编译通过。

---

### Task 1: `EndpointVisibility` 枚举

**Files:**
- Create: `internal/common/enum/endpoint_visibility.go`
- Test: `test/unit/endpoint_visibility/endpoint_visibility_test.go`

**Interfaces:**
- Consumes: 无。
- Produces:
  ```go
  type EndpointVisibility string
  const (
      EndpointVisibilityPrivate EndpointVisibility = "private"
      EndpointVisibilityPublic  EndpointVisibility = "public"
  )
  var EndpointVisibilities []EndpointVisibility
  func (v EndpointVisibility) IsValid() bool
  func (v EndpointVisibility) IsPublic() bool
  func (v EndpointVisibility) Normalize() EndpointVisibility // "" -> private
  ```

- [ ] **Step 1: 写失败测试**

```go
// Package endpoint_visibility 验证端点可见性枚举的合法性、默认值与公开判定。
package endpoint_visibility

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func TestIsValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   enum.EndpointVisibility
		want bool
	}{
		{"private", enum.EndpointVisibilityPrivate, true},
		{"public", enum.EndpointVisibilityPublic, true},
		{"empty", "", false},
		{"unknown", "shared", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.IsValid(); got != tc.want {
				t.Fatalf("IsValid(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()
	if got := enum.EndpointVisibility("").Normalize(); got != enum.EndpointVisibilityPrivate {
		t.Fatalf("empty must normalize to private, got %q", got)
	}
	if got := enum.EndpointVisibilityPublic.Normalize(); got != enum.EndpointVisibilityPublic {
		t.Fatalf("public must stay public, got %q", got)
	}
}

func TestIsPublic(t *testing.T) {
	t.Parallel()
	if !enum.EndpointVisibilityPublic.IsPublic() {
		t.Fatal("public must report public")
	}
	if enum.EndpointVisibilityPrivate.IsPublic() || enum.EndpointVisibility("").IsPublic() {
		t.Fatal("private/empty must not report public")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/endpoint_visibility/`
Expected: FAIL — `undefined: enum.EndpointVisibility`.

- [ ] **Step 3: 实现枚举**

按 `internal/common/enum/oauth_provider.go` 的写法：常量集合 + `slices.Contains` 实现 `IsValid`，`Normalize` 把空串归一为 `EndpointVisibilityPrivate`，`IsPublic` 等价于 `== EndpointVisibilityPublic`。

- [ ] **Step 4: 跑测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/endpoint_visibility/`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/common/enum/endpoint_visibility.go test/unit/endpoint_visibility/
git commit -m "feat(endpoint): add EndpointVisibility enum"
```

---

### Task 2: 聚合根与持久化

**Files:**
- Modify: `internal/domain/llmproxy/aggregate/endpoint.go`
- Modify: `internal/infrastructure/database/model/endpoint.go`
- Modify: `internal/common/constant/string.go`（`FieldEndpointAuthType` 之后）
- Modify: `internal/common/constant/sql.go:153`（`EndpointRepoFieldsFull`）
- Modify: `internal/infrastructure/repository/endpoint_repository.go`（`toEndpointAggregate`、`toEndpointModel`、`Update`）
- Create: `internal/infrastructure/repository/visibility_scope.go`
- Test: `test/unit/domain_llmproxy/endpoint_visibility_test.go`、`test/unit/llmproxy_repo_scope/endpoint_visibility_test.go`

**Interfaces:**
- Consumes: `enum.EndpointVisibility`（Task 1）。
- Produces:
  - `aggregate.Endpoint.Visibility() enum.EndpointVisibility`
  - `aggregate.Endpoint.SetVisibility(v enum.EndpointVisibility) error`（写路径：非法值 `ierr.ErrValidation`）
  - `aggregate.Endpoint.RestoreVisibility(v enum.EndpointVisibility)`（读路径恢复：非法/空值归一为 private，**不返回 error**——`errcheck` 开了 `check-blank`，仓储恢复不能吞 error）
  - `dbmodel.Endpoint.Visibility string`（列 `visibility`，`not null;default:'private'`）
  - `constant.FieldEndpointVisibility = "visibility"`
  - `repository` 包内（同包共用）：`whereModelInPublicEndpoint`、`whereEndpointVisibilityEquals` 两个 SQL 条件常量

- [ ] **Step 1: 写聚合根失败测试**

`test/unit/domain_llmproxy/endpoint_visibility_test.go`（同包已有 `mustEndpoint` 之类辅助就复用，没有就本地写一个）：

```go
func TestEndpoint_DefaultsToPrivate(t *testing.T) {
	t.Parallel()
	ep, err := aggregate.CreateEndpoint(0, "ep", "https://o.example.com", "", "k", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if ep.Visibility() != enum.EndpointVisibilityPrivate {
		t.Fatalf("new endpoint visibility = %q, want private", ep.Visibility())
	}
}

func TestEndpoint_SetVisibilityRejectsUnknown(t *testing.T) {
	t.Parallel()
	ep := mustEndpoint(t)
	if err := ep.SetVisibility("shared"); err == nil {
		t.Fatal("unknown visibility must be rejected")
	}
	if err := ep.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatalf("set public: %v", err)
	}
	if ep.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatalf("visibility = %q, want public", ep.Visibility())
	}
}

func TestEndpoint_RestoreVisibilityNormalizesInvalid(t *testing.T) {
	t.Parallel()
	ep := mustEndpoint(t)
	ep.RestoreVisibility("garbage")
	if ep.Visibility() != enum.EndpointVisibilityPrivate {
		t.Fatalf("invalid restored visibility = %q, want private", ep.Visibility())
	}
	ep.RestoreVisibility(enum.EndpointVisibilityPublic)
	if ep.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatal("restore must keep valid value")
	}
}

func TestEndpoint_UpdatePreservesVisibility(t *testing.T) {
	t.Parallel()
	ep := mustEndpoint(t)
	if err := ep.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatal(err)
	}
	name := "renamed"
	if err := ep.Update(&name, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if ep.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatal("Update must not reset visibility")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/domain_llmproxy/ -run TestEndpoint_`
Expected: FAIL — `ep.Visibility undefined`.

- [ ] **Step 3: 实现聚合根**

`Endpoint` 加 `visibility enum.EndpointVisibility`；`newEndpoint` 里 `visibility: visibility.Normalize()`（**不要**改 `CreateEndpoint` / `RehydrateEndpoint` 的签名——两者都经 `newEndpoint` 用零值初始化，再由 setter 覆盖）；`Update` 的 `next := *e` 已天然保留该字段。

两个入口分工：
- `SetVisibility`：写路径用，`IsValid()` 失败返回 `ierr.ErrValidation`；
- `RestoreVisibility`：仓储读路径用，不走 error（`v.Normalize()` 后直接赋值，脏值按 private 处理）。

- [ ] **Step 4: 跑聚合测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/domain_llmproxy/ -run TestEndpoint_`
Expected: PASS。

- [ ] **Step 5: 写仓储 roundtrip 失败测试**

`test/unit/llmproxy_repo_scope/endpoint_visibility_test.go`（复用本包 `newTestDB(t)`）：

```go
func TestEndpointRepo_VisibilityRoundTrip(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := repository.NewEndpointRepository(db)

	ep, err := aggregate.CreateEndpoint(0, "ep-public", "https://o.example.com", "", "k", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := ep.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatal(err)
	}
	id, err := repo.Create(ctx, ep, 101)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByID(ctx, id, lo.ToPtr(uint(101)))
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatalf("visibility not persisted: %+v", got)
	}

	name := "ep-public-renamed"
	if err := got.Update(&name, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	after, err := repo.FindByID(ctx, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if after.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatalf("repo.Update dropped visibility: %q", after.Visibility())
	}
}
```

- [ ] **Step 6: 跑仓储测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ -run TestEndpointRepo_VisibilityRoundTrip`
Expected: FAIL — 读回的 visibility 为 `""`（字段白名单未含新列时的表现）。

- [ ] **Step 7: 实现持久化**

1. `dbmodel.Endpoint` 在 `AuthType` 之后加：
   ```go
   Visibility string `json:"visibility" gorm:"column:visibility;not null;default:'private';comment:可见性(private/public)"`
   ```
2. `constant/string.go` 加 `FieldEndpointVisibility = "visibility"`。
3. `constant/sql.go` 的 `EndpointRepoFieldsFull` 追加 `FieldEndpointVisibility`。
4. `endpoint_repository.go`：
   - `toEndpointAggregate`：`ep.RestoreVisibility(enum.EndpointVisibility(m.Visibility))`（脏值归一为 private）；
   - `toEndpointModel`：`Visibility: string(ep.Visibility())`；
   - `Update` 的 `updates` map 追加 `constant.FieldEndpointVisibility: string(ep.Visibility())`。
5. 新文件 `internal/infrastructure/repository/visibility_scope.go`：
   ```go
   // whereModelInPublicEndpoint 模型所属端点公开且未删除（参数为 visibility 枚举值）。
   const whereModelInPublicEndpoint = "endpoint_id IN (SELECT id FROM endpoints WHERE visibility = ? AND deleted_at = 0)"

   // whereEndpointVisibilityEquals 端点可见性等值条件。
   const whereEndpointVisibilityEquals = "visibility = ?"
   ```

- [ ] **Step 8: 跑测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ ./test/unit/domain_llmproxy/`
Expected: PASS。

- [ ] **Step 9: 提交**

```bash
git add internal/domain/llmproxy/aggregate/endpoint.go internal/infrastructure/database/model/endpoint.go \
  internal/common/constant/string.go internal/common/constant/sql.go \
  internal/infrastructure/repository/endpoint_repository.go internal/infrastructure/repository/visibility_scope.go \
  test/unit/domain_llmproxy/endpoint_visibility_test.go test/unit/llmproxy_repo_scope/endpoint_visibility_test.go
git commit -m "feat(endpoint): persist endpoint visibility"
```

---

### Task 3: Upstream 分组列表（`FindIDsByScope` 签名 + includePublic + 展示字段）

**Files:**
- Modify: `internal/domain/llmproxy/repository.go`（`EndpointRepository.FindIDsByScope`）
- Modify: `internal/infrastructure/repository/endpoint_repository.go`
- Modify: `internal/application/upstream/port/handler.go`、`internal/application/upstream/query/list_upstream.go`
- Modify: `internal/handler/upstream.go`
- Modify: `internal/dto/upstream.go`
- Modify（编译修复）: `test/unit/endpoint_command/*.go`、`test/unit/endpoint_resolver/*.go`、`test/unit/oauth_flow/*.go` 里所有 `FindIDsByScope` stub
- Test: `test/unit/llmproxy_repo_scope/endpoint_scope_public_test.go`

**Interfaces:**
- Consumes: `enum.EndpointVisibility`（Task 1）、`whereEndpointVisibilityEquals`（Task 2）。
- Produces:
  - `EndpointRepository.FindIDsByScope(ctx context.Context, scopeUserID *uint, includePublic bool) ([]uint, error)`
  - `port.ListUpstreamQuery.IncludePublic bool`
  - `port.UpstreamEndpointView.Visibility string`
  - `dto.UpstreamEndpointItem.Visibility string`

- [ ] **Step 1: 写仓储失败测试**

`test/unit/llmproxy_repo_scope/endpoint_scope_public_test.go`：seed 一个公开端点（`Visibility: "public"`）、用户 101/202 各一个私有端点、一个共享池端点。断言：

- `FindIDsByScope(ctx, lo.ToPtr(uint(101)), true)` 含 101 自己的 + 公开端点，不含 202 的私有；
- `includePublic=false` 只含 101 自己的；
- `scope=nil` 返回全部（admin 视角），与 `includePublic` 无关。

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ -run TestFindIDsByScope`
Expected: FAIL — 参数个数不匹配。

- [ ] **Step 3: 实现仓储与查询链路**

- `FindIDsByScope`：
  ```go
  q := db.Model(&dbmodel.Endpoint{}).Where(constant.DBConditionDeletedAtZero)
  if scopeUserID != nil {
      userCond := db.Where(constant.WhereUserIDEquals, *scopeUserID)
      if includePublic {
          q = q.Where(userCond.Or(whereEndpointVisibilityEquals, enum.EndpointVisibilityPublic))
      } else {
          q = q.Where(userCond)
      }
  }
  ```
  **必须**用 `db.Where(A).Or(B)` 的分组形式：直接把 `"user_id = ? OR visibility = ?"` 塞进一次 `Where` 会被 GORM 拼成 `deleted_at=0 AND user_id=? OR visibility='public'`，优先级错误会把已删除的公开端点带出来。
- `port.ListUpstreamQuery` 加 `IncludePublic bool`；`list_upstream.go` 透传到 `FindIDsByScope`。
- `handler/upstream.go`：`IncludePublic: perm != enum.PermissionAdmin`。
- `port.UpstreamEndpointView` 加 `Visibility string`；`toEndpointView` 填 `string(ep.Visibility())`。
- `dto.UpstreamEndpointItem` 加 `Visibility string \`json:"visibility" doc:"可见性(private/public)"\``；DTO 映射函数补一行。

- [ ] **Step 4: 修复所有 stub 编译**

全仓搜索 `FindIDsByScope(`，给每个实现补上 `bool` 参数（`test/unit/endpoint_command`、`test/unit/endpoint_resolver`、`test/unit/oauth_flow`）。跑 `go build ./... && go vet ./...` 直到干净。

- [ ] **Step 5: 跑测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ ./test/unit/endpoint_command/ ./test/unit/endpoint_resolver/ ./test/unit/oauth_flow/ ./test/e2e/cross_tenant_reference/`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/domain/llmproxy/repository.go internal/infrastructure/repository/endpoint_repository.go \
  internal/application/upstream/ internal/handler/upstream.go internal/dto/upstream.go test/unit/ test/e2e/
git commit -m "feat(web): include public endpoints in upstream list"
```

---

### Task 4: 权限规则（DTO / handler / 命令）

**Files:**
- Modify: `internal/dto/endpoint.go`
- Modify: `internal/application/endpoint/port/handler.go`
- Modify: `internal/application/endpoint/command/create_endpoint.go`、`update_endpoint.go`、`delete_endpoint.go`、`reauth_oauth.go`
- Modify: `internal/handler/endpoint.go`
- Test: `test/unit/endpoint_command/endpoint_visibility_guard_test.go`

**Interfaces:**
- Consumes: `aggregate.Endpoint.SetVisibility` / `Visibility`（Task 2）。
- Produces:
  - `port.CreateEndpointCommand.Visibility enum.EndpointVisibility`
  - `port.UpdateEndpointCommand.Visibility *enum.EndpointVisibility`、`.IsAdmin bool`
  - `port.DeleteEndpointCommand.IsAdmin bool`
  - `port.ReauthOAuthCommand.IsAdmin bool`

- [ ] **Step 1: 写命令层守卫失败测试**

`test/unit/endpoint_command/endpoint_visibility_guard_test.go`，新增可注入 endpoint 的 stub（**不要**改现有 `capturingEndpointRepo`，避免影响既有断言）：

```go
type fakeEndpointRepo struct {
	ep      *aggregate.Endpoint
	updated bool
	deleted bool
}

func (f *fakeEndpointRepo) FindByID(context.Context, uint, *uint) (*aggregate.Endpoint, error) {
	return f.ep, nil
}
func (f *fakeEndpointRepo) BatchFindByIDs(context.Context, []uint) (map[uint]*aggregate.Endpoint, error) {
	return nil, nil
}
func (f *fakeEndpointRepo) Create(context.Context, *aggregate.Endpoint, uint) (uint, error) { return 1, nil }
func (f *fakeEndpointRepo) Update(context.Context, *aggregate.Endpoint) error { f.updated = true; return nil }
func (f *fakeEndpointRepo) Delete(context.Context, uint, *uint) error        { f.deleted = true; return nil }
func (f *fakeEndpointRepo) DeleteCascade(context.Context, uint, *uint) error { f.deleted = true; return nil }
func (f *fakeEndpointRepo) List(context.Context) ([]*aggregate.Endpoint, error) { return nil, nil }
func (f *fakeEndpointRepo) Paginate(context.Context, model.CommonParam, *uint) ([]*aggregate.Endpoint, *model.PageInfo, error) {
	return nil, nil, nil
}
func (f *fakeEndpointRepo) FindIDsByScope(context.Context, *uint, bool) ([]uint, error) { return nil, nil }

var _ llmproxy.EndpointRepository = (*fakeEndpointRepo)(nil)
```

用例（4 个）：

1. `TestUpdateEndpoint_NonAdminCannotModifyPublic`：`IsAdmin:false` + 公开端点 → 返回错误，且 `updated == false`。
2. `TestUpdateEndpoint_AdminCanModifyPublic`：`IsAdmin:true` → 无错误且 `updated == true`。
3. `TestDeleteEndpoint_NonAdminCannotDeletePublic`：`IsAdmin:false` → 错误且 `deleted == false`。
4. `TestUpdateEndpoint_WithoutVisibilityKeepsCurrent`：admin 只改名字 → 端点仍是 public。

公开端点 fixture：

```go
func publicEndpoint(t *testing.T) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(7, "ep", "https://o.example.com", "", "k", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := ep.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatal(err)
	}
	return ep
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/endpoint_command/ -run 'TestUpdateEndpoint_NonAdminCannotModifyPublic|TestDeleteEndpoint_NonAdminCannotDeletePublic'`
Expected: FAIL — 命令没有 `IsAdmin` 字段 / 没有守卫。

- [ ] **Step 3: 实现命令层**

- `update_endpoint.go`：`FindByID` 之后、`ep.Update` 之前：
  ```go
  if ep.Visibility().IsPublic() && !cmd.IsAdmin {
      return ierr.New(ierr.ErrUnauthorized, "only admin can modify a public endpoint")
  }
  ```
  `ep.Update(...)` 之后：
  ```go
  if cmd.Visibility != nil {
      if err := ep.SetVisibility(*cmd.Visibility); err != nil { return err }
  }
  ```
- `delete_endpoint.go`：`FindByID` 之后同样守卫（`ierr.ErrUnauthorized`）。
- `reauth_oauth.go`：先读文件确认端点查找位置，再插入同样的守卫。

- [ ] **Step 4: 跑测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/endpoint_command/`
Expected: PASS。

- [ ] **Step 5: 实现 DTO 与 handler**

- `dto/endpoint.go` 两个 ReqBody 各加：
  ```go
  Visibility *enum.EndpointVisibility `json:"visibility,omitempty" enum:"private,public" doc:"可见性(仅管理员可设为 public)"`
  ```
  （加 import `internal/common/enum`。）
- `handler/endpoint.go`：
  - `HandleCreateEndpoint`：
    ```go
    perm := util.CtxValuePermission(ctx)
    isAdmin := perm == enum.PermissionAdmin
    visibility := enum.EndpointVisibilityPrivate
    if req.Body.Visibility != nil {
        if !isAdmin {
            return nil, apiutil.NewHumaBizError(ctx, ierr.ErrUnauthorized, ierr.ErrUnauthorized.BizError())
        }
        visibility = req.Body.Visibility.Normalize()
        if !visibility.IsValid() {
            return nil, apiutil.NewHumaBizError(ctx, ierr.ErrValidation, ierr.ErrValidation.BizError())
        }
    }
    ownerUserID := lo.Ternary(isAdmin && req.Body.OwnerUserID != nil && !visibility.IsPublic(), lo.FromPtr(req.Body.OwnerUserID), userID)
    ```
    命令带 `Visibility: visibility`。
  - `HandleUpdateEndpoint`：
    ```go
    isAdmin := util.CtxValuePermission(ctx) == enum.PermissionAdmin
    if req.Body.Visibility != nil && !isAdmin {
        return nil, apiutil.NewHumaBizError(ctx, ierr.ErrUnauthorized, ierr.ErrUnauthorized.BizError())
    }
    if req.Body.Visibility != nil && !req.Body.Visibility.IsValid() {
        return nil, apiutil.NewHumaBizError(ctx, ierr.ErrValidation, ierr.ErrValidation.BizError())
    }
    ```
    命令带 `Visibility: req.Body.Visibility, IsAdmin: isAdmin`。
  - `HandleDeleteEndpoint` / `HandleReauthOAuth`：命令带 `IsAdmin: util.CtxValuePermission(ctx) == enum.PermissionAdmin`。

- [ ] **Step 6: 编译并跑相关包**

Run: `GOCACHE=/tmp/aris-go-cache go build ./... && GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/endpoint_command/ ./test/unit/oauth_flow/`
Expected: PASS（`go build` 无输出）。

- [ ] **Step 7: 提交**

```bash
git add internal/dto/endpoint.go internal/application/endpoint/ internal/handler/endpoint.go test/unit/endpoint_command/
git commit -m "feat(endpoint): restrict visibility writes to admin"
```

---

### Task 5: 解析链路（公开池回退）

**Files:**
- Modify: `internal/domain/llmproxy/repository.go`（`ModelRepository`）
- Modify: `internal/infrastructure/repository/model_repository.go`
- Modify: `internal/domain/llmproxy/service/resolver.go`
- Test: `test/unit/endpoint_resolver/public_pool_test.go`

**Interfaces:**
- Consumes: `whereModelInPublicEndpoint`（Task 2）。
- Produces: `ModelRepository.FindPublicByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error)`。

- [ ] **Step 1: 写失败测试**

`test/unit/endpoint_resolver/public_pool_test.go`：fake 仓储（内存 map），记录 `FindPublicByAlias` 调用次数与每次 `FindByID` 收到的 scope。

```go
type visibilityModelRepo struct {
	own         map[string][]*aggregate.Model
	public      map[string][]*aggregate.Model
	publicCalls int
}

func (r *visibilityModelRepo) FindByAlias(_ context.Context, alias vo.EndpointAlias, userID *uint) ([]*aggregate.Model, error) {
	if userID != nil && *userID == 0 {
		return nil, nil // 共享池：本用例不涉及
	}
	return r.own[alias.String()], nil
}

func (r *visibilityModelRepo) FindPublicByAlias(_ context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error) {
	r.publicCalls++
	return r.public[alias.String()], nil
}
// 其余 llmproxy.ModelRepository 方法返回零值；FindIDsByScope 等不涉及
```

5 个用例（各一个 subtest）：

1. `TestResolveCandidates_OwnHitSkipsPublic`：自有与公开同 alias → 候选来自自有，`publicCalls == 0`。
2. `TestResolveCandidates_FallsBackToPublic`：自有为空、公开有 → 返回公开候选。
3. `TestResolveCandidates_PublicWorksWithoutSharedPoolFallback`：`NewEndpointResolver(..., sharedPoolFallback=false, nil)` 下公开仍命中。
4. `TestResolveCandidates_MultipleAdminsUseOwnScope`：公开两条（owner 201/202）→ endpointRepo 收到的 scope 分别是 201、202。
5. `TestResolveCandidates_DisabledOwnModelFallsBackToPublic`：自有返回空（模拟禁用被过滤）→ 走公开。

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/endpoint_resolver/ -run TestResolveCandidates_`
Expected: FAIL — `FindPublicByAlias` 未定义。

- [ ] **Step 3: 实现仓储方法**

`ModelRepository` 接口加方法；`modelRepository` 实现：

```go
func (r *modelRepository) FindPublicByAlias(ctx context.Context, alias vo.EndpointAlias) ([]*aggregate.Model, error) {
	db := r.db.WithContext(ctx).Where(whereModelInPublicEndpoint, enum.EndpointVisibilityPublic)
	models, err := r.dao.BatchGet(db, &dbmodel.Model{Alias: alias.String(), Enabled: true}, constant.ModelRepoFieldsFull)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "find public models by alias")
	}
	return util.MapErr(models, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) { return toModelAggregate(m) })
}
```

- [ ] **Step 4: 实现 resolver 回退**

`collectCandidates` 顺序改为：自有租户 → 公开池（无条件）→ 共享池（受 `sharedPoolFallback` 控制）；endpoint scope 改为逐模型 `&m.UserID()`（不要再用 `models[0].UserID()` 特判）。

- [ ] **Step 5: 跑测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/endpoint_resolver/ ./test/unit/llmproxy_usecase/`
Expected: PASS（既有 `candidates_test.go`、`oauth_filter_test.go` 保持绿）。

- [ ] **Step 6: 提交**

```bash
git add internal/domain/llmproxy/repository.go internal/infrastructure/repository/model_repository.go \
  internal/domain/llmproxy/service/resolver.go test/unit/endpoint_resolver/public_pool_test.go
git commit -m "feat(gateway): resolve aliases from public endpoint pool"
```

---

### Task 6: Web 平铺模型列表（includePublic + 端点可见性）

**Files:**
- Modify: `internal/domain/llmproxy/repository.go`（`ModelListFilter`）
- Modify: `internal/infrastructure/repository/model_repository.go`（`loadModelPage`）
- Modify: `internal/application/model/port/list_model.go`、`internal/application/model/query/list_model.go`
- Modify: `internal/handler/model.go`
- Modify: `internal/dto/model.go`
- Test: `test/unit/llmproxy_repo_scope/model_list_public_test.go`

**Interfaces:**
- Consumes: `whereModelInPublicEndpoint`（Task 2）。
- Produces:
  - `llmproxy.ModelListFilter.IncludePublic bool`
  - `port.ListModelQuery.IncludePublic bool`
  - `port.ListModelEndpointView.Visibility string`
  - `dto.ModelListEndpointItem.Visibility string`

- [ ] **Step 1: 写仓储失败测试**

seed：用户 101 一个私有模型、admin 公开端点下一个模型、用户 202 一个私有模型。断言：

- `PaginateWithFilter(param, ModelListFilter{IncludePublic:true}, lo.ToPtr(uint(101)))` 返回 101 的 + 公开模型，不含 202 的；
- `IncludePublic:false` 只返回 101 的。

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ -run TestPaginateModels_IncludePublic`
Expected: FAIL — 公开模型没出现。

- [ ] **Step 3: 实现**

- `ModelListFilter` 加 `IncludePublic bool`；`loadModelPage` 的 scope 分支：
  ```go
  if scopeUserID != nil {
      if filter.IncludePublic {
          db = db.Where(db.Where(constant.WhereUserIDEquals, *scopeUserID).Or(whereModelInPublicEndpoint, enum.EndpointVisibilityPublic))
      } else {
          db = db.Where(constant.WhereUserIDEquals, *scopeUserID)
      }
  }
  ```
- `port.ListModelQuery` 加 `IncludePublic bool`；`list_model.go` 放进 `ModelListFilter`。
- `handler/model.go`：`IncludePublic: perm != enum.PermissionAdmin`。
- `port.ListModelEndpointView` 加 `Visibility string`；`toListModelView` 从 `epsByID` 取 `string(ep.Visibility())`。
- `dto.ModelListEndpointItem` 加 `Visibility string`；DTO 映射补一行。

- [ ] **Step 4: 跑测试与编译**

Run: `GOCACHE=/tmp/aris-go-cache go build ./... && GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ ./test/unit/endpoint_query/`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/domain/llmproxy/repository.go internal/infrastructure/repository/model_repository.go \
  internal/application/model/ internal/handler/model.go internal/dto/model.go \
  test/unit/llmproxy_repo_scope/model_list_public_test.go
git commit -m "feat(web): include public models in flat model list"
```

---

### Task 7: 网关模型目录（`/v1/models`、CLI 导出、count_tokens）

**Files:**
- Modify: `internal/infrastructure/repository/endpoint_repository.go`（`loadAliases`、`loadModelDetails`、`FindEndpointByAlias`）
- Test: `test/unit/llmproxy_repo_scope/endpoint_read_public_test.go`

**Interfaces:**
- Consumes: `whereModelInPublicEndpoint`（Task 2）、`FindPublicByAlias`（Task 5 不改这三条读路径）。
- Produces: 无新签名。

- [ ] **Step 1: 写失败测试**

seed（sqlite + `cache.NewReadCache(miniredis)` + `repository.NewCachedEndpointReadRepository`）：用户 101 自有 alias `gpt-x`（context=1000），admin 公开端点下同名 `gpt-x`（context=9999）与 `gpt-public`。断言：

- `ListAliases(ctx, 101)` 同时含 `gpt-x`、`gpt-public`，`gpt-x` 只出现一次；
- `ListEnabledModelDetails(ctx, 101)` 中 `gpt-x` 的 `ContextLength == 1000`（自有优先，不被公开行覆盖）；
- `FindEndpointByAlias(ctx, 101, "gpt-public", nil)` 返回公开端点投影；
- `ListAliases(ctx, 0)` 返回空（认证缺失 fail-closed）。

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ -run TestEndpointRead`
Expected: FAIL — 公开别名不出现。

- [ ] **Step 3: 实现**

三条查询改为「自有 ∪ 公开」，并加自有优先排序，保证 `lo.UniqBy(alias)` 保留自有行：

```go
db := r.db.WithContext(ctx).
    Where(db.Where(constant.WhereUserIDEquals, userID).Or(whereModelInPublicEndpoint, enum.EndpointVisibilityPublic)).
    Order(clause.OrderBy{Expression: clause.Expr{SQL: "(user_id = ?) DESC", Vars: []any{userID}}})
```

`FindEndpointByAlias` 保持随机选 endpoint 的行为，只改候选集合来源。

- [ ] **Step 4: 写缓存失效失败测试（Review Focus #1）**

同一文件加 `TestEndpointRead_PublicFlipInvalidatesCache`：

1. 调 `ListAliases(ctx, 101)` 预热缓存（此时只有自有 alias）；
2. 直接改库把 admin 端点置为 `public`，再调 `repository.NewCachedEndpointRepository(db, rc).Update(ctx, ep)`（走真实失效钩子）；
3. 再调 `ListAliases(ctx, 101)`，断言公开 alias 已出现。

- [ ] **Step 5: 跑测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/unit/llmproxy_repo_scope/ ./test/unit/cache/`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/infrastructure/repository/endpoint_repository.go test/unit/llmproxy_repo_scope/endpoint_read_public_test.go
git commit -m "feat(gateway): expose public models in catalog reads"
```

---

### Task 8: 前端（可见性表单、徽标、只读门禁）

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/app/(dashboard)/upstream/endpoint-form.ts`
- Modify: `web/src/app/(dashboard)/upstream/endpoint-dialog.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`
- Modify: `web/src/app/(dashboard)/upstream/grouped-view.tsx`、`flat-view.tsx`
- Modify: i18n 语言文件（末尾追加 key）
- Test: `web/src/app/(dashboard)/upstream/__tests__/`

**Interfaces:**
- Consumes: `UpstreamEndpointItem.visibility`、`ModelListEndpoint.visibility`（Task 3/6）。
- Produces: `EndpointVisibility = "private" | "public"`；`EndpointForm.visibility: EndpointVisibility`。

- [ ] **Step 1: 写失败的前端单测**

在 `web/src/app/(dashboard)/upstream/__tests__/` 加：

```ts
it("默认创建表单为私有", () => {
  expect(emptyEndpointForm.visibility).toBe("private");
});
```

- [ ] **Step 2: 跑测试确认失败**

Run（`web/` 下）：`npx vitest run src/app/\(dashboard\)/upstream/__tests__`
Expected: FAIL — `visibility` 不存在。

- [ ] **Step 3: 实现**

- `types.ts`：`export type EndpointVisibility = "private" | "public";`；`UpstreamEndpointItem` 加 `visibility: EndpointVisibility`；`ModelListEndpoint` 加 `visibility: EndpointVisibility`；`CreateEndpointReqBody` / `UpdateEndpointReqBody` 加 `visibility?: EndpointVisibility`。
- `endpoint-form.ts`：`EndpointForm` 加 `visibility: EndpointVisibility`，`emptyEndpointForm.visibility = "private"`。
- `endpoint-dialog.tsx`：`isAdmin` 时渲染「公开 / 私有」Select（`t("endpoints.visibility_private")` / `t("endpoints.visibility_public")`）；`form.ownerUserID` 有值时禁用并重置为 `private`。
- `page.tsx`：create/update payload 带 `visibility: isAdmin() ? endpointForm.visibility : undefined`；打开编辑时用 `endpoint.visibility` 初始化表单。
- `grouped-view.tsx` / `flat-view.tsx`：`visibility === "public"` 渲染公开徽标；`!isAdmin && visibility === "public"` 时隐藏该端点与其模型的编辑/删除/授权按钮。
- i18n：追加 `endpoints.visibility`、`endpoints.visibility_private`、`endpoints.visibility_public`、`upstream.public_badge` 到各语言文件末尾。

- [ ] **Step 4: 验证**

Run（`web/` 下）：
```bash
npx tsc --noEmit
npm run lint
npx vitest run
```
Expected: 全部通过。

- [ ] **Step 5: 提交**

```bash
git add web/src/lib/types.ts "web/src/app/(dashboard)/upstream/" web/src/lib/i18n/
git commit -m "feat(web): endpoint visibility form, badge and read-only gating"
```

---

### Task 9: 端到端用例（进程内真实路由）

**Files:**
- Create: `test/e2e/public_endpoint/public_endpoint_test.go`

**Interfaces:**
- Consumes: 全部后端改动（Task 1–7）。
- Produces: 无。

- [ ] **Step 1: 写 E2E 用例**

照 `test/e2e/cross_tenant_reference/cross_tenant_test.go` 的进程内装配（fiber + `router.RegisterAPIRouter` + sqlite + miniredis + stub handlers），新 fixture：

- 用户 `alice`（user）、`admin`（admin）；
- admin `POST /api/web/v1/endpoint/`（`visibility: "public"`）→ `POST /api/web/v1/model/`；
- `GET /api/web/v1/upstream/list`（alice）能看到且 `endpoint.visibility == "public"`；
- alice 对该 endpoint 的 PATCH/DELETE、对 model 的 PATCH/DELETE 全部失败；
- alice 带 `visibility: "public"` 创建端点失败；
- admin PATCH 回 `private` 后，alice 的列表不再出现该端点。

- [ ] **Step 2: 跑测试确认失败**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/e2e/public_endpoint/`
Expected: FAIL。

- [ ] **Step 3: 让用例通过**

只修后端真实缺陷；若发现设计缺口（如某条路由没带 `IncludePublic`），回到对应任务补测试再修。

- [ ] **Step 4: 跑测试确认通过**

Run: `GOCACHE=/tmp/aris-go-cache go test -count=1 ./test/e2e/public_endpoint/ -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add test/e2e/public_endpoint/
git commit -m "test(e2e): public endpoint visibility and read-only enforcement"
```

---

### Task 10: 文档与经验沉淀

**Files:**
- Modify: `CONTEXT.md`
- Create: Serena memory（`mcp__serena__write_memory`）

- [ ] **Step 1: 更新 `CONTEXT.md`**

- `Endpoint` 词条补 `visibility`：`private`（默认，仅归属者与 admin）/ `public`（全体登录用户只读、可调用；仅 admin 可设置，归属仍是 admin）。
- 新增 **EndpointVisibility（端点可见性）** 与 **PublicEndpoint（公开端点）** 词条：公开模型消耗调用者自己的 API Key 配额，但上游鉴权用端点共享密钥，因此消耗 admin 的上游额度。
- `Model` 词条补「模型可见性继承所属 endpoint」。
- `EndpointResolver` 词条补回退顺序：自有租户 → 公开池 → 共享池（受 `gateway.shared_pool_fallback` 控制）。

- [ ] **Step 2: 全量回归 + lint**

Run:
```bash
GOCACHE=/tmp/aris-go-cache go test -count=1 ./cmd/... ./internal/... ./test/...
GOCACHE=/tmp/aris-go-cache make lint
```
Expected: 全绿、0 issues。

- [ ] **Step 3: 写 Serena memory**

记录：用 `visibility` 枚举而非 `is_public`/共享池的理由；「只读性由租户 scope 天然保证 + 降级保护」的权限模型；GORM OR 必须分组；`EndpointRepoFieldsFull` 漏列的表现；读缓存 `InvalidateAll` 已覆盖公开化切换。

- [ ] **Step 4: `ponytail-review` 审 diff**

对本分支 diff 跑 `ponytail-review`，逐条处理或记录拒绝理由。

- [ ] **Step 5: 提交**

```bash
git add CONTEXT.md docs/superpowers/
git commit -m "docs(context): document endpoint visibility and public endpoint pool"
```
