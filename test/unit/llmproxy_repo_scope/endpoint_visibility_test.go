package llmproxy_repo_scope

import (
	"context"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

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
