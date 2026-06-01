package middleware

import (
	"github.com/bytedance/sonic"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/inflight"
)

type serviceUnavailableResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

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
			c.Set(constant.HTTPTitleHeaderContentType, constant.HTTPContentTypeJSON)
			c.Status(fiber.StatusServiceUnavailable)

			resp := serviceUnavailableResponse{}
			resp.Error.Message = constant.ServerShuttingDownMsg
			resp.Error.Type = constant.ServerErrorType

			body, _ := sonic.Marshal(resp)
			return c.Send(body)
		}
		defer tracker.Untrack()
		return c.Next()
	}
}
