package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
)

// updateCronJobHandler 更新 CronJob 处理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type updateCronJobHandler struct{ repo port.CronJobRepository }

// NewUpdateCronJobHandler 构造更新 CronJob 处理器
//
//	@param repo port.CronJobRepository
//	@return port.UpdateCronJobHandler
func NewUpdateCronJobHandler(repo port.CronJobRepository) port.UpdateCronJobHandler {
	return &updateCronJobHandler{repo: repo}
}

// Handle 处理更新 CronJob 请求
//
//	@receiver h *updateCronJobHandler
//	@param ctx context.Context
//	@param name string
//	@param enabled bool
//	@return error
func (h *updateCronJobHandler) Handle(ctx context.Context, name string, enabled bool) error {
	return h.repo.Update(ctx, name, enabled)
}
