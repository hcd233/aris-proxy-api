// Package query APIKey 域只读查询处理器
package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/apikey/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityaggregate "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type listAPIKeysHandler struct {
	repo     apikey.APIKeyRepository
	userRepo identity.UserRepository
}

// NewListAPIKeysHandler 构造查询处理器
//
//	@param repo apikey.APIKeyRepository
//	@return ListAPIKeysHandler
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewListAPIKeysHandler(repo apikey.APIKeyRepository, userRepo identity.UserRepository) port.ListAPIKeysHandler {
	return &listAPIKeysHandler{repo: repo, userRepo: userRepo}
}

// Handle 执行列表查询
//
//	@receiver h *listAPIKeysHandler
//	@param ctx context.Context
//	@param q ListAPIKeysQuery
//	@return []*APIKeyView
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-05-27 10:00:00
func (h *listAPIKeysHandler) Handle(ctx context.Context, q port.ListAPIKeysQuery) ([]*port.APIKeyView, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	var (
		keys     []*aggregate.ProxyAPIKey
		pageInfo *model.PageInfo
		err      error
	)
	if q.RequesterPermission == enum.PermissionAdmin {
		keys, pageInfo, err = h.repo.PaginateAll(ctx, q.CommonParam)
	} else {
		keys, pageInfo, err = h.repo.PaginateByUser(ctx, q.RequesterID, q.CommonParam)
	}
	if err != nil {
		log.Error("[APIKeyQuery] List api keys failed", zap.Error(err))
		return nil, nil, err
	}

	usersByID, err := h.loadUsers(ctx, keys)
	if err != nil {
		log.Error("[APIKeyQuery] Load users failed", zap.Error(err))
		return nil, nil, err
	}

	views := lo.Map(keys, func(k *aggregate.ProxyAPIKey, _ int) *port.APIKeyView {
		return &port.APIKeyView{
			ID:        k.AggregateID(),
			Name:      k.Name().String(),
			MaskedKey: k.Secret().Masked(),
			CreatedAt: k.CreatedAt(),
			User:      toUserView(usersByID[k.UserID()]),
		}
	})

	log.Info("[APIKeyQuery] List api keys",
		zap.Uint("requesterID", q.RequesterID),
		zap.Bool("isAdmin", q.RequesterPermission == enum.PermissionAdmin),
		zap.Int("count", len(views)))
	return views, pageInfo, nil
}

// loadUsers 一次性拉取本页所有 key 关联的 user，避免 N+1；过滤 legacy key 的 UserID=0。
func (h *listAPIKeysHandler) loadUsers(ctx context.Context, keys []*aggregate.ProxyAPIKey) (map[uint]*identityaggregate.User, error) {
	ids := lo.Uniq(lo.Map(keys, func(k *aggregate.ProxyAPIKey, _ int) uint { return k.UserID() }))
	ids = lo.Filter(ids, func(id uint, _ int) bool { return id != 0 })
	return h.userRepo.BatchFindByIDs(ctx, ids)
}

// toUserView 将 user 聚合映射为嵌套视图；nil 聚合返回 nil。
func toUserView(u *identityaggregate.User) *port.UserView {
	if u == nil {
		return nil
	}
	return &port.UserView{
		ID:     u.AggregateID(),
		Name:   u.Name().String(),
		Avatar: u.Avatar().String(),
	}
}
