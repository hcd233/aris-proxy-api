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
	// 非 admin 且 userID=0（认证缺失）：显式短路拒绝，禁止退化为全量归属
	if userID == 0 {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.TraceNotFoundMessage)
	}
	ownerIDs, err := a.apiKeyRepo.LookupIDsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// 归属为 0（存量未知）的 trace 对普通用户一律不可见，不得因零值匹配放行
	if item.APIKeyID == 0 || !slices.Contains(ownerIDs, item.APIKeyID) {
		return nil, ierr.New(ierr.ErrDataNotExists, constant.TraceNotFoundMessage)
	}
	return item, nil
}
