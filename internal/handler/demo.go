// Package handler Demo 演示账户处理器
package handler

import (
	"context"

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
}

// DemoHandlerDependencies DemoHandler 依赖项（用于依赖注入）
//
//	@author centonhuang
//	@update 2026-08-16 10:00:00
type DemoHandlerDependencies struct {
	Login        port.DemoLoginHandler
	Status       port.DemoStatusHandler
	GetConfig    port.GetDemoConfigHandler
	UpdateConfig port.UpdateDemoConfigHandler
}

type demoHandler struct {
	login        port.DemoLoginHandler
	status       port.DemoStatusHandler
	getConfig    port.GetDemoConfigHandler
	updateConfig port.UpdateDemoConfigHandler
}

// NewDemoHandler 创建 Demo 处理器
//
//	@param deps DemoHandlerDependencies 依赖项（由调用方注入）
//	@return DemoHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewDemoHandler(deps DemoHandlerDependencies) DemoHandler {
	return &demoHandler{
		login:        deps.Login,
		status:       deps.Status,
		getConfig:    deps.GetConfig,
		updateConfig: deps.UpdateConfig,
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
