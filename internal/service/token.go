// Package service 业务逻辑
//
//	update 2025-01-04 21:13:05
package service

import (
	"context"
	"errors"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/jwt"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TokenService 令牌服务
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type TokenService interface {
	RefreshToken(ctx context.Context, req *dto.RefreshTokenReq) (rsp *dto.RefreshTokenRsp, err error)
}

type tokenService struct {
	userDAO            *dao.UserDAO
	accessTokenSigner  jwt.TokenSigner
	refreshTokenSigner jwt.TokenSigner
}

// NewTokenService 创建令牌服务
//
//	return TokenService
//	author centonhuang
//	update 2025-01-05 21:00:00
func NewTokenService() TokenService {
	return &tokenService{
		userDAO:            dao.GetUserDAO(),
		accessTokenSigner:  jwt.GetAccessTokenSigner(),
		refreshTokenSigner: jwt.GetRefreshTokenSigner(),
	}
}

// RefreshToken 刷新令牌
//
//	receiver s *tokenService
//	param ctx context.Context
//	param req *dto.RefreshTokenRequest
//	return rsp *dto.RefreshTokenResponse
//	return err error
//	author centonhuang
//	update 2025-01-05 21:00:00
func (s *tokenService) RefreshToken(ctx context.Context, req *dto.RefreshTokenReq) (*dto.RefreshTokenRsp, error) {
	rsp := &dto.RefreshTokenRsp{}

	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	userID, err := s.refreshTokenSigner.DecodeToken(req.Body.RefreshToken)
	if err != nil {
		logger.Error("[TokenService] failed to decode refresh token", zap.String("refreshToken", req.Body.RefreshToken), zap.Error(err))
		rsp.Error = constant.ErrUnauthorized
		return rsp, nil
	}

	_, err = s.userDAO.Get(db, &model.User{ID: userID}, []string{"id"})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.Error("[TokenService] user not found", zap.Uint("userID", userID))
			rsp.Error = constant.ErrDataNotExists
			return rsp, nil
		}
		logger.Error("[TokenService] failed to get user by id", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	accessToken, err := s.accessTokenSigner.EncodeToken(userID)
	if err != nil {
		logger.Error("[TokenService] failed to encode access token", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	refreshToken, err := s.refreshTokenSigner.EncodeToken(userID)
	if err != nil {
		logger.Error("[TokenService] failed to encode refresh token", zap.Error(err))
		rsp.Error = constant.ErrInternalError
		return rsp, nil
	}

	logger.Info("[TokenService] refresh token success", zap.Uint("userID", userID))

	rsp.AccessToken = accessToken
	rsp.RefreshToken = refreshToken

	return rsp, nil
}
