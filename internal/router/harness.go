package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

// initHarnessRouter 注册 harness 规则管理路由（GET/POST/PATCH/DELETE 均为 admin only）。
func initHarnessRouter(group huma.API, harnessHandler handler.HarnessHandler) {
	huma.Register(group, huma.Operation{
		OperationID: "listHarnessRules",
		Method:      http.MethodGet,
		Path:        constant.RoutePathRuleList,
		Summary:     "ListHarnessRules",
		Description: "List agent harness rules ordered by (sort, id). Admin only.",
		Tags:        []string{constant.TagHarness},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("listHarnessRules", enum.PermissionAdmin)},
	}, harnessHandler.HandleListHarnessRules)

	huma.Register(group, huma.Operation{
		OperationID: "createHarnessRule",
		Method:      http.MethodPost,
		Path:        constant.RoutePathRule,
		Summary:     "CreateHarnessRule",
		Description: "Create an agent harness rule. Admin only.",
		Tags:        []string{constant.TagHarness},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("createHarnessRule", enum.PermissionAdmin)},
	}, harnessHandler.HandleCreateHarnessRule)

	huma.Register(group, huma.Operation{
		OperationID: "updateHarnessRule",
		Method:      http.MethodPatch,
		Path:        constant.RoutePathRule,
		Summary:     "UpdateHarnessRule",
		Description: "Update an agent harness rule. Admin only.",
		Tags:        []string{constant.TagHarness},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("updateHarnessRule", enum.PermissionAdmin)},
	}, harnessHandler.HandleUpdateHarnessRule)

	huma.Register(group, huma.Operation{
		OperationID: "deleteHarnessRule",
		Method:      http.MethodDelete,
		Path:        constant.RoutePathRule,
		Summary:     "DeleteHarnessRule",
		Description: "Delete an agent harness rule. Admin only.",
		Tags:        []string{constant.TagHarness},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("deleteHarnessRule", enum.PermissionAdmin)},
	}, harnessHandler.HandleDeleteHarnessRule)

	huma.Register(group, huma.Operation{
		OperationID: "previewHarnessRule",
		Method:      http.MethodPost,
		Path:        constant.RoutePathRulePreview,
		Summary:     "PreviewHarnessRule",
		Description: "Resolve a raw User-Agent against the current rules. Admin only.",
		Tags:        []string{constant.TagHarness},
		Security:    []map[string][]string{{constant.SecuritySchemeJWT: {}}},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("previewHarnessRule", enum.PermissionAdmin)},
	}, harnessHandler.HandlePreviewHarness)
}
