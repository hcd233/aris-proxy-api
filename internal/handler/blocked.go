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
	HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
}

type BlockedDependencies struct {
	Create port.CreateBlockedHandler
	Delete port.DeleteBlockedHandler
	List   port.ListBlockedHandler
}

type blockedHandler struct {
	create port.CreateBlockedHandler
	delete port.DeleteBlockedHandler
	list   port.ListBlockedHandler
}

func NewBlockedHandler(deps BlockedDependencies) BlockedHandler {
	return &blockedHandler{
		create: deps.Create,
		delete: deps.Delete,
		list:   deps.List,
	}
}

func (h *blockedHandler) HandleCreateBlocked(ctx context.Context, req *dto.CreateBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)

	result, err := h.create.Handle(ctx, port.CreateBlockedCommand{
		Word: req.Body.Word,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Create blocked word failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
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
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Blocked = lo.Map(views, func(v *port.BlockedView, _ int) *dto.BlockedItem {
		return &dto.BlockedItem{
			ID:        v.ID,
			Word:      v.Word,
			HitCount:  v.HitCount,
			CreatedAt: v.CreatedAt,
		}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *blockedHandler) HandleDeleteBlocked(ctx context.Context, req *dto.DeleteBlockedReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}

	err := h.delete.Handle(ctx, port.DeleteBlockedCommand{BlockedID: req.ID})
	if err != nil {
		logger.WithCtx(ctx).Error("[BlockedHandler] Delete blocked word failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	return apiutil.WrapHTTPResponse(rsp, nil)
}
