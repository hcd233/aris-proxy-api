package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

// CronJobView CronJob 展示视图
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJobView struct {
	Name        string
	Spec        string
	Description string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
	Handle(ctx context.Context, name string, enabled bool) error
}

// CronJobRepository CronJob 仓储接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronJobRepository interface {
	Sync(ctx context.Context, jobs []*CronJobView) error
	List(ctx context.Context, param dao.CommonParam) ([]*CronJobView, *model.PageInfo, error)
	Update(ctx context.Context, name string, enabled bool) error
	Get(ctx context.Context, name string) (*CronJobView, error)
}
