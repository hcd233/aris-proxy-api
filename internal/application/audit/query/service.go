package query

import (
	"context"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/dto"
)

type auditService struct {
	listAll                 ListAllAuditLogsHandler
	listByUser              ListAuditLogsByUserHandler
	listAuditOption         ListAuditOptionHandler
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
	listAuditOption ListAuditOptionHandler,
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
) port.AuditService {
	return &auditService{
		listAll:                 listAll,
		listByUser:              listByUser,
		listAuditOption:         listAuditOption,
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

func (s *auditService) ListLogs(ctx context.Context, permission enum.Permission, userID uint, p port.ListAuditLogsParams) ([]*port.AuditLogView, *model.PageInfo, error) {
	switch permission {
	case enum.PermissionAdmin:
		views, pageInfo, err := s.listAll.Handle(ctx, ListAllAuditLogsQuery{
			Page:      p.Page,
			PageSize:  p.PageSize,
			Query:     p.Query,
			Sort:      p.Sort,
			SortField: p.SortField,
			StartTime: p.StartTime,
			EndTime:   p.EndTime,
			Filter:    p.Filter,
		})
		if err != nil {
			return nil, nil, err
		}
		return toPortAuditLogViews(views), pageInfo, nil
	case enum.PermissionUser:
		views, pageInfo, err := s.listByUser.Handle(ctx, ListAuditLogsByUserQuery{
			UserID:    userID,
			Page:      p.Page,
			PageSize:  p.PageSize,
			Query:     p.Query,
			Sort:      p.Sort,
			SortField: p.SortField,
			StartTime: p.StartTime,
			EndTime:   p.EndTime,
			Filter:    p.Filter,
		})
		if err != nil {
			return nil, nil, err
		}
		return toPortAuditLogViews(views), pageInfo, nil
	default:
		return nil, nil, ierr.ErrUnauthorized
	}
}

func toPortAuditLogViews(views []*AuditLogView) []*port.AuditLogView {
	result := make([]*port.AuditLogView, 0, len(views))
	for _, v := range views {
		result = append(result, &port.AuditLogView{
			ID:                       v.ID,
			CreatedAt:                v.CreatedAt,
			Model:                    v.Model,
			UpstreamProtocol:         v.UpstreamProtocol,
			APIProtocol:              v.APIProtocol,
			Endpoint:                 v.Endpoint,
			InputTokens:              v.InputTokens,
			OutputTokens:             v.OutputTokens,
			CacheCreationInputTokens: v.CacheCreationInputTokens,
			CacheReadInputTokens:     v.CacheReadInputTokens,
			FirstTokenLatencyMs:      v.FirstTokenLatencyMs,
			StreamDurationMs:         v.StreamDurationMs,
			UserAgent:                v.UserAgent,
			UpstreamStatusCode:       v.UpstreamStatusCode,
			ErrorMessage:             v.ErrorMessage,
			TraceID:                  v.TraceID,
			APIKeyName:               v.APIKeyName,
			UserName:                 v.UserName,
			UserEmail:                v.UserEmail,
		})
	}
	return result
}

func (s *auditService) ListAuditOption(ctx context.Context, field, keyword string, startTime, endTime time.Time) ([]string, error) {
	return s.listAuditOption.Handle(ctx, ListAuditOptionQuery{Field: field, Keyword: keyword, StartTime: startTime, EndTime: endTime})
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
