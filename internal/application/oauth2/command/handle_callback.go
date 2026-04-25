// Package command OAuth2 域命令处理器
//
// 负责 OAuth2 登录发起与回调处理。由于涉及跨域协作（identity + objstorage +
// jwt），此处通过依赖注入汇聚各域能力，不新建 oauth2 聚合根（登录行为本身
// 是跨域流程而非单一聚合状态变更）。
package command

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"
	xoauth2 "golang.org/x/oauth2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityaggregate "github.com/hcd233/aris-proxy-api/internal/domain/identity/aggregate"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	identityvo "github.com/hcd233/aris-proxy-api/internal/domain/identity/vo"
	oauth2service "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/service"
	oauth2vo "github.com/hcd233/aris-proxy-api/internal/domain/oauth2/vo"
	infraoauth2 "github.com/hcd233/aris-proxy-api/internal/infrastructure/oauth2"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// ============================================================
// InitiateLoginCommand 发起 OAuth 登录：生成 state + 构造授权 URL
// ============================================================

// InitiateLoginCommand 登录发起命令
//
//	@author centonhuang
//	@update 2026-04-22 20:30:00
type InitiateLoginCommand struct {
	Platform string
}

// InitiateLoginResult 登录发起结果
type InitiateLoginResult struct {
	RedirectURL string
}

// InitiateLoginHandler 登录发起命令处理器
type InitiateLoginHandler interface {
	Handle(ctx context.Context, cmd InitiateLoginCommand) (*InitiateLoginResult, error)
}

type initiateLoginHandler struct {
	platforms map[string]oauth2service.Platform
}

// NewInitiateLoginHandler 构造发起登录处理器
//
//	@param platforms map[string]oauth2service.Platform 按平台名索引的策略实例（github / google）
//	@return InitiateLoginHandler
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func NewInitiateLoginHandler(platforms map[string]oauth2service.Platform) InitiateLoginHandler {
	return &initiateLoginHandler{platforms: platforms}
}

// Handle 执行登录发起
//
//	@receiver h *initiateLoginHandler
//	@param ctx context.Context
//	@param cmd InitiateLoginCommand
//	@return *InitiateLoginResult
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func (h *initiateLoginHandler) Handle(ctx context.Context, cmd InitiateLoginCommand) (*InitiateLoginResult, error) {
	log := logger.WithCtx(ctx)

	platform, ok := h.platforms[cmd.Platform]
	if !ok {
		log.Warn("[OAuth2Command] Invalid platform on initiate", zap.String("platform", cmd.Platform))
		return nil, ierr.New(ierr.ErrBadRequest, "invalid oauth platform")
	}

	state, err := infraoauth2.GenerateOAuth2State()
	if err != nil {
		log.Error("[OAuth2Command] Failed to generate state", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrInternal, err, "generate oauth state")
	}

	url := platform.GetAuthURLWithState(state)

	log.Info("[OAuth2Command] Initiate login",
		zap.String("platform", cmd.Platform),
		zap.String("redirectURL", url))
	return &InitiateLoginResult{RedirectURL: url}, nil
}

// ============================================================
// HandleCallbackCommand OAuth 回调处理：验证 state → 交换 token
//   → 获取用户信息 → 查/建用户 → 签发 token pair
// ============================================================

// HandleCallbackCommand 回调处理命令
//
//	@author centonhuang
//	@update 2026-04-22 20:30:00
type HandleCallbackCommand struct {
	Platform string
	Code     string
	State    string
}

// HandleCallbackResult 回调处理结果（供 handler 写响应）
//
//	@author centonhuang
//	@update 2026-04-22 20:30:00
type HandleCallbackResult struct {
	TokenPair *identityvo.TokenPair
	UserID    uint
	IsNewUser bool
}

// ObjectStorageDirCreator 对象存储目录创建器（跨域适配接口）
//
// 由 application/oauth2 负责注入（内部实现由 infrastructure/storage/obj_dao 适配）。
// 返回值忽略，仅关注是否成功创建。
//
//	@author centonhuang
//	@update 2026-04-22 20:30:00
type ObjectStorageDirCreator interface {
	CreateDir(ctx context.Context, userID uint) error
}

// HandleCallbackHandler 回调命令处理器
type HandleCallbackHandler interface {
	Handle(ctx context.Context, cmd HandleCallbackCommand) (*HandleCallbackResult, error)
}

type handleCallbackHandler struct {
	platforms      map[string]oauth2service.Platform
	userRepo       identity.UserRepository
	accessSigner   identityservice.TokenSigner
	refreshSigner  identityservice.TokenSigner
	objStorageDirC ObjectStorageDirCreator
}

// NewHandleCallbackHandler 构造回调处理器
//
//	@param platforms map[string]oauth2service.Platform
//	@param userRepo identity.UserRepository
//	@param accessSigner identityservice.TokenSigner
//	@param refreshSigner identityservice.TokenSigner
//	@param objStorageDirC ObjectStorageDirCreator 可选；nil 时跳过存储目录创建
//	@return HandleCallbackHandler
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func NewHandleCallbackHandler(
	platforms map[string]oauth2service.Platform,
	userRepo identity.UserRepository,
	accessSigner, refreshSigner identityservice.TokenSigner,
	objStorageDirC ObjectStorageDirCreator,
) HandleCallbackHandler {
	return &handleCallbackHandler{
		platforms:      platforms,
		userRepo:       userRepo,
		accessSigner:   accessSigner,
		refreshSigner:  refreshSigner,
		objStorageDirC: objStorageDirC,
	}
}

// Handle 执行回调（严格对齐原 service.oauth2Service.Callback 行为）
//
//	@receiver h *handleCallbackHandler
//	@param ctx context.Context
//	@param cmd HandleCallbackCommand
//	@return *HandleCallbackResult
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func (h *handleCallbackHandler) Handle(ctx context.Context, cmd HandleCallbackCommand) (*HandleCallbackResult, error) {
	log := logger.WithCtx(ctx)

	platform, err := h.validateStateAndPlatform(cmd.State, cmd.Platform)
	if err != nil {
		return nil, err
	}

	userInfo, err := h.exchangeAndFetchUser(ctx, platform, cmd.Code)
	if err != nil {
		return nil, err
	}

	userID, isNewUser, err := h.resolveUser(ctx, cmd.Platform, userInfo)
	if err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := h.signTokenPair(ctx, userID)
	if err != nil {
		return nil, err
	}

	log.Info("[OAuth2Command] Callback success",
		zap.String("platform", cmd.Platform),
		zap.Uint("userID", userID),
		zap.Bool("isNewUser", isNewUser))

	tp := identityvo.NewTokenPair(accessToken, refreshToken)
	return &HandleCallbackResult{
		TokenPair: &tp,
		UserID:    userID,
		IsNewUser: isNewUser,
	}, nil
}

// validateStateAndPlatform 验证 OAuth state 并获取平台策略实例
//
//	@receiver h *handleCallbackHandler
//	@param state string
//	@param platform string
//	@return oauth2service.Platform
//	@return error
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func (h *handleCallbackHandler) validateStateAndPlatform(state, platform string) (oauth2service.Platform, error) {
	log := logger.WithCtx(context.Background())

	if !infraoauth2.VerifyOAuth2State(state) {
		log.Error("[OAuth2Command] Invalid or expired state",
			zap.String("platform", platform),
			zap.String("state", state))
		return nil, ierr.New(ierr.ErrUnauthorized, "invalid oauth state")
	}

	p, ok := h.platforms[platform]
	if !ok {
		log.Error("[OAuth2Command] Invalid platform", zap.String("platform", platform))
		return nil, ierr.New(ierr.ErrBadRequest, "invalid oauth platform")
	}

	return p, nil
}

// exchangeAndFetchUser 交换授权码获取 token 并拉取用户信息
//
//	@receiver h *handleCallbackHandler
//	@param ctx context.Context
//	@param platform oauth2service.Platform
//	@param code string
//	@return oauth2vo.OAuthUserInfo
//	@return error
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func (h *handleCallbackHandler) exchangeAndFetchUser(ctx context.Context, platform oauth2service.Platform, code string) (oauth2vo.OAuthUserInfo, error) {
	log := logger.WithCtx(ctx)

	log.Info("[OAuth2Command] Exchanging token")
	token, err := platform.ExchangeToken(ctx, code)
	if err != nil {
		log.Error("[OAuth2Command] Failed to exchange token", zap.Error(err))
		return oauth2vo.OAuthUserInfo{}, ierr.Wrap(ierr.ErrOAuth2Exchange, err, "exchange oauth token")
	}
	h.logTokenReceived(ctx, token)

	userInfo, err := platform.GetUserInfo(ctx, token)
	if err != nil {
		log.Error("[OAuth2Command] Failed to get user info", zap.Error(err))
		return oauth2vo.OAuthUserInfo{}, ierr.Wrap(ierr.ErrOAuth2UserInfo, err, "get oauth user info")
	}

	return userInfo, nil
}

// resolveUser 查找或注册用户
//
//	@receiver h *handleCallbackHandler
//	@param ctx context.Context
//	@param platformName string
//	@param userInfo oauth2vo.OAuthUserInfo
//	@return uint userID
//	@return bool isNewUser
//	@return error
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func (h *handleCallbackHandler) resolveUser(ctx context.Context, platformName string, userInfo oauth2vo.OAuthUserInfo) (uint, bool, error) {
	log := logger.WithCtx(ctx)
	thirdPartyID := userInfo.ID()
	userName, email, avatar := userInfo.Name(), userInfo.Email(), userInfo.Avatar()

	existing, err := h.findByBindID(ctx, platformName, thirdPartyID)
	if err != nil {
		log.Error("[OAuth2Command] Failed to find user by third party bind id",
			zap.String("platform", platformName), zap.String("thirdPartyID", thirdPartyID), zap.Error(err))
		return 0, false, err
	}

	var (
		userID    uint
		isNewUser bool
	)
	if existing != nil {
		// 已存在用户：只更新 last_login（与原 service.userDAO.Update(db, user, {last_login}) 行为一致）
		if err := h.userRepo.TouchLastLogin(ctx, existing.AggregateID()); err != nil {
			log.Error("[OAuth2Command] Failed to update user login time",
				zap.String("platform", platformName), zap.Error(err))
			return 0, false, err
		}
		userID = existing.AggregateID()
	} else {
		// 新用户：若 userName 非法，回退到 "default_<unixts>"
		if validateErr := util.ValidateUserName(userName); validateErr != nil {
			userName = constant.DefaultUserNamePrefix + strconv.FormatInt(time.Now().UTC().Unix(), 10)
		}

		user, regErr := identityaggregate.RegisterUser(
			identityvo.UserName(userName),
			identityvo.Email(email),
			identityvo.Avatar(avatar),
			platformName,
			thirdPartyID,
			time.Now(),
		)
		if regErr != nil {
			log.Error("[OAuth2Command] Register user aggregate failed",
				zap.String("platform", platformName), zap.String("userName", userName), zap.Error(regErr))
			return 0, false, regErr
		}
		if err := h.userRepo.Save(ctx, user); err != nil {
			log.Error("[OAuth2Command] Failed to save new user",
				zap.String("platform", platformName), zap.String("userName", userName), zap.Error(err))
			return 0, false, err
		}
		userID = user.AggregateID()
		isNewUser = true

		// 创建存储目录（与原 service.audioObjDAO.CreateDir 行为一致）
		if h.objStorageDirC != nil {
			if dirErr := h.objStorageDirC.CreateDir(ctx, userID); dirErr != nil {
				log.Error("[OAuth2Command] Failed to create audio dir",
					zap.String("platform", platformName), zap.Error(dirErr))
				return 0, false, dirErr
			}
			log.Info("[OAuth2Command] Audio dir created", zap.String("platform", platformName))
		}
	}

	return userID, isNewUser, nil
}

// signTokenPair 签发 JWT token 对
//
//	@receiver h *handleCallbackHandler
//	@param ctx context.Context
//	@param userID uint
//	@return string accessToken
//	@return string refreshToken
//	@return error
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func (h *handleCallbackHandler) signTokenPair(ctx context.Context, userID uint) (string, string, error) {
	log := logger.WithCtx(ctx)

	accessToken, err := h.accessSigner.EncodeToken(userID)
	if err != nil {
		log.Error("[OAuth2Command] Failed to encode access token", zap.Error(err))
		return "", "", ierr.Wrap(ierr.ErrJWTEncode, err, "encode access token")
	}
	refreshToken, err := h.refreshSigner.EncodeToken(userID)
	if err != nil {
		log.Error("[OAuth2Command] Failed to encode refresh token", zap.Error(err))
		return "", "", ierr.Wrap(ierr.ErrJWTEncode, err, "encode refresh token")
	}

	return accessToken, refreshToken, nil
}

// findByBindID 按平台查找已绑定的用户（跨平台的分发逻辑）
//
//	@receiver h *handleCallbackHandler
//	@param ctx context.Context
//	@param platform string
//	@param bindID string
//	@return *identityaggregate.User 未找到返回 nil
//	@return error
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func (h *handleCallbackHandler) findByBindID(ctx context.Context, platform, bindID string) (*identityaggregate.User, error) {
	switch platform {
	case constant.OAuthProviderGithub:
		return h.userRepo.FindByGithubBindID(ctx, bindID)
	case constant.OAuthProviderGoogle:
		return h.userRepo.FindByGoogleBindID(ctx, bindID)
	default:
		return nil, ierr.New(ierr.ErrBadRequest, "invalid oauth platform")
	}
}

// logTokenReceived 打印 token 元信息（不暴露 access token 原值）
//
//	@receiver h *handleCallbackHandler
//	@param ctx context.Context
//	@param token *xoauth2.Token
//	@author centonhuang
//	@update 2026-04-26 12:00:00
func (h *handleCallbackHandler) logTokenReceived(ctx context.Context, token *xoauth2.Token) {
	logger.WithCtx(ctx).Info("[OAuth2Command] Token exchange successful",
		zap.String("tokenType", token.TokenType),
		zap.Bool("valid", token.Valid()))
}
