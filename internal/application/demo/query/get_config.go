// Package query Demo 演示账户应用查询处理器
package query

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
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
		LoginEnabled:  entity.LoginEnabled,
		SampleModulus: entity.SampleModulus,
		Modules:       entity.Modules,
		UpdatedAt:     entity.UpdatedAt,
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

// demoScopeProvider Demo 行为数据抽样模数提供者
type demoScopeProvider struct {
	repo port.DemoConfigRepository
}

// NewDemoScopeProvider 构造
//
//	@param repo port.DemoConfigRepository
//	@return port.DemoScopeProvider
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewDemoScopeProvider(repo port.DemoConfigRepository) port.DemoScopeProvider {
	return &demoScopeProvider{repo: repo}
}

// SampleModulus 返回抽样模数；配置读取失败返回 error（fail-closed，调用方拒绝请求）
//
//	@receiver p *demoScopeProvider
//	@param ctx context.Context
//	@return uint
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (p *demoScopeProvider) SampleModulus(ctx context.Context) (uint, error) {
	entity, err := p.repo.Get(ctx)
	if err != nil {
		logger.WithCtx(ctx).Error("[DemoQuery] Read demo config failed for sample modulus", zap.Error(err))
		return 0, err
	}
	if entity.SampleModulus < 2 {
		logger.WithCtx(ctx).Warn("[DemoQuery] Sample modulus < 2, fallback to default",
			zap.Uint("sampleModulus", entity.SampleModulus))
		return constant.DemoDefaultSampleModulus, nil
	}
	return entity.SampleModulus, nil
}
