package query

import (
	"context"
	"slices"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	apikeydomain "github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
)

type traceAuthorizer struct {
	repo       trace.TraceRepository
	apiKeyRepo apikeydomain.APIKeyRepository
}

func newTraceAuthorizer(
	repo trace.TraceRepository,
	apiKeyRepo apikeydomain.APIKeyRepository,
) *traceAuthorizer {
	return &traceAuthorizer{repo: repo, apiKeyRepo: apiKeyRepo}
}

func (a *traceAuthorizer) Find(
	ctx context.Context,
	userID uint,
	isAdmin bool,
	traceID uint,
) (*trace.Trace, error) {
	item, err := a.repo.FindByID(ctx, traceID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.TraceNotFoundMessage)
	}
	if isAdmin {
		return item, nil
	}
	owners, err := a.apiKeyRepo.LookupOwnerNamesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(owners, item.APIKeyName) {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.TraceNotFoundMessage)
	}
	return item, nil
}
