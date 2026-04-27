package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initOauth2Router(oauth2Group huma.API, oauth2Handler handler.Oauth2Handler) {
	huma.Register(oauth2Group, huma.Operation{
		OperationID: "oauth2Login",
		Method:      http.MethodGet,
		Path:        "/login",
		Summary:     "OAuth2Login",
		Description: "Get OAuth2 authorization URL for the specified platform (github/google/qq)",
		Tags:        []string{"OAuth2"},
	}, oauth2Handler.HandleLogin)

	huma.Register(oauth2Group, huma.Operation{
		OperationID: "oauth2Callback",
		Method:      http.MethodPost,
		Path:        "/callback",
		Summary:     "OAuth2Callback",
		Description: "Handle OAuth2 callback with authorization code and state",
		Tags:        []string{"OAuth2"},
		Middlewares: huma.Middlewares{middleware.TokenBucketRateLimiterMiddleware("oauth2Callback", "", constant.PeriodOAuth2Callback, constant.LimitOAuth2Callback)},
	}, oauth2Handler.HandleCallback)
}
