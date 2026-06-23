package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

// CronJobView CronJob 展示视图
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJobView struct {
	Name        string
	Type        string
	Spec        string
	Description string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UpdateCronJobParams 更新 CronJob 参数（部分更新，非 nil 字段才更新）
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type UpdateCronJobParams struct {
	Enabled *bool
	Spec    *string
}

// ListCronJobsHandler 列出 CronJob 处理器接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type ListCronJobsHandler interface {
	Handle(ctx context.Context, param model.CommonParam) ([]*CronJobView, *model.PageInfo, error)
}

// UpdateCronJobHandler 更新 CronJob 处理器接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type UpdateCronJobHandler interface {
	Handle(ctx context.Context, name string, params UpdateCronJobParams) error
}

// CronJobRepository CronJob 仓储接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJobRepository interface {
	Sync(ctx context.Context, jobs []*CronJobView) error
	List(ctx context.Context, param model.CommonParam) ([]*CronJobView, *model.PageInfo, error)
	Update(ctx context.Context, name string, params UpdateCronJobParams) error
	Get(ctx context.Context, name string) (*CronJobView, error)
}

// CronManager cron 实例热重载管理器接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronManager interface {
	Restart(name string, newSpec string) error
	Disable(name string) error
	Enable(name string, spec string) error
}
