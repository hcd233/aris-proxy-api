// Package query Identity 域查询处理器
package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
)

type listUsersHandler struct {
	repo identity.UserRepository
}

// NewListUsersHandler 构造
//
//	@param repo identity.UserRepository
//	@return ListUsersHandler
//	@author centonhuang
//	@update 2026-08-07 10:00:00
func NewListUsersHandler(repo identity.UserRepository) port.ListUsersHandler {
	return &listUsersHandler{repo: repo}
}

// Handle 执行用户列表查询（管理员视图）
//
//	@receiver h *listUsersHandler
//	@param ctx context.Context
//	@param q ListUsersQuery
//	@return []*UserView
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-08-07 10:00:00
func (h *listUsersHandler) Handle(ctx context.Context, q port.ListUsersQuery) ([]*port.UserView, *model.PageInfo, error) {
	users, pageInfo, err := h.repo.ListUsers(ctx, q.CommonParam, q.Permission)
	if err != nil {
		return nil, nil, err
	}
	views := lo.Map(users, func(u *aggregate.User, _ int) *port.UserView {
		return &port.UserView{
			ID:         u.AggregateID(),
			Name:       u.Name().String(),
			Email:      u.Email().String(),
			Avatar:     u.Avatar().String(),
			Permission: u.Permission(),
			CreatedAt:  u.CreatedAt(),
			LastLogin:  u.LastLogin(),
		}
	})
	return views, pageInfo, nil
}
