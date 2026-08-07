// Package user_review 用户审核闭环的单元测试
package user_review

import (
	"context"
	"sort"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
)

// fakeUserRepo 内存版 UserRepository，支持 Save/FindByID/ListUsers。
type fakeUserRepo struct {
	users []*aggregate.User
	next  uint
}

func newFakeUserRepo(seed ...*aggregate.User) *fakeUserRepo {
	r := &fakeUserRepo{next: 1}
	for _, u := range seed {
		u.SetID(r.next)
		r.next++
		r.users = append(r.users, u)
	}
	return r
}

func (r *fakeUserRepo) Save(ctx context.Context, user *aggregate.User) error {
	if user.AggregateID() == 0 {
		user.SetID(r.next)
		r.next++
		r.users = append(r.users, user)
		return nil
	}
	for i, u := range r.users {
		if u.AggregateID() == user.AggregateID() {
			r.users[i] = user
			return nil
		}
	}
	r.users = append(r.users, user)
	return nil
}

func (r *fakeUserRepo) FindByID(ctx context.Context, id uint) (*aggregate.User, error) {
	for _, u := range r.users {
		if u.AggregateID() == id {
			return u, nil
		}
	}
	return nil, nil
}

func (r *fakeUserRepo) FindByGithubBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	return nil, nil
}

func (r *fakeUserRepo) FindByGoogleBindID(ctx context.Context, bindID string) (*aggregate.User, error) {
	return nil, nil
}

func (r *fakeUserRepo) TouchLastLogin(ctx context.Context, userID uint) error {
	return nil
}

func (r *fakeUserRepo) ListUsers(ctx context.Context, param model.CommonParam, permission enum.Permission) ([]*aggregate.User, *model.PageInfo, error) {
	matches := make([]*aggregate.User, 0, len(r.users))
	for _, u := range r.users {
		if permission != "" && u.Permission() != permission {
			continue
		}
		if param.Query != "" {
			q := strings.ToLower(param.Query)
			if !strings.Contains(strings.ToLower(u.Name().String()), q) &&
				!strings.Contains(strings.ToLower(u.Email().String()), q) {
				continue
			}
		}
		matches = append(matches, u)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].AggregateID() < matches[j].AggregateID() })
	total := int64(len(matches))
	page, pageSize := param.Page, param.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start > len(matches) {
		start = len(matches)
	}
	end := start + pageSize
	if end > len(matches) {
		end = len(matches)
	}
	return matches[start:end], &model.PageInfo{Page: page, PageSize: pageSize, Total: total}, nil
}
