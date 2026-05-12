package router

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/hcd233/aris-proxy-api/internal/handler"
	"github.com/hcd233/aris-proxy-api/internal/middleware"
)

func initAuditRouter(auditGroup huma.API, auditHandler handler.AuditHandler) {
	auditGroup.UseMiddleware(middleware.APIKeyMiddleware())

	huma.Register(auditGroup, huma.Operation{
		OperationID: "listAuditLogs",
		Method:      http.MethodGet,
		Path:        "/logs",
		Summary:     "ListAuditLogs",
		Description: "Paginate audit logs filtered by current API key, supports search by traceID/model and time range filtering",
		Tags:        []string{"Audit"},
		Security: []map[string][]string{
			{"apiKeyAuth": {}},
		},
	}, auditHandler.HandleListAuditLogs)
}
