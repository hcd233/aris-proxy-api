package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initUserRouter(userGroup huma.API, userHandler handler.UserHandler, db *gorm.DB, cache *redis.Client, accessSigner jwt.TokenSigner) {
	userGroup.UseMiddleware(middleware.JwtMiddleware(db, cache, accessSigner))
	// 限流: 防止快速枚举/审核用户
	userGroup.UseMiddleware(middleware.TokenBucketRateLimiterMiddleware(
		cache,
		"userManage",
		constant.CtxKeyUserID,
		constant.PeriodManageUser,
		constant.LimitManageUser,
	))

	huma.Register(userGroup, huma.Operation{
		OperationID: "getCurrentUser",
		Method:      http.MethodGet,
		Path:        "/current",
		Summary:     "GetCurrentUser",
		Description: "Get the current user's detailed information, including user ID, username, email, avatar, and permission information",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
	}, userHandler.HandleGetCurUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "updateUser",
		Method:      http.MethodPatch,
		Path:        "",
		Summary:     "UpdateUser",
		Description: "Update the current user's information, including the username and other fields",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("updateUser", enum.PermissionUser)},
	}, userHandler.HandleUpdateUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "listUsers",
		Method:      http.MethodGet,
		Path:        constant.RoutePathList,
		Summary:     "ListUsers",
		Description: "List all users with pagination, keyword and permission filter (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("listUsers", enum.PermissionAdmin),
		},
	}, userHandler.HandleListUsers)

	huma.Register(userGroup, huma.Operation{
		OperationID: "approveUser",
		Method:      http.MethodPost,
		Path:        "/approve",
		Summary:     "ApproveUser",
		Description: "Approve a pending user to regular user (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("approveUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleApproveUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "demoteUser",
		Method:      http.MethodPost,
		Path:        "/demote",
		Summary:     "DemoteUser",
		Description: "Demote a regular user back to pending (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("demoteUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleDemoteUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "deleteUser",
		Method:      http.MethodDelete,
		Path:        "/delete",
		Summary:     "DeleteUser",
		Description: "Soft-delete a user and cascade revoke their API keys (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("deleteUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleDeleteUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "setDemoUser",
		Method:      http.MethodPost,
		Path:        "/demo",
		Summary:     "SetDemoUser",
		Description: "Set a pending/user account as the global demo account, replacing the existing one (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("setDemoUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleSetDemoUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "restoreDemoUser",
		Method:      http.MethodPost,
		Path:        "/demo/restore",
		Summary:     "RestoreDemoUser",
		Description: "Restore the demo account back to a regular user (admin only)",
		Tags:        []string{constant.TagUser},
		Security: []map[string][]string{
			{constant.SecuritySchemeJWT: {}},
		},
		Middlewares: huma.Middlewares{
			middleware.LimitUserPermissionMiddleware("restoreDemoUser", enum.PermissionAdmin),
		},
	}, userHandler.HandleRestoreDemoUser)
}
