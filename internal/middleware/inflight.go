package middleware

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

var healthCheckPaths = map[string]struct{}{
	constant.RoutePathHealth:    {},
	constant.RoutePathReady:     {},
	constant.RoutePathSSEHealth: {},
}

func InflightMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if _, skip := healthCheckPaths[c.Path()]; skip {
			return c.Next()
		}

		tracker := inflight.GetTracker()
		if !tracker.Track() {
			c.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
			c.Status(fiber.StatusServiceUnavailable)

			body, _ := sonic.Marshal(&dto.CommonRsp{ //nolint:errcheck // Marshal always succeeds for static struct
				Error: ierr.ErrInternal.BizError(),
			})
			return c.Send(body)
		}
		defer tracker.Untrack()
		return c.Next()
	}
}
