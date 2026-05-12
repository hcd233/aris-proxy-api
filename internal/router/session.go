package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
	"gorm.io/gorm"
)

func initSessionRouter(sessionGroup huma.API, sessionHandler handler.SessionHandler, db *gorm.DB) {
	sessionGroup.UseMiddleware(middleware.APIKeyMiddleware(db))

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "listSessions",
		Method:      http.MethodGet,
		Path:        "/list",
		Summary:     "ListSessions",
		Description: "Paginate session list filtered by current API key",
		Tags:        []string{"Session"},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, sessionHandler.HandleListSessions)

	huma.Register(sessionGroup, huma.Operation{
		OperationID: "getSession",
		Method:      http.MethodGet,
		Path:        "/",
		Summary:     "GetSession",
		Description: "Get session detail by session ID, including messages and tools",
		Tags:        []string{"Session"},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, sessionHandler.HandleGetSession)
}
