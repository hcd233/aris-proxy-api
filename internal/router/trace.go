// Package router Trace 查询分组视图路由（Web 分区，JWT 鉴权）
//
// 上报与 key 校验属 CLI 分区，见 cli.go 的 RegisterCLIAPIRoutes。
package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initTraceQueryRouter(traceGroup huma.API, traceHandler handler.TraceHandler) {
	huma.Register(traceGroup, huma.Operation{
		OperationID: "listTraces", Method: http.MethodGet, Path: constant.RoutePathList,
		Summary: "ListTraces", Description: "Paginate trace list for current user",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listTraces", enum.PermissionUser)},
	}, traceHandler.HandleListTraces)

	huma.Register(traceGroup, huma.Operation{
		OperationID: "getTrace", Method: http.MethodGet, Path: "",
		Summary: "GetTrace", Description: "Get trace detail by trace ID",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("getTrace", enum.PermissionUser)},
	}, traceHandler.HandleGetTrace)

	huma.Register(traceGroup, huma.Operation{
		OperationID: "listTraceEvents", Method: http.MethodGet, Path: "/event/list",
		Summary: "ListTraceEvents", Description: "Paginate trace event timeline",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listTraceEvents", enum.PermissionUser)},
	}, traceHandler.HandleListTraceEvents)

	huma.Register(traceGroup, huma.Operation{
		OperationID: "deleteTrace", Method: http.MethodDelete, Path: "",
		Summary: "DeleteTrace", Description: "Delete traces by IDs (owner or admin, comma separated)",
		Tags:        []string{constant.TagTrace},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteTrace", enum.PermissionUser)},
	}, traceHandler.HandleDeleteTraces)
}
