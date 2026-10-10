package middleware

import (
	"context"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	apikeyagg "github.com/hcd233/aris-proxy-api/internal/domain/apikey/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// PlaygroundAPIKeyFinder 按 ID 查询 API Key（apikey.APIKeyRepository 的子集；未找到返回 nil, nil）
type PlaygroundAPIKeyFinder interface {
	FindByID(ctx context.Context, id uint) (*apikeyagg.ProxyAPIKey, error)
}

// PlaygroundAPIKeyMiddleware Playground 调用归属的 API Key 校验中间件（须挂在 JWT 之后）。
//
// Playground 走 JWT 鉴权没有 API Key，而审计/用量统计按 api_key_id 归属。
// 由调用方通过 query apiKeyID 选择自己名下的一把 Key：校验归属后注入与
// APIKeyMiddleware 相同的 ctx（APIKeyID/APIKeyName/Client），使审计、限流与代理链路同口径。
//
//	@param keys PlaygroundAPIKeyFinder
//	@return func(ctx huma.Context, next func(huma.Context))
//	@author centonhuang
//	@update 2026-10-09 10:00:00
func PlaygroundAPIKeyMiddleware(keys PlaygroundAPIKeyFinder) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		log := logger.WithCtx(ctx.Context())
		userID := util.CtxValueUint(ctx.Context(), constant.CtxKeyUserID)
		raw := ctx.Query(constant.QueryParamAPIKeyID)
		keyID, err := strconv.ParseUint(raw, constant.DecimalBase, constant.ParseFloat64BitSize)
		// keyID 必须非零：仓储 FindByID 走 GORM struct 条件，零值主键会被忽略而退化成「取第一条」
		if err != nil || keyID == 0 {
			log.Info("[PlaygroundAPIKeyMiddleware] Invalid apiKeyID", zap.String("idParam", raw))
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrValidation.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		apiKey, err := keys.FindByID(ctx.Context(), uint(keyID))
		if err != nil {
			log.Error("[PlaygroundAPIKeyMiddleware] Find api key failed", zap.Error(err))
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrDBQuery.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}
		// 不区分「不存在」与「不属于你」，避免探测他人 Key ID；IsOwnedBy 对 userID=0 恒 false
		if apiKey == nil || !apiKey.IsOwnedBy(userID) {
			log.Info("[PlaygroundAPIKeyMiddleware] API key not owned by user", zap.Uint64("id", keyID), zap.Uint("userID", userID))
			lo.Must0(apiutil.WriteErrorResponse(ctx.BodyWriter(), ierr.ErrNoPermission.BizError().Localize(i18n.FromCtx(ctx.Context()))))
			return
		}

		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyID, apiKey.AggregateID())
		ctx = huma.WithValue(ctx, constant.CtxKeyAPIKeyName, apiKey.Name().String())
		ctx = huma.WithValue(ctx, constant.CtxKeyClient, ctx.Header(constant.HTTPHeaderUserAgent))
		next(ctx)
	}
}
