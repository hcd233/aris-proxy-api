package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// AuditService 统一按当前 JWT 权限派发审计查询，handler 只负责 DTO 映射。
type AuditService interface {
	ListLogs(ctx context.Context, permission enum.Permission, userID uint, q ListAuditLogsParams) ([]*AuditLogView, *model.PageInfo, error)
	ModelTrend(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error)
	RequestRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error)
	TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error)
	TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error)
	ModelUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.ModelUsageItem, error)
	FirstTokenLatency(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.FirstTokenLatencyItem, error)
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
	listAll                 ListAllAuditLogsHandler
	listByUser              ListAuditLogsByUserHandler
	modelTrend              ModelTrendHandler
	modelTrendByUser        ModelTrendByUserHandler
	requestRate             RequestRateHandler
	requestRateByUser       RequestRateByUserHandler
	tokenThroughput         TokenThroughputHandler
	tokenThroughputByUser   TokenThroughputByUserHandler
	tokenRate               TokenRateHandler
	tokenRateByUser         TokenRateByUserHandler
	modelUsage              ModelUsageHandler
	modelUsageByUser        ModelUsageByUserHandler
	firstTokenLatency       FirstTokenLatencyHandler
	firstTokenLatencyByUser FirstTokenLatencyByUserHandler
}

// NewAuditService 构造权限派发服务。
func NewAuditService(
	listAll ListAllAuditLogsHandler,
	listByUser ListAuditLogsByUserHandler,
	modelTrend ModelTrendHandler,
	modelTrendByUser ModelTrendByUserHandler,
	requestRate RequestRateHandler,
	requestRateByUser RequestRateByUserHandler,
	tokenThroughput TokenThroughputHandler,
	tokenThroughputByUser TokenThroughputByUserHandler,
	tokenRate TokenRateHandler,
	tokenRateByUser TokenRateByUserHandler,
	modelUsage ModelUsageHandler,
	modelUsageByUser ModelUsageByUserHandler,
	firstTokenLatency FirstTokenLatencyHandler,
	firstTokenLatencyByUser FirstTokenLatencyByUserHandler,
) AuditService {
	return &auditService{
		listAll:                 listAll,
		listByUser:              listByUser,
		modelTrend:              modelTrend,
		modelTrendByUser:        modelTrendByUser,
		requestRate:             requestRate,
		requestRateByUser:       requestRateByUser,
		tokenThroughput:         tokenThroughput,
		tokenThroughputByUser:   tokenThroughputByUser,
		tokenRate:               tokenRate,
		tokenRateByUser:         tokenRateByUser,
		modelUsage:              modelUsage,
		modelUsageByUser:        modelUsageByUser,
		firstTokenLatency:       firstTokenLatency,
		firstTokenLatencyByUser: firstTokenLatencyByUser,
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

func (s *auditService) TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.tokenThroughput.Handle(ctx, TokenThroughputQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.tokenThroughputByUser.Handle(ctx, TokenThroughputByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.tokenRate.Handle(ctx, TokenRateQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.tokenRateByUser.Handle(ctx, TokenRateByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) ModelUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.ModelUsageItem, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.modelUsage.Handle(ctx, ModelUsageQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.modelUsageByUser.Handle(ctx, ModelUsageByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) FirstTokenLatency(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.FirstTokenLatencyItem, error) {
	switch permission {
	case enum.PermissionAdmin:
		return s.firstTokenLatency.Handle(ctx, FirstTokenLatencyQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.firstTokenLatencyByUser.Handle(ctx, FirstTokenLatencyByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}
