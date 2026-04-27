package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/jwt"
	infrarepository "github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initTokenRouter(tokenGroup huma.API) {
	tokenHandler := handler.NewTokenHandler(handler.TokenDependencies{
		UserRepo:      infrarepository.NewUserRepository(),
		AccessSigner:  jwt.GetAccessTokenSigner(),
		RefreshSigner: jwt.GetRefreshTokenSigner(),
	})

	huma.Register(tokenGroup, huma.Operation{
		OperationID: "refreshToken",
		Method:      http.MethodPost,
		Path:        "/refresh",
		Summary:     "RefreshToken",
		Description: "Refresh the access token using a refresh token",
		Tags:        []string{"Token"},
		Middlewares: huma.Middlewares{middleware.TokenBucketRateLimiterMiddleware("refreshToken", "", constant.PeriodRefreshToken, constant.LimitRefreshToken)},
	}, tokenHandler.HandleRefreshToken)
}
