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

	if !infraoauth2.VerifyOAuth2State(cmd.State) {
		log.Error("[OAuth2Command] Invalid or expired state",
			zap.String("platform", cmd.Platform),
			zap.String("state", cmd.State))
		return nil, ierr.New(ierr.ErrUnauthorized, "invalid oauth state")
	}

	platform, ok := h.platforms[cmd.Platform]
	if !ok {
		log.Error("[OAuth2Command] Invalid platform", zap.String("platform", cmd.Platform))
		return nil, ierr.New(ierr.ErrBadRequest, "invalid oauth platform")
	}

	log.Info("[OAuth2Command] Exchanging token", zap.String("platform", cmd.Platform))
	token, err := platform.ExchangeToken(ctx, cmd.Code)
	if err != nil {
		log.Error("[OAuth2Command] Failed to exchange token",
			zap.String("platform", cmd.Platform), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrOAuth2Exchange, err, "exchange oauth token")
	}
	h.logTokenReceived(ctx, cmd.Platform, token)

	userInfo, err := platform.GetUserInfo(ctx, token)
	if err != nil {
		log.Error("[OAuth2Command] Failed to get user info",
			zap.String("platform", cmd.Platform), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrOAuth2UserInfo, err, "get oauth user info")
	}

	thirdPartyID := userInfo.ID
	userName, email, avatar := userInfo.Name, userInfo.Email, userInfo.Avatar

	existing, err := h.findByBindID(ctx, cmd.Platform, thirdPartyID)
	if err != nil {
		log.Error("[OAuth2Command] Failed to find user by third party bind id",
			zap.String("platform", cmd.Platform), zap.String("thirdPartyID", thirdPartyID), zap.Error(err))
		return nil, err
	}

	var (
		userID    uint
		isNewUser bool
	)
	if existing != nil {
		// 已存在用户：只更新 last_login（与原 service.userDAO.Update(db, user, {last_login}) 行为一致）
		if err := h.userRepo.TouchLastLogin(ctx, existing.AggregateID()); err != nil {
			log.Error("[OAuth2Command] Failed to update user login time",
				zap.String("platform", cmd.Platform), zap.Error(err))
			return nil, err
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
			cmd.Platform,
			thirdPartyID,
		)
		if regErr != nil {
			log.Error("[OAuth2Command] Register user aggregate failed",
				zap.String("platform", cmd.Platform), zap.String("userName", userName), zap.Error(regErr))
			return nil, regErr
		}
		if err := h.userRepo.Save(ctx, user); err != nil {
			log.Error("[OAuth2Command] Failed to save new user",
				zap.String("platform", cmd.Platform), zap.String("userName", userName), zap.Error(err))
			return nil, err
		}
		userID = user.AggregateID()
		isNewUser = true

		// 创建存储目录（与原 service.audioObjDAO.CreateDir 行为一致）
		if h.objStorageDirC != nil {
			if dirErr := h.objStorageDirC.CreateDir(ctx, userID); dirErr != nil {
				log.Error("[OAuth2Command] Failed to create audio dir",
					zap.String("platform", cmd.Platform), zap.Error(dirErr))
				return nil, dirErr
			}
			log.Info("[OAuth2Command] Audio dir created", zap.String("platform", cmd.Platform))
		}
	}

	// 签发 token 对
	accessToken, err := h.accessSigner.EncodeToken(userID)
	if err != nil {
		log.Error("[OAuth2Command] Failed to encode access token",
			zap.String("platform", cmd.Platform), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrJWTEncode, err, "encode access token")
	}
	refreshToken, err := h.refreshSigner.EncodeToken(userID)
	if err != nil {
		log.Error("[OAuth2Command] Failed to encode refresh token",
			zap.String("platform", cmd.Platform), zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrJWTEncode, err, "encode refresh token")
	}

	log.Info("[OAuth2Command] Callback success",
		zap.String("platform", cmd.Platform),
		zap.Uint("userID", userID),
		zap.Bool("isNewUser", isNewUser))

	return &HandleCallbackResult{
		TokenPair: &identityvo.TokenPair{AccessToken: accessToken, RefreshToken: refreshToken},
		UserID:    userID,
		IsNewUser: isNewUser,
	}, nil
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
//	@param platform string
//	@param token *xoauth2.Token
//	@author centonhuang
//	@update 2026-04-22 20:30:00
func (h *handleCallbackHandler) logTokenReceived(ctx context.Context, platform string, token *xoauth2.Token) {
	logger.WithCtx(ctx).Info("[OAuth2Command] Token exchange successful",
		zap.String("platform", platform),
		zap.String("tokenType", token.TokenType),
		zap.Bool("valid", token.Valid()))
}
