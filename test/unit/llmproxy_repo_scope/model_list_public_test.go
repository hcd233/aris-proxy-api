package llmproxy_repo_scope

import (
	"context"
	"slices"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

func TestPaginateModels_IncludePublic(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}

	eps := []*dbmodel.Endpoint{
		{UserID: 101, Name: "ep-own", APIKey: "k", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true},
		{UserID: 202, Name: "ep-other", APIKey: "k", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true},
		{UserID: 999, Name: "ep-public", APIKey: "k", OpenaiBaseURL: "https://o.example.com", SupportOpenAIChatCompletion: true, Visibility: string(enum.EndpointVisibilityPublic)},
	}
	for _, ep := range eps {
		if err := db.Create(ep).Error; err != nil {
			t.Fatal(err)
		}
	}

	caps := []enum.InputModality{enum.InputModalityText}
	rows := []*dbmodel.Model{
		{UserID: 101, Alias: "own-model", ModelID: "own-model", UpstreamModel: "up-own", EndpointID: eps[0].ID, Capabilities: caps},
		{UserID: 202, Alias: "other-model", ModelID: "other-model", UpstreamModel: "up-other", EndpointID: eps[1].ID, Capabilities: caps},
		{UserID: 999, Alias: "public-model", ModelID: "public-model", UpstreamModel: "up-public", EndpointID: eps[2].ID, Capabilities: caps},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	repo := repository.NewModelRepository(db)
	param := model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 20}}
	scope := lo.ToPtr(uint(101))

	withPublic, _, err := repo.PaginateWithFilter(context.Background(), param, llmproxy.ModelListFilter{IncludePublic: true}, scope)
	if err != nil {
		t.Fatal(err)
	}
	assertModelAliases(t, withPublic, "own-model", "public-model")

	ownOnly, _, err := repo.PaginateWithFilter(context.Background(), param, llmproxy.ModelListFilter{IncludePublic: false}, scope)
	if err != nil {
		t.Fatal(err)
	}
	assertModelAliases(t, ownOnly, "own-model")
}

// assertModelAliases 无序比较别名集合。
func assertModelAliases(t *testing.T, models []*aggregate.Model, want ...string) {
	t.Helper()
	got := lo.Map(models, func(m *aggregate.Model, _ int) string { return m.Alias().String() })
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("model aliases = %v, want %v", got, want)
	}
}
