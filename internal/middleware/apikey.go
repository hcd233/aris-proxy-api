package middleware

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
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
// 每次请求从数据库查询 API Key 进行验证。
//
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-04-04 10:00:00
func APIKeyMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	proxyAPIKeyDAO := dao.GetProxyAPIKeyDAO()

	return func(ctx huma.Context, next func(huma.Context)) {
		tokenString := ctx.Header("Authorization")
		tokenString = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tokenString), "Bearer "))

		if tokenString == "" {
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrUnauthorized.BizError()))
			return
		}

		db := database.GetDBInstance(ctx.Context())
		apiKey, err := proxyAPIKeyDAO.Get(db, &dbmodel.ProxyAPIKey{Key: tokenString}, []string{"id", "user_id", "name"})
		if err != nil {
			logger.WithCtx(ctx.Context()).Info("[APIKeyMiddleware] API key not found", zap.Error(err))
			lo.Must0(util.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrUnauthorized.BizError()))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, apiKey.UserID)
		ctx = huma.WithValue(ctx, constant.CtxKeyUserName, apiKey.Name)
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, apiKey.ID)
		ctx = huma.WithValue(ctx, constant.CtxKeyClient, ctx.Header("User-Agent"))
		next(ctx)
	}
}
