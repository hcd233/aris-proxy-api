package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type ModelHandler interface {
	HandleCreateModel(ctx context.Context, req *dto.CreateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.ModelUpdateRsp], error)
	HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error)
	HandleListModels(ctx context.Context, req *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error)
}

type ModelDependencies struct {
	Create port.CreateModelHandler
	Update port.UpdateModelHandler
	Delete port.DeleteModelHandler
	List   port.ListModelHandler
}

type modelHandler struct {
	create port.CreateModelHandler
	update port.UpdateModelHandler
	delete port.DeleteModelHandler
	list   port.ListModelHandler
}

func NewModelHandler(deps ModelDependencies) ModelHandler {
	return &modelHandler{
		create: deps.Create,
		update: deps.Update,
		delete: deps.Delete,
		list:   deps.List,
	}
}

func (h *modelHandler) HandleCreateModel(ctx context.Context, req *dto.CreateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	_, err = h.create.Handle(ctx, port.CreateModelCommand{
		ScopeUserID:     scope,
		Alias:           req.Body.Alias,
		ModelID:         req.Body.ModelID,
		UpstreamModel:   req.Body.UpstreamModel,
		EndpointID:      req.Body.EndpointID,
		ContextLength:   req.Body.ContextLength,
		MaxOutputTokens: req.Body.MaxOutputTokens,
		Capabilities:    req.Body.Capabilities,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Create model failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	logger.WithCtx(ctx).Info("[ModelHandler] Create model success",
		zap.Uint("userID", userID), zap.String("alias", req.Body.Alias))
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func (h *modelHandler) HandleUpdateModel(ctx context.Context, req *dto.UpdateModelReq) (*dto.HTTPResponse[*dto.ModelUpdateRsp], error) {
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	result, err := h.update.Handle(ctx, port.UpdateModelCommand{
		ScopeUserID:     scope,
		ID:              req.ID,
		Alias:           req.Body.Alias,
		UpstreamModel:   req.Body.UpstreamModel,
		EndpointID:      req.Body.EndpointID,
		Enabled:         req.Body.Enabled,
		ContextLength:   req.Body.ContextLength,
		MaxOutputTokens: req.Body.MaxOutputTokens,
		Capabilities:    req.Body.Capabilities,
		ModelID:         req.Body.ModelID,
		SyncHistory:     req.Body.SyncHistory,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Update model failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.ModelUpdateRsp{
		AuditCount:   result.AuditCount,
		SessionCount: result.SessionCount,
		MessageCount: result.MessageCount,
	}, nil)
}

func (h *modelHandler) HandleDeleteModel(ctx context.Context, req *dto.DeleteModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	err = h.delete.Handle(ctx, port.DeleteModelCommand{
		ScopeUserID: scope,
		ModelID:     req.ID,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Delete model failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleListModels 平铺模型列表查询（Web 管理端）
//
// scope 用 *uint 三态：admin → nil（全量），非 admin → 自身，未认证 → 401。
func (h *modelHandler) HandleListModels(ctx context.Context, req *dto.ListModelsReq) (*dto.HTTPResponse[*dto.ListModelsRsp], error) {
	rsp := &dto.ListModelsRsp{}
	perm := util.CtxValuePermission(ctx)
	scope, err := scopePtrFor(ctx, perm)
	if err != nil {
		logger.WithCtx(ctx).Warn("[ModelHandler] List models rejected", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	views, pageInfo, err := h.list.Handle(ctx, port.ListModelQuery{
		CommonParam: commonmodel.CommonParam{
			PageParam:  commonmodel.PageParam{Page: req.Page, PageSize: req.PageSize},
			QueryParam: commonmodel.QueryParam{Query: req.Query},
			SortParam:  commonmodel.SortParam{Sort: req.Sort, SortField: req.SortField},
		},
		IsDemo:      perm == enum.PermissionDemo,
		ScopeUserID: scope,
		Username:    req.Username,
		Status:      req.Status,
		EndpointID:  req.EndpointID,
		Capability:  req.Capability,
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] List models failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}

	rsp.Items = lo.Map(views, func(v *port.ListModelView, _ int) *dto.ModelListItem {
		return toModelListItem(v)
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// scopePtrFor 平铺模型列表专用 scope：admin → nil（不过滤），非 admin → 自身 ID。
//
// 不能沿用 scopeFor：后者用 0 同时表达"admin 全量"与"非 admin 未拿到 userID"，
// 直接把 0 映射成 nil 会让认证缺失静默退化成全平台可见。故此处对非 admin 的
// userID==0 显式报错。
func scopePtrFor(ctx context.Context, perm enum.Permission) (*uint, error) {
	if perm == enum.PermissionAdmin {
		return nil, nil
	}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	if userID == 0 {
		return nil, ierr.New(ierr.ErrUnauthorized, "missing authenticated user id")
	}
	return &userID, nil
}

func toModelListItem(v *port.ListModelView) *dto.ModelListItem {
	item := &dto.ModelListItem{
		ID:              v.ID,
		Alias:           v.Alias,
		ModelID:         v.ModelID,
		UpstreamModel:   v.UpstreamModel,
		Enabled:         v.Enabled,
		ContextLength:   v.ContextLength,
		MaxOutputTokens: v.MaxOutputTokens,
		Capabilities:    v.Capabilities,
		CreatedAt:       v.CreatedAt,
		UpdatedAt:       v.UpdatedAt,
	}
	if v.User != nil {
		item.User = &dto.UpstreamUserItem{ID: v.User.ID, Name: v.User.Name, Avatar: v.User.Avatar}
	}
	if v.Endpoint != nil {
		item.Endpoint = &dto.ModelListEndpointItem{ID: v.Endpoint.ID, Name: v.Endpoint.Name}
	}
	return item
}
