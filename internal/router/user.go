package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	infrarepository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initUserRouter(userGroup huma.API) {
	userHandler := handler.NewUserHandler(handler.UserDependencies{
		UserRepo: infrarepository.NewUserRepository(),
	})

	userGroup.UseMiddleware(middleware.JwtMiddleware())

	huma.Register(userGroup, huma.Operation{
		OperationID: "getCurrentUser",
		Method:      http.MethodGet,
		Path:        "/current",
		Summary:     "GetCurrentUser",
		Description: "Get the current user's detailed information, including user ID, username, email, avatar, and permission information",
		Tags:        []string{"User"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
	}, userHandler.HandleGetCurUser)

	huma.Register(userGroup, huma.Operation{
		OperationID: "updateUser",
		Method:      http.MethodPatch,
		Path:        "/",
		Summary:     "UpdateUser",
		Description: "Update the current user's information, including the username and other fields",
		Tags:        []string{"User"},
		Security: []map[string][]string{
			{"jwtAuth": {}},
		},
		Middlewares: huma.Middlewares{middleware.LimitUserPermissionMiddleware("updateUser", enum.PermissionUser)},
	}, userHandler.HandleUpdateUser)
}
