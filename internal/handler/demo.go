// Package handler Demo 演示账户处理器
package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// DemoHandler Demo 演示账户处理器
//
//	@author centonhuang
//	@update 2026-08-16 10:00:00
type DemoHandler interface {
	HandleLogin(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.DemoLoginRsp], error)
	HandleStatus(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.DemoStatusRsp], error)
	HandleGetConfig(ctx context.Context, req *dto.EmptyReq) (*dto.HTTPResponse[*dto.GetDemoConfigRsp], error)
	HandleUpdateConfig(ctx context.Context, req *dto.UpdateDemoConfigReq) (*dto.HTTPResponse[*dto.GetDemoConfigRsp], error)
	HandleListDemoSessions(ctx context.Context, req *dto.ListDemoSessionsReq) (*dto.HTTPResponse[*dto.ListDemoSessionsRsp], error)
	HandleAddDemoSessions(ctx context.Context, req *dto.AddDemoSessionsReq) (*dto.HTTPResponse[*dto.ListDemoSessionsRsp], error)
	HandleRemoveDemoSessions(ctx context.Context, req *dto.RemoveDemoSessionsReq) (*dto.HTTPResponse[*dto.RemoveDemoSessionsRsp], error)
}

// DemoHandlerDependencies DemoHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-08-16 10:00:00
type DemoHandlerDependencies struct {
	Login              port.DemoLoginHandler
	Status             port.DemoStatusHandler
	GetConfig          port.GetDemoConfigHandler
	UpdateConfig       port.UpdateDemoConfigHandler
	ListDemoSessions   port.ListDemoSessionsHandler
	AddDemoSessions    port.AddDemoSessionsHandler
	RemoveDemoSessions port.RemoveDemoSessionsHandler
}

type demoHandler struct {
	login              port.DemoLoginHandler
	status             port.DemoStatusHandler
	getConfig          port.GetDemoConfigHandler
	updateConfig       port.UpdateDemoConfigHandler
	listDemoSessions   port.ListDemoSessionsHandler
	addDemoSessions    port.AddDemoSessionsHandler
	removeDemoSessions port.RemoveDemoSessionsHandler
}

// NewDemoHandler 创建 Demo 处理器
//
//	@param deps DemoHandlerDependencies 依赖项（由调用方注入）
//	@return DemoHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewDemoHandler(deps DemoHandlerDependencies) DemoHandler {
	return &demoHandler{
		login:              deps.Login,
		status:             deps.Status,
		getConfig:          deps.GetConfig,
		updateConfig:       deps.UpdateConfig,
		listDemoSessions:   deps.ListDemoSessions,
		addDemoSessions:    deps.AddDemoSessions,
		removeDemoSessions: deps.RemoveDemoSessions,
	}
}

// HandleLogin Demo 账户登录（无需 OAuth）
//
//	@receiver h *demoHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.DemoLoginRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *demoHandler) HandleLogin(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.DemoLoginRsp], error) {
	rsp := &dto.DemoLoginRsp{}
	result, err := h.login.Handle(ctx, port.DemoLoginCommand{})
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoHandler] Demo login failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.AccessToken = result.AccessToken
	rsp.RefreshToken = result.RefreshToken
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleStatus Demo 登录入口状态（无需鉴权）
//
//	@receiver h *demoHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.DemoStatusRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *demoHandler) HandleStatus(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.DemoStatusRsp], error) {
	rsp := &dto.DemoStatusRsp{}
	result, err := h.status.Handle(ctx, port.DemoStatusQuery{})
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoHandler] Demo status failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.LoginEnabled = result.LoginEnabled
	rsp.DemoUserExists = result.DemoUserExists
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleGetConfig 读取 Demo 配置（登录用户均可）
//
//	@receiver h *demoHandler
//	@param ctx context.Context
//	@param req *dto.EmptyReq
//	@return *dto.HTTPResponse[*dto.GetDemoConfigRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *demoHandler) HandleGetConfig(ctx context.Context, _ *dto.EmptyReq) (*dto.HTTPResponse[*dto.GetDemoConfigRsp], error) {
	rsp := &dto.GetDemoConfigRsp{}
	view, err := h.getConfig.Handle(ctx, port.GetDemoConfigQuery{})
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoHandler] Get demo config failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.Config = toDemoConfigDTO(view)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleUpdateConfig 更新 Demo 配置（admin）
//
//	@receiver h *demoHandler
//	@param ctx context.Context
//	@param req *dto.UpdateDemoConfigReq
//	@return *dto.HTTPResponse[*dto.GetDemoConfigRsp]
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *demoHandler) HandleUpdateConfig(ctx context.Context, req *dto.UpdateDemoConfigReq) (*dto.HTTPResponse[*dto.GetDemoConfigRsp], error) {
	rsp := &dto.GetDemoConfigRsp{}
	body := req.Body.Config
	view, err := h.updateConfig.Handle(ctx, port.UpdateDemoConfigCommand{
		LoginEnabled:  body.LoginEnabled,
		SampleModulus: body.SampleModulus,
		Modules:       toDemoModules(body.Modules),
	})
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoHandler] Update demo config failed", zap.Error(err))
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.Config = toDemoConfigDTO(view)
	return apiutil.WrapHTTPResponse(rsp, nil)
}

func toDemoConfigDTO(view *port.DemoConfigView) *dto.DemoConfig {
	modules := make([]string, 0, len(view.Modules))
	modules = append(modules, view.Modules...)
	return &dto.DemoConfig{
		LoginEnabled:  view.LoginEnabled,
		SampleModulus: view.SampleModulus,
		Modules:       modules,
		UpdatedAt:     view.UpdatedAt,
	}
}

func toDemoModules(modules []string) []enum.DemoModule {
	result := make([]enum.DemoModule, 0, len(modules))
	result = append(result, modules...)
	return result
}

// HandleListDemoSessions 列出白名单会话（admin）
func (h *demoHandler) HandleListDemoSessions(ctx context.Context, req *dto.ListDemoSessionsReq) (*dto.HTTPResponse[*dto.ListDemoSessionsRsp], error) {
	rsp := &dto.ListDemoSessionsRsp{}
	views, pageInfo, err := h.listDemoSessions.Handle(ctx, port.ListDemoSessionsQuery{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	rsp.Sessions = lo.Map(views, func(v *port.DemoSessionView, _ int) *dto.DemoSession {
		return &dto.DemoSession{ID: v.ID, Summary: v.Summary, MessageCount: v.MessageCount, ToolCount: v.ToolCount, CreatedAt: v.CreatedAt}
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// HandleAddDemoSessions 批量添加白名单会话（admin）
func (h *demoHandler) HandleAddDemoSessions(ctx context.Context, req *dto.AddDemoSessionsReq) (*dto.HTTPResponse[*dto.ListDemoSessionsRsp], error) {
	_, err := h.addDemoSessions.Handle(ctx, port.AddDemoSessionsCommand{SessionIDs: req.Body.SessionIDs})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return h.HandleListDemoSessions(ctx, &dto.ListDemoSessionsReq{Page: 1, PageSize: 100})
}

// HandleRemoveDemoSessions 批量移除白名单会话（admin）
func (h *demoHandler) HandleRemoveDemoSessions(ctx context.Context, req *dto.RemoveDemoSessionsReq) (*dto.HTTPResponse[*dto.RemoveDemoSessionsRsp], error) {
	err := h.removeDemoSessions.Handle(ctx, port.RemoveDemoSessionsCommand{SessionIDs: req.IDs})
	if err != nil {
		return nil, apiutil.NewHumaBizError(ctx, err, ierr.ErrInternal.BizError())
	}
	return apiutil.WrapHTTPResponse(&dto.RemoveDemoSessionsRsp{}, nil)
}
