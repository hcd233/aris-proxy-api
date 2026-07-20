package middleware

import (
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	traceport "github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

func TraceClientTicketMiddleware(
	store traceport.TraceClientTicketStore,
) func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		ticket := strings.TrimSpace(ctx.Header(constant.HTTPHeaderAuthorization))
		ticket = strings.TrimSpace(strings.TrimPrefix(ticket, constant.HTTPAuthBearerPrefix))
		userID, found, err := store.Consume(ctx.Context(), ticket)
		if err != nil {
			logger.WithCtx(ctx.Context()).Error(
				"[TraceClientTicketMiddleware] Failed to consume ticket",
				zap.Error(err),
			)
			lo.Must0(apiutil.WriteErrorHTTPResponse(
				ctx,
				fiber.StatusInternalServerError,
				ierr.ErrInternal.BizError().Localize(i18n.FromCtx(ctx.Context())),
			))
			return
		}
		if !found {
			lo.Must0(apiutil.WriteErrorHTTPResponse(
				ctx,
				fiber.StatusUnauthorized,
				ierr.ErrUnauthorized.BizError().Localize(i18n.FromCtx(ctx.Context())),
			))
			return
		}
		ctx = huma.WithValue(ctx, constant.CtxKeyUserID, userID)
		next(ctx)
	}
}
