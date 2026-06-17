package port

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
)

// CronCallAuditView CronCallAudit 展示视图
//
//	@author centonhuang
//	@update 2026-06-18 10:00:00
type CronCallAuditView struct {
	ID         uint
	CronName   string
	TraceID    string
	StartedAt  time.Time
	EndedAt    time.Time
	DurationMs int64
	Status     string
	Message    string
	Metadata   map[string]any
	CreatedAt  time.Time
}

// ListCronCallAuditsHandler 列出 CronCallAudit 处理器接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type ListCronCallAuditsHandler interface {
	Handle(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, filter string) ([]*CronCallAuditView, *model.PageInfo, error)
}

// ListCronCallAuditOptionsHandler 获取 CronCallAudit 筛选项处理器接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type ListCronCallAuditOptionsHandler interface {
	Handle(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error)
}

// CronCallAuditRepository CronCallAudit 仓储接口
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type CronCallAuditRepository interface {
	Save(ctx context.Context, audit *CronCallAuditView) error
	List(ctx context.Context, param dao.CommonParam, startTime, endTime time.Time, filterExp string) ([]*CronCallAuditView, *model.PageInfo, error)
	ListDistinctTypes(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
	ListDistinctStatuses(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
}
