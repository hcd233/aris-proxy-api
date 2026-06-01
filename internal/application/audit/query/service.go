package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
)

// AuditService 统一按当前 JWT 权限派发审计查询，handler 只负责 DTO 映射。
type AuditService interface {
	ListLogs(ctx context.Context, permission enum.Permission, userID uint, q ListAuditLogsParams) ([]*AuditLogView, *model.PageInfo, error)
	ModelTrend(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error)
	RequestRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error)
}

// ListAuditLogsParams 列表查询的通用参数（不带权限相关字段）。
type ListAuditLogsParams struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
}

// toAllQuery 转换为 admin 全量查询的入参。
func (p ListAuditLogsParams) toAllQuery() ListAllAuditLogsQuery {
	return ListAllAuditLogsQuery(p)
}

// toByUserQuery 转换为 user 维度查询的入参。
func (p ListAuditLogsParams) toByUserQuery(userID uint) ListAuditLogsByUserQuery {
	return ListAuditLogsByUserQuery{
		UserID:    userID,
		Page:      p.Page,
		PageSize:  p.PageSize,
		Query:     p.Query,
		Sort:      p.Sort,
		SortField: p.SortField,
		StartTime: p.StartTime,
		EndTime:   p.EndTime,
	}
}

type auditService struct {
	listAll           ListAllAuditLogsHandler
	listByUser        ListAuditLogsByUserHandler
	modelTrend        ModelTrendHandler
	modelTrendByUser  ModelTrendByUserHandler
	requestRate       RequestRateHandler
	requestRateByUser RequestRateByUserHandler
}

// NewAuditService 构造权限派发服务。
func NewAuditService(
	listAll ListAllAuditLogsHandler,
	listByUser ListAuditLogsByUserHandler,
	modelTrend ModelTrendHandler,
	modelTrendByUser ModelTrendByUserHandler,
	requestRate RequestRateHandler,
	requestRateByUser RequestRateByUserHandler,
) AuditService {
	return &auditService{
		listAll:           listAll,
		listByUser:        listByUser,
		modelTrend:        modelTrend,
		modelTrendByUser:  modelTrendByUser,
		requestRate:       requestRate,
		requestRateByUser: requestRateByUser,
	}
}

func (s *auditService) ListLogs(ctx context.Context, permission enum.Permission, userID uint, p ListAuditLogsParams) ([]*AuditLogView, *model.PageInfo, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.listAll.Handle(ctx, p.toAllQuery())
	case enum.PermissionUser:
		return s.listByUser.Handle(ctx, p.toByUserQuery(userID))
	default:
		return nil, nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) ModelTrend(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.modelTrend.Handle(ctx, ModelTrendQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.modelTrendByUser.Handle(ctx, ModelTrendByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) RequestRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.requestRate.Handle(ctx, RequestRateQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.requestRateByUser.Handle(ctx, RequestRateByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}
