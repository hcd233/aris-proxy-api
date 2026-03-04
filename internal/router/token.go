package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/handler"
)

func initTokenRouter(tokenGroup huma.API) {
	tokenHandler := handler.NewTokenHandler()

	huma.Register(tokenGroup, huma.Operation{
		OperationID: "refreshToken",
		Method:      http.MethodPost,
		Path:        "/refresh",
		Summary:     "RefreshToken",
		Description: "Refresh the access token using a refresh token",
		Tags:        []string{"Token"},
	}, tokenHandler.HandleRefreshToken)
}
