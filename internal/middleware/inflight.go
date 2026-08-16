package middleware

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/i18n"
)

var healthCheckPaths = map[string]struct{}{
	constant.RoutePathHealth:    {},
	constant.RoutePathReady:     {},
	constant.RoutePathSSEHealth: {},
}

func InflightMiddleware(tracker *inflight.Tracker) fiber.Handler {
	return func(c fiber.Ctx) error {
		if _, skip := healthCheckPaths[c.Path()]; skip {
			return c.Next()
		}

		if !tracker.Track() {
			c.Set(constant.HTTPHeaderContentType, constant.HTTPContentTypeJSON)
			// 保持 503：inflight 拒绝是优雅退出的流量摘除信号（K8s/负载均衡据此
			// 停止转发），不属于业务错误语义，不受统一 200 契约约束。
			c.Status(fiber.StatusServiceUnavailable)

			body, _ := sonic.Marshal(&dto.CommonRsp{ //nolint:errcheck // Marshal always succeeds for static struct
				Error: ierr.ErrInternal.BizError().Localize(i18n.FromCtx(c)),
			})
			return c.Send(body)
		}
		defer tracker.Untrack()
		return c.Next()
	}
}
