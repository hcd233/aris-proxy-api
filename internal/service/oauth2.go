package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	objdao "github.com/hcd233/aris-proxy-api/internal/infrastructure/storage/obj_dao"
	"github.com/hcd233/aris-proxy-api/internal/jwt"
	"github.com/hcd233/aris-proxy-api/internal/logger"

	"github.com/hcd233/aris-proxy-api/internal/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Oauth2Service OAuth2服务接口
//
//	author centonhuang
//	update 2025-01-05 21:00:00
type Oauth2Service interface {
	Login(ctx context.Context, req *dto.LoginReq) (rsp *dto.LoginResp, err error)
	Callback(ctx context.Context, req *dto.CallbackReq) (rsp *dto.CallbackRsp, err error)
}

// oauth2Service OAuth2服务基础实现
type oauth2Service struct {
	platform           oauth2.Platform
	userDAO            *dao.UserDAO
	audioObjDAO        objdao.ObjDAO
	accessTokenSigner  jwt.TokenSigner
	refreshTokenSigner jwt.TokenSigner
}

// NewGithubOauth2Service 创建Github OAuth2服务
func NewGithubOauth2Service() Oauth2Service {
	return &oauth2Service{
		platform:           oauth2.NewGithubPlatform(),
		userDAO:            dao.GetUserDAO(),
		audioObjDAO:        objdao.GetAudioObjDAO(),
		accessTokenSigner:  jwt.GetAccessTokenSigner(),
		refreshTokenSigner: jwt.GetRefreshTokenSigner(),
	}
}

// NewGoogleOauth2Service 创建Google OAuth2服务
func NewGoogleOauth2Service() Oauth2Service {
	return &oauth2Service{
		platform:           oauth2.NewGooglePlatform(),
		userDAO:            dao.GetUserDAO(),
		audioObjDAO:        objdao.GetAudioObjDAO(),
		accessTokenSigner:  jwt.GetAccessTokenSigner(),
		refreshTokenSigner: jwt.GetRefreshTokenSigner(),
	}
}

// Login 登录
//
//	receiver s *oauth2Service
//	param ctx context.Context
//	param req *dto.LoginRequest
//	return rsp *dto.LoginResponse
//	return err error
//	author centonhuang
//	update 2025-01-05 21:00:00
func (s *oauth2Service) Login(ctx context.Context, req *dto.LoginReq) (rsp *dto.LoginResp, err error) {
	rsp = &dto.LoginResp{}

	logger := logger.WithCtx(ctx)

	url := s.platform.GetAuthURL()
	rsp.RedirectURL = url

	logger.Info("[Oauth2Service] login", zap.String("platform", req.Platform), zap.String("redirectURL", url))

	return rsp, nil
}

// Callback 回调
//
//	receiver s *oauth2Service
//	param ctx context.Context
//	param req *dto.CallbackRequest
//	return rsp *dto.CallbackResponse
//	return err error
//	author centonhuang
//	update 2025-01-05 21:00:00
func (s *oauth2Service) Callback(ctx context.Context, req *dto.CallbackReq) (*dto.CallbackRsp, error) {
	rsp := &dto.CallbackRsp{}

	logger := logger.WithCtx(ctx)
	db := database.GetDBInstance(ctx)

	if req.Body.State != config.Oauth2StateString {
		logger.Error("[Oauth2Service] Invalid state",
			zap.String("platform", req.Body.Platform),
			zap.String("state", req.Body.State),
			zap.String("expectedState", config.Oauth2StateString))
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return rsp, nil
	}

	logger.Info("[Oauth2Service] Exchanging token",
		zap.String("platform", req.Body.Platform),
		zap.String("code", req.Body.Code),
		zap.String("state", req.Body.State))

	token, err := s.platform.ExchangeToken(ctx, req.Body.Code)
	if err != nil {
		logger.Error("[Oauth2Service] Failed to exchange token",
			zap.String("platform", req.Body.Platform),
			zap.String("code", req.Body.Code),
			zap.Error(err))
		rsp.Error = ierr.ErrOAuth2Exchange.BizError()
		return rsp, nil
	}

	logger.Info("[Oauth2Service] Token exchange successful",
		zap.String("platform", req.Body.Platform),
		zap.String("tokenType", token.TokenType),
		zap.Bool("valid", token.Valid()))

	userInfo, err := s.platform.GetUserInfo(ctx, token)
	if err != nil {
		logger.Error("[Oauth2Service] Failed to get user info",
			zap.String("platform", req.Body.Platform),
			zap.Error(err))
		rsp.Error = ierr.ErrOAuth2UserInfo.BizError()
		return rsp, nil
	}

	thirdPartyID := userInfo.GetID()
	userName, email, avatar := userInfo.GetName(), userInfo.GetEmail(), userInfo.GetAvatar()

	var user *model.User
	switch req.Body.Platform {
	case enum.Oauth2PlatformGithub:
		user, err = s.userDAO.Get(db, &model.User{GithubBindID: thirdPartyID}, []string{"id"})
	case enum.Oauth2PlatformGoogle:
		user, err = s.userDAO.Get(db, &model.User{GoogleBindID: thirdPartyID}, []string{"id"})
	default:
		logger.Error("[Oauth2Service] Invalid platform", zap.String("platform", req.Body.Platform))
		rsp.Error = ierr.ErrBadRequest.BizError()
		return rsp, nil
	}

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("[Oauth2Service] Failed to get user by third party bind id",
			zap.String("platform", req.Body.Platform),
			zap.String("thirdPartyID", thirdPartyID),
			zap.Error(err))
		rsp.Error = ierr.ErrDBQuery.BizError()
		return rsp, nil
	}

	if user.ID != 0 {
		// 更新已存在用户的登录时间
		if err := s.userDAO.Update(db, user, map[string]interface{}{
			"last_login": time.Now().UTC(),
		}); err != nil {
			logger.Error("[Oauth2Service] Failed to update user login time",
				zap.String("platform", req.Body.Platform),
				zap.Error(err))
			rsp.Error = ierr.ErrDBUpdate.BizError()
			return rsp, nil
		}
	} else {
		// 创建新用户
		if validateErr := util.ValidateUserName(userName); validateErr != nil {
			userName = "ArisUser" + strconv.FormatInt(time.Now().UTC().Unix(), 10)
		}
		user = &model.User{
			Name:       userName,
			Email:      email,
			Avatar:     avatar,
			Permission: enum.PermissionPending,
			LastLogin:  time.Now().UTC(),
		}

		switch req.Body.Platform {
		case enum.Oauth2PlatformGithub:
			user.GithubBindID = thirdPartyID
		case enum.Oauth2PlatformGoogle:
			user.GoogleBindID = thirdPartyID
		}

		if err := s.userDAO.Create(db, user); err != nil {
			logger.Error("[Oauth2Service] Failed to create user",
				zap.String("platform", req.Body.Platform),
				zap.String("userName", userName),
				zap.Error(err))
			rsp.Error = ierr.ErrDBCreate.BizError()
			return rsp, nil
		}

		_, err = s.audioObjDAO.CreateDir(ctx, user.ID)
		if err != nil {
			logger.Error("[Oauth2Service] Failed to create audio dir",
				zap.String("platform", req.Body.Platform),
				zap.Error(err))
			rsp.Error = ierr.ErrObjStorage.BizError()
			return rsp, nil
		}
		logger.Info("[Oauth2Service] Audio dir created", zap.String("platform", req.Body.Platform))

	}

	accessToken, err := s.accessTokenSigner.EncodeToken(user.ID)
	if err != nil {
		logger.Error("[Oauth2Service] Failed to encode access token",
			zap.String("platform", req.Body.Platform),
			zap.Error(err))
		rsp.Error = ierr.ErrJWTEncode.BizError()
		return rsp, nil
	}

	refreshToken, err := s.refreshTokenSigner.EncodeToken(user.ID)
	if err != nil {
		logger.Error("[Oauth2Service] Failed to encode refresh token",
			zap.String("platform", req.Body.Platform),
			zap.Error(err))
		rsp.Error = ierr.ErrJWTEncode.BizError()
		return rsp, nil
	}

	logger.Info("[Oauth2Service] Callback success",
		zap.String("platform", req.Body.Platform),
		zap.Uint("userID", user.ID))

	rsp.AccessToken = accessToken
	rsp.RefreshToken = refreshToken

	return rsp, nil
}
