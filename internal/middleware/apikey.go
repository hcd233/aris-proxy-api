package middleware

import (
	"cmp"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// APIKeyMiddleware API Key 验证中间件
//
// 每次请求从数据库查询 API Key 进行验证，并通过 UserID 查询用户名。
//
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-04-09 17:10:00
func APIKeyMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	proxyAPIKeyDAO := dao.GetProxyAPIKeyDAO()
	userDAO := dao.GetUserDAO()

	return func(ctx huma.Context, next func(huma.Context)) {
		tokenString := cmp.Or(ctx.Header(constant.HTTPLowerHeaderAuthorization), ctx.Header(constant.HTTPTitleHeaderAuthorization))
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), constant.HTTPAuthBearerPrefix))

		if tokenString == "" {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key is empty")
			lo.Must0(util.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError()))
			return
		}

		db := database.GetDBInstance(ctx.Context())
		apiKey, err := proxyAPIKeyDAO.Get(db, &dbmodel.ProxyAPIKey{Key: tokenString}, constant.ProxyAPIKeyRepoFieldsAuth)
		if err != nil {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key not found", zap.Error(err))
			lo.Must0(util.WriteErrorHTTPResponse(ctx, fiber.StatusUnauthorized, ierr.ErrUnauthorized.BizError()))
			return
		}

		// 通过 UserID 查询用户名
		user, err := userDAO.Get(db, &dbmodel.User{ID: apiKey.UserID}, constant.UserRepoFieldsBasic)
		if err != nil {
			logger.WithCtx(ctx.Context()).Error("[APIKeyMiddleware] Failed to get user", zap.Error(err))
			lo.Must0(util.WriteErrorHTTPResponse(ctx, fiber.StatusInternalServerError, ierr.ErrInternal.BizError()))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, user.ID)
		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, user.Name)
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, apiKey.ID)
		ctx = huma.WithValue(ctx, constant.CtxKeyClient, ctx.Header(constant.HTTPTitleHeaderUserAgent))

		next(ctx)
	}
}
