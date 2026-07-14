// Package query trace 读侧 usecase
package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	apikeydomain "github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type listTracesHandler struct {
	repo       trace.TraceRepository
	apiKeyRepo apikeydomain.APIKeyRepository
}

// NewListTracesHandler 构造列表 handler
func NewListTracesHandler(repo trace.TraceRepository, apiKeyRepo apikeydomain.APIKeyRepository) port.ListTracesHandler {
	return &listTracesHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

func (h *listTracesHandler) Handle(ctx context.Context, q port.ListTracesQuery) ([]*port.TraceSummaryView, *model.PageInfo, error) {
	owners, err := resolveOwners(ctx, h.apiKeyRepo, q.UserID, q.IsAdmin)
	if err != nil {
		return nil, nil, err
	}
	traces, pageInfo, err := h.repo.PaginateByOwners(ctx, owners, model.CommonParam{PageParam: model.PageParam{Page: q.Page, PageSize: q.PageSize}})
	if err != nil {
		return nil, nil, err
	}
	return lo.Map(traces, func(item *trace.Trace, _ int) *port.TraceSummaryView {
		return &port.TraceSummaryView{
			ID: item.ID, SessionID: item.SessionID, Agent: item.Agent, APIKeyName: item.APIKeyName,
			Model: item.Model, Source: item.Source, Status: item.Status, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		}
	}), pageInfo, nil
}

func resolveOwners(ctx context.Context, repo apikeydomain.APIKeyRepository, userID uint, isAdmin bool) ([]string, error) {
	if isAdmin {
		return nil, nil
	}
	return repo.LookupOwnerNamesByUserID(ctx, userID)
}
