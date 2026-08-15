package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/identity"
	identityservice "github.com/hcd233/aris-proxy-api/internal/domain/identity/service"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type demoLoginHandler struct {
	configRepo    port.DemoConfigRepository
	userRepo      identity.UserRepository
	accessSigner  identityservice.TokenSigner
	refreshSigner identityservice.TokenSigner
}

// NewDemoLoginHandler 构造
//
//	@param configRepo port.DemoConfigRepository
//	@param userRepo identity.UserRepository
//	@param accessSigner identityservice.TokenSigner
//	@param refreshSigner identityservice.TokenSigner
//	@return port.DemoLoginHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewDemoLoginHandler(
	configRepo port.DemoConfigRepository,
	userRepo identity.UserRepository,
	accessSigner, refreshSigner identityservice.TokenSigner,
) port.DemoLoginHandler {
	return &demoLoginHandler{
		configRepo:    configRepo,
		userRepo:      userRepo,
		accessSigner:  accessSigner,
		refreshSigner: refreshSigner,
	}
}

// Handle 执行 Demo 账户登录：校验入口开关 → 定位单例 Demo 用户 → 签发 token pair
//
// 规则：
//
//   - 入口开关关闭 → ErrNoPermission
//
//   - 不存在 Demo 用户（admin 尚未设置）→ ErrDataNotExists
//
//     @receiver h *demoLoginHandler
//     @param ctx context.Context
//     @param cmd port.DemoLoginCommand
//     @return *port.DemoLoginResult
//     @return error
//     @author centonhuang
//     @update 2026-08-16 10:00:00
func (h *demoLoginHandler) Handle(ctx context.Context, cmd port.DemoLoginCommand) (*port.DemoLoginResult, error) {
	log := logger.WithCtx(ctx)

	config, err := h.configRepo.Get(ctx)
	if err != nil {
		log.Error("[DemoCommand] Read demo config failed", zap.Error(err))
		return nil, err
	}
	if !config.LoginEnabled {
		log.Info("[DemoCommand] Demo login rejected, entry disabled")
		return nil, ierr.New(ierr.ErrNoPermission, "demo login entry is disabled")
	}

	user, err := h.userRepo.FindByPermission(ctx, enum.PermissionDemo)
	if err != nil {
		log.Error("[DemoCommand] Find demo user failed", zap.Error(err))
		return nil, err
	}
	if user == nil {
		log.Info("[DemoCommand] Demo login rejected, no demo user configured")
		return nil, ierr.New(ierr.ErrDataNotExists, "demo user is not configured")
	}

	if err := h.userRepo.TouchLastLogin(ctx, user.AggregateID()); err != nil {
		log.Error("[DemoCommand] Touch demo user login failed", zap.Error(err))
		return nil, err
	}

	accessToken, err := h.accessSigner.EncodeToken(user.AggregateID())
	if err != nil {
		log.Error("[DemoCommand] Encode access token failed", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrJWTEncode, err, "encode demo access token")
	}
	refreshToken, err := h.refreshSigner.EncodeToken(user.AggregateID())
	if err != nil {
		log.Error("[DemoCommand] Encode refresh token failed", zap.Error(err))
		return nil, ierr.Wrap(ierr.ErrJWTEncode, err, "encode demo refresh token")
	}

	log.Info("[DemoCommand] Demo login success", zap.Uint("userID", user.AggregateID()))
	return &port.DemoLoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.AggregateID(),
	}, nil
}

type demoStatusHandler struct {
	configRepo port.DemoConfigRepository
	userRepo   identity.UserRepository
}

// NewDemoStatusHandler 构造
//
//	@param configRepo port.DemoConfigRepository
//	@param userRepo identity.UserRepository
//	@return port.DemoStatusHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewDemoStatusHandler(configRepo port.DemoConfigRepository, userRepo identity.UserRepository) port.DemoStatusHandler {
	return &demoStatusHandler{configRepo: configRepo, userRepo: userRepo}
}

// Handle 查询登录页 Demo 入口状态
//
//	@receiver h *demoStatusHandler
//	@param ctx context.Context
//	@param q port.DemoStatusQuery
//	@return *port.DemoStatusResult
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *demoStatusHandler) Handle(ctx context.Context, q port.DemoStatusQuery) (*port.DemoStatusResult, error) {
	log := logger.WithCtx(ctx)

	config, err := h.configRepo.Get(ctx)
	if err != nil {
		log.Error("[DemoCommand] Read demo config failed", zap.Error(err))
		return nil, err
	}
	if !config.LoginEnabled {
		return &port.DemoStatusResult{LoginEnabled: false, DemoUserExists: false}, nil
	}

	user, err := h.userRepo.FindByPermission(ctx, enum.PermissionDemo)
	if err != nil {
		log.Error("[DemoCommand] Find demo user failed", zap.Error(err))
		return nil, err
	}
	return &port.DemoStatusResult{
		LoginEnabled:   true,
		DemoUserExists: user != nil,
	}, nil
}
