// Package query Demo 演示账户应用查询处理器
package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type getDemoConfigHandler struct {
	repo port.DemoConfigRepository
}

// NewGetDemoConfigHandler 构造
//
//	@param repo port.DemoConfigRepository
//	@return port.GetDemoConfigHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewGetDemoConfigHandler(repo port.DemoConfigRepository) port.GetDemoConfigHandler {
	return &getDemoConfigHandler{repo: repo}
}

// Handle 读取 Demo 配置
//
//	@receiver h *getDemoConfigHandler
//	@param ctx context.Context
//	@param q port.GetDemoConfigQuery
//	@return *port.DemoConfigView
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (h *getDemoConfigHandler) Handle(ctx context.Context, q port.GetDemoConfigQuery) (*port.DemoConfigView, error) {
	entity, err := h.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	return toDemoConfigView(entity), nil
}

func toDemoConfigView(entity *port.DemoConfigEntity) *port.DemoConfigView {
	return &port.DemoConfigView{
		LoginEnabled: entity.LoginEnabled,
		Modules:      entity.Modules,
		UpdatedAt:    entity.UpdatedAt,
	}
}

// demoModuleAccessor Demo 模块放行判断（权限中间件依赖）
type demoModuleAccessor struct {
	repo port.DemoConfigRepository
}

// NewDemoModuleAccessor 构造
//
//	@param repo port.DemoConfigRepository
//	@return port.DemoModuleAccessor
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewDemoModuleAccessor(repo port.DemoConfigRepository) port.DemoModuleAccessor {
	return &demoModuleAccessor{repo: repo}
}

// IsModuleOpen 判断模块是否对 Demo 开放；配置读取失败 fail-closed 返回 false
//
//	@receiver a *demoModuleAccessor
//	@param ctx context.Context
//	@param module enum.DemoModule
//	@return bool
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (a *demoModuleAccessor) IsModuleOpen(ctx context.Context, module enum.DemoModule) bool {
	entity, err := a.repo.Get(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoQuery] Read demo config failed, deny module (fail-closed)",
			zap.String("module", module), zap.Error(err))
		return false
	}
	return lo.Contains(entity.Modules, module)
}
