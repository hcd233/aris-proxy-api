// Package query Identity 域查询处理器
package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	commonenum "github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// UserView 用户详情只读投影
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type UserView struct {
	ID         uint
	Name       string
	Email      string
	Avatar     string
	Permission commonenum.Permission
	CreatedAt  time.Time
	LastLogin  time.Time
}

// GetCurrentUserQuery 查询当前用户命令
//
//	@author centonhuang
//	@update 2026-04-22 17:00:00
type GetCurrentUserQuery struct {
	UserID uint
}

// GetCurrentUserHandler 查询处理器
type GetCurrentUserHandler interface {
	Handle(ctx context.Context, q GetCurrentUserQuery) (*UserView, error)
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
func (h *getCurrentUserHandler) Handle(ctx context.Context, q GetCurrentUserQuery) (*UserView, error) {
	log := logger.WithCtx(ctx)

	user, err := h.repo.FindByID(ctx, q.UserID)
	if err != nil {
		log.Error("[IdentityQuery] FindByID failed", zap.Error(err), zap.Uint("userID", q.UserID))
		return nil, err
	}
	if user == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, "user not found")
	}
	return &UserView{
		ID:         user.AggregateID(),
		Name:       user.Name().String(),
		Email:      user.Email().String(),
		Avatar:     user.Avatar().String(),
		Permission: user.Permission(),
		CreatedAt:  user.CreatedAt(),
		LastLogin:  user.LastLogin(),
	}, nil
}
