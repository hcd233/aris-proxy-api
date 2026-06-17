package command

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/robfig/cron/v3"
)

// updateCronJobHandler 更新 CronJob 处理器
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type updateCronJobHandler struct {
	repo    port.CronJobRepository
	manager port.CronManager
}

// NewUpdateCronJobHandler 构造更新 CronJob 处理器
//
//	@param repo port.CronJobRepository
//	@param manager port.CronManager
//	@return port.UpdateCronJobHandler
func NewUpdateCronJobHandler(repo port.CronJobRepository, manager port.CronManager) port.UpdateCronJobHandler {
	return &updateCronJobHandler{repo: repo, manager: manager}
}

// Handle 处理更新 CronJob 请求
//
//	@receiver h *updateCronJobHandler
//	@param ctx context.Context
//	@param name string
//	@param params port.UpdateCronJobParams
//	@return error
func (h *updateCronJobHandler) Handle(ctx context.Context, name string, params port.UpdateCronJobParams) error {
	// 校验 spec 合法性
	if params.Spec != nil {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(*params.Spec); err != nil {
			return ierr.New(ierr.ErrValidation, "invalid cron spec: "+*params.Spec)
		}
	}

	// 查询当前任务信息，校验核心任务不允许关闭
	if params.Enabled != nil && !*params.Enabled {
		job, err := h.repo.Get(ctx, name)
		if err != nil {
			return err
		}
		if job.Type == constant.CronTypeCore {
			return ierr.New(ierr.ErrValidation, "core cron job cannot be disabled")
		}
	}

	// DB 更新
	if err := h.repo.Update(ctx, name, params); err != nil {
		return err
	}

	// 运行时热重载
	if h.manager != nil {
		job, err := h.repo.Get(ctx, name)
		if err != nil {
			return err
		}

		if !job.Enabled {
			return h.manager.Disable(name)
		}

		specChanged := params.Spec != nil
		enabledFromFalse := params.Enabled != nil && *params.Enabled

		if specChanged {
			return h.manager.Restart(name, job.Spec)
		}
		if enabledFromFalse {
			return h.manager.Enable(name, job.Spec)
		}
	}

	return nil
}
