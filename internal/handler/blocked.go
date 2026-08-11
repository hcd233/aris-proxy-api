package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/blocked/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type BlockedHandler interface {
	HandleCreateBlocked(ctx context.Context, req *dto.CreateBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleListBlocked(ctx context.Context, req *dto.ListBlockedReq) (*dto.HTTPResponse[*dto.ListBlockedRsp], error)
	HandleUpdateBlocked(ctx context.Context, req *dto.UpdateBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.DeleteBlockedRsp], error)
}

type BlockedDependencies struct {
	Create port.CreateBlockedHandler
	Update port.UpdateBlockedHandler
	Delete port.DeleteBlockedHandler
	List   port.ListBlockedHandler
}

type blockedHandler struct {
	create port.CreateBlockedHandler
	update port.UpdateBlockedHandler
	delete port.DeleteBlockedHandler
	list   port.ListBlockedHandler
}

func NewBlockedHandler(deps BlockedDependencies) BlockedHandler {
	return &blockedHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
		list:   deps.List,
	}
}

func (h *blockedHandler) HandleCreateBlocked(ctx context.Context, req *dto.CreateBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.create.Handle(ctx, port.CreateBlockedCommand{
		Word:   req.Body.Word,
		Action: req.Body.Action,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Create blocked word failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	_ = result.BlockedID
	logger.WithCtx(ctx).Info("[BlockedHandler] Create blocked word success",
		zap.Uint("userID", userID), zap.String("word", req.Body.Word))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *blockedHandler) HandleListBlocked(ctx context.Context, req *dto.ListBlockedReq) (*dto.HTTPResponse[*dto.ListBlockedRsp], error) {
	rsp := &dto.ListBlockedRsp{}

	views, pageInfo, err := h.list.Handle(ctx, port.ListBlockedQuery{
		CommonParam: req.CommonParam,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] List blocked words failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Blocked = lo.Map(views, func(v *port.BlockedView, _ int) *dto.BlockedItem {
		return &dto.BlockedItem{
			ID:        v.ID,
			Word:      v.Word,
			Action:    v.Action,
			HitCount:  v.HitCount,
			CreatedAt: v.CreatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *blockedHandler) HandleUpdateBlocked(ctx context.Context, req *dto.UpdateBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	if req.Body == nil || req.Body.Action == nil {
		return nil, apiutil.NewHumaBizErrorFromModel(ctx, ierr.ErrValidation.BizError())
	}

	err := h.update.Handle(ctx, port.UpdateBlockedCommand{BlockedID: req.ID, Action: *req.Body.Action})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Update blocked word failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	logger.WithCtx(ctx).Info("[BlockedHandler] Update blocked word success",
		zap.Uint("blockedID", req.ID), zap.String("action", *req.Body.Action))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleDeleteBlocked 删除敏感词（支持逗号分隔批量删除）
func (h *blockedHandler) HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.DeleteBlockedRsp], error) {
	rsp := &dto.DeleteBlockedRsp{}

	ids, parseErr := parseCommaSeparatedIDs(req.IDs)
	if parseErr != nil {
		return nil, apiutil.NewHumaBizError(ctx, parseErr, ierr.ErrValidation.BizError())
	}

	err := h.delete.Handle(ctx, port.DeleteBlockedCommand{BlockedIDs: ids})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Delete blocked word failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.DeletedCount = len(ids)
	logger.WithCtx(ctx).Info("[BlockedHandler] Blocked word(s) deleted", zap.Int("total", len(ids)))

	return apiutil.WrapHTTPResponse(rsp, nil)
}
