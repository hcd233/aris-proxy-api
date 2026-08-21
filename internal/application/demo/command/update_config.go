// Package command Demo 演示账户应用命令处理器
package command

import (
	"context"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

type updateDemoConfigHandler struct {
	repo port.DemoConfigRepository
}

// NewUpdateDemoConfigHandler 构造
//
//	@param repo port.DemoConfigRepository
//	@return port.UpdateDemoConfigHandler
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewUpdateDemoConfigHandler(repo port.DemoConfigRepository) port.UpdateDemoConfigHandler {
	return &updateDemoConfigHandler{repo: repo}
}

// Handle 更新 Demo 配置（nil 字段不修改）
//
// 规则：
//
//   - Modules 每项必须是合法 DemoModule 枚举
//
//     @receiver h *updateDemoConfigHandler
//     @param ctx context.Context
//     @param cmd port.UpdateDemoConfigCommand
//     @return *port.DemoConfigView
//     @return error
//     @author centonhuang
//     @update 2026-08-16 10:00:00
func (h *updateDemoConfigHandler) Handle(ctx context.Context, cmd port.UpdateDemoConfigCommand) (*port.DemoConfigView, error) {
	log := logger.WithCtx(ctx)

	if invalid := lo.Filter(cmd.Modules, func(m enum.DemoModule, _ int) bool {
		return !enum.IsValidDemoModule(m)
	}); len(invalid) > 0 {
		return nil, ierr.Newf(ierr.ErrValidation, "invalid demo module: %v", invalid)
	}

	entity, err := h.repo.Get(ctx)
	if err != nil {
		log.Error("[DemoCommand] Read demo config failed", zap.Error(err))
		return nil, err
	}

	if cmd.LoginEnabled != nil {
		entity.LoginEnabled = *cmd.LoginEnabled
	}
	if cmd.Modules != nil {
		entity.Modules = lo.Uniq(cmd.Modules)
	}
	entity.UpdatedAt = time.Now().UTC()

	if err := h.repo.Save(ctx, entity); err != nil {
		log.Error("[DemoCommand] Save demo config failed", zap.Error(err))
		return nil, err
	}

	log.Info("[DemoCommand] Update demo config",
		zap.Bool("loginEnabled", entity.LoginEnabled),
		zap.Strings("modules", entity.Modules))
	return &port.DemoConfigView{
		LoginEnabled: entity.LoginEnabled,
		Modules:      entity.Modules,
		UpdatedAt:    entity.UpdatedAt,
	}, nil
}
