// Package model_list_repo 验证模型列表分页的筛选、排序白名单与租户隔离。
package model_list_repo

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"),
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
		{UserID: 101, Alias: "a-enabled", ModelID: "a", UpstreamModel: "up-a", EndpointID: 1,
			Enabled: true, ContextLength: 1000, MaxOutputTokens: 100, Capabilities: text},
		{UserID: 101, Alias: "b-disabled", ModelID: "b", UpstreamModel: "up-b", EndpointID: 2,
			Enabled: false, ContextLength: 2000, MaxOutputTokens: 200, Capabilities: both},
		{UserID: 202, Alias: "c-other", ModelID: "c", UpstreamModel: "up-c", EndpointID: 3,
			Enabled: true, ContextLength: 3000, MaxOutputTokens: 300, Capabilities: text},
		{UserID: 0, Alias: "d-shared", ModelID: "d", UpstreamModel: "up-d", EndpointID: 4,
			Enabled: true, ContextLength: 4000, MaxOutputTokens: 400, Capabilities: text},
	}
	for _, r := range rows {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// GORM 对带 default 标签的字段：零值会被跳过 INSERT、由 DB 默认值填充。
	// enabled 列 default:true，故 Enabled:false 必须显式 UPDATE 才落得进去。
	if err := db.Model(&dbmodel.Model{}).
		Where(constant.FieldAlias, "b-disabled").
		Update(constant.FieldEnabled, false).Error; err != nil {
		t.Fatalf("disable b-disabled: %v", err)
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
	ctx := t.Context()

	// admin(nil) 见全部 4 条
	_, page, err := repo.PaginateWithFilter(ctx, param, llmproxy.ModelListFilter{}, nil)
	if err != nil {
		t.Fatalf("admin paginate: %v", err)
	}
	if page.Total != 4 {
		t.Errorf("admin total: want 4, got %d", page.Total)
	}

	// scope=101 只见自己的 2 条，绝不含 202/共享池
	mine, minePage, err := repo.PaginateWithFilter(ctx, param, llmproxy.ModelListFilter{}, lo.ToPtr(uint(101)))
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
	ctx := t.Context()

	// status=disabled 只返回停用的 b-disabled
	got, _, err := repo.PaginateWithFilter(ctx, param, llmproxy.ModelListFilter{Status: "disabled"}, scope)
	if err != nil {
		t.Fatalf("status filter: %v", err)
	}
	if len(got) != 1 || got[0].Alias().String() != "b-disabled" {
		t.Errorf("status=disabled: want [b-disabled], got %v", aliasesOf(got))
	}

	// capability=image 只返回含 image 的 b-disabled（text 模型不得命中）
	got, _, err = repo.PaginateWithFilter(ctx, param,
		llmproxy.ModelListFilter{Capability: enum.InputModalityImage}, scope)
	if err != nil {
		t.Fatalf("capability filter: %v", err)
	}
	if len(got) != 1 || got[0].Alias().String() != "b-disabled" {
		t.Errorf("capability=image: want [b-disabled], got %v", aliasesOf(got))
	}

	// capability 未知值视为不过滤（防前端拼错导致空白页）
	got, _, err = repo.PaginateWithFilter(ctx, param, llmproxy.ModelListFilter{Capability: "audio"}, scope)
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
	ctx := t.Context()

	// 错开 scope 内两条的 created_at，使"回退保留方向"可断言（seed 默认同刻写入）
	base := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	for alias, offset := range map[string]time.Duration{
		"a-enabled":  0,
		"b-disabled": time.Minute,
	} {
		if err := db.Model(&dbmodel.Model{}).Where(constant.FieldAlias, alias).
			Update(constant.FieldCreatedAt, base.Add(offset)).Error; err != nil {
			t.Fatalf("stagger %s: %v", alias, err)
		}
	}

	// 非白名单字段（含合法字符集但敏感的 api_key）必须回退默认排序列，不得进 ORDER BY；
	// 回退只换列、保留调用方排序方向
	param := model.CommonParam{
		PageParam: model.PageParam{Page: 1, PageSize: 50},
		SortParam: model.SortParam{Sort: enum.SortAsc, SortField: "api_key"},
	}
	got, _, err := repo.PaginateWithFilter(ctx, param, llmproxy.ModelListFilter{}, scope)
	if err != nil {
		t.Fatalf("illegal sort field must not error: %v", err)
	}
	if len(got) != 2 || got[0].Alias().String() != "a-enabled" {
		t.Errorf("fallback keeps asc: want [a-enabled b-disabled], got %v", aliasesOf(got))
	}

	param.Sort = enum.SortDesc
	got, _, err = repo.PaginateWithFilter(ctx, param, llmproxy.ModelListFilter{}, scope)
	if err != nil {
		t.Fatalf("illegal sort field desc: %v", err)
	}
	if len(got) != 2 || got[0].Alias().String() != "b-disabled" {
		t.Errorf("fallback keeps desc: want [b-disabled a-enabled], got %v", aliasesOf(got))
	}

	// 白名单字段正常生效：alias 升序
	param.SortField = "alias"
	param.Sort = enum.SortAsc
	got, _, err = repo.PaginateWithFilter(ctx, param, llmproxy.ModelListFilter{}, scope)
	if err != nil {
		t.Fatalf("alias sort: %v", err)
	}
	if len(got) != 2 || got[0].Alias().String() != "a-enabled" {
		t.Errorf("alias asc: want a-enabled first, got %v", aliasesOf(got))
	}
}

func TestPaginateWithFilter_EndpointFilter(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	seedModels(t, db)
	repo := repository.NewModelRepository(db)
	param := model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 50}}

	got, _, err := repo.PaginateWithFilter(t.Context(), param,
		llmproxy.ModelListFilter{EndpointID: 3}, nil)
	if err != nil {
		t.Fatalf("endpoint filter: %v", err)
	}
	if len(got) != 1 || got[0].Alias().String() != "c-other" {
		t.Errorf("endpointID=3: want [c-other], got %v", aliasesOf(got))
	}
}
