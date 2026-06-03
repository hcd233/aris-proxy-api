// Package query Identity 域查询处理器
package query

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/identity/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// GetCurrentUserHandler 查询处理器
type GetCurrentUserHandler interface {
	Handle(ctx context.Context, q port.GetCurrentUserQuery) (*port.UserView, error)
}

type getCurrentUserHandler struct {
	repo identity.UserRepository
}

// NewGetCurrentUserHandler 构造
//
//	@param repo identity.UserRepository
//	@return GetCurrentUserHandler
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewGetCurrentUserHandler(repo identity.UserRepository) GetCurrentUserHandler {
	return &getCurrentUserHandler{repo: repo}
}

// Handle 执行查询
//
//	@receiver h *getCurrentUserHandler
//	@param ctx context.Context
//	@param q GetCurrentUserQuery
//	@return *UserView
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (h *getCurrentUserHandler) Handle(ctx context.Context, q port.GetCurrentUserQuery) (*port.UserView, error) {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, q.UserID)
	if err != nil {
		log.Error("[IdentityQuery] FindByID failed", zap.Error(err), zap.Uint("userID", q.UserID))
		return nil, err
	}
	if user == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	return &port.UserView{
		ID:         user.AggregateID(),
		Name:       user.Name().String(),
		Email:      user.Email().String(),
		Avatar:     user.Avatar().String(),
		Permission: user.Permission(),
		CreatedAt:  user.CreatedAt(),
		LastLogin:  user.LastLogin(),
	}, nil
}
