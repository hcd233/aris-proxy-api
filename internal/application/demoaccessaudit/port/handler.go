// Package port Demo 访问审计应用的端口定义（handler 接口 + 视图 + 仓储）
package port

import (
	"context"
	"time"

	commonmodel "github.com/hcd233/aris-proxy-api/internal/common/model"
)

// DemoAccessAuditView Demo 访问审计展示视图
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAuditView struct {
	ID        uint
	Action    string
	Module    string
	Path      string
	IP        string
	UserAgent string
	Reason    string
	CreatedAt time.Time
}

// ListDemoAccessAuditsHandler 列出 DemoAccessAudit 处理器接口
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type ListDemoAccessAuditsHandler interface {
	Handle(ctx context.Context, param commonmodel.CommonParam, startTime, endTime time.Time, filterExp string) ([]*DemoAccessAuditView, *commonmodel.PageInfo, error)
}

// ListDemoAccessAuditOptionsHandler 获取 DemoAccessAudit 筛选项处理器接口
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type ListDemoAccessAuditOptionsHandler interface {
	Handle(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error)
}

// DemoAccessAuditRepository DemoAccessAudit 仓储接口（异步写路径与查询共用）
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAuditRepository interface {
	Save(ctx context.Context, view *DemoAccessAuditView) error
	List(ctx context.Context, param commonmodel.CommonParam, startTime, endTime time.Time, filterExp string) ([]*DemoAccessAuditView, *commonmodel.PageInfo, error)
	ListDistinctActions(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
	ListDistinctModules(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error)
}
