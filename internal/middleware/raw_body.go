package middleware

import (
	"io"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"
	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/samber/lo"
)

// RawBodyMiddleware 保存原始请求体，供需要 raw JSON 语义保真的代理链路使用。
func RawBodyMiddleware() func(ctx huma.Context, next func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		body, err := io.ReadAll(ctx.BodyReader())
		if err != nil {
			lo.Must0(apiutil.WriteErrorHTTPResponse(ctx, fiber.StatusInternalServerError, ierr.ErrInternal.BizError()))
			return
		}
		ctx = huma.WithValue(ctx, constant.CtxKeyRawRequestBody, append([]byte(nil), body...))
		next(ctx)
	}
}
