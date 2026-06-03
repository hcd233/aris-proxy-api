// Package query APIKey 域只读查询处理器
package query

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/apikey/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// ListAPIKeysHandler 查询处理器
//
//	@author centonhuang
//	@update 2026-05-27 10:00:00
type ListAPIKeysHandler interface {
	Handle(ctx context.Context, q port.ListAPIKeysQuery) ([]*port.APIKeyView, *model.PageInfo, error)
}

type listAPIKeysHandler struct {
	repo apikey.APIKeyRepository
}

// NewListAPIKeysHandler 构造查询处理器
//
//	@param repo apikey.APIKeyRepository
//	@return ListAPIKeysHandler
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewListAPIKeysHandler(repo apikey.APIKeyRepository) ListAPIKeysHandler {
	return &listAPIKeysHandler{repo: repo}
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

	views := make([]*port.APIKeyView, 0, len(keys))
	for _, k := range keys {
		views = append(views, &port.APIKeyView{
			ID:        k.AggregateID(),
			Name:      k.Name().String(),
			MaskedKey: k.Secret().Masked(),
			CreatedAt: k.CreatedAt(),
		})
	}

	log.Info("[APIKeyQuery] List api keys",
		zap.Uint("requesterID", q.RequesterID),
		zap.Bool("isAdmin", q.RequesterPermission == enum.PermissionAdmin),
		zap.Int("count", len(views)))
	return views, pageInfo, nil
}
