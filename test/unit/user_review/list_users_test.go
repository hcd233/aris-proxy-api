package user_review

import (
	"context"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/application/identity/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
)

func newUser(t *testing.T, name, email string, perm enum.Permission) *aggregate.User {
	t.Helper()
	u, err := aggregate.RegisterUser(vo.UserName(name), vo.Email(email), vo.Avatar(""), "github", "bind-"+name, time.Now())
	if err != nil {
		t.Fatalf("register user failed: %v", err)
	}
	u.ChangePermission(perm)
	return u
}

func TestListUsers_PaginateAndFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := newFakeUserRepo(
		newUser(t, "alice", "alice@example.com", enum.PermissionUser),
		newUser(t, "bob", "bob@example.com", enum.PermissionPending),
		newUser(t, "carol", "carol@example.com", enum.PermissionAdmin),
	)
	handler := query.NewListUsersHandler(repo)

	// 全部（默认分页）
	views, pageInfo, err := handler.Handle(ctx, port.ListUsersQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
	})
	if err != nil {
		t.Fatalf("list all failed: %v", err)
	}
	if len(views) != 3 || pageInfo.Total != 3 {
		t.Fatalf("expected 3 users, got %d (total %d)", len(views), pageInfo.Total)
	}

	// 按权限过滤 pending
	views, pageInfo, err = handler.Handle(ctx, port.ListUsersQuery{
		CommonParam: model.CommonParam{PageParam: model.PageParam{Page: 1, PageSize: 10}},
		Permission:  enum.PermissionPending,
	})
	if err != nil {
		t.Fatalf("list pending failed: %v", err)
	}
	if len(views) != 1 || views[0].Name != "bob" || pageInfo.Total != 1 {
		t.Fatalf("expected only bob, got %+v", views)
	}

	// 关键词模糊匹配 email
	views, _, err = handler.Handle(ctx, port.ListUsersQuery{
		CommonParam: model.CommonParam{
			PageParam:  model.PageParam{Page: 1, PageSize: 10},
			QueryParam: model.QueryParam{Query: "carol"},
		},
	})
	if err != nil {
		t.Fatalf("list by keyword failed: %v", err)
	}
	if len(views) != 1 || views[0].Name != "carol" {
		t.Fatalf("expected carol by email, got %+v", views)
	}
}
