package service

import (
	"context"
	"errors"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserService 用户服务
//
//	author centonhuang
//	update 2025-01-04 21:04:00
type UserService interface {
	GetCurUser(ctx context.Context, req *dto.EmptyReq) (rsp *dto.GetCurUserRsp, err error)
	UpdateUser(ctx context.Context, req *dto.UpdateUserReq) (rsp *dto.EmptyRsp, err error)
}

type userService struct {
	userDAO *dao.UserDAO
}

// NewUserService 创建用户服务
//
//	return UserService
//	author centonhuang
//	update 2025-01-04 21:03:45
func NewUserService() UserService {
	return &userService{
		userDAO: dao.GetUserDAO(),
	}
}

// GetCurUser 获取当前用户信息
//
//	@receiver s *userService
//	@param ctx context.Context
//	@param _ *dto.EmptyReq
//	@return *dto.GetCurUserRsp
//	@return error
//	@author centonhuang
//	@update 2025-11-11 04:59:13
func (s *userService) GetCurUser(ctx context.Context, _ *dto.EmptyReq) (*dto.GetCurUserRsp, error) {
	rsp := &dto.GetCurUserRsp{}

	userID := ctx.Value(constant.CtxKeyUserID).(uint)

	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	user, err := s.userDAO.Get(db, &model.User{ID: userID}, []string{"id", "name", "email", "avatar", "created_at", "last_login", "permission"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("[UserService] user not found")
			rsp.Error = constant.ErrDataNotExists
			return rsp, nil
		}
		logger.Error("[UserService] failed to get user by id", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	rsp.User = &dto.DetailedUser{
		ID:         user.ID,
		CreatedAt:  user.CreatedAt.Format(time.DateTime),
		LastLogin:  user.LastLogin.Format(time.DateTime),
		Permission: string(user.Permission),
		User: dto.User{
			Name:   user.Name,
			Email:  user.Email,
			Avatar: user.Avatar,
		},
	}

	logger.Info("[UserService] get cur user info",
		zap.String("email", user.Email),
		zap.String("avatar", user.Avatar),
		zap.Time("createdAt", user.CreatedAt),
		zap.Time("lastLogin", user.LastLogin),
		zap.String("permission", string(user.Permission)))

	return rsp, nil
}

func (s *userService) UpdateUser(ctx context.Context, req *dto.UpdateUserReq) (*dto.EmptyRsp, error) {
	rsp := &dto.EmptyRsp{}

	userID := ctx.Value(constant.CtxKeyUserID).(uint)

	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	if err := s.userDAO.Update(db, &model.User{ID: userID}, map[string]interface{}{
		"name":   req.Body.User.Name,
		"email":  req.Body.User.Email,
		"avatar": req.Body.User.Avatar,
	}); err != nil {
		logger.Error("[UserService] Failed to update user", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	return rsp, nil
}
