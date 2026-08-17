package query

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
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
	demoScope               demoport.DemoScopeProvider
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
	demoScope demoport.DemoScopeProvider,
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
		demoScope:               demoScope,
	}
}

// resolveSampleModulus 解析 demo 视角抽样模数；非 demo 用户零开销返回 0
func (s *auditService) resolveSampleModulus(ctx context.Context, permission enum.Permission) (uint, error) {
	if permission != enum.PermissionDemo {
		return 0, nil
	}
	return s.demoScope.SampleModulus(ctx)
}

func (s *auditService) ListLogs(ctx context.Context, permission enum.Permission, userID uint, p port.ListAuditLogsParams) ([]*port.AuditLogView, *model.PageInfo, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		modulus, err := s.resolveSampleModulus(ctx, permission)
		if err != nil {
			return nil, nil, err
		}
		views, pageInfo, err := s.listAll.Handle(ctx, ListAllAuditLogsQuery{
			Page:          p.Page,
			PageSize:      p.PageSize,
			Query:         p.Query,
			Sort:          p.Sort,
			SortField:     p.SortField,
			StartTime:     p.StartTime,
			EndTime:       p.EndTime,
			Filter:        p.Filter,
			SampleModulus: modulus,
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
	return lo.Map(views, func(v *AuditLogView, _ int) *port.AuditLogView {
		return &port.AuditLogView{
			ID:                       v.ID,
			CreatedAt:                v.CreatedAt,
			ModelID:                  v.ModelID,
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
		}
	})
}

func (s *auditService) ListAuditOption(ctx context.Context, permission enum.Permission, field, keyword string, startTime, endTime time.Time) ([]string, error) {
	modulus, err := s.resolveSampleModulus(ctx, permission)
	if err != nil {
		return nil, err
	}
	return s.listAuditOption.Handle(ctx, ListAuditOptionQuery{
		Field:         field,
		Keyword:       keyword,
		StartTime:     startTime,
		EndTime:       endTime,
		SampleModulus: modulus,
	})
}

func (s *auditService) ModelTrend(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		modulus, err := s.resolveSampleModulus(ctx, permission)
		if err != nil {
			return nil, err
		}
		return s.modelTrend.Handle(ctx, ModelTrendQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity, SampleModulus: modulus})
	case enum.PermissionUser:
		return s.modelTrendByUser.Handle(ctx, ModelTrendByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) RequestRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		modulus, err := s.resolveSampleModulus(ctx, permission)
		if err != nil {
			return nil, err
		}
		return s.requestRate.Handle(ctx, RequestRateQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity, SampleModulus: modulus})
	case enum.PermissionUser:
		return s.requestRateByUser.Handle(ctx, RequestRateByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		modulus, err := s.resolveSampleModulus(ctx, permission)
		if err != nil {
			return nil, err
		}
		return s.tokenThroughput.Handle(ctx, TokenThroughputQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity, SampleModulus: modulus})
	case enum.PermissionUser:
		return s.tokenThroughputByUser.Handle(ctx, TokenThroughputByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		modulus, err := s.resolveSampleModulus(ctx, permission)
		if err != nil {
			return nil, err
		}
		return s.tokenRate.Handle(ctx, TokenRateQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity, SampleModulus: modulus})
	case enum.PermissionUser:
		return s.tokenRateByUser.Handle(ctx, TokenRateByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) ModelUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.ModelUsageItem, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		modulus, err := s.resolveSampleModulus(ctx, permission)
		if err != nil {
			return nil, err
		}
		return s.modelUsage.Handle(ctx, ModelUsageQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity, SampleModulus: modulus})
	case enum.PermissionUser:
		return s.modelUsageByUser.Handle(ctx, ModelUsageByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) FirstTokenLatency(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.FirstTokenLatencyItem, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		modulus, err := s.resolveSampleModulus(ctx, permission)
		if err != nil {
			return nil, err
		}
		return s.firstTokenLatency.Handle(ctx, FirstTokenLatencyQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity, SampleModulus: modulus})
	case enum.PermissionUser:
		return s.firstTokenLatencyByUser.Handle(ctx, FirstTokenLatencyByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}
