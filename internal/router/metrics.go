package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initMetricsRouter(metricsGroup huma.API, metricsHandler handler.MetricsHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner, demoAccessor demoport.DemoModuleAccessor) {
	metricsGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))
	metricsGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(cache, "demoAccess", "", constant.PeriodDemoAccess, constant.LimitDemoAccess, middleware.WithPermissionFilter(enum.PermissionDemo)))

	huma.Register(metricsGroup, huma.Operation{
		OperationID: "getRuntimeMetrics",
		Method:      http.MethodGet,
		Path:        "/runtime",
		Summary:     "GetRuntimeMetrics",
		Description: "Get cross-pod aggregated runtime metrics time series for the monitor dashboard. Admin only.",
		Tags:        []string{constant.TagMonitor},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionWithDemoMiddleware("getRuntimeMetrics", enum.PermissionAdmin, enum.DemoModuleMonitor, demoAccessor)},
	}, metricsHandler.HandleGetRuntimeMetrics)
}
