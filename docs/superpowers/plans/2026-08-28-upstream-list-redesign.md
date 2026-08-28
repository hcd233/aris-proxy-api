# Upstream 列表重设计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重做 `/upstream` 页列表展示为「带列名的缩进树表」，并新增后端模型分页接口以支撑可切换的「平铺」全部模型视图。

**Architecture:** 后端在既有 `modelGroup`（`/api/web/v1/model`）下新增 `GET /list` 读路径，走 CQRS query handler + 仓储新方法 `PaginateWithFilter`；前端把 1359 行单文件按职责拆为 8 个模块，两视图各持一个 `useFilterBar` 实例。

**Tech Stack:** Go 1.25 / huma v2 / GORM / samber/lo；Next.js 15 App Router / React 19 / Tailwind v4 / vitest

**Spec:** `docs/superpowers/specs/2026-08-28-upstream-list-redesign-design.md`

## Global Constraints

- 排序字段必须走**显式列白名单**，禁止依赖 `util.SafeSortField`（它只校验字符集 `[a-zA-Z0-9_]`，`api_key` 一样放行）；非法值回退 `created_at desc`，不报错。
- scope 一律用 `*uint` 三态：admin `nil`、非 admin 真实 userID、`userID==0` → 401。**必须复用 `internal/handler/endpoint.go:118` 的 `scopeFor(ctx, perm)`**，禁止另写一份。
- `internal/` 下**禁止** `*_test.go`（conv lint 规则 `testing.internal_file`）；单测放 `test/unit/<pkg>/`，e2e 放 `test/e2e/<name>/`。
- 禁用 `errors.New`，用 `ierr.New(ierr.ErrXxx, msg)`；禁止业务包内 `const` 块（规则 `style.local_const`），常量放 `internal/common/constant` 或 `internal/enum`。
- DTO 层禁止导入 `internal/infrastructure/database/model`，禁用 `any`/`interface{}`/`json.RawMessage`。
- 中文注释 `//` 后必须半角空格（gocritic commentFormatting）；相邻同类型参数须合并声明（gocritic paramTypeCombine）。
- 前端新增截断文案必须配 Tooltip（自定义 ESLint 规则 `truncate-requires-tooltip`）；`TableCell` 上的 `truncate` 要移到内部 `span`。
- 列表项数组 JSON 字段名统一 `items`（对齐 API Keys 惯例）。
- 回归测试写完后必须**临时注入 buggy 版本确认 FAIL**，再改回——否则可能写出永远绿的假测试。

---

## File Structure

**后端（新建）**

| 文件 | 职责 |
|---|---|
| `internal/application/model/port/list_model.go` | `ListModelQuery` / `ListModelView` / `ListModelHandler` 契约 |
| `internal/application/model/query/list_model.go` | query handler：调仓储 + 批量回填 endpoint/user + demo 脱敏 |
| `test/unit/model_list_query/list_model_test.go` | handler 单测（fake 仓储） |
| `test/unit/model_list_repo/paginate_filter_test.go` | 仓储单测（sqlite 内存库，验 capability/status/排序白名单） |
| `test/e2e/model_list/model_list_test.go` | e2e：走生产路由入口 |

**后端（修改）**

| 文件 | 变更 |
|---|---|
| `internal/common/constant/sql.go` | 排序白名单 + `FieldModelCreatedAt` 等列名常量 |
| `internal/domain/llmproxy/repository.go` | `ModelRepository` 加 `PaginateWithFilter` + `ModelListFilter` 结构 |
| `internal/infrastructure/repository/endpoint_repository.go` | 实现 `PaginateWithFilter` |
| `internal/dto/model.go` | `ListModelsReq` / `ListModelsRsp` / `ModelListItem` / `ModelListEndpointItem` |
| `internal/handler/model.go` | `HandleListModels` + 接口方法 + `ModelDependencies.List` |
| `internal/router/model.go` | 注册 `GET /list` |
| `internal/bootstrap/modules/application.go` | 装配 `NewListModelHandler` |

**前端（拆分后）**

| 文件 | 职责 |
|---|---|
| `web/src/app/(dashboard)/upstream/page.tsx` | 容器：视图切换 + 编排 + 弹窗挂载（目标 <300 行） |
| `.../upstream/shared.tsx` | `OwnerCell` / `CapabilityBadges` / `SpecBadges` / `formatTokens` |
| `.../upstream/endpoint-dialog.tsx` | 端点弹窗 |
| `.../upstream/model-dialog.tsx` | 模型弹窗 + `TokenPresetPopover` |
| `.../upstream/grouped-view.tsx` | 分组树表 + 移动卡片 |
| `.../upstream/flat-view.tsx` | 平铺表 + 移动卡片 |
| `.../upstream/use-model-list.ts` | 平铺数据 hook（含排序状态） |
| `web/src/components/view-switch.tsx` | segmented control（分组/平铺） |
| `web/src/lib/types.ts` | 新增 3 类型，删除 4 死类型 |
| `web/src/lib/api-client.ts` | `listModelsPage` |
| `web/src/locales/{en,ja,zh}.json` | 文案 |

---

## Task 1: 排序白名单常量 + 仓储筛选分页

**Files:**
- Modify: `internal/common/constant/sql.go`
- Modify: `internal/domain/llmproxy/repository.go:34-46`
- Modify: `internal/infrastructure/repository/endpoint_repository.go`（在 `Paginate` 后新增方法）
- Test: `test/unit/model_list_repo/paginate_filter_test.go`

**Interfaces:**
- Produces:
  - `constant.ModelListSortFields` `[]string`
  - `constant.ModelListDefaultSortField` `= "created_at"`
  - `llmproxy.ModelListFilter{ Status string; EndpointID uint; Capability string }`
  - `llmproxy.ModelRepository.PaginateWithFilter(ctx context.Context, param model.CommonParam, filter ModelListFilter, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error)`

- [ ] **Step 1: 写失败测试**

创建 `test/unit/model_list_repo/paginate_filter_test.go`：

```go
// Package model_list_repo 验证模型列表分页的筛选、排序白名单与租户隔离。
package model_list_repo

import (
	"context"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/samber/lo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"),
		&gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Model{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// seedModels 造 4 条：u101 两条(1 启用 text / 1 停用 text+image)、u202 一条、共享池一条
func seedModels(t *testing.T, db *gorm.DB) {
	t.Helper()
	text := []enum.InputModality{enum.InputModalityText}
	both := []enum.InputModality{enum.InputModalityText, enum.InputModalityImage}
	rows := []*dbmodel.Model{
		{UserID: 101, Alias: "a-enabled", ModelID: "a", UpstreamModel: "up-a", EndpointID: 1, Enabled: true, Capabilities: text},
		{UserID: 101, Alias: "b-disabled", ModelID: "b", UpstreamModel: "up-b", EndpointID: 2, Enabled: false, Capabilities: both},
		{UserID: 202, Alias: "c-other", ModelID: "c", UpstreamModel: "up-c", EndpointID: 3, Enabled: true, Capabilities: text},
		{UserID: 0, Alias: "d-shared", ModelID: "d", UpstreamModel: "up-d", EndpointID: 4, Enabled: true, Capabilities: text},
	}
	for _, r := range rows {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func aliasesOf(ms []*aggregate.Model) []string {
	return lo.Map(ms, func(m *aggregate.Model, _ int) string { return m.Alias().String() })
}

func TestPaginateWithFilter_ScopeIsolation(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	seedModels(t, db)
	repo := repository.NewModelRepository(db)
	param := model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}}

	// admin(nil) 见全部 4 条
	all, page, err := repo.PaginateWithFilter(context.Background(), param, llmproxy.ModelListFilter{}, nil)
	if err != nil {
		t.Fatalf("admin paginate: %v", err)
	}
	if page.Total != 4 {
		t.Errorf("admin total: want 4, got %d", page.Total)
	}
	_ = all

	// scope=101 只见自己的 2 条，绝不含 202/共享池
	mine, minePage, err := repo.PaginateWithFilter(context.Background(), param, llmproxy.ModelListFilter{}, lo.ToPtr(uint(101)))
	if err != nil {
		t.Fatalf("scoped paginate: %v", err)
	}
	if minePage.Total != 2 {
		t.Errorf("scoped total: want 2, got %d", minePage.Total)
	}
	for _, a := range aliasesOf(mine) {
		if a == "c-other" || a == "d-shared" {
			t.Errorf("tenant leak: got %s", a)
		}
	}
}

func TestPaginateWithFilter_StatusAndCapability(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	seedModels(t, db)
	repo := repository.NewModelRepository(db)
	param := model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}}
	scope := lo.ToPtr(uint(101))

	// status=disabled 只返回停用的 b-disabled
	got, _, err := repo.PaginateWithFilter(context.Background(), param,
		llmproxy.ModelListFilter{Status: "disabled"}, scope)
	if err != nil {
		t.Fatalf("status filter: %v", err)
	}
	if len(got) != 1 || got[0].Alias().String() != "b-disabled" {
		t.Errorf("status=disabled: want [b-disabled], got %v", aliasesOf(got))
	}

	// capability=image 只返回含 image 的 b-disabled（text 模型不得命中）
	got, _, err = repo.PaginateWithFilter(context.Background(), param,
		llmproxy.ModelListFilter{Capability: enum.InputModalityImage}, scope)
	if err != nil {
		t.Fatalf("capability filter: %v", err)
	}
	if len(got) != 1 || got[0].Alias().String() != "b-disabled" {
		t.Errorf("capability=image: want [b-disabled], got %v", aliasesOf(got))
	}

	// capability 未知值视为不过滤（防前端拼错导致空白页）
	got, _, err = repo.PaginateWithFilter(context.Background(), param,
		llmproxy.ModelListFilter{Capability: "audio"}, scope)
	if err != nil {
		t.Fatalf("unknown capability: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("unknown capability should not filter: want 2, got %d", len(got))
	}
}

func TestPaginateWithFilter_SortFieldWhitelist(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	seedModels(t, db)
	repo := repository.NewModelRepository(db)
	scope := lo.ToPtr(uint(101))

	// 非白名单字段（含合法字符集但敏感的 api_key）必须回退默认排序，不得进 ORDER BY
	param := model.CommonParam{
		PageParam: model.PageParam{Page: 1, PageSize: 50},
		SortParam: model.SortParam{Sort: enum.SortAsc, SortField: "api_key"},
	}
	got, _, err := repo.PaginateWithFilter(context.Background(), param, llmproxy.ModelListFilter{}, scope)
	if err != nil {
		t.Fatalf("illegal sort field must not error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 rows after fallback, got %d", len(got))
	}

	// 白名单字段正常生效：alias 升序
	param.SortField = "alias"
	got, _, err = repo.PaginateWithFilter(context.Background(), param, llmproxy.ModelListFilter{}, scope)
	if err != nil {
		t.Fatalf("alias sort: %v", err)
	}
	if len(got) != 2 || got[0].Alias().String() != "a-enabled" {
		t.Errorf("alias asc: want a-enabled first, got %v", aliasesOf(got))
	}
}
```

补 import：文件顶部需加 `"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"`（`aliasesOf` 用到）。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./test/unit/model_list_repo/ -run TestPaginateWithFilter -v
```

Expected: 编译失败 —— `repo.PaginateWithFilter undefined` 与 `llmproxy.ModelListFilter undefined`。

- [ ] **Step 3: 加常量**

在 `internal/common/constant/sql.go` 的 upstream 常量附近追加：

```go
// ModelListDefaultSortField 模型列表默认排序列（配合 enum.SortDesc）
ModelListDefaultSortField = "created_at"
```

在同文件的 var 区（若无则新建）追加：

```go
// ModelListSortFields 模型列表允许的排序列白名单。
//
// 不可用 util.SafeSortField 代替：它只校验字符集，api_key 之类敏感列同样放行。
// 白名单外的取值回退 ModelListDefaultSortField，不报错（避免前端拼错导致整页 500）。
var ModelListSortFields = []string{
	"alias", "context_length", "max_output_tokens", "created_at", "endpoint_id", "enabled",
}
```

- [ ] **Step 4: 加领域接口**

`internal/domain/llmproxy/repository.go` 中 `ModelRepository` 接口内追加（紧随 `Paginate` 之后）：

```go
	// PaginateWithFilter 带筛选的分页查询（Web 平铺列表专用）。
	//
	// scopeUserID 语义同 Paginate；filter 各字段为零值时该维度不过滤。
	// param.SortField 必须落在 constant.ModelListSortFields 内，否则回退默认列。
	PaginateWithFilter(ctx context.Context, param model.CommonParam, filter ModelListFilter, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error)
```

在同文件 `ModelRepository` 接口定义之前追加：

```go
// ModelListFilter 模型列表筛选条件（零值表示该维度不过滤）
//
// Status 取 "enabled"/"disabled"，其余值视为不过滤；
// Capability 取 enum.InputModality 成员，未知值视为不过滤。
type ModelListFilter struct {
	Status     string
	EndpointID uint
	Capability string
}
```

- [ ] **Step 5: 实现仓储方法**

在 `internal/infrastructure/repository/endpoint_repository.go` 的 `Paginate` 方法之后插入：

```go
// PaginateWithFilter 带筛选的模型分页查询（Web 平铺列表专用）
//
// capabilities 是 text 列 + serializer:json（非 PG jsonb），且 enum.InputModalities
// 是封闭枚举，故 LIKE '%"x"%' 在 PG 与 sqlite 上语法与行为一致，无需分库写法。
// 代价是不走索引；models 表千行量级可接受。
func (r *modelRepository) PaginateWithFilter(ctx context.Context, param model.CommonParam, filter llmproxy.ModelListFilter, scopeUserID *uint) ([]*aggregate.Model, *model.PageInfo, error) {
	db := scopedDB(r.db.WithContext(ctx), scopeUserID)

	switch filter.Status {
	case constant.ModelStatusEnabled:
		db = db.Where(constant.WhereModelEnabledEquals, true)
	case constant.ModelStatusDisabled:
		db = db.Where(constant.WhereModelEnabledEquals, false)
	}
	if filter.EndpointID != 0 {
		db = db.Where(constant.WhereEndpointIDEquals, filter.EndpointID)
	}
	// 未知能力值视为不过滤，避免前端拼错导致空白页
	if lo.Contains(enum.InputModalities, filter.Capability) {
		db = db.Where(constant.WhereCapabilitiesLike, `%"`+filter.Capability+`"%`)
	}

	// 排序列白名单：非法值回退默认列，不报错
	if !lo.Contains(constant.ModelListSortFields, param.SortField) {
		param.SortField = constant.ModelListDefaultSortField
		param.Sort = enum.SortDesc
	}

	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Model{},
		constant.ModelRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldAlias, constant.FieldModelID, constant.FieldModelUpstreamModel}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate models with filter")
	}
	out, convErr := util.MapErr(records, func(m *dbmodel.Model, _ int) (*aggregate.Model, error) {
		return toModelAggregate(m)
	})
	if convErr != nil {
		return nil, nil, convErr
	}
	return out, pageInfo, nil
}
```

补齐 `internal/common/constant/sql.go` 中被引用的字符串常量（若已存在则跳过，勿重复定义）：

```go
// ModelStatusEnabled/Disabled 模型列表 status 筛选取值
ModelStatusEnabled  = "enabled"
ModelStatusDisabled = "disabled"

// WhereModelEnabledEquals/WhereEndpointIDEquals/WhereCapabilitiesLike 模型列表筛选条件
WhereModelEnabledEquals = "enabled = ?"
WhereEndpointIDEquals   = "endpoint_id = ?"
WhereCapabilitiesLike   = "capabilities LIKE ?"
```

同时确认 `internal/infrastructure/repository/endpoint_repository.go` 已 import `enum`（`github.com/hcd233/aris-proxy-api/internal/common/enum`），无则补。

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./test/unit/model_list_repo/ -v
```

Expected: 3 个测试全 PASS。

- [ ] **Step 7: 验证测试能捕获缺陷**

临时把 Step 5 的白名单守卫改为 `if false {`，重跑：

```bash
go test ./test/unit/model_list_repo/ -run TestPaginateWithFilter_SortFieldWhitelist -v
```

Expected: FAIL（sqlite 报 `no such column: api_key`）。确认后**改回**。

同理把 capability 守卫的 `lo.Contains(...)` 改成 `filter.Capability != ""`，重跑 `TestPaginateWithFilter_StatusAndCapability`，Expected: FAIL（unknown capability 返回 0 条而非 2 条）。确认后改回。

- [ ] **Step 8: 提交**

```bash
git add internal/common/constant/sql.go internal/domain/llmproxy/repository.go \
        internal/infrastructure/repository/endpoint_repository.go \
        test/unit/model_list_repo/
git commit -m "feat(model): add PaginateWithFilter with sort whitelist and capability filter"
```

---

## Task 2: application 层 query handler

**Files:**
- Create: `internal/application/model/port/list_model.go`
- Create: `internal/application/model/query/list_model.go`
- Test: `test/unit/model_list_query/list_model_test.go`

**Interfaces:**
- Consumes: `llmproxy.ModelRepository.PaginateWithFilter`、`llmproxy.ModelListFilter`（Task 1）
- Produces:
  - `port.ListModelQuery{ model.CommonParam; IsDemo bool; ScopeUserID *uint; Username string; Status string; EndpointID uint; Capability string }`
  - `port.ListModelView{ ID uint; User *port.ListModelUserView; Endpoint *port.ListModelEndpointView; Alias, ModelID, UpstreamModel string; Enabled bool; ContextLength, MaxOutputTokens int; Capabilities []enum.InputModality; CreatedAt, UpdatedAt time.Time }`
  - `port.ListModelUserView{ ID uint; Name, Avatar string }`
  - `port.ListModelEndpointView{ ID uint; Name string }`
  - `port.ListModelHandler.Handle(ctx, q) ([]*ListModelView, *model.PageInfo, error)`
  - `query.NewListModelHandler(modelRepo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository, userRepo identity.UserRepository) port.ListModelHandler`

- [ ] **Step 1: 写 port 契约**

创建 `internal/application/model/port/list_model.go`：

```go
package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// ListModelUserView 归属用户只读投影
type ListModelUserView struct {
	ID     uint
	Name   string
	Avatar string
}

// ListModelEndpointView 所属端点只读投影（仅列表展示所需最小字段）
type ListModelEndpointView struct {
	ID   uint
	Name string
}

// ListModelView 平铺模型列表行投影
type ListModelView struct {
	ID              uint
	User            *ListModelUserView     // 用户缺失/软删时为 nil
	Endpoint        *ListModelEndpointView // 端点缺失时为 nil
	Alias           string
	ModelID         string
	UpstreamModel   string
	Enabled         bool
	ContextLength   int
	MaxOutputTokens int
	Capabilities    []enum.InputModality
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ListModelQuery 平铺模型列表查询命令
//
// ScopeUserID 三态：nil（admin 视角）不过滤；非 nil 精确匹配 user_id。
// Username 仅在 ScopeUserID 为 nil（admin）时生效。
type ListModelQuery struct {
	model.CommonParam
	IsDemo      bool
	ScopeUserID *uint
	Username    string
	Status      string
	EndpointID  uint
	Capability  string
}

// ListModelHandler 平铺模型列表查询处理器
type ListModelHandler interface {
	Handle(ctx context.Context, q ListModelQuery) ([]*ListModelView, *model.PageInfo, error)
}
```

- [ ] **Step 2: 写失败测试**

创建 `test/unit/model_list_query/list_model_test.go`：

```go
// Package model_list_query 验证平铺模型列表查询的嵌套回填、username 解析与 demo 脱敏。
package model_list_query

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/application/model/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	useragg "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	uservo "github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	llmvo "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// testTime 固定时间戳，避免断言受当前时钟影响
func testTime() time.Time { return time.Unix(1756000000, 0) }

// mustUser 构造用户聚合。
//
// 注意：identity 聚合只暴露 RestoreUser（9 参数），没有 CreateUser——
// 参数顺序为 id, name, email, avatar, permission, lastLogin, createdAt,
// githubBindID, googleBindID。参见 test/unit/upstream_query 的既有用法。
func mustUser(id uint, name string) *useragg.User {
	return useragg.RestoreUser(
		id,
		uservo.UserName(name),
		uservo.Email(name+"@example.com"),
		uservo.Avatar("https://cdn.example.com/"+name+".png"),
		enum.PermissionUser,
		testTime(), testTime(), "", "",
	)
}

// ── fakes：嵌入接口后仅覆写用到的方法 ──

type fakeModelRepo struct {
	llmproxy.ModelRepository
	models     []*llmagg.Model
	gotFilter  llmproxy.ModelListFilter
	gotScope   *uint
}

func (f *fakeModelRepo) PaginateWithFilter(_ context.Context, param model.CommonParam, filter llmproxy.ModelListFilter, scope *uint) ([]*llmagg.Model, *model.PageInfo, error) {
	f.gotFilter = filter
	f.gotScope = scope
	return f.models, &model.PageInfo{Page: param.Page, PageSize: param.PageSize, Total: int64(len(f.models))}, nil
}

type fakeEndpointRepo struct {
	llmproxy.EndpointRepository
	endpoints map[uint]*llmagg.Endpoint
}

func (f *fakeEndpointRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*llmagg.Endpoint, error) {
	out := make(map[uint]*llmagg.Endpoint, len(ids))
	for _, id := range ids {
		if ep, ok := f.endpoints[id]; ok {
			out[id] = ep
		}
	}
	return out, nil
}

type fakeUserRepo struct {
	identity.UserRepository
	users   map[uint]*useragg.User
	byName  map[string]*useragg.User
	gotIDs  []uint
}

func (f *fakeUserRepo) BatchFindByIDs(_ context.Context, ids []uint) (map[uint]*useragg.User, error) {
	f.gotIDs = ids
	out := make(map[uint]*useragg.User, len(ids))
	for _, id := range ids {
		if u, ok := f.users[id]; ok {
			out[id] = u
		}
	}
	return out, nil
}

func (f *fakeUserRepo) FindByName(_ context.Context, name string) (*useragg.User, error) {
	return f.byName[name], nil
}

// ── helpers ──

func mustModel(t *testing.T, id, epID, userID uint, alias, upstream string, caps []enum.InputModality) *llmagg.Model {
	t.Helper()
	m, err := llmagg.CreateModel(id, llmvo.EndpointAlias(alias), upstream, epID, true, 1000, 100, caps)
	if err != nil {
		t.Fatalf("create model aggregate: %v", err)
	}
	m.SetUserID(userID)
	m.SetModelID(alias)
	return m
}

func mustEndpoint(t *testing.T, id uint, name string) *llmagg.Endpoint {
	t.Helper()
	ep, err := llmagg.CreateEndpoint(id, name, "https://o.example.com", "https://a.example.com", "sk-secret", true, false, false)
	if err != nil {
		t.Fatalf("create endpoint aggregate: %v", err)
	}
	return ep
}

func TestListModel_NestedBackfill(t *testing.T) {
	t.Parallel()
	text := []enum.InputModality{enum.InputModalityText}
	mRepo := &fakeModelRepo{models: []*llmagg.Model{
		mustModel(t, 11, 1, 101, "alias-a", "up-a", text),
		mustModel(t, 12, 999, 0, "alias-orphan", "up-b", text), // 端点不存在 + userID=0
	}}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{1: mustEndpoint(t, 1, "ep-one")}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{101: mustUser(101, "centonhuang")}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	views, page, err := h.Handle(context.Background(), port.ListModelQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}},
		ScopeUserID: lo.ToPtr(uint(101)),
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("total: want 2, got %d", page.Total)
	}
	if views[0].Endpoint == nil || views[0].Endpoint.Name != "ep-one" {
		t.Errorf("row 0 endpoint should be backfilled, got %+v", views[0].Endpoint)
	}
	if views[0].User == nil || views[0].User.Name != "centonhuang" {
		t.Errorf("row 0 user should be backfilled, got %+v", views[0].User)
	}
	// 端点缺失 → nil（不得造出空壳对象）
	if views[1].Endpoint != nil {
		t.Errorf("row 1 endpoint should be nil, got %+v", views[1].Endpoint)
	}
	if views[1].User != nil {
		t.Errorf("row 1 user should be nil for userID=0, got %+v", views[1].User)
	}
	// userID=0 不得进入用户批查
	if lo.Contains(uRepo.gotIDs, uint(0)) {
		t.Errorf("userID 0 must be filtered out, got %v", uRepo.gotIDs)
	}
}

func TestListModel_DemoMasksUpstreamModel(t *testing.T) {
	t.Parallel()
	text := []enum.InputModality{enum.InputModalityText}
	mRepo := &fakeModelRepo{models: []*llmagg.Model{mustModel(t, 11, 1, 101, "alias-a", "secret-upstream-name", text)}}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{1: mustEndpoint(t, 1, "ep-one")}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	views, _, err := h.Handle(context.Background(), port.ListModelQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}},
		IsDemo:      true,
		ScopeUserID: lo.ToPtr(uint(101)),
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if views[0].UpstreamModel == "secret-upstream-name" {
		t.Error("demo must mask upstreamModel")
	}
}

func TestListModel_UsernameResolvesToScope(t *testing.T) {
	t.Parallel()
	target := mustUser(202, "someone")
	mRepo := &fakeModelRepo{}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{}, byName: map[string]*useragg.User{"someone": target}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	if _, _, err := h.Handle(context.Background(), port.ListModelQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}},
		ScopeUserID: nil, // admin
		Username:    "someone",
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if mRepo.gotScope == nil || *mRepo.gotScope != 202 {
		t.Errorf("username should resolve to scope 202, got %v", mRepo.gotScope)
	}

	// 用户不存在 → 空结果，且不得退化为全量（scope 不能是 nil）
	h2 := query.NewListModelHandler(mRepo, epRepo, uRepo)
	views, page, err := h2.Handle(context.Background(), port.ListModelQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}},
		Username:    "ghost",
	})
	if err != nil {
		t.Fatalf("handle ghost: %v", err)
	}
	if len(views) != 0 || page.Total != 0 {
		t.Errorf("unknown username must yield empty result, got %d rows total=%d", len(views), page.Total)
	}
}

func TestListModel_FilterPassthrough(t *testing.T) {
	t.Parallel()
	mRepo := &fakeModelRepo{}
	epRepo := &fakeEndpointRepo{endpoints: map[uint]*llmagg.Endpoint{}}
	uRepo := &fakeUserRepo{users: map[uint]*useragg.User{}}

	h := query.NewListModelHandler(mRepo, epRepo, uRepo)
	if _, _, err := h.Handle(context.Background(), port.ListModelQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}},
		ScopeUserID: lo.ToPtr(uint(101)),
		Status:      "disabled",
		EndpointID:  7,
		Capability:  enum.InputModalityImage,
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}
	want := llmproxy.ModelListFilter{Status: "disabled", EndpointID: 7, Capability: enum.InputModalityImage}
	if mRepo.gotFilter != want {
		t.Errorf("filter passthrough: want %+v, got %+v", want, mRepo.gotFilter)
	}
}
```

注：`llmagg.CreateModel` / `llmagg.CreateEndpoint` 的实际签名以 `internal/domain/llmproxy/aggregate` 为准；若参数不同（如需 vo 包装或有 Restore 变体），按该包实际签名调整，测试语义不变。`useragg.RestoreUser` 的 9 参数顺序已在 `mustUser` 注释中固定，可直接照用。

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./test/unit/model_list_query/ -v
```

Expected: 编译失败 —— `query.NewListModelHandler undefined`。

- [ ] **Step 4: 实现 handler**

创建 `internal/application/model/query/list_model.go`：

```go
package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityaggregate "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	llmagg "github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listModelHandler struct {
	modelRepo    llmproxy.ModelRepository
	endpointRepo llmproxy.EndpointRepository
	userRepo     identity.UserRepository
}

// NewListModelHandler 构造平铺模型列表查询处理器
func NewListModelHandler(modelRepo llmproxy.ModelRepository, endpointRepo llmproxy.EndpointRepository, userRepo identity.UserRepository) port.ListModelHandler {
	return &listModelHandler{modelRepo: modelRepo, endpointRepo: endpointRepo, userRepo: userRepo}
}

// Handle 执行平铺模型列表查询：SQL 侧分页/筛选/排序，端点与用户批量回填。
func (h *listModelHandler) Handle(ctx context.Context, q port.ListModelQuery) ([]*port.ListModelView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	scope := q.ScopeUserID
	if scope == nil && q.Username != "" {
		u, err := h.userRepo.FindByName(ctx, q.Username)
		if err != nil {
			log.Error("[ModelQuery] Find user by name failed", zap.Error(err))
			return nil, nil, err
		}
		if u == nil {
			// 用户不存在 → 空结果而非错误；绝不可让 scope 保持 nil 退化为全量
			return []*port.ListModelView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
		}
		scope = lo.ToPtr(u.AggregateID())
	}

	filter := llmproxy.ModelListFilter{Status: q.Status, EndpointID: q.EndpointID, Capability: q.Capability}
	models, pageInfo, err := h.modelRepo.PaginateWithFilter(ctx, q.CommonParam, filter, scope)
	if err != nil {
		log.Error("[ModelQuery] Paginate models failed", zap.Error(err))
		return nil, nil, err
	}

	epsByID := h.loadEndpoints(ctx, models)
	usersByID := h.loadUsers(ctx, models)

	views := lo.Map(models, func(m *llmagg.Model, _ int) *port.ListModelView {
		return toListModelView(m, epsByID, usersByID, q.IsDemo)
	})
	log.Info("[ModelQuery] List models", zap.Int("rows", len(views)), zap.Int64("total", pageInfo.Total))
	return views, pageInfo, nil
}

// loadEndpoints 批量拉取本页模型所属端点，避免 N+1
func (h *listModelHandler) loadEndpoints(ctx context.Context, models []*llmagg.Model) map[uint]*llmagg.Endpoint {
	ids := lo.Uniq(lo.Filter(
		lo.Map(models, func(m *llmagg.Model, _ int) uint { return m.EndpointID() }),
		func(id uint, _ int) bool { return id != 0 },
	))
	out, err := h.endpointRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[ModelQuery] Load endpoints failed", zap.Error(err))
		return map[uint]*llmagg.Endpoint{}
	}
	return out
}

// loadUsers 批量拉取归属用户；userID==0（共享池/遗留）不查，避免拼出错误归属
func (h *listModelHandler) loadUsers(ctx context.Context, models []*llmagg.Model) map[uint]*identityaggregate.User {
	ids := lo.Uniq(lo.Filter(
		lo.Map(models, func(m *llmagg.Model, _ int) uint { return m.UserID() }),
		func(id uint, _ int) bool { return id != 0 },
	))
	out, err := h.userRepo.BatchFindByIDs(ctx, ids)
	if err != nil {
		logger.WithCtx(ctx).Warn("[ModelQuery] Load users failed", zap.Error(err))
		return map[uint]*identityaggregate.User{}
	}
	return out
}

func toListModelView(m *llmagg.Model, epsByID map[uint]*llmagg.Endpoint, usersByID map[uint]*identityaggregate.User, isDemo bool) *port.ListModelView {
	upstreamModel := m.UpstreamModel()
	if isDemo {
		upstreamModel = commonutil.MaskSecret(upstreamModel)
	}
	v := &port.ListModelView{
		ID:              m.AggregateID(),
		Alias:           m.Alias().String(),
		ModelID:         m.ModelID(),
		UpstreamModel:   upstreamModel,
		Enabled:         m.Enabled(),
		ContextLength:   m.ContextLength(),
		MaxOutputTokens: m.MaxOutputTokens(),
		Capabilities:    m.Capabilities(),
		CreatedAt:       m.CreatedAt(),
		UpdatedAt:       m.UpdatedAt(),
	}
	if ep := epsByID[m.EndpointID()]; ep != nil {
		v.Endpoint = &port.ListModelEndpointView{ID: ep.AggregateID(), Name: ep.Name()}
	}
	if u := usersByID[m.UserID()]; u != nil {
		v.User = &port.ListModelUserView{ID: u.AggregateID(), Name: string(u.Name()), Avatar: string(u.Avatar())}
	}
	return v
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./test/unit/model_list_query/ -v
```

Expected: 4 个测试全 PASS。

- [ ] **Step 6: 验证测试能捕获缺陷**

临时把 Step 4 中「用户不存在」分支的 `return` 删掉（让流程带着 `scope=nil` 继续），重跑：

```bash
go test ./test/unit/model_list_query/ -run TestListModel_UsernameResolvesToScope -v
```

Expected: FAIL（scope 为 nil 意味着 admin 全量，越权）。确认后改回。

- [ ] **Step 7: 提交**

```bash
git add internal/application/model/port/list_model.go \
        internal/application/model/query/list_model.go \
        test/unit/model_list_query/
git commit -m "feat(model): add list model query handler with nested endpoint/user backfill"
```

---

## Task 3: DTO + handler + 路由 + DI 装配

**Files:**
- Modify: `internal/dto/model.go`
- Modify: `internal/handler/model.go`
- Modify: `internal/router/model.go`
- Modify: `internal/bootstrap/modules/application.go`
- Test: `test/e2e/model_list/model_list_test.go`

**Interfaces:**
- Consumes: `port.ListModelHandler`、`port.ListModelQuery`、`port.ListModelView`（Task 2）
- Produces:
  - `dto.ListModelsReq{ model.CommonParam; Username, Status, Capability string; EndpointID uint }`
  - `dto.ListModelsRsp{ CommonRsp; Items []*dto.ModelListItem; PageInfo *model.PageInfo }`
  - `dto.ModelListItem` / `dto.ModelListEndpointItem` / 复用 `dto.UpstreamUserItem`
  - `handler.ModelHandler.HandleListModels(ctx, *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error)`
  - 路由 `GET /api/web/v1/model/list`

- [ ] **Step 1: 写 DTO**

在 `internal/dto/model.go` 末尾追加（`UpstreamUserItem` 已在 `dto/upstream.go` 定义，同包直接复用，勿重复定义）：

```go
// ListModelsReq 平铺模型列表请求
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelsReq struct {
	model.CommonParam
	Username   string `query:"username,omitempty" doc:"按归属用户名过滤(仅管理员生效)"`
	Status     string `query:"status,omitempty" enum:"enabled,disabled" doc:"启用状态筛选(缺省为全部)"`
	EndpointID uint   `query:"endpointID,omitempty" doc:"按所属端点过滤(0=不过滤)"`
	Capability string `query:"capability,omitempty" enum:"text,image" doc:"按输入模态过滤(缺省为全部)"`
}

// ListModelsRsp 平铺模型列表响应
//
// 分页对象是 model 行：pageInfo.total 为当前筛选范围内模型总数。
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ListModelsRsp struct {
	CommonRsp
	Items    []*ModelListItem `json:"items,omitempty" doc:"模型列表"`
	PageInfo *model.PageInfo  `json:"pageInfo,omitempty" doc:"分页信息(total=模型数)"`
}

// ModelListEndpointItem 平铺列表中的所属端点（仅展示所需最小字段）
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ModelListEndpointItem struct {
	ID   uint   `json:"id" doc:"Endpoint ID"`
	Name string `json:"name" doc:"Endpoint 名称"`
}

// ModelListItem 平铺模型列表行
//
//	@author centonhuang
//	@update 2026-08-28 10:00:00
type ModelListItem struct {
	ID              uint                   `json:"id" doc:"Model ID"`
	User            *UpstreamUserItem      `json:"user,omitempty" doc:"归属用户信息（用户缺失或已删除时缺省）"`
	Endpoint        *ModelListEndpointItem `json:"endpoint,omitempty" doc:"所属端点（端点缺失时缺省）"`
	Alias           string                 `json:"alias" doc:"模型别名"`
	ModelID         string                 `json:"modelId" doc:"业务模型ID"`
	UpstreamModel   string                 `json:"upstreamModel" doc:"上游实际模型名"`
	Enabled         bool                   `json:"enabled" doc:"是否启用"`
	ContextLength   int                    `json:"contextLength" doc:"上下文窗口长度（tokens）"`
	MaxOutputTokens int                    `json:"maxOutputTokens" doc:"最大输出长度（tokens）"`
	Capabilities    []enum.InputModality   `json:"capabilities" doc:"模型能力（输入模态集合）"`
	CreatedAt       time.Time              `json:"createdAt" doc:"创建时间"`
	UpdatedAt       time.Time              `json:"updatedAt" doc:"更新时间"`
}
```

确认 `internal/dto/model.go` 已 import `time`、`enum`、`model`，缺则补。

- [ ] **Step 2: 写 handler**

`internal/handler/model.go` 改动三处。

接口加方法（`ModelHandler` 内）：

```go
	HandleListModels(ctx context.Context, req *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error)
```

依赖结构与构造：

```go
type ModelDependencies struct {
	Create port.CreateModelHandler
	Update port.UpdateModelHandler
	Delete port.DeleteModelHandler
	List   port.ListModelHandler
}

type modelHandler struct {
	create port.CreateModelHandler
	update port.UpdateModelHandler
	delete port.DeleteModelHandler
	list   port.ListModelHandler
}

func NewModelHandler(deps ModelDependencies) ModelHandler {
	return &modelHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
		list:   deps.List,
	}
}
```

文件末尾追加方法（`scopeFor` 已在同包 `endpoint.go:118`，直接调用）：

```go
// HandleListModels 平铺模型列表查询（Web 管理端）
//
// scope 复用 scopeFor：admin → nil（全量），非 admin → 自身，userID==0 → 401。
func (h *modelHandler) HandleListModels(ctx context.Context, req *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error) {
	rsp := &dto.ListModelsRsp{}
	perm := util.CtxValuePermission(ctx)
	scope, err := scopeFor(ctx, perm)
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	views, pageInfo, err := h.list.Handle(ctx, port.ListModelQuery{
		CommonParam: req.CommonParam,
		IsDemo:      perm == enum.PermissionDemo,
		ScopeUserID: scope,
		Username:    req.Username,
		Status:      req.Status,
		EndpointID:  req.EndpointID,
		Capability:  req.Capability,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] List models failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Items = lo.Map(views, func(v *port.ListModelView, _ int) *dto.ModelListItem {
		return toModelListItem(v)
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func toModelListItem(v *port.ListModelView) *dto.ModelListItem {
	item := &dto.ModelListItem{
		ID:              v.ID,
		Alias:           v.Alias,
		ModelID:         v.ModelID,
		UpstreamModel:   v.UpstreamModel,
		Enabled:         v.Enabled,
		ContextLength:   v.ContextLength,
		MaxOutputTokens: v.MaxOutputTokens,
		Capabilities:    v.Capabilities,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
	if v.User != nil {
		item.User = &dto.UpstreamUserItem{ID: v.User.ID, Name: v.User.Name, Avatar: v.User.Avatar}
	}
	if v.Endpoint != nil {
		item.Endpoint = &dto.ModelListEndpointItem{ID: v.Endpoint.ID, Name: v.Endpoint.Name}
	}
	return item
}
```

补 import：`"github.com/samber/lo"`、`"github.com/hcd233/aris-proxy-api/internal/common/enum"`。

- [ ] **Step 3: 注册路由**

`internal/router/model.go` 的 `initModelRouter` 内，在 `createModel` 注册之前插入：

```go
	huma.Register(modelGroup, huma.Operation{
		OperationID: "listModels",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListModels",
		Description: "List models in a flat, paginated view",
		Tags:        []string{constant.TagModel},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listModels", enum.PermissionUser),
		},
	}, modelHandler.HandleListModels)
```

- [ ] **Step 4: DI 装配**

在 `internal/bootstrap/modules/application.go` 中，找到已有的 model command handler 装配（`NewCreateModelHandler` 等）附近，加入 `modelquery.NewListModelHandler` 的 Provide，并把它接到 `handler.ModelDependencies.List`。参照同文件内 `NewListUpstreamHandler` 的写法（同样需要 endpointRepo + modelRepo + userRepo 三个依赖）。若该文件用 wrapper 函数把仓储转小接口，沿用同一风格。

- [ ] **Step 5: 编译 + 全量测试 + lint**

```bash
go build ./... && go test ./test/unit/... && go run ./cmd/server lint conv ./... && go run ./cmd/server lint static ./...
```

Expected: 全部通过。若 conv lint 报 `style.local_const`，把常量移到 `internal/common/constant`。

- [ ] **Step 6: 写 e2e 测试**

创建 `test/e2e/model_list/model_list_test.go`。**必须走生产入口 `router.RegisterAPIRouter`**，不得自己拼 group 前缀（#165 教训：自拼路径的测试注入缺陷后仍 PASS）。

参照 `test/e2e/upstream_list/` 的既有骨架（同样是 JWT + 分页列表），断言：

1. `GET /api/web/v1/model/list?page=1&pageSize=20` 带合法 JWT → 200，`items` 非空，`pageInfo.total` 等于该用户可见模型数
2. 无凭据 → 401（区分 404 路由缺失与 401 鉴权失败：**必须断言 401 而非 404**，若得 404 说明路由未注册）
3. 普通用户 A 的响应中不含用户 B 的模型（租户隔离）
4. `status=disabled` 仅返回停用模型
5. `capability=image` 仅返回含 image 的模型
6. `sortField=api_key` → 200（回退默认排序，不 500）
7. 每行 `endpoint.name` 非空（当端点存在时），且 `endpoint` 不含 baseURL/apiKey 字段（最小暴露面）

- [ ] **Step 7: 运行 e2e**

```bash
go test ./test/e2e/model_list/ -v
```

Expected: 全 PASS。

- [ ] **Step 8: 提交**

```bash
git add internal/dto/model.go internal/handler/model.go internal/router/model.go \
        internal/bootstrap/modules/application.go test/e2e/model_list/
git commit -m "feat(api): add GET /api/web/v1/model/list flat model list endpoint"
```

---

## Task 4: 前端类型与 API 客户端

**Files:**
- Modify: `web/src/lib/types.ts:374-508`
- Modify: `web/src/lib/api-client.ts`

**Interfaces:**
- Consumes: Task 3 的 `dto.ListModelsRsp` JSON 形状
- Produces:
  - `ModelListEndpoint{ id: number; name: string }`
  - `ModelListItem{ ... endpoint?: ModelListEndpoint; user?: UpstreamUser }`
  - `ListModelsPageRsp{ items?: ModelListItem[]; pageInfo?: PageInfo }`
  - `ModelListSortField = "alias" | "context_length" | "max_output_tokens" | "created_at" | "endpoint_id" | "enabled"`
  - `api.listModelsPage(params: ListModelsPageParams): Promise<ListModelsPageRsp>`

- [ ] **Step 1: 删死类型、加新类型**

在 `web/src/lib/types.ts` 中**删除** `EndpointItem`、`ListEndpointsRsp`、`ModelItem`、`ListModelsRsp` 四个接口（经全量检索，除注释外零引用；其扁平 `username` 口径与本页嵌套 `user` 惯例冲突，不得复用）。

在 `UpstreamGroupItem` 之后追加：

```ts
// ─── Model flat list（平铺视图，GET /api/web/v1/model/list） ──────────────────

/** 平铺列表中的所属端点（仅展示所需最小字段，不含 baseURL / apiKey） */
export interface ModelListEndpoint {
  id: number;
  name: string;
}

export interface ModelListItem {
  id: number;
  user?: UpstreamUser;
  endpoint?: ModelListEndpoint;
  alias: string;
  modelId: string;
  upstreamModel: string;
  enabled: boolean;
  contextLength: number;
  maxOutputTokens: number;
  capabilities: ModelCapability[];
  createdAt: string;
  updatedAt: string;
}

export interface ListModelsPageRsp extends CommonRsp {
  items?: ModelListItem[];
  pageInfo?: PageInfo;
}

/** 后端排序白名单（constant.ModelListSortFields），前端不得传白名单外的值 */
export type ModelListSortField =
  | "alias"
  | "context_length"
  | "max_output_tokens"
  | "created_at"
  | "endpoint_id"
  | "enabled";
```

注意 `ModelCapability` 定义在被删除的 `ModelItem` 附近，**必须保留**该 type alias。

- [ ] **Step 2: 加 API 方法**

在 `web/src/lib/api-client.ts` 的 `listUpstream` 之后追加：

```ts
  async listModelsPage(params: {
    page: number;
    pageSize: number;
    query?: string;
    sortField?: ModelListSortField;
    sort?: "asc" | "desc";
    status?: "enabled" | "disabled";
    endpointID?: number;
    capability?: ModelCapability;
    username?: string;
  }): Promise<ListModelsPageRsp> {
    const sp = new URLSearchParams({
      page: String(params.page),
      pageSize: String(params.pageSize),
    });
    if (params.query) sp.set("query", params.query);
    if (params.sortField) sp.set("sortField", params.sortField);
    if (params.sort) sp.set("sort", params.sort);
    if (params.status) sp.set("status", params.status);
    if (params.endpointID) sp.set("endpointID", String(params.endpointID));
    if (params.capability) sp.set("capability", params.capability);
    if (params.username) sp.set("username", params.username);
    return this.request<ListModelsPageRsp>(`/api/web/v1/model/list?${sp}`);
  }
```

在文件顶部的 type import 块中加入 `ListModelsPageRsp`、`ModelListSortField`、`ModelCapability`，并移除已删类型的 import（`ListEndpointsRsp` / `ListModelsRsp` 等，若存在）。

- [ ] **Step 3: 类型检查**

```bash
cd web && npx tsc --noEmit
```

Expected: 无错误。若报某处仍引用已删类型，说明该处是活引用——改为新类型或恢复该类型（勿盲目删）。

- [ ] **Step 4: 提交**

```bash
git add web/src/lib/types.ts web/src/lib/api-client.ts
git commit -m "feat(web): add model flat list types and api client, drop dead legacy types"
```

---

## Task 5: 抽出共享展示组件与弹窗（纯搬迁，零行为变更）

**Files:**
- Create: `web/src/app/(dashboard)/upstream/shared.tsx`
- Create: `web/src/app/(dashboard)/upstream/endpoint-dialog.tsx`
- Create: `web/src/app/(dashboard)/upstream/model-dialog.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`

**Interfaces:**
- Produces:
  - `formatTokens(n: number): string`
  - `OwnerCell({ user }: { user?: UpstreamUser })`
  - `CapabilityBadges({ capabilities }: { capabilities?: ModelCapability[] })`
  - `SpecBadges({ contextLength, maxOutputTokens }: { contextLength: number; maxOutputTokens: number })`
  - `EndpointDialog(props: EndpointDialogProps)`
  - `ModelDialog(props: ModelDialogProps)`
  - `EndpointForm` / `ModelForm` interfaces、`emptyEndpointForm` / `emptyModelForm`

- [ ] **Step 1: 建 shared.tsx**

把 `page.tsx` 现有的 `formatTokens`（138-149 行）、`OwnerCell`（152-176）、`CapabilityBadges`（179-201）原样移入新文件并 `export`。`OwnerCell` 的 props 类型从 `UpstreamEndpointItem["user"]` 改为直接用 `UpstreamUser`（语义相同，解耦）。

新增 `SpecBadges` —— 把 `renderModelRow` 里 680-707 行的「上下文 + 最大输出」两个徽标提取为组件（当前桌面/移动重复实现，移动端只显示 contextLength）：

```tsx
/** 规格徽标：上下文窗口 + 最大输出，两者共用同一视觉规格 */
export function SpecBadges({
  contextLength,
  maxOutputTokens,
}: {
  contextLength: number;
  maxOutputTokens: number;
}) {
  const t = useT();
  return (
    <div className="flex items-center gap-1.5">
      <TooltipRoot>
        <TooltipTrigger
          render={
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
              <ArrowLeftRight className="size-3 text-muted-foreground" />
              {formatTokens(contextLength)}
            </span>
          }
        />
        <TooltipContent side="top" align="start">
          {`${t("models.context_length")}: ${contextLength.toLocaleString()}`}
        </TooltipContent>
      </TooltipRoot>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
              <ArrowUpFromLine className="size-3 text-muted-foreground" />
              {formatTokens(maxOutputTokens)}
            </span>
          }
        />
        <TooltipContent side="top" align="start">
          {`${t("models.max_output")}: ${maxOutputTokens.toLocaleString()}`}
        </TooltipContent>
      </TooltipRoot>
    </div>
  );
}
```

同时把 `EndpointForm` / `ModelForm` 接口与 `emptyEndpointForm` / `emptyModelForm` 常量移入 shared.tsx 并导出。

- [ ] **Step 2: 建 endpoint-dialog.tsx**

把 `page.tsx` 1033-1178 行的端点 Dialog 整块移入，包成组件：

```tsx
export interface EndpointDialogProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  editingId: number | null;
  form: EndpointForm;
  setForm: React.Dispatch<React.SetStateAction<EndpointForm>>;
  userOptions: { id: number; name: string }[];
  isAdmin: boolean;
  saving: boolean;
  onSave: () => void;
}
```

JSX 完全照搬，仅把 `endpointForm` → `form`、`setEndpointForm` → `setForm`、`editingEndpointId` → `editingId`、`isAdmin()` → `isAdmin`、`setEndpointDialogOpen(false)` → `onOpenChange(false)`、`handleSaveEndpoint` → `onSave`。

- [ ] **Step 3: 建 model-dialog.tsx**

把 `TokenPresetPopover`（212-256 行）与模型 Dialog（1181-1331 行）整块移入。`CONTEXT_LENGTH_PRESETS` / `MAX_OUTPUT_PRESETS` 两个常量一并移入。

```tsx
export interface ModelDialogProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  editing: boolean;
  form: ModelForm;
  setForm: React.Dispatch<React.SetStateAction<ModelForm>>;
  onModelIdTouched: () => void;
  modelIdTouched: boolean;
  saving: boolean;
  onSave: () => void;
}
```

- [ ] **Step 4: page.tsx 改为消费这些组件**

删除已移出的定义，改为 `import`；渲染处换成 `<EndpointDialog ... />` 与 `<ModelDialog ... />`。此步**不改任何行为**。

- [ ] **Step 5: 验证零行为变更**

```bash
cd web && npm run lint && npx tsc --noEmit && npm run build
```

Expected: 全绿。人工确认 `page.tsx` 行数明显下降（约 -450 行）。

- [ ] **Step 6: 提交**

```bash
git add "web/src/app/(dashboard)/upstream/"
git commit -m "refactor(web): extract upstream shared cells and dialogs into modules"
```

---

## Task 6: 分组视图重做（树表 + 真表头 + 连通详情 + 折叠）

**Files:**
- Create: `web/src/app/(dashboard)/upstream/grouped-view.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`
- Modify: `web/src/locales/{en,ja,zh}.json`

**Interfaces:**
- Consumes: `shared.tsx` 的 `OwnerCell` / `CapabilityBadges` / `SpecBadges` / `formatTokens`（Task 5）
- Produces:
  - `GroupedView(props: GroupedViewProps)`，其中
    `GroupedViewProps = { groups: UpstreamGroupItem[]; isMobile: boolean; isDemo: boolean; togglePending: boolean; onToggleEnabled: (m: UpstreamModelItem) => void; onEditEndpoint: (ep: UpstreamEndpointItem) => void; onDeleteEndpoint: (ep: UpstreamEndpointItem) => void; onAddModel: (ep: UpstreamEndpointItem) => void; onEditModel: (m: UpstreamModelItem, ep: UpstreamEndpointItem) => void; onDeleteModel: (m: UpstreamModelItem) => void; onCopyAlias: (alias: string) => void; deletingEndpointID?: number; deletingModelID?: number }`
  - `EndpointDetailPopover({ endpoint }: { endpoint: UpstreamEndpointItem })`

- [ ] **Step 1: 加 i18n 文案**

三个 locale 文件各加（以 zh 为例，en/ja 同键位对应翻译）：

```json
"upstream.view_grouped": "分组",
"upstream.view_flat": "平铺",
"upstream.col_model": "模型 / 端点",
"upstream.col_upstream": "上游真名",
"upstream.col_spec": "规格",
"upstream.col_status": "状态",
"upstream.col_created": "创建",
"upstream.col_actions": "操作",
"upstream.col_endpoint": "端点",
"upstream.connection_details": "连通详情",
"upstream.connection_desc": "该端点的上游地址与凭据（凭据已掩码）",
"upstream.created_at": "创建于",
"upstream.truncated_detail": "{total} 中显示 {shown}",
"upstream.collapse_group": "折叠该端点",
"upstream.expand_group": "展开该端点",
"upstream.disabled_hint": "已停用",
"upstream.filter_status": "状态",
"upstream.filter_endpoint": "端点",
"upstream.filter_capability": "能力",
"upstream.status_enabled": "启用",
"upstream.status_disabled": "停用",
"upstream.sort_asc": "升序",
"upstream.sort_desc": "降序",
"upstream.no_models_in_group": "该端点下暂无模型"
```

- [ ] **Step 2: 写 grouped-view.tsx**

结构要点（逐条对应 spec 缺陷修复）：

1. **真表头**：7 个 `TableHead`（模型/端点、上游真名、规格、状态、创建、操作），删掉原 `colSpan={9}` 空表头与 `<TableCell>{null}</TableCell>`；`colSpan` 统一改为 7。
2. **组头行**：`className="bg-card hover:bg-card"`，首个 `TableCell` 内用 `border-l-[3px] border-l-primary pl-3` 造左侧色条；内容 = 折叠按钮（`ChevronDown`/`ChevronRight`）+ `OwnerCell` + 端点名（带 Tooltip）+ 协议 badges + `EndpointDetailPopover` + 截断徽标 + 计数 + 操作区。
3. **`EndpointDetailPopover`**：`Popover` + `Info` 图标触发，内容为 2×2 网格：OpenAI URL / Anthropic URL / `maskedAPIKey` / 创建时间；每项可点击复制（复用 `copyTextToClipboard`）。URL 用 `break-all` 完整换行，**不截断**。
4. **模型行**：首格用 `pl-8` + `border-l border-dashed border-border` 造树枝缩进；`alias` 加粗为主体，`modelId` 降级为同格 `· id:` 后缀（`text-[10px] text-muted-foreground`）。
5. **停用降权**：模型行 `className={cn(!m.enabled && "opacity-45")}`，alias 加 `line-through`。
6. **截断徽标**：`t("upstream.truncated_detail").replace("{total}", String(group.totalModelCount)).replace("{shown}", String(group.modelCount))`；仅当 `group.truncated` 为真时渲染。
7. **折叠**：`usePersistentState<number[]>("dashboard.upstream.collapsed", [])` 存折叠的 endpointID；折叠时不渲染模型行，组头计数与截断徽标仍显示。
8. **空组**：`group.models.length === 0` 时渲染一行 `t("upstream.no_models_in_group")`。
9. **移动端**：卡片布局照搬现有结构，但改用 `SpecBadges`（补上原本缺失的 maxOutputTokens）并加停用降权。

所有截断文案（端点名、alias、upstreamModel、modelId）**必须**包 `TooltipRoot`/`TooltipTrigger`(render prop)/`TooltipContent className="max-w-xs break-all"`，否则 `truncate-requires-tooltip` lint 报错。`TableCell` 上不得直接加 `truncate`，须移到内部 `span`。

- [ ] **Step 3: page.tsx 接入**

删除 `renderGroupHead` / `renderModelRow` / `TableRowGroup` / 移动端内联 JSX，改为：

```tsx
<GroupedView
  groups={groups}
  isMobile={isMobile}
  isDemo={isDemo()}
  togglePending={toggleEnabled.updatingKey !== null}
  onToggleEnabled={(m) => toggleEnabled.apply(m, { enabled: !m.enabled })}
  onEditEndpoint={openEditEndpoint}
  onDeleteEndpoint={(ep) => deleteEndpointConfirm.openDelete(ep)}
  onAddModel={openCreateModel}
  onEditModel={openEditModel}
  onDeleteModel={(m) => deleteModelConfirm.openDelete({ model: m })}
  onCopyAlias={handleCopyAlias}
  deletingEndpointID={deleteEndpointConfirm.loading ? deleteEndpointConfirm.target?.id : undefined}
  deletingModelID={deleteModelConfirm.loading ? deleteModelConfirm.target?.model.id : undefined}
/>
```

`findModelOwner` / `openEditModelFromRow` 可删除 —— `GroupedView` 渲染时天然知道所属 group，直接传 `(m, group.endpoint)`。

- [ ] **Step 4: 验证**

```bash
cd web && npm run lint && npx tsc --noEmit && npm run build
```

Expected: 全绿，尤其无 `truncate-requires-tooltip` 违规。

- [ ] **Step 5: 提交**

```bash
git add "web/src/app/(dashboard)/upstream/" web/src/locales/
git commit -m "feat(web): redesign upstream grouped view as indented tree table"
```

---

## Task 7: 平铺视图 + 视图切换

**Files:**
- Create: `web/src/components/view-switch.tsx`
- Create: `web/src/app/(dashboard)/upstream/use-model-list.ts`
- Create: `web/src/app/(dashboard)/upstream/flat-view.tsx`
- Modify: `web/src/app/(dashboard)/upstream/page.tsx`
- Test: `web/src/app/(dashboard)/upstream/__tests__/use-model-list.test.ts`

**Interfaces:**
- Consumes: `api.listModelsPage`、`ModelListItem`、`ModelListSortField`（Task 4）；`shared.tsx` 组件（Task 5）
- Produces:
  - `ViewSwitch({ value, onChange, options }: { value: string; onChange: (v: string) => void; options: { value: string; label: string }[] })`
  - `buildModelListParams(input): Parameters<typeof api.listModelsPage>[0]`（纯函数，可测）
  - `useModelList(opts)` → `{ items, pageInfo, loading, sortField, sort, toggleSort, refresh }`
  - `FlatView(props: FlatViewProps)`

- [ ] **Step 1: 写纯函数的失败测试**

创建 `web/src/app/(dashboard)/upstream/__tests__/use-model-list.test.ts`：

```ts
import { describe, expect, it } from "vitest";
import { buildModelListParams, nextSortState } from "../use-model-list";

describe("buildModelListParams", () => {
  it("omits empty optional params", () => {
    const p = buildModelListParams({
      page: 2,
      pageSize: 20,
      freeText: "",
      params: {},
      sortField: "created_at",
      sort: "desc",
    });
    expect(p).toEqual({ page: 2, pageSize: 20, sortField: "created_at", sort: "desc" });
  });

  it("maps facet params to api params", () => {
    const p = buildModelListParams({
      page: 1,
      pageSize: 10,
      freeText: "gpt",
      params: { status: "disabled", capability: "image", username: "alice" },
      sortField: "alias",
      sort: "asc",
    });
    expect(p.query).toBe("gpt");
    expect(p.status).toBe("disabled");
    expect(p.capability).toBe("image");
    expect(p.username).toBe("alice");
  });

  it("drops status/capability values outside the backend whitelist", () => {
    const p = buildModelListParams({
      page: 1,
      pageSize: 10,
      freeText: "",
      params: { status: "bogus", capability: "audio" },
      sortField: "alias",
      sort: "asc",
    });
    expect(p.status).toBeUndefined();
    expect(p.capability).toBeUndefined();
  });
});

describe("nextSortState", () => {
  it("toggles direction when clicking the active column", () => {
    expect(nextSortState({ sortField: "alias", sort: "asc" }, "alias")).toEqual({
      sortField: "alias",
      sort: "desc",
    });
  });

  it("switches column and resets to asc", () => {
    expect(nextSortState({ sortField: "alias", sort: "desc" }, "created_at")).toEqual({
      sortField: "created_at",
      sort: "asc",
    });
  });
});
```

- [ ] **Step 2: 运行确认失败**

```bash
cd web && npm test
```

Expected: FAIL —— 找不到 `../use-model-list`。

- [ ] **Step 3: 实现 use-model-list.ts**

导出两个纯函数（供测试）与 hook：

```ts
const STATUS_VALUES = ["enabled", "disabled"] as const;
const CAPABILITY_VALUES = ["text", "image"] as const;

/** 组装 api.listModelsPage 参数；白名单外的 status/capability 丢弃（后端也会忽略，前端先净化） */
export function buildModelListParams(input: {
  page: number;
  pageSize: number;
  freeText: string;
  params: Record<string, string>;
  sortField: ModelListSortField;
  sort: "asc" | "desc";
}) { /* 按测试断言实现 */ }

/** 点击列头的排序状态迁移：同列反向，换列重置为 asc */
export function nextSortState(
  cur: { sortField: ModelListSortField; sort: "asc" | "desc" },
  clicked: ModelListSortField,
) {
  return cur.sortField === clicked
    ? { sortField: clicked, sort: cur.sort === "asc" ? ("desc" as const) : ("asc" as const) }
    : { sortField: clicked, sort: "asc" as const };
}
```

hook 内用 `usePersistentState` 持久化 `dashboard.upstream.flat.page` / `.pageSize` / `.sortField` / `.sort`，`useEffect` 依赖 `[queryParams, sortField, sort]` 触发拉取，错误走 `showErrorToast`。

- [ ] **Step 4: 运行确认通过**

```bash
cd web && npm test
```

Expected: 5 个断言全 PASS。

- [ ] **Step 5: 写 view-switch.tsx**

```tsx
"use client";

import { cn } from "@/lib/utils";

/** 分段控件：等宽选项，选中项深墨底白字（与主按钮语义一致） */
export function ViewSwitch({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <div
      role="tablist"
      className="inline-flex shrink-0 overflow-hidden rounded-lg border border-input bg-background"
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="tab"
          aria-selected={value === o.value}
          className={cn(
            "px-3 py-1.5 text-xs transition-colors",
            value === o.value
              ? "bg-foreground font-semibold text-background"
              : "text-muted-foreground hover:bg-accent hover:text-foreground",
          )}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
```

- [ ] **Step 6: 写 flat-view.tsx**

桌面表列：别名（含 `· id:` 后缀）、上游真名、端点（`OwnerCell` 风格的头像 + 名称）、规格（`SpecBadges`）、能力（`CapabilityBadges`）、状态（`Switch`）、创建、操作。

列头可排序的 6 列用 `<TableHead>` 内包 `<button>`：

```tsx
<button
  type="button"
  className="inline-flex items-center gap-1 uppercase tracking-[0.08em] hover:text-foreground"
  aria-label={`${label} ${sort === "asc" ? t("upstream.sort_asc") : t("upstream.sort_desc")}`}
  onClick={() => onSort(field)}
>
  {label}
  {sortField === field ? (
    sort === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />
  ) : (
    <ArrowUpDown className="size-3 opacity-40" />
  )}
</button>
```

停用行同样 `opacity-45` + alias `line-through`。移动端为紧凑卡片列表（每行显示 alias / 端点名 / 规格 / 开关）。

- [ ] **Step 7: page.tsx 接入双 filterBar 与切换**

关键：**两个 `useFilterBar` 实例**，切换时同步共有 token。

```tsx
const [view, setView] = usePersistentState<"grouped" | "flat">("dashboard.upstream.view", "grouped");

const groupedFilterBar = useFilterBar({
  persistKey: "dashboard.upstream.grouped",
  facets: groupedFacets,
  freeTextPlaceholder: t("upstream.search_placeholder"),
});
const flatFilterBar = useFilterBar({
  persistKey: "dashboard.upstream.flat",
  facets: flatFacets,
  freeTextPlaceholder: t("upstream.search_placeholder"),
});
```

`flatFacets` 含 4 项（均 `target: "param"`、`single: true`）：`status`（静态选项 启用/停用）、`capability`（静态 text/image）、`endpoint`（`paramName: "endpointID"`，选项异步取自当前分组数据的端点名→id 映射）、`username`（仅 admin）。

切换处理：

```tsx
// 共有维度（关键词 / username）跨视图同步，特有维度各自保留
const handleViewChange = (next: string) => {
  const from = view === "grouped" ? groupedFilterBar : flatFilterBar;
  const to = next === "grouped" ? groupedFilterBar : flatFilterBar;
  for (const key of [null, "username"] as const) {
    const token = from.tokens.find((t) => t.key === key);
    if (token) to.addToken(token);
  }
  setView(next as "grouped" | "flat");
};
```

工具条：

```tsx
<div className="mb-4 flex items-center gap-2">
  <ViewSwitch
    value={view}
    onChange={handleViewChange}
    options={[
      { value: "grouped", label: t("upstream.view_grouped") },
      { value: "flat", label: t("upstream.view_flat") },
    ]}
  />
  <FilterBar
    {...(view === "grouped" ? groupedFilterBar : flatFilterBar)}
    facets={view === "grouped" ? groupedFacets : flatFacets}
    placeholder={t("upstream.search_placeholder")}
  />
</div>
```

注意：现有代码给 `FilterBar` 传的是 `facets={[]}`（第 787 行），导致 facet 建议不可用；这里必须传真实 facets。

- [ ] **Step 8: 验证**

```bash
cd web && npm run lint && npm test && npx tsc --noEmit && npm run build
```

Expected: 全绿。

- [ ] **Step 9: 提交**

```bash
git add web/src/components/view-switch.tsx "web/src/app/(dashboard)/upstream/" web/src/locales/
git commit -m "feat(web): add flat model view with sortable columns and view switch"
```

---

## Task 8: 端到端联调与三主题核查

**Files:**
- Modify: 视验证结果修补前述文件

- [ ] **Step 1: 起后端**

```bash
go run ./cmd/server server
```

- [ ] **Step 2: 起前端**

```bash
cd web && npm run dev
```

- [ ] **Step 3: 功能核查清单**

- 分组视图：表头 7 列有名；组头色条与折叠可用；`ⓘ 连通详情` 显示双 URL/Key 掩码且可复制；停用模型整行降权；截断端点显示 `N 中显示 M`
- 平铺视图：6 列可排序且箭头正确；分页总数与后端一致；`status`/`capability`/`endpoint` 筛选生效
- 切换：关键词在两视图间保留；平铺的 status 不影响分组请求（**开 DevTools Network 确认分组请求的 query string 里没有 status/capability**）
- 分页独立：分组翻到第 2 页 → 切平铺 → 切回，分组仍在第 2 页
- CRUD：四类弹窗（建/改端点、建/改模型）与两类删除确认均正常

- [ ] **Step 4: 三主题核查**

切 light / dark / moonshot，确认组头左侧色条、`opacity-45` 停用行、虚线树枝在三主题下都可辨（对比度 ≥3:1）。moonshot 主题下组头 `bg-card` 会带 `backdrop-filter`，确认色条不被玻璃效果吞掉。

- [ ] **Step 5: 键盘与无障碍**

Tab 走查：`ViewSwitch` 可聚焦并有 `aria-selected`；排序按钮有 `aria-label`；折叠按钮有 `aria-expanded`。

- [ ] **Step 6: 全量回归**

```bash
go test ./... && go run ./cmd/server lint conv ./... && go run ./cmd/server lint static ./... \
  && cd web && npm run lint && npm test && npm run build
```

- [ ] **Step 7: 提交修补**

```bash
git add -A
git commit -m "fix(web): polish upstream list per cross-theme and a11y review"
```

---

## Self-Review

**1. Spec coverage**

| Spec 要求 | 覆盖任务 |
|---|---|
| 文件拆分（8 模块） | Task 5（shared/dialogs）、6（grouped）、7（flat/hook/switch） |
| 新接口 + 参数面 7 项 | Task 3（DTO/handler/router） |
| 排序白名单（不用 SafeSortField） | Task 1 Step 3/5/7 |
| scope 三态 + userID==0 → 401 | Task 3 Step 2（复用 `scopeFor`）、Task 1 Step 1（仓储隔离测试） |
| capability LIKE 实现 | Task 1 Step 5 |
| endpoint/user 嵌套批量回填 | Task 2 Step 4 |
| demo 脱敏 | Task 2 Step 4 + 测试 |
| 前端 3 新类型 + 4 死类型删除 | Task 4 |
| 缺陷 #1–#9 逐条 | Task 6 Step 2（#1–#8）、Task 5（#9） |
| segmented control 分组/平铺 | Task 7 Step 5/7 |
| 双 filterBar 状态语义 | Task 7 Step 7 |
| 边界情况 9 条 | Task 1（未知 capability/非法 sortField）、Task 2（端点缺失/用户缺失/username 不存在）、Task 3 e2e（401/跨租户）、Task 6（空组/折叠） |
| 测试计划（后端单测/e2e、前端 vitest/lint/三主题） | Task 1/2/3/7/8 |
| i18n 三语言 | Task 6 Step 1 |

无遗漏。

**2. Placeholder scan**

无 "TBD"/"TODO"/"类似 Task N"。两处刻意的实现自由度已给出判据：Task 3 Step 4（DI 装配"参照同文件 `NewListUpstreamHandler` 写法"——该文件风格可能用 wrapper，硬写会错）、Task 3 Step 6（e2e "参照 `test/e2e/upstream_list/` 骨架"并列出 7 条断言）。Task 7 Step 3 的 `buildModelListParams` 函数体标注"按测试断言实现"——其行为已由 Step 1 的三个用例完全固定。

**3. Type consistency**

- `PaginateWithFilter` 签名在 Task 1（定义）、Task 2（fake + 调用）三处一致
- `ModelListFilter` 三字段 `Status/EndpointID/Capability` 全程一致
- `port.ListModelView` 字段名与 Task 3 的 `toModelListItem` 映射一一对应
- 前端 `ModelListItem`（Task 4）→ `FlatView` 消费（Task 7）字段一致

---

## 实施偏差记录（Task 1–3 完成后回填）

计划里有三处在实施中被证伪，已按实际代码修正。**后续任务若引用被修正的条目，以本节为准。**

**偏差 1：`ListModelsReq` 不得嵌入 `model.CommonParam`**

计划原写 `type ListModelsReq struct { model.CommonParam; ... }`。实际不成立：
`model.SortParam.SortField` 只有 `json:"sortField"` 而无 `query` 标签，而 huma 只认
`path`/`query`/`header`/`form`/`cookie` 这类**来源**标签（huma.go:145-180，无匹配时
`return nil, false`），`json` 标签不被当作来源。项目未开启 `RejectUnknownQueryParameters`，
所以参数被**静默忽略**而非报 422——按原计划写，平铺视图的排序会静默失效。

修正：按 `dto/audit.go` / `session.go` / `cron.go` 的既有惯例，显式声明
`Page/PageSize/Query/Sort/SortField` 五个 `query` 字段。

连带发现：`dto/upstream.go` 的 `ListUpstreamReq` 也嵌了 `CommonParam`，其 `sortField`
同样绑不上。属既有缺陷，但分组视图当前无排序列故未暴露，本次未动。

**偏差 2：`scopeFor` 不满足"userID==0 → 401"**

计划写"复用 `internal/handler/endpoint.go:118` 的 `scopeFor`"。实际该函数签名是
`scopeFor(ctx, perm) uint`（**不返回 error**），admin 返回 0、其余返回 `CtxValueUint`。
它对"非 admin 但未拿到 userID"也返回 0，与 admin 的"全量"同义；直接把 0 映射成
`*uint` 的 nil 会让认证缺失退化为全平台可见——正是 spec 要防的事。

修正：新增 `scopePtrFor(ctx, perm) (*uint, error)`，admin → `nil`，非 admin 且
`userID==0` → 返回 `ierr.ErrUnauthorized`。两个函数并存，旧调用点不动。

**偏差 3：OperationID 不能叫 `listModels`**

该 ID 已被 OpenAI 兼容路由 `GET /api/openai/v1/models` 占用（Anthropic 侧用
`anthropicListModels`），重复会让 huma 直接 `panic: duplicate operation ID`。
既有 e2e `test/e2e/client_models` 当场捕获。

修正：命名为 `listWebModels`（沿用前缀式命名），限流中间件的 serviceName 同步。

**补充：鉴权失败的响应形态**

本项目鉴权失败是 **HTTP 200 + body `{"error":{"code":10001,"message":"Unauthorized"}}`**，
不是 HTTP 401（前端 `api-client` 对 `code=10001` 抛 `ApiError(401)`）。
e2e 断言须判 `error.code == constant.BizErrorCodeUnauthorized`，不能判 HTTP 状态码。
路由缺失则是 HTTP 404 + 纯文本 `Not Found`。
另：既有 e2e 里 `bizError.Code` 声明为 `string`，与实际的数值 code 不符（那些用例从未
走到错误分支故未暴露），新 e2e 声明为 `int`。
- `ModelListSortField` 六个字面量与后端 `constant.ModelListSortFields` 六个值逐字对应
- DTO `ListModelsRsp.Items` 的 JSON 名 `items` 与前端 `ListModelsPageRsp.items` 一致
- `nextSortState` / `buildModelListParams` 在测试与实现中同名
