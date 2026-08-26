// Package llmproxy_repo_scope 验证 endpoint/model 仓储的用户级隔离。
//
// 背景（feature/user-level-model-endpoint-multitenancy）：
//   - scopeUserID > 0 时查询/删除必须限定在该用户名下；scopeUserID == 0（admin 视角）不过滤；
//   - Create 写入归属 ownerUserID；
//   - FindByAlias 为网关解析专用，userID 必传真实值。
package llmproxy_repo_scope

import (
	"context"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
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

func seed(t *testing.T, db *gorm.DB) (epA, epB *dbmodel.Endpoint, mA *dbmodel.Model) {
	t.Helper()
	var mB *dbmodel.Model
	epA = &dbmodel.Endpoint{UserID: 101, Name: "ep-a", APIKey: "k", OpenaiBaseURL: "https://o.example.com", AnthropicBaseURL: "https://a.example.com", SupportOpenAIChatCompletion: true}
	epB = &dbmodel.Endpoint{UserID: 202, Name: "ep-b", APIKey: "k", OpenaiBaseURL: "https://o.example.com", AnthropicBaseURL: "https://a.example.com", SupportOpenAIChatCompletion: true}
	if err := db.Create(epA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(epB).Error; err != nil {
		t.Fatal(err)
	}
	textCaps := []enum.InputModality{enum.InputModalityText}
	mA = &dbmodel.Model{UserID: 101, Alias: "gpt-x", ModelID: "gpt-x", UpstreamModel: "up-x", EndpointID: epA.ID, Capabilities: textCaps}
	mB = &dbmodel.Model{UserID: 202, Alias: "gpt-y", ModelID: "gpt-y", UpstreamModel: "up-y", EndpointID: epB.ID, Capabilities: textCaps}
	if err := db.Create(mA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(mB).Error; err != nil {
		t.Fatal(err)
	}
	return epA, epB, mA
}

func mustAggregateEndpoint(t *testing.T) *aggregate.Endpoint {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(0, "ep-new", "https://o.example.com", "https://a.example.com", "sk-k", true, false, false)
	if err != nil {
		t.Fatalf("aggregate endpoint: %v", err)
	}
	return ep
}

func TestEndpointRepo_UserIsolation(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}
	epA, epB, _ := seed(t, db)
	ctx := context.Background()
	repo := repository.NewEndpointRepository(db)

	// scope=101 看不到 202 的端点
	ep, err := repo.FindByID(ctx, epB.ID, 101)
	if err != nil {
		t.Fatal(err)
	}
	if ep != nil {
		t.Fatal("user 101 must not see user 202's endpoint")
	}

	// scope=101 能看到自己的
	ep, err = repo.FindByID(ctx, epA.ID, 101)
	if err != nil {
		t.Fatalf("own lookup: %v", err)
	}
	if ep == nil {
		t.Fatal("user 101 must see own endpoint")
	}

	// scope=0（admin）全量可见
	ep, err = repo.FindByID(ctx, epB.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if ep == nil {
		t.Fatal("admin scope must see all")
	}

	// 分页过滤
	list, page, err := repo.Paginate(ctx, model.CommonParam{}, 101)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || page.Total != 1 {
		t.Fatalf("scoped paginate: got %d items, total %d", len(list), page.Total)
	}

	// 创建时写入 owner
	id, err := repo.Create(ctx, mustAggregateEndpoint(t), 303)
	if err != nil {
		t.Fatal(err)
	}
	var row dbmodel.Endpoint
	if err := db.First(&row, id).Error; err != nil {
		t.Fatal(err)
	}
	if row.UserID != 303 {
		t.Fatalf("created user_id = %d, want 303", row.UserID)
	}

	// 删除越权无效
	if err := repo.Delete(ctx, epB.ID, 101); err != nil {
		t.Fatal(err)
	}
	var cnt int64
	db.Model(&dbmodel.Endpoint{}).Where("id = ? AND deleted_at = 0", epB.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatal("cross-user delete must be a no-op")
	}

	// 本人删除有效
	if err := repo.Delete(ctx, epA.ID, 101); err != nil {
		t.Fatal(err)
	}
	db.Model(&dbmodel.Endpoint{}).Where("id = ? AND deleted_at = 0", epA.ID).Count(&cnt)
	if cnt != 0 {
		t.Fatal("owner delete must take effect")
	}
}

func TestModelRepo_UserIsolation(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}
	_, _, mA := seed(t, db)
	ctx := context.Background()
	repo := repository.NewModelRepository(db)

	// FindByAlias 按 userID 过滤（网关解析语义）
	ms, err := repo.FindByAlias(ctx, vo.EndpointAlias("gpt-x"), 202)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 0 {
		t.Fatal("user 202 must not resolve user 101's alias")
	}
	ms, err = repo.FindByAlias(ctx, vo.EndpointAlias("gpt-x"), 101)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("user 101 must resolve own alias, got %d", len(ms))
	}

	// FindByID 隔离
	m, err := repo.FindByID(ctx, mA.ID, 202)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("user 202 must not see user 101's model")
	}

	// 分页过滤
	list, page, err := repo.Paginate(ctx, model.CommonParam{}, 202)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || page.Total != 1 {
		t.Fatalf("scoped paginate: got %d items, total %d", len(list), page.Total)
	}

	// admin 全量
	_, pageAll, err := repo.Paginate(ctx, model.CommonParam{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if pageAll.Total != 2 {
		t.Fatalf("admin total = %d, want 2", pageAll.Total)
	}

	// 删除越权无效
	if err := repo.Delete(ctx, mA.ID, 202); err != nil {
		t.Fatal(err)
	}
	var cnt int64
	db.Model(&dbmodel.Model{}).Where("id = ? AND deleted_at = 0", mA.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatal("cross-user delete must be a no-op")
	}
}

var (
	_ llmproxy.EndpointRepository = repository.NewEndpointRepository(nil)
	_ llmproxy.ModelRepository    = repository.NewModelRepository(nil)
)
