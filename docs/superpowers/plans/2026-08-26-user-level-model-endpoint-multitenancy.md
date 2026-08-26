# Model / Endpoint 配置多租户化（用户级隔离）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 endpoint / model 配置下放到 user 级别做多租户隔离：`/v1/models` 只返回当前 API Key 所属用户的模型，转发解析只在用户自己的配置内进行，管理后台对所有 user 级用户开放自管、admin 可全局视图并按 username 过滤。

**Architecture:** 现有 `endpoints` / `models` 表直接加 `user_id` 列并重建唯一索引（方案一）。利用 GORM 结构体 Where 忽略零值字段的特性：`scopeUserID > 0` 表示按用户过滤，`0` 表示不过滤（admin 全局视图）。网关侧（API Key 鉴权路径）永远强制传真实 userID；管理后台由 handler 按 permission 计算有效 scope。存量数据经手动 `database migrate-data` 命令划归主 admin。

**Tech Stack:** Go 1.x + GORM (AutoMigrate / 手动索引重建) + huma v2 + Next.js (web/) + sqlite 内存库单测

**设计文档:** `docs/superpowers/specs/2026-08-26-user-level-model-endpoint-multitenancy-design.md`

## Global Constraints

- 所有回复与文档使用中文；代码标识符保留英文
- 工作分支：`feature/user-level-model-endpoint-multitenancy-2026-08-26`，worktree 位于 `.worktrees/user-level-model-endpoint`，禁止直接在主工作区开发
- shell 命令统一加 `rtk` 前缀；**测试命令禁止 rtk 过滤**（历史经验：rtk 会吞测试输出细节）
- 验证命令：`go build ./...`、`go test -count=1 ./cmd/... ./internal/... ./test/...`、`make lint`
- 迁移必须幂等：backfill 只填 `user_id = 0` 的行；索引用 `DROP INDEX IF EXISTS` + `CREATE UNIQUE INDEX IF NOT EXISTS` 守卫（PG DDL 自动提交，重跑不能卡死——见 memory `model-id-management/overview` 坑 3/4）
- 错误处理原则：不泄露其他用户资源的存在性——非本人资源一律 404 `ErrDataNotExists`
- demo 用户写接口按权限等级比较天然拒绝（`PermissionUser` < 要求等级不成立），无需额外逻辑
- 每个任务结束跑该任务的聚焦测试并 commit

---

### Task 1: 数据库模型加 `user_id` 列与新唯一索引

**Files:**
- Modify: `internal/infrastructure/database/model/endpoint.go`
- Modify: `internal/infrastructure/database/model/model.go`
- Modify: `internal/common/constant/sql.go`（字段列表加 `FieldUserID`）
- Test: `test/unit/db_index/db_index_test.go`

**Interfaces:**
- Produces: `dbmodel.Endpoint.UserID uint`、`dbmodel.Model.UserID uint`（GORM 列 `user_id`）；唯一索引名不变、列组合变为 `(user_id, name, deleted_at)` 与 `(user_id, alias, endpoint_id, deleted_at)`

- [ ] **Step 1: 先写失败的索引断言测试**

在 `test/unit/db_index/db_index_test.go` 中新增（复用文件内已有的 `newTestDB` helper）：

```go
// TestEndpointModelAutoMigrateUserScopeIndexes 验证多租户化后的复合唯一索引包含 user_id。
func TestEndpointModelAutoMigrateUserScopeIndexes(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	want := map[string][]string{
		"idx_endpoint_name_deleted":         {"user_id", "name", "deleted_at"},
		"idx_model_alias_endpoint_deleted":  {"user_id", "alias", "endpoint_id", "deleted_at"},
	}
	got := map[string][]string{}
	rows, err := db.Raw("SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name IN ('endpoints','models')").Rows()
	if err != nil {
		t.Fatalf("failed to query sqlite_master: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, sql string
		if err := rows.Scan(&name, &sql); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		if cols, ok := want[name]; ok {
			for _, c := range cols {
				if !strings.Contains(sql, c) {
					t.Errorf("index %s missing column %s, sql: %s", name, c, sql)
				}
			}
			got[name] = cols
		}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("expected index %q not found", name)
		}
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/db_index/ -run TestEndpointModelAutoMigrateUserScopeIndexes -v`
Expected: FAIL（索引列不含 user_id）

- [ ] **Step 3: 修改两个 dbmodel**

`internal/infrastructure/database/model/endpoint.go`：

```go
type Endpoint struct {
	BaseModel
	ID                          uint   `json:"id" gorm:"column:id;primary_key;auto_increment;comment:端点ID"`
	UserID                      uint   `json:"user_id" gorm:"column:user_id;not null;default:0;uniqueIndex:idx_endpoint_name_deleted,priority:1;comment:归属用户ID(逻辑外键→users.id)"`
	Name                        string `json:"name" gorm:"column:name;not null;uniqueIndex:idx_endpoint_name_deleted,priority:2;comment:端点名称"`
	// ...其余字段不动，DeletedAt priority 改为 3
}
```

`internal/infrastructure/database/model/model.go` 同理：加 `UserID uint`（priority:1），`Alias`→2、`EndpointID`→3、`DeletedAt`→4。

`internal/common/constant/sql.go`：`EndpointRepoFieldsFull` 开头插入 `FieldUserID`；`ModelRepoFieldsFull` 同理。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/db_index/ -v`
Expected: PASS（含原有 TestModelCallAuditAutoMigrateIndexes）

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/database/model/endpoint.go internal/infrastructure/database/model/model.go internal/common/constant/sql.go test/unit/db_index/db_index_test.go
git commit -m "feat(model-db): endpoints/models 增加 user_id 列与用户级复合唯一索引"
```

---

### Task 2: 存量数据迁移命令 `database migrate-data`

**Files:**
- Create: `internal/infrastructure/database/migration_user_scope.go`
- Modify: `cmd/server/database.go`
- Test: `test/unit/migration_user_scope/migration_user_scope_test.go`

**Interfaces:**
- Consumes: Task 1 的 `dbmodel.Endpoint.UserID` / `dbmodel.Model.UserID`
- Produces: `database.MigrateUserScopeData(ctx context.Context) error`（幂等：只回填 `user_id=0` 行到主 admin（permission=admin 中 ID 最小者），再重建两个唯一索引）

- [ ] **Step 1: 先写失败测试**

```go
// Package migration_user_scope 验证多租户化存量数据回填与唯一索引重建。
package migration_user_scope

import (
	"context"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	return db
}

func TestMigrateUserScopeData_BackfillAndIdempotent(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.User{}, &dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	admin1 := &dbmodel.User{Name: "old-admin", Permission: enum.PermissionAdmin}
	admin2 := &dbmodel.User{Name: "newer-admin", Permission: enum.PermissionAdmin}
	if err := db.Create(admin1).Error; err != nil { t.Fatal(err) }
	if err := db.Create(admin2).Error; err != nil { t.Fatal(err) }

	eps := []*dbmodel.Endpoint{{Name: "ep-a"}, {Name: "ep-b"}}
	mdls := []*dbmodel.Model{{Alias: "m-a", EndpointID: eps[0].ID}}
	for _, ep := range eps { if err := db.Create(ep).Error; err != nil { t.Fatal(err) } }
	if err := db.Create(mdls[0]).Error; err != nil { t.Fatal(err) }
	// 已有归属的行不被覆盖
	preOwned := &dbmodel.Endpoint{Name: "ep-owned", UserID: 999}
	if err := db.Create(preOwned).Error; err != nil { t.Fatal(err) }

	// 幂等性验证：连跑两次
	for i := 0; i < 2; i++ {
		if err := database.MigrateUserScopeDataWith(db, context.Background()); err != nil {
			t.Fatalf("round %d failed: %v", i+1, err)
		}
	}

	var gotEp dbmodel.Endpoint
	if err := db.Where("name = ?", "ep-a").First(&gotEp).Error; err != nil { t.Fatal(err) }
	if gotEp.UserID != admin1.ID {
		t.Fatalf("endpoint user_id = %d, want %d (min-ID admin)", gotEp.UserID, admin1.ID)
	}
	if err := db.Where("name = ?", "ep-owned").First(&gotEp).Error; err != nil { t.Fatal(err) }
	if gotEp.UserID != 999 {
		t.Fatalf("pre-owned endpoint overwritten: user_id = %d", gotEp.UserID)
	}

	// 无 admin 用户时报错而非静默成功
	empty := newTestDB(t)
	if err := empty.AutoMigrate(&dbmodel.User{}, &dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil { t.Fatal(err) }
	if err := database.MigrateUserScopeDataWith(empty, context.Background()); err == nil {
		t.Fatal("expected error when no admin user exists")
	}
}
```

注意：测试无法直接用 `InitDatabase()`（读全局配置），因此实现需暴露 `MigrateUserScopeDataWith(db *gorm.DB, ctx context.Context) error`，公开入口 `MigrateUserScopeData(ctx)` 内部调它。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/migration_user_scope/ -v`
Expected: FAIL（函数未定义）

- [ ] **Step 3: 实现迁移函数**

`internal/infrastructure/database/migration_user_scope.go`：

```go
package database

import (
	"context"
	"errors"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm"
)

// MigrateUserScopeData 多租户化存量数据迁移入口（database migrate-data 命令调用）。
func MigrateUserScopeData(ctx context.Context) error {
	return MigrateUserScopeDataWith(InitDatabase(), ctx)
}

// MigrateUserScopeDataWith 可注入 DB 的迁移实现：
//  1. 回填 user_id=0 的 endpoints/models 到主 admin（permission=admin 中 ID 最小者），幂等；
//  2. 重建两个复合唯一索引（旧库 AutoMigrate 不会改已有同名索引的列组合）。
func MigrateUserScopeDataWith(db *gorm.DB, ctx context.Context) error {
	db = db.WithContext(ctx)
	return db.Transaction(func(tx *gorm.DB) error {
		var admin dbmodel.User
		err := tx.Where("permission = ?", enum.PermissionAdmin).Order("id ASC").First(&admin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ierr.New(ierr.ErrDataNotExists, "no admin user found for user-scope backfill")
		}
		if err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "find primary admin")
		}
		if err := tx.Model(&dbmodel.Endpoint{}).Where("user_id = 0").Update("user_id", admin.ID).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, err, "backfill endpoint user_id")
		}
		if err := tx.Model(&dbmodel.Model{}).Where("user_id = 0").Update("user_id", admin.ID).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, err, "backfill model user_id")
		}
		if err := rebuildUniqueIndex(tx, "endpoints", "idx_endpoint_name_deleted",
			"user_id, name, deleted_at"); err != nil {
			return err
		}
		return rebuildUniqueIndex(tx, "models", "idx_model_alias_endpoint_deleted",
			"user_id, alias, endpoint_id, deleted_at")
	})
}

// rebuildUniqueIndex 幂等重建唯一索引：DROP IF EXISTS 后按新列组合重建。
func rebuildUniqueIndex(tx *gorm.DB, table, indexName, columns string) error {
	if err := tx.Exec("DROP INDEX IF EXISTS " + indexName).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "drop index "+indexName)
	}
	if err := tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS " + indexName + " ON " + table + "(" + columns + ")").Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "create index "+indexName)
	}
	return nil
}
```

`cmd/server/database.go` 重新加回命令（放在 `init()` 前）：

```go
var migrateDatabaseDataCmd = &cobra.Command{
	Use:   "migrate-data",
	Short: "Migrate Database Data",
	Long:  `Execute data migration operation, e.g. backfilling user-scope columns and rebuilding unique indexes.`,
	Run: func(cmd *cobra.Command, _ []string) {
		lo.Must0(database.MigrateUserScopeData(cmd.Context()))
	},
}

func init() {
	databaseCmd.AddCommand(migrateDatabaseCmd)
	databaseCmd.AddCommand(migrateDatabaseDataCmd)
	rootCmd.AddCommand(databaseCmd)
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/migration_user_scope/ ./cmd/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/infrastructure/database/migration_user_scope.go cmd/server/database.go test/unit/migration_user_scope/
git commit -m "feat(migrate): 新增 database migrate-data 用户级回填与唯一索引重建命令"
```

---

### Task 3: Repository 层用户隔离（含聚合创建写入 user_id）

**Files:**
- Modify: `internal/domain/llmproxy/repository.go`
- Modify: `internal/infrastructure/repository/endpoint_repository.go`
- Test: `test/unit/llmproxy_repo_scope/endpoint_repository_scope_test.go`（新建，参照 `test/unit/db_index` 的 sqlite 模式）

**Interfaces:**
- Produces（后续任务依赖的新签名，`scopeUserID uint`：`>0` 过滤归属，`0` 不过滤）:

```go
type EndpointRepository interface {
	FindByID(ctx context.Context, id uint, scopeUserID uint) (*aggregate.Endpoint, error)
	BatchFindByIDs(ctx context.Context, ids []uint) (map[uint]*aggregate.Endpoint, error) // 不变
	Create(ctx context.Context, endpoint *aggregate.Endpoint, ownerUserID uint) (uint, error)
	Update(ctx context.Context, endpoint *aggregate.Endpoint) error                       // 不变（先 FindByID 校验归属）
	Delete(ctx context.Context, id uint, scopeUserID uint) error
	DeleteCascade(ctx context.Context, id uint, scopeUserID uint) error
	List(ctx context.Context) ([]*aggregate.Endpoint, error)                              // 不变（内部使用）
	Paginate(ctx context.Context, param model.CommonParam, scopeUserID uint) ([]*aggregate.Endpoint, *model.PageInfo, error)
}

type ModelRepository interface {
	FindByAlias(ctx context.Context, alias vo.EndpointAlias, userID uint) ([]*aggregate.Model, error) // 网关解析专用，必传真实 userID
	FindByID(ctx context.Context, id uint, scopeUserID uint) (*aggregate.Model, error)
	Create(ctx context.Context, model *aggregate.Model, ownerUserID uint) (uint, error)
	Update(ctx context.Context, model *aggregate.Model) error // 不变
	Delete(ctx context.Context, id uint, scopeUserID uint) error
	DeleteByEndpointID(ctx context.Context, endpointID uint) error // 不变（级联删除前已校验端点归属）
	List(ctx context.Context) ([]*aggregate.Model, error)
	Paginate(ctx context.Context, param model.CommonParam, scopeUserID uint) ([]*aggregate.Model, *model.PageInfo, error)
}
```

核心机制：GORM 结构体条件忽略零值字段——`dao.Get(db, &dbmodel.Endpoint{ID: id, UserID: scopeUserID}, ...)` 当 `scopeUserID == 0` 时自动退化为全局查询。**不要**手写 `if scopeUserID > 0` 分支。

- [ ] **Step 1: 先写失败的隔离测试**

新建包，fake-free 直接打 sqlite：

```go
// Package llmproxy_repo_scope 验证 endpoint/model 仓储的用户级隔离。
package llmproxy_repo_scope

import (
	"context"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB { /* 同 Task 2 模式 */ }

func seed(t *testing.T, db *gorm.DB) (epA, epB *model.Endpoint) {
	t.Helper()
	epA = &model.Endpoint{UserID: 101, Name: "ep-a", APIKey: "k"}
	epB = &model.Endpoint{UserID: 202, Name: "ep-b", APIKey: "k"}
	if err := db.Create(epA).Error; err != nil { t.Fatal(err) }
	if err := db.Create(epB).Error; err != nil { t.Fatal(err) }
	mA := &model.Model{UserID: 101, Alias: "gpt-x", UpstreamModel: "up-x", EndpointID: epA.ID, Capabilities: []enum.InputModality{enum.InputModalityText}}
	mB := &model.Model{UserID: 202, Alias: "gpt-y", UpstreamModel: "up-y", EndpointID: epB.ID, Capabilities: []enum.InputModality{enum.InputModalityText}}
	if err := db.Create(mA).Error; err != nil { t.Fatal(err) }
	if err := db.Create(mB).Error; err != nil { t.Fatal(err) }
	return epA, epB
}

func TestEndpointRepo_UserIsolation(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&model.Endpoint{}, &model.Model{}); err != nil { t.Fatal(err) }
	epA, _ := seed(t, db)

	repo := repository.NewEndpointRepository(db)

	// scope=101 能看到自己的，看不到 202 的
	ep, err := repo.FindByID(context.Background(), epB.ID, 101)
	if err != nil { t.Fatal(err) }
	if ep != nil { t.Fatal("user 101 must not see user 202's endpoint") }

	ep, err = repo.FindByID(context.Background(), epA.ID, 101)
	if err != nil { t.Fatal(err) }
	if ep == nil { t.Fatal("user 101 must see own endpoint") }

	// scope=0（admin）全量可见
	ep, _ = repo.FindByID(context.Background(), epB.ID, 0)
	if ep == nil { t.Fatal("admin scope must see all") }

	// 分页过滤
	list, page, err := repo.Paginate(context.Background(), model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}}, 101)
	if err != nil { t.Fatal(err) }
	if len(list) != 1 || page.Total != 1 { t.Fatalf("scoped paginate: got %d items, total %d", len(list), page.Total) }

	// 创建时写入 owner
	id, err := repo.Create(context.Background(), mustEndpoint(t), 303)
	if err != nil { t.Fatal(err) }
	var row model.Endpoint
	if err := db.First(&row, id).Error; err != nil { t.Fatal(err) }
	if row.UserID != 303 { t.Fatalf("created user_id = %d, want 303", row.UserID) }

	// 删除越权无效
	if err := repo.Delete(context.Background(), epB.ID, 101); err != nil { t.Fatal(err) }
	var cnt int64
	db.Model(&model.Endpoint{}).Where("id = ? AND deleted_at = 0", epB.ID).Count(&cnt)
	if cnt != 1 { t.Fatal("cross-user delete must be a no-op") }
}
```

（`mustEndpoint` 构造合法聚合根；`Model` 仓储同构测试：FindByAlias 按 userID 过滤、Paginate 过滤、Create 写入 owner。）

- [ ] **Step 2: 跑测试确认编译失败（签名未变）**

Run: `go build ./internal/infrastructure/repository/ && go test -count=1 ./test/unit/llmproxy_repo_scope/ -v`
Expected: 编译 FAIL

- [ ] **Step 3: 改接口与实现**

按 Interfaces 块修改 `repository.go` 接口签名；`endpoint_repository.go` 实现要点：

```go
// FindByID：结构体 Where 天然支持 scope
func (r *endpointRepository) FindByID(ctx context.Context, id uint, scopeUserID uint) (*aggregate.Endpoint, error) {
	db := r.db.WithContext(ctx)
	ep, err := r.endpointDAO.Get(db, &dbmodel.Endpoint{ID: id, UserID: scopeUserID}, constant.EndpointRepoFieldsFull)
	// 其余错误处理不变
}

// Create：写入归属
func (r *endpointRepository) Create(ctx context.Context, ep *aggregate.Endpoint, ownerUserID uint) (uint, error) {
	m := toEndpointModel(ep)
	m.UserID = ownerUserID
	...
}

// Delete / DeleteCascade / Paginate：where 结构体带上 UserID 字段，
// BatchDeleteByField 用于 cascade 删除 models 时保持原样（端点归属已在 FindByID 校验过）。
// modelRepository.FindByAlias / FindByID / Create / Delete / Paginate 同样处理。
```

同时把 `toEndpointAggregate` / `toEndpointProjection` 保持不映射 UserID（聚合根不感知租户字段，YAGNI）。

- [ ] **Step 4: 修复全部编译点**

Run: `go build ./... 2>&1 | head -30`
Expected: 因接口变更产生的编译错误集中在 `internal/application/{endpoint,model,llmproxy}` 与 `test/unit/{endpoint_query,model_query,endpoint_resolver}`。本任务先以最小方式让编译通过：调用处临时传 `0` 或从 ctx 取 userID（Task 4/5 再接真实语义）。fake repo 测试桩同步补参。

- [ ] **Step 5: 跑测试确认通过**

Run: `go test -count=1 ./test/unit/llmproxy_repo_scope/ ./test/unit/endpoint_query/ ./test/unit/model_query/ ./test/unit/endpoint_resolver/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/domain/llmproxy/repository.go internal/infrastructure/repository/endpoint_repository.go test/unit/llmproxy_repo_scope/ internal/application/ test/unit/endpoint_query/ test/unit/model_query/ test/unit/endpoint_resolver/
git commit -m "feat(repo): endpoint/model 仓储增加用户级隔离查询与归属写入"
```

---

### Task 4: EndpointResolver 与网关读模型按用户解析

**Files:**
- Modify: `internal/domain/llmproxy/service/resolver.go`
- Modify: `internal/application/llmproxy/usecase/query.go`
- Modify: `internal/application/llmproxy/usecase/openai.go`
- Modify: `internal/application/llmproxy/usecase/anthropic.go`
- Test: `test/unit/endpoint_resolver/endpoint_resolver_test.go`（更新 stub 签名）

**Interfaces:**
- Consumes: Task 3 的 `ModelRepository.FindByAlias(ctx, alias, userID)`、`EndpointReadRepository` 新签名
- Produces:

```go
type EndpointResolver interface {
	Resolve(ctx context.Context, userID uint, alias vo.EndpointAlias, matcher func(*aggregate.Endpoint) bool) (*aggregate.Endpoint, *aggregate.Model, error)
}

type EndpointReadRepository interface {
	ListAliases(ctx context.Context, userID uint) ([]*ModelAliasProjection, error)
	FindEndpointByAlias(ctx context.Context, userID uint, alias string, matcher func(*EndpointProjection) bool) (*EndpointProjection, *ModelAliasProjection, error)
}
```

语义：网关路径（openai/anthropic usecase、query.go 的三个 Handler）一律 `userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)` 从 context 取（APIKeyMiddleware 已注入），透传给 Resolve / ListAliases / FindEndpointByAlias。找不到即走现有协议化 404，不改错误格式。

- [ ] **Step 1: 更新 resolver 单测 stub 并新增隔离用例**

`stubModelRepo.FindByAlias` 签名加 `userID uint`；新增用例：user 101 的 stub 只对 `userID==101` 返回 hit，`userID==202` 返回 miss，断言 `Resolve(ctx, 202, "gpt-x", nil)` 返回 `ErrDataNotExists`。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/endpoint_resolver/ -v`
Expected: FAIL / 编译错

- [ ] **Step 3: 实现**

`resolver.go`：`Resolve` 加 `userID uint` 参数并透传给 `r.modelRepo.FindByAlias(ctx, alias, userID)`。`endpoint_repository.go` 读模型两个方法把 userID 放进 `BatchGet` 的 where 结构体（`&dbmodel.Model{Enabled: true, UserID: userID}`）。

`query.go` 三个 Handler 内部取 ctx userID：

```go
func (q *listOpenAIModels) Handle(ctx context.Context) (*dto.OpenAIListModelsRsp, error) {
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	projections, err := q.readRepo.ListAliases(ctx, userID)
	// 其余不变
}
```

`openai.go` 两处、`anthropic.go` 一处 `u.resolver.Resolve(ctx, userID, vo.EndpointAlias(...), matcher)`；`countTokens.Handle` 同样处理。

- [ ] **Step 4: 跑测试确认通过 + 全量编译**

Run: `go build ./... && go test -count=1 ./test/unit/endpoint_resolver/ ./test/unit/llmproxy_usecase/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/domain/llmproxy/service/resolver.go internal/application/llmproxy/usecase/ internal/infrastructure/repository/endpoint_repository.go test/unit/endpoint_resolver/
git commit -m "feat(gateway): 模型列表与端点解析按 API Key 所属用户隔离"
```

---

### Task 5: 管理后台权限下放（endpoint CRUD）

**Files:**
- Modify: `internal/application/endpoint/port/handler.go`
- Modify: `internal/application/endpoint/command/*.go`、`internal/application/endpoint/query/list_endpoints.go`
- Modify: `internal/dto/endpoint.go`
- Modify: `internal/handler/endpoint.go`
- Modify: `internal/router/endpoint.go`
- Test: `test/unit/endpoint_command/create_endpoint_owner_test.go`（新建）+ 更新 `test/unit/endpoint_query/list_endpoints_demo_test.go` fake

**Interfaces:**
- Consumes: Task 3 仓储新签名；`util.CtxValuePermission` / `util.CtxValueUint`；identity 侧 `FindByName`（本任务一并添加，见 Step 3）
- Produces:
  - `port.ListEndpointsQuery{ model.CommonParam; IsDemo bool; ScopeUserID uint }`（handler 计算：admin→0，其余→自身）
  - `port.CreateEndpointCommand` 增加 `OwnerUserID uint`（admin 可指定他人，缺省自身；普通用户强制自身且忽略传入值）
  - DTO：`ListEndpointsReq` 加 `Username string \`query:"username,omitempty"\``；`CreateEndpointReqBody` 加 `OwnerUserID *uint \`json:"ownerUserID,omitempty" minimum:"1" doc:"归属用户ID(仅管理员生效)"\``
  - `ListEndpointsRsp.EndpointItem` 加 `Username string \`json:"username" doc:"归属用户名"\``（admin 全局视图需要辨认归属；普通用户视图即自己，无害冗余）

- [ ] **Step 1: 先写失败的归属测试**

```go
func TestCreateEndpoint_OwnerScoping(t *testing.T) {
	// fake repo 断言 Create 收到的 ownerUserID：
	//   普通用户命令 OwnerUserID=999（伪造）+ ctx userID=101 → repo 收到 101
	//   admin 命令 OwnerUserID=999 + ctx userID=1    → repo 收到 999
}
```

fake repo 参照 `test/unit/endpoint_query/list_endpoints_demo_test.go` 的模式（接口断言 `var _ llmproxy.EndpointRepository = (*fakeEndpointRepo)(nil)`）。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/endpoint_command/ -v`
Expected: FAIL

- [ ] **Step 3: 实现**

1. identity 仓储加精确按名查询（供 username 过滤）：

`internal/domain/identity/repository.go` 接口加：

```go
// FindByName 按用户名精确查询；未找到返回 (nil, nil)
FindByName(ctx context.Context, name string) (*aggregate.User, error)
```

GORM 实现一行：`dao.Get(db, &dbmodel.User{Name: name}, constant.UserRepoFieldsBasic)`（找到对应 repository 文件同步）。

2. `list_endpoints.go` 注入 `identity.UserRepository`，Handle 逻辑：

```go
scope := q.ScopeUserID
username := q.Username // 仅 admin 视图（ScopeUserID==0）允许
if q.Username != "" && scope == 0 {
	u, err := h.userRepo.FindByName(ctx, q.Username)
	if err != nil { return nil, nil, err }
	if u == nil { // 用户不存在 → 空结果而非错误
		return []*port.EndpointView{}, &model.PageInfo{Page: q.Page, PageSize: q.PageSize}, nil
	}
	scope = u.ID()
}
endpoints, pageInfo, err := h.repo.Paginate(ctx, q.CommonParam, scope)
```

View 组装需要 username：批量 `userRepo.BatchFindByIDs`（identity 仓储已有）取 `map[uint]name` 填充。

3. `handler/endpoint.go` 各方法计算 scope：

```go
perm := util.CtxValuePermission(ctx)
selfID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
is_admin := perm == enum.PermissionAdmin

// list
port.ListEndpointsQuery{
	CommonParam: req.CommonParam,
	IsDemo:      perm == enum.PermissionDemo,
	ScopeUserID: lo.Ternary(is_admin, 0, selfID),
	Username:    req.Username, // port 结构加同名字段
}

// create
owner := lo.Ternary(is_admin && req.Body.OwnerUserID != nil, *req.Body.OwnerUserID, selfID)
port.CreateEndpointCommand{ ..., OwnerUserID: owner }

// update/delete：命令结构加 ScopeUserID uint，传 lo.Ternary(is_admin, 0, selfID)
```

4. `router/endpoint.go`：四处 `enum.PermissionAdmin` → `enum.PermissionUser`（demo 的 `LimitUserPermissionWithDemoMiddleware` 保持不变）。

- [ ] **Step 4: 跑测试确认通过 + 编译**

Run: `go build ./... && go test -count=1 ./test/unit/endpoint_command/ ./test/unit/endpoint_query/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/endpoint/ internal/dto/endpoint.go internal/handler/endpoint.go internal/router/endpoint.go internal/domain/identity/ internal/infrastructure/repository/ test/unit/endpoint_command/ test/unit/endpoint_query/
git commit -m "feat(endpoint-api): endpoint 管理接口下放至用户级，admin 支持 username 过滤与代建"
```

---

### Task 6: 管理后台权限下放（model CRUD + 跨租户校验）

**Files:**
- Modify: `internal/application/model/port/handler.go`、`internal/application/model/command/*.go`、`internal/application/model/query/list_models.go`
- Modify: `internal/dto/model.go`、`internal/handler/model.go`、`internal/router/model.go`
- Test: `test/unit/model_command/create_model_scope_test.go`（新建）

**Interfaces:**
- Consumes: Task 3 / Task 5 全部产出
- Produces:
  - `CreateModelCommand` 不加归属字段——model 归属**从其 endpoint 带入**（跨租户校验天然成立）：command 处理器里 `FindByID(ctx, cmd.EndpointID, scope)` 查到的 ep 即归属来源；repo `Create(ctx, m, ep.UserID())`
  - 为此 `aggregate.Endpoint` 增加 `SetUserID(uint)` / `UserID() uint`（repository `toEndpointAggregate` 映射 dbmodel 的 UserID）
  - `UpdateModelCommand` / `DeleteModelCommand` 加 `ScopeUserID uint`（换绑 endpoint 时同样校验目标 endpoint 归属）
  - DTO/路由与 Task 5 对称：`ListModelsReq.Username`、`router/model.go` 权限降级

- [ ] **Step 1: 先写失败测试**

用例一：普通用户（scope=101）对自己 endpoint 建 model → 成功且 repo 收到 ownerUserID=101；用例二：对他人 endpoint（202）建 model → `FindByID(scope=101)` 返回 nil → 400 `ierr.ErrValidation`（"endpoint 不存在或无权访问"，对外表现为数据不存在）；用例三：update 换绑到他人 endpoint → 同样拒绝。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test -count=1 ./test/unit/model_command/ -v`
Expected: FAIL

- [ ] **Step 3: 实现**

`aggregate/endpoint.go`：struct 加 `userID uint` 字段 + getter/setter（参照现有 `SetTimestamps` 风格）；`endpoint_repository.go` 的 `toEndpointAggregate` 设置之。

`create_model.go` 核心 diff：

```go
scope := cmd.ScopeUserID // port 结构新增；admin 为 0
ep, err := h.endpointRepo.FindByID(ctx, cmd.EndpointID, scope)
if err != nil { ... }
if ep == nil {
	return nil, ierr.New(ierr.ErrValidation, "endpoint not found")
}
...
id, err := h.modelRepo.Create(ctx, m, ep.UserID())
```

`update_model.go`：换绑分支（`cmd.EndpointID != nil`）时以新 scope 查目标 endpoint 校验；`delete_model.go`：`FindByID(ctx, cmd.ModelID, cmd.ScopeUserID)` 为 nil 时返回 `ErrDataNotExists`。

`list_models.go`：与 Task 5 的 list_endpoints 完全对称（注入 userRepo、username 解析、View 填 username——`ModelView` 加 `Username string`）。

`dto/model.go` / `handler/model.go` / `router/model.go`：与 Task 5 对称修改。

- [ ] **Step 4: 跑测试确认通过**

Run: `go go build ./... && go test -count=1 ./test/unit/model_command/ ./test/unit/model_query/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/application/model/ internal/dto/model.go internal/handler/model.go internal/router/model.go internal/domain/llmproxy/aggregate/endpoint.go internal/infrastructure/repository/endpoint_repository.go test/unit/model_command/
git commit -m "feat(model-api): model 管理接口下放至用户级，归属继承 endpoint 并做跨租户校验"
```

---

### Task 7: Web 前端适配

**Files:**
- Modify: `web/src/app/(dashboard)/layout.tsx`（endpoints / models 两项去掉 `adminOnly: true`）
- Modify: `web/src/lib/api-client.ts`（`listEndpoints` / `listModels` 加可选 `username` 参数拼 query；`CreateEndpointReqBody` 类型加 `ownerUserID?: number`）
- Modify: `web/src/lib/types.ts` 或类型所在文件（`EndpointItem` 加 `username: string`；`CreateEndpointReqBody` 同步）
- Modify: `web/src/app/(dashboard)/endpoints/page.tsx`、`web/src/app/(dashboard)/models/page.tsx`
- Modify: `CONTEXT.md`（词汇表补「私有配置」条目）

**Interfaces:**
- Consumes: Task 5/6 的 API 变更（query `username`、body `ownerUserID`、响应项 `username`）

- [ ] **Step 1: 导航去 adminOnly**

`layout.tsx` 中 `nav.endpoints` 与 `nav.models` 两项删除 `adminOnly: true,` 行（保留 `demoModule`）。

- [ ] **Step 2: api-client 与类型**

```ts
async listEndpoints(
  params: { page?: number; pageSize?: number; query?: string; sort?: string; sortField?: string; username?: string },
): Promise<ListEndpointsRsp> {
  const q = new URLSearchParams(/* 现有逻辑 */);
  if (params.username) q.set("username", params.username);
  ...
}
```

`CreateEndpointReqBody` 类型加 `ownerUserID?: number`，`createEndpoint` body 原样透传。

- [ ] **Step 3: 页面加 admin 用户名过滤框**

两个页面在现有搜索/筛选区追加：当 `isAdmin`（来自 `auth-context`）时渲染一个文本输入（placeholder 用 i18n key `endpoints.filterByUsername` / `models.filterByUsername`，en/zh/ja 三份 locale 文件各加一条），debounce 后并入 list 请求参数。创建表单不加归属选择控件（admin 代建走 API/未来需求，YAGNI）。

- [ ] **Step 4: 前端验证**

Run: `cd web && npx tsc --noEmit && npm run lint && npm run build`
Expected: 0 error（Turbopack 字体缓存问题出现时 `rm -rf web/.next` 重试——已知环境问题）

- [ ] **Step 5: Commit**

```bash
git add web/src CONTEXT.md
git commit -m "feat(web): endpoints/models 页面向全体用户开放，admin 支持按用户名过滤"
```

---

### Task 8: Bootstrap 装配核对 + 全量回归

**Files:**
- Verify: `internal/bootstrap/modules/application.go`（NewListEndpointsHandler / NewListModelsHandler 构造参数多了 userRepo，dig 自动注入无需改 provider，但要确认无 fx 报错）
- Verify: `web/src` 之外的残留编译点

- [ ] **Step 1: 全量构建与测试**

Run: `go build ./... ; go test -count=1 ./cmd/... ./internal/... ./test/...`
Expected: 编译通过、全部测试绿（`internal/web/static.go` 的 web dist 报错为已知环境问题，前端 build 后消失）

- [ ] **Step 2: lint**

Run: `make lint`
Expected: 0 error（存量 warning 允许）

- [ ] **Step 3: 修复发现的问题并 commit（如有）**

```bash
git add -A :!web/.next
git commit -m "fix: 多租户化改造回归修复"
```

---

### Task 9: 双用户隔离 E2E 用例沉淀

**Files:**
- Create: `test/e2e/user_scope_config/user_scope_config_test.go`

**Interfaces:**
- Consumes: 生产式 HTTP API；env `BASE_URL`、`JWT_TOKEN`（参照 `test/e2e/model_id/model_id_test.go` 的 mustE2EEnv 模式）
- 说明：双账号隔离的核心断言由 Task 3 的仓储级单测覆盖（sqlite 真库行为）；E2E 聚焦「权限下放 + username 过滤 + ownerUserID 代建 + /v1/models 归属」的装配正确性，单 token（admin）即可闭环

- [ ] **Step 1: 写 E2E**

用例流（admin JWT）：
1. `POST /api/v1/endpoint` 带 `ownerUserID: 1`（admin 自身）创建 `e2e-uscope-ep-{ts}`
2. `GET /api/v1/endpoint/list?username=<admin 名>` 断言列表含该端点且响应项 `username` 正确；`username=不存在的用户xyz` 断言空列表
3. `POST /api/v1/model` 在该端点上建别名 `e2e-uscope-m-{ts}`
4. `GET /api/v1/model/list?username=<admin 名>` 含该别名
5. 清理：删除 model、endpoint（cascade）

HTTP 细节（鉴权头、bizError 解析）直接复制 `test/e2e/model_id/model_id_test.go` 的既有 helper。

- [ ] **Step 2: 本地起服务跑通**

Run: `BASE_URL=http://localhost:8080 JWT_TOKEN=<admin jwt> go test -count=1 ./test/e2e/user_scope_config/ -v`
Expected: PASS（若本地无服务则标记 `t.Skip` 条件与 model_id E2E 保持一致）

- [ ] **Step 3: Commit**

```bash
git add test/e2e/user_scope_config/
git commit -m "test(e2e): 用户级配置隔离与 admin 过滤链路 E2E 用例"
```

---

### Task 10: ponytail-review + 文档收尾

- [ ] **Step 1: 过度工程审查**

对本分支 diff 执行 `ponytail-review` skill：检查投机抽象（如不必要的 scope 封装层）、重复造轮子、死代码（如 `EndpointRepository.List` 是否因改造彻底无人调用可删）。逐条处理或记录理由。

- [ ] **Step 2: Serena 沉淀工程经验**

`serena_write_memory` 写入：多租户化的 scope 机制（GORM 零值结构体条件 = scope 开关）、迁移幂等三件套（default 0 回填 / DROP+CREATE IF EXISTS / HasIndex 思路）、model 归属继承 endpoint 的设计决策。

- [ ] **Step 3: 最终提交状态确认**

Run: `rtk git status && rtk git log --oneline master..HEAD`
Expected: 工作区干净，任务序列提交完整。**不推送、不合 master**——等待用户指示。
