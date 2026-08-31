package middleware

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// APIKeyMiddleware API Key 验证中间件
//
// 每次请求从数据库查询 API Key 进行验证，并通过 UserID 查询用户名。
//
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-04-09 17:10:00
func APIKeyMiddleware(db *gorm.DB) func(ctx huma.Context, next func(huma.Context)) {
	proxyAPIKeyDAO := dao.GetProxyAPIKeyDAO()
	userDAO := dao.GetUserDAO()

	return func(ctx huma.Context, next func(huma.Context)) {
		tokenString := ctx.Header(constant.HTTPHeaderAuthorization)
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), constant.HTTPAuthBearerPrefix))

		if tokenString == "" {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key is empty")
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		if db == nil {
			logger.WithCtx(ctx.Context()).Error("[APIKeyMiddleware] DB dependency is nil")
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusInternalServerError, ierr.ErrInternal.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}
		reqDB := db.WithContext(ctx.Context())
		apiKey, err := proxyAPIKeyDAO.Get(reqDB, &dbmodel.ProxyAPIKey{Key: tokenString}, constant.ProxyAPIKeyRepoFieldsAuth)
		if err != nil {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key not found", zap.Error(err))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		// 通过 UserID 查询用户名与权限
		user, err := userDAO.Get(reqDB, &dbmodel.User{ID: apiKey.UserID}, constant.UserRepoFieldsAuth)
		if err != nil {
			logger.WithCtx(ctx.Context()).Error("[APIKeyMiddleware] Failed to get user", zap.Error(err))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusInternalServerError, ierr.ErrInternal.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		// 无主 key（user_id=0）防御：struct 零值条件下 userDAO.Get 会忽略 user_id 过滤，
		// 返回主键最小的用户，把请求错误认成别人的租户——多租户化后必须显式拒绝。
		if apiKey.UserID == 0 {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] Orphan API key rejected (user_id=0)",
				zap.Uint("apiKeyID", apiKey.ID))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		// Demo 账户的存量 API Key 一律拒绝：demo 只读受限，不允许调用 LLM 转发
		if user.Permission == enum.PermissionDemo {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] Demo user API key rejected",
				zap.Uint("userID", user.ID), zap.Uint("apiKeyID", apiKey.ID))
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, user.ID)
		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, user.Name)
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, apiKey.ID)
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyName, apiKey.Name)
		ctx = huma.WithValue(ctx, constant.CtxKeyClient, ctx.Header(constant.HTTPHeaderUserAgent))

		next(ctx)
	}
}
