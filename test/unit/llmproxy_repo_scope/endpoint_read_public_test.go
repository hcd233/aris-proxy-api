package llmproxy_repo_scope

import (
	"context"
	"slices"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/gorm"
)

// seedReadCatalog 造「自有 alias + 公开端点别名」两套数据。
//
//	own:        101 名下 gpt-x（context=1000）
//	public-EP:  999 名下 gpt-x（context=9999）与 gpt-public（context=8000），端点 visibility=public
func seedReadCatalog(t *testing.T, db *gorm.DB) (publicEndpointID uint) {
	t.Helper()
	epOwn := &dbmodel.Endpoint{UserID: 101, Name: "ep-own", APIKey: "k", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true}
	epPublic := &dbmodel.Endpoint{UserID: 999, Name: "ep-public", APIKey: "k", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true, Visibility: string(enum.EndpointVisibilityPublic)}
	for _, ep := range []*dbmodel.Endpoint{epOwn, epPublic} {
		if err := db.Create(ep).Error; err != nil {
			t.Fatal(err)
		}
	}

	caps := []enum.InputModality{enum.InputModalityText}
	rows := []*dbmodel.Model{
		{UserID: 101, Alias: "gpt-x", ModelID: "gpt-x", UpstreamModel: "up-own", EndpointID: epOwn.ID, ContextLength: 1000, Capabilities: caps},
		{UserID: 999, Alias: "gpt-x", ModelID: "gpt-x", UpstreamModel: "up-public", EndpointID: epPublic.ID, ContextLength: 9999, Capabilities: caps},
		{UserID: 999, Alias: "gpt-public", ModelID: "gpt-public", UpstreamModel: "up-public", EndpointID: epPublic.ID, ContextLength: 8000, Capabilities: caps},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return epPublic.ID
}

func newReadCache(t *testing.T) *cache.ReadCache {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return cache.NewReadCache(rdb)
}

func TestEndpointRead_PublicAliasesVisible(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}
	publicEndpointID := seedReadCatalog(t, db)

	rc := newReadCache(t)
	repo := repository.NewCachedEndpointReadRepository(db, rc)
	ctx := context.Background()

	aliases, err := repo.ListAliases(ctx, 101)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(aliases))
	for _, a := range aliases {
		names = append(names, a.Alias)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"gpt-public", "gpt-x"}) {
		t.Fatalf("aliases = %v, want [gpt-public gpt-x]", names)
	}

	details, err := repo.ListEnabledModelDetails(ctx, 101)
	if err != nil {
		t.Fatal(err)
	}
	ownDetail, found := findDetail(details, "gpt-x")
	if !found {
		t.Fatal("gpt-x detail missing")
	}
	if ownDetail.ContextLength != 1000 {
		t.Fatalf("gpt-x context = %d, want own 1000 (own must win over public)", ownDetail.ContextLength)
	}

	proj, _, err := repo.FindEndpointByAlias(ctx, 101, "gpt-public", nil)
	if err != nil {
		t.Fatal(err)
	}
	if proj == nil || proj.ID != publicEndpointID {
		t.Fatalf("FindEndpointByAlias(gpt-public) = %+v, want endpoint %d", proj, publicEndpointID)
	}

	empty, err := repo.ListAliases(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("userID=0 must fail closed, got %d aliases", len(empty))
	}
}

func findDetail(details []*llmproxy.ModelDetailProjection, alias string) (*llmproxy.ModelDetailProjection, bool) {
	for _, d := range details {
		if d.Alias == alias {
			return d, true
		}
	}
	return nil, false
}

// TestEndpointRead_PublicFlipInvalidatesCache 公开化后普通用户的别名缓存必须立刻失效。
func TestEndpointRead_PublicFlipInvalidatesCache(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}

	ep := &dbmodel.Endpoint{UserID: 999, Name: "ep-flip", APIKey: "k", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true}
	if err := db.Create(ep).Error; err != nil {
		t.Fatal(err)
	}
	caps := []enum.InputModality{enum.InputModalityText}
	row := &dbmodel.Model{UserID: 999, Alias: "gpt-late", ModelID: "gpt-late", UpstreamModel: "up-late", EndpointID: ep.ID, Capabilities: caps}
	if err := db.Create(row).Error; err != nil {
		t.Fatal(err)
	}

	rc := newReadCache(t)
	readRepo := repository.NewCachedEndpointReadRepository(db, rc)
	writeRepo := repository.NewCachedEndpointRepository(db, rc)
	ctx := context.Background()

	before, err := readRepo.ListAliases(ctx, 101)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 0 {
		t.Fatalf("private endpoint must not be visible before flip, got %d aliases", len(before))
	}

	agg, err := writeRepo.FindByID(ctx, ep.ID, nil)
	if err != nil || agg == nil {
		t.Fatalf("find endpoint: %v", err)
	}
	if err := agg.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatal(err)
	}
	if err := writeRepo.Update(ctx, agg); err != nil {
		t.Fatal(err)
	}

	after, err := readRepo.ListAliases(ctx, 101)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0].Alias != "gpt-late" {
		t.Fatalf("alias cache must be invalidated after flip, got %+v", after)
	}
}
