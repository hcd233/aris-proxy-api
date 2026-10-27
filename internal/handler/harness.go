package handler

import (
	"context"

	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	harnessport "github.com/hcd233/aris-proxy-api/internal/application/agentharness/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// HarnessHandler harness 规则管理 handler
type HarnessHandler interface {
	HandleListHarnessRules(ctx context.Context, req *dto.ListHarnessRulesReq) (*dto.HTTPResponse[*dto.ListHarnessRulesRsp], error)
	HandleCreateHarnessRule(ctx context.Context, req *dto.CreateHarnessRuleReq) (*dto.HTTPResponse[*dto.CreateHarnessRuleRsp], error)
	HandleUpdateHarnessRule(ctx context.Context, req *dto.UpdateHarnessRuleReq) (*dto.HTTPResponse[*dto.UpdateHarnessRuleRsp], error)
	HandleDeleteHarnessRule(ctx context.Context, req *dto.DeleteHarnessRuleReq) (*dto.HTTPResponse[*dto.DeleteHarnessRuleRsp], error)
	HandlePreviewHarness(ctx context.Context, req *dto.PreviewHarnessReq) (*dto.HTTPResponse[*dto.PreviewHarnessRsp], error)
}

// HarnessDependencies harness handler 依赖
type HarnessDependencies struct {
	List    harnessport.ListHarnessRulesHandler
	Create  harnessport.CreateHarnessRuleHandler
	Update  harnessport.UpdateHarnessRuleHandler
	Delete  harnessport.DeleteHarnessRuleHandler
	Preview harnessport.PreviewHarnessHandler
}

type harnessHandler struct {
	list    harnessport.ListHarnessRulesHandler
	create  harnessport.CreateHarnessRuleHandler
	update  harnessport.UpdateHarnessRuleHandler
	delete  harnessport.DeleteHarnessRuleHandler
	preview harnessport.PreviewHarnessHandler
}

// NewHarnessHandler 构造 harness handler
func NewHarnessHandler(deps HarnessDependencies) HarnessHandler {
	return &harnessHandler{
		list:    deps.List,
		create:  deps.Create,
		update:  deps.Update,
		delete:  deps.Delete,
		preview: deps.Preview,
	}
}

// HandleListHarnessRules 规则列表
func (h *harnessHandler) HandleListHarnessRules(ctx context.Context, _ *dto.ListHarnessRulesReq) (*dto.HTTPResponse[*dto.ListHarnessRulesRsp], error) {
	views, err := h.list.Handle(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[HarnessHandler] List rules failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp := &dto.ListHarnessRulesRsp{Rules: make([]*dto.HarnessRuleItem, 0, len(views))}
	for _, view := range views {
		rsp.Rules = append(rsp.Rules, toHarnessRuleItem(view))
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleCreateHarnessRule 新增规则
func (h *harnessHandler) HandleCreateHarnessRule(ctx context.Context, req *dto.CreateHarnessRuleReq) (*dto.HTTPResponse[*dto.CreateHarnessRuleRsp], error) {
	view, err := h.create.Handle(ctx, harnessport.CreateHarnessRuleCommand{
		Pattern: req.Body.Pattern,
		Name:    req.Body.Name,
		Sort:    req.Body.Sort,
	})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.CreateHarnessRuleRsp{Rule: toHarnessRuleItem(view)}, nil)
}

// HandleUpdateHarnessRule 修改规则
func (h *harnessHandler) HandleUpdateHarnessRule(ctx context.Context, req *dto.UpdateHarnessRuleReq) (*dto.HTTPResponse[*dto.UpdateHarnessRuleRsp], error) {
	view, err := h.update.Handle(ctx, harnessport.UpdateHarnessRuleCommand{
		ID:      req.ID,
		Pattern: req.Body.Pattern,
		Name:    req.Body.Name,
		Sort:    req.Body.Sort,
	})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.UpdateHarnessRuleRsp{Rule: toHarnessRuleItem(view)}, nil)
}

// HandleDeleteHarnessRule 删除规则
func (h *harnessHandler) HandleDeleteHarnessRule(ctx context.Context, req *dto.DeleteHarnessRuleReq) (*dto.HTTPResponse[*dto.DeleteHarnessRuleRsp], error) {
	if err := h.delete.Handle(ctx, harnessport.DeleteHarnessRuleCommand{ID: req.ID}); err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.DeleteHarnessRuleRsp{}, nil)
}

// HandlePreviewHarness 用真实规则解析一条 UA
func (h *harnessHandler) HandlePreviewHarness(ctx context.Context, req *dto.PreviewHarnessReq) (*dto.HTTPResponse[*dto.PreviewHarnessRsp], error) {
	result, err := h.preview.Handle(ctx, harnessport.PreviewHarnessQuery{UA: req.Body.UA})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp := &dto.PreviewHarnessRsp{Harness: result.Harness}
	if result.Rule != nil {
		rsp.Rule = toHarnessRuleItem(result.Rule)
	}
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func toHarnessRuleItem(view *harnessport.HarnessRuleView) *dto.HarnessRuleItem {
	if view == nil {
		return nil
	}
	return &dto.HarnessRuleItem{
		ID:      view.ID,
		Pattern: view.Pattern,
		Name:    view.Name,
		Sort:    view.Sort,
	}
}
