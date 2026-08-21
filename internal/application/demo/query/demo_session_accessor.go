package query

import (
	"context"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type demoSessionAccessor struct {
	repo port.DemoSessionRepository
}

func NewDemoSessionAccessor(repo port.DemoSessionRepository) port.DemoSessionAccessor {
	return &demoSessionAccessor{repo: repo}
}

func (a *demoSessionAccessor) AllowedIDs(ctx context.Context) ([]uint, error) {
	ids, err := a.repo.List(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoQuery] List demo sessions failed (fail-closed)")
		return nil, err
	}
	return ids, nil
}

func (a *demoSessionAccessor) IsAllowed(ctx context.Context, sessionID uint) (bool, error) {
	ids, err := a.AllowedIDs(ctx)
	if err != nil {
		return false, err
	}
	return lo.Contains(ids, sessionID), nil
}
