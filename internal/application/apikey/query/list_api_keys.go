// Package query APIKey 域只读查询处理器
package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	commonenum "github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// APIKeyView 只读 API Key 投影（列表响应）
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type APIKeyView struct {
	ID        uint
	Name      string
	MaskedKey string
	CreatedAt time.Time
}

// ListAPIKeysQuery 列出 API Keys 查询命令
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type ListAPIKeysQuery struct {
	// RequesterID 查询者用户 ID
	RequesterID uint
	// RequesterPermission 查询者权限（admin 可见全量，其他只见自己）
	RequesterPermission commonenum.Permission
}

// ListAPIKeysHandler 查询处理器
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type ListAPIKeysHandler interface {
	Handle(ctx context.Context, q ListAPIKeysQuery) ([]*APIKeyView, error)
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
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (h *listAPIKeysHandler) Handle(ctx context.Context, q ListAPIKeysQuery) ([]*APIKeyView, error) {
	log := logger.WithCtx(ctx)

	var (
		keys []*aggregate.ProxyAPIKey
		err  error
	)
	if q.RequesterPermission == commonenum.PermissionAdmin {
		keys, err = h.repo.ListAll(ctx)
	} else {
		keys, err = h.repo.ListByUser(ctx, q.RequesterID)
	}
	if err != nil {
		log.Error("[APIKeyQuery] List api keys failed", zap.Error(err))
		return nil, err
	}

	views := make([]*APIKeyView, 0, len(keys))
	for _, k := range keys {
		views = append(views, &APIKeyView{
			ID:        k.AggregateID(),
			Name:      k.Name().String(),
			MaskedKey: k.Secret().Masked(),
			CreatedAt: k.CreatedAt(),
		})
	}

	log.Info("[APIKeyQuery] List api keys",
		zap.Uint("requesterID", q.RequesterID),
		zap.Bool("isAdmin", q.RequesterPermission == commonenum.PermissionAdmin),
		zap.Int("count", len(views)))
	return views, nil
}
