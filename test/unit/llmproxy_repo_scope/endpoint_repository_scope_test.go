// Package llmproxy_repo_scope 验证 endpoint/model 仓储的用户级隔离。
//
// 背景（feature/user-level-model-endpoint-multitenancy + CR 2026-08-28）：
//   - scopeUserID 三态：nil（admin 视角）不过滤；非 nil（含 0）精确匹配 user_id，
//     0 用于共享池（user_id=0 的存量/共享配置）；
//   - Create 写入归属 ownerUserID；
//   - FindByAlias 为网关解析专用，userID 传真实值（0=共享池）。
package llmproxy_repo_scope

import (
	"context"
	"strings"
	"testing"

	"github.com/samber/lo"

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

func seed(t *testing.T, db *gorm.DB) (epA, epB, epShared *dbmodel.Endpoint, mA *dbmodel.Model) {
	t.Helper()
	epA = &dbmodel.Endpoint{UserID: 101, Name: "ep-a", APIKey: "k", OpenaiBaseURL: "https://o.example.com", AnthropicBaseURL: "https://a.example.com", SupportOpenAIChatCompletion: true}
	epB = &dbmodel.Endpoint{UserID: 202, Name: "ep-b", APIKey: "k", OpenaiBaseURL: "https://o.example.com", AnthropicBaseURL: "https://a.example.com", SupportOpenAIChatCompletion: true}
	epShared = &dbmodel.Endpoint{UserID: 0, Name: "ep-shared", APIKey: "k", OpenaiBaseURL: "https://o.example.com", AnthropicBaseURL: "https://a.example.com", SupportOpenAIChatCompletion: true}
	for _, ep := range []*dbmodel.Endpoint{epA, epB, epShared} {
		if err := db.Create(ep).Error; err != nil {
			t.Fatal(err)
		}
	}
	textCaps := []enum.InputModality{enum.InputModalityText}
	mA = &dbmodel.Model{UserID: 101, Alias: "gpt-x", ModelID: "gpt-x", UpstreamModel: "up-x", EndpointID: epA.ID, Capabilities: textCaps}
	mB := &dbmodel.Model{UserID: 202, Alias: "gpt-y", ModelID: "gpt-y", UpstreamModel: "up-y", EndpointID: epB.ID, Capabilities: textCaps}
	mShared := &dbmodel.Model{UserID: 0, Alias: "gpt-shared", ModelID: "gpt-shared", UpstreamModel: "up-shared", EndpointID: epShared.ID, Capabilities: textCaps}
	for _, m := range []*dbmodel.Model{mA, mB, mShared} {
		if err := db.Create(m).Error; err != nil {
			t.Fatal(err)
		}
	}
	return epA, epB, epShared, mA
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
	epA, epB, epShared, _ := seed(t, db)
	ctx := context.Background()
	repo := repository.NewEndpointRepository(db)

	// scope=101 看不到 202 的端点
	ep, err := repo.FindByID(ctx, epB.ID, lo.ToPtr(uint(101)))
	if err != nil {
		t.Fatal(err)
	}
	if ep != nil {
		t.Fatal("user 101 must not see user 202's endpoint")
	}

	// scope=101 能看到自己的
	ep, err = repo.FindByID(ctx, epA.ID, lo.ToPtr(uint(101)))
	if err != nil {
		t.Fatalf("own lookup: %v", err)
	}
	if ep == nil {
		t.Fatal("user 101 must see own endpoint")
	}

	// scope=nil（admin）全量可见
	ep, err = repo.FindByID(ctx, epB.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ep == nil {
		t.Fatal("admin scope must see all")
	}

	// scope=0 精确匹配共享池：能看到 user_id=0 的端点，看不到他人的
	ep, err = repo.FindByID(ctx, epShared.ID, lo.ToPtr(uint(0)))
	if err != nil {
		t.Fatal(err)
	}
	if ep == nil {
		t.Fatal("shared-pool scope (0) must match user_id=0 endpoint exactly")
	}
	ep, err = repo.FindByID(ctx, epA.ID, lo.ToPtr(uint(0)))
	if err != nil {
		t.Fatal(err)
	}
	if ep != nil {
		t.Fatal("shared-pool scope (0) must not match user-owned endpoint")
	}

	// 分页过滤
	list, page, err := repo.Paginate(ctx, model.CommonParam{}, lo.ToPtr(uint(101)))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || page.Total != 1 {
		t.Fatalf("scoped paginate: got %d items, total %d", len(list), page.Total)
	}

	// admin 分页全量（3 个端点）
	_, pageAll, err := repo.Paginate(ctx, model.CommonParam{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pageAll.Total != 3 {
		t.Fatalf("admin total = %d, want 3", pageAll.Total)
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
	if err := repo.Delete(ctx, epB.ID, lo.ToPtr(uint(101))); err != nil {
		t.Fatal(err)
	}
	var cnt int64
	db.Model(&dbmodel.Endpoint{}).Where("id = ? AND deleted_at = 0", epB.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatal("cross-user delete must be a no-op")
	}

	// 本人删除有效
	if err := repo.Delete(ctx, epA.ID, lo.ToPtr(uint(101))); err != nil {
		t.Fatal(err)
	}
	db.Model(&dbmodel.Endpoint{}).Where("id = ? AND deleted_at = 0", epA.ID).Count(&cnt)
	if cnt != 0 {
		t.Fatal("owner delete must take effect")
	}

	// FindIDsByScope：scope=nil 全量；scope=0 只返回共享池
	allIDs, err := repo.FindIDsByScope(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(allIDs) != 3 {
		t.Fatalf("admin FindIDsByScope = %d ids, want 3", len(allIDs))
	}
	sharedIDs, err := repo.FindIDsByScope(ctx, lo.ToPtr(uint(0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(sharedIDs) != 1 || sharedIDs[0] != epShared.ID {
		t.Fatalf("shared-pool FindIDsByScope = %v, want [epShared.ID]", sharedIDs)
	}
}

func TestModelRepo_UserIsolation(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}
	_, _, _, mA := seed(t, db)
	ctx := context.Background()
	repo := repository.NewModelRepository(db)

	// FindByAlias 按 userID 过滤（网关解析语义）
	ms, err := repo.FindByAlias(ctx, vo.EndpointAlias("gpt-x"), lo.ToPtr(uint(202)))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 0 {
		t.Fatal("user 202 must not resolve user 101's alias")
	}
	ms, err = repo.FindByAlias(ctx, vo.EndpointAlias("gpt-x"), lo.ToPtr(uint(101)))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("user 101 must resolve own alias, got %d", len(ms))
	}

	// FindByAlias 共享池语义（0）：只返回 user_id=0 的行
	ms, err = repo.FindByAlias(ctx, vo.EndpointAlias("gpt-shared"), lo.ToPtr(uint(0)))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 {
		t.Fatalf("shared-pool alias resolve, got %d", len(ms))
	}

	// FindByID 隔离
	m, err := repo.FindByID(ctx, mA.ID, lo.ToPtr(uint(202)))
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Fatal("user 202 must not see user 101's model")
	}

	// 分页过滤
	list, page, err := repo.Paginate(ctx, model.CommonParam{}, lo.ToPtr(uint(202)))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || page.Total != 1 {
		t.Fatalf("scoped paginate: got %d items, total %d", len(list), page.Total)
	}

	// admin 全量（3 个模型）
	_, pageAll, err := repo.Paginate(ctx, model.CommonParam{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pageAll.Total != 3 {
		t.Fatalf("admin total = %d, want 3", pageAll.Total)
	}

	// 删除越权无效
	if err := repo.Delete(ctx, mA.ID, lo.ToPtr(uint(202))); err != nil {
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
