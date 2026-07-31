package command

import (
	"context"
	"slices"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/trace/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/apikey"
	"github.com/hcd233/aris-proxy-api/internal/domain/trace"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// NewDeleteTraceHandler 构造删除命令处理器
func NewDeleteTraceHandler(repo trace.TraceRepository, apiKeyRepo apikey.APIKeyRepository) port.DeleteTraceHandler {
	return &deleteTraceHandler{repo: repo, apiKeyRepo: apiKeyRepo}
}

type deleteTraceHandler struct {
	repo       trace.TraceRepository
	apiKeyRepo apikey.APIKeyRepository
}

func (h *deleteTraceHandler) Handle(ctx context.Context, cmd port.DeleteTraceCommand) (*port.DeleteTraceResult, error) {
	log := logger.WithCtx(ctx)

	var ownerNames []string
	if !cmd.IsAdmin {
		names, lookupErr := h.apiKeyRepo.LookupOwnerNamesByUserID(ctx, cmd.UserID)
		if lookupErr != nil {
			log.Error("[TraceCommand] Delete: lookup owner names failed",
				zap.Error(lookupErr), zap.Uint("userID", cmd.UserID))
			return nil, lookupErr
		}
		ownerNames = names
	}

	result := &port.DeleteTraceResult{}

	for _, id := range cmd.IDs {
		t, err := h.repo.FindByID(ctx, id)
		if err != nil {
			log.Error("[TraceCommand] Delete: FindByID failed", zap.Error(err), zap.Uint("traceID", id))
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorFindFailed})
			continue
		}
		if t == nil {
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorNotFound})
			continue
		}

		if !cmd.IsAdmin && !slices.Contains(ownerNames, t.APIKeyName) {
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorNoPermission})
			continue
		}

		if err := h.repo.Delete(ctx, id); err != nil {
			log.Error("[TraceCommand] Delete: delete failed", zap.Error(err), zap.Uint("traceID", id))
			result.Failures = append(result.Failures, port.DeleteTraceFailedItem{ID: id, Error: constant.TraceDeleteErrorDeleteFailed})
			continue
		}

		result.DeletedCount++
		log.Info("[TraceCommand] Trace deleted",
			zap.Uint("traceID", id),
			zap.Uint("requesterID", cmd.UserID),
			zap.String("owner", t.APIKeyName))
	}

	log.Info("[TraceCommand] Delete completed",
		zap.Int("total", len(cmd.IDs)),
		zap.Int("deleted", result.DeletedCount),
		zap.Int("failed", len(result.Failures)),
		zap.Uint("requesterID", cmd.UserID))

	return result, nil
}
