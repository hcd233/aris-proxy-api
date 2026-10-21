package llmproxy_repo_scope

import (
	"context"
	"slices"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
)

func TestFindIDsByScope_IncludePublic(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	if err := db.AutoMigrate(&dbmodel.Endpoint{}, &dbmodel.Model{}); err != nil {
		t.Fatal(err)
	}
	epA, epB, epShared, _ := seed(t, db)
	epPublic := &dbmodel.Endpoint{
		UserID:                      999,
		Name:                        "ep-public",
		APIKey:                      "k",
		OpenaiBaseURL:               "https://o.example.com",
		SupportOpenAIChatCompletion: true,
		Visibility:                  string(enum.EndpointVisibilityPublic),
	}
	if err := db.Create(epPublic).Error; err != nil {
		t.Fatal(err)
	}
	// 已软删的公开端点：OR 分组若写错（deleted_at=0 AND user_id=? OR visibility='public'），
	// 它会被带进普通用户视角——这里守住那个优先级陷阱。
	epDeletedPublic := &dbmodel.Endpoint{
		UserID:                      999,
		Name:                        "ep-deleted-public",
		APIKey:                      "k",
		OpenaiBaseURL:               "https://o.example.com",
		SupportOpenAIChatCompletion: true,
		Visibility:                  string(enum.EndpointVisibilityPublic),
		DeletedAt:                   1,
	}
	if err := db.Create(epDeletedPublic).Error; err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	repo := repository.NewEndpointRepository(db)

	visible, err := repo.FindIDsByScope(ctx, lo.ToPtr(uint(101)), true)
	if err != nil {
		t.Fatal(err)
	}
	assertEndpointIDs(t, visible, epA.ID, epPublic.ID)

	ownOnly, err := repo.FindIDsByScope(ctx, lo.ToPtr(uint(101)), false)
	if err != nil {
		t.Fatal(err)
	}
	assertEndpointIDs(t, ownOnly, epA.ID)

	all, err := repo.FindIDsByScope(ctx, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assertEndpointIDs(t, all, epA.ID, epB.ID, epShared.ID, epPublic.ID)
}

// assertEndpointIDs 无序比较 ID 集合（重复项也会被发现）。
func assertEndpointIDs(t *testing.T, got []uint, want ...uint) {
	t.Helper()
	sortedGot := slices.Clone(got)
	sortedWant := slices.Clone(want)
	slices.Sort(sortedGot)
	slices.Sort(sortedWant)
	if !slices.Equal(sortedGot, sortedWant) {
		t.Fatalf("endpoint ids = %v, want %v", got, want)
	}
}
