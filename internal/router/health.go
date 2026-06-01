package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
)

func initHealthRouter(healthGroup huma.API, pingHandler handler.PingHandler) {
	huma.Register(healthGroup, huma.Operation{
		OperationID: "healthCheck",
		Method:      http.MethodGet,
		Path:        constant.RoutePathHealth,
		Summary:     "HealthCheck",
		Description: "Check the server health",
		Tags:        []string{"Health"},
	}, pingHandler.HandlePing)

	huma.Register(healthGroup, huma.Operation{
		OperationID: "readinessCheck",
		Method:      http.MethodGet,
		Path:        constant.RoutePathReady,
		Summary:     "ReadinessCheck",
		Description: "Check if the server is ready to accept traffic",
		Tags:        []string{"Health"},
	}, pingHandler.HandleReady)

	huma.Register(healthGroup, huma.Operation{
		OperationID: "sseHealthCheck",
		Method:      http.MethodGet,
		Path:        constant.RoutePathSSEHealth,
		Summary:     "SSEHealthCheck",
		Description: "Check the server health",
		Tags:        []string{"Health"},
	}, pingHandler.HandleSSEPing)
}
