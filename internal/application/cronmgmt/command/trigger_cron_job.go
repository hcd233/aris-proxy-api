package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// triggerCronJobHandler 手动触发 CronJob 处理器
//
//	@author centonhuang
//	@update 2026-08-05 10:00:00
type triggerCronJobHandler struct {
	manager port.CronManager
}

// NewTriggerCronJobHandler 构造手动触发 CronJob 处理器
//
//	@param manager port.CronManager
//	@return port.TriggerCronJobHandler
func NewTriggerCronJobHandler(manager port.CronManager) port.TriggerCronJobHandler {
	return &triggerCronJobHandler{manager: manager}
}

// Handle 处理手动触发 CronJob 请求
//
//	@receiver h *triggerCronJobHandler
//	@param ctx context.Context
//	@param name string
//	@return error
func (h *triggerCronJobHandler) Handle(ctx context.Context, name string) error {
	if h.manager == nil {
		return ierr.New(ierr.ErrInternal, "cron manager not initialized")
	}
	return h.manager.Trigger(name)
}
