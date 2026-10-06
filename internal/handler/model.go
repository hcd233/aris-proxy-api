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
	HandlePrefillPricing(ctx context.Context, req *dto.ModelPricingPrefillReq) (*dto.HTTPResponse[*dto.ModelPricingPrefillRsp], error)
}

type ModelDependencies struct {
	Create  port.CreateModelHandler
	Update  port.UpdateModelHandler
	Delete  port.DeleteModelHandler
	List    port.ListModelHandler
	Prefill port.PrefillPricingHandler
}

type modelHandler struct {
	create  port.CreateModelHandler
	update  port.UpdateModelHandler
	delete  port.DeleteModelHandler
	list    port.ListModelHandler
	prefill port.PrefillPricingHandler
}

func NewModelHandler(deps ModelDependencies) ModelHandler {
	return &modelHandler{
		create:  deps.Create,
		update:  deps.Update,
		delete:  deps.Delete,
		list:    deps.List,
		prefill: deps.Prefill,
	}
}

func (h *modelHandler) HandleCreateModel(ctx context.Context, req *dto.CreateModelReq) (*dto.HTTPResponse[*dto.EmptyRsp], error) {
	rsp := &dto.EmptyRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrUnauthorized.BizError())
	}

	pricing, perr := port.PricingFromDTO(req.Body.Pricing)
	if perr != nil {
		return nil, apiutil.NewHumaBizError(ctx, perr, ierr.ErrValidation.BizError())
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
		Pricing:         pricing,
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
	rsp := &dto.ModelUpdateRsp{}
	scope, err := scopeFor(ctx, util.CtxValuePermission(ctx))
	if err != nil {
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrUnauthorized.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	cmd := port.UpdateModelCommand{
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
	}
	if req.Body.Pricing != nil {
		pricing, perr := port.PricingFromDTO(req.Body.Pricing)
		if perr != nil {
			rsp.Error = ierr.ToBizErrorLocalized(ctx, perr, ierr.ErrValidation.BizError())
			return apiutil.WrapHTTPResponse(rsp, nil)
		}
		cmd.Pricing = pricing
		cmd.PricingSet = true
	}
	counts, err := h.update.Handle(ctx, cmd)
	if err != nil {
		logger.WithCtx(ctx).Error("[ModelHandler] Update model failed", zap.Error(err))
		rsp.Error = ierr.ToBizErrorLocalized(ctx, err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}
	rsp.AuditCount = counts.AuditCount
	rsp.SessionCount = counts.SessionCount
	rsp.MessageCount = counts.MessageCount
	return apiutil.WrapHTTPResponse(rsp, nil)
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
	scope, err := scopeFor(ctx, perm)
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
		Pricing:         port.PricingToDTO(v.Pricing),
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

// HandlePrefillPricing models.dev 定价导入（仅填充表单，未命中/失败降级为 found=false）
//
//	@receiver h *modelHandler
//	@param ctx context.Context
//	@param req *dto.ModelPricingPrefillReq
//	@return *dto.HTTPResponse[*dto.ModelPricingPrefillRsp]
//	@return error
//	@author centonhuang
//	@update 2026-10-07 10:00:00
func (h *modelHandler) HandlePrefillPricing(ctx context.Context, req *dto.ModelPricingPrefillReq) (*dto.HTTPResponse[*dto.ModelPricingPrefillRsp], error) {
	rsp := &dto.ModelPricingPrefillRsp{}
	res, err := h.prefill.Handle(ctx, port.PrefillPricingQuery{UpstreamModel: req.UpstreamModel})
	// 未命中/失败统一降级为 found=false（录入不被阻塞），因此只走正向分支
	if err == nil && res != nil && res.Found {
		rsp.Found = true
		rsp.Currency = res.Currency
		rsp.Pricing = &dto.PricingDTO{Currency: res.Currency, Rules: res.Rules}
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}
