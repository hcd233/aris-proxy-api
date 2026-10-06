package query

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
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
	costSummary             CostSummaryHandler
	costSummaryByUser       CostSummaryByUserHandler
	costDistribution        CostDistributionHandler
	costDistributionByUser  CostDistributionByUserHandler
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
	costSummary CostSummaryHandler,
	costSummaryByUser CostSummaryByUserHandler,
	costDistribution CostDistributionHandler,
	costDistributionByUser CostDistributionByUserHandler,
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
		costSummary:             costSummary,
		costSummaryByUser:       costSummaryByUser,
		costDistribution:        costDistribution,
		costDistributionByUser:  costDistributionByUser,
	}
}

// CostSummary 估算费用合计与趋势：admin/demo 全量视角，user 限定名下 key。
func (s *auditService) CostSummary(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) (*port.CostSummaryView, error) {
	var result *modelcall.CostSummaryResult
	var err error
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		result, err = s.costSummary.Handle(ctx, CostSummaryQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		result, err = s.costSummaryByUser.Handle(ctx, CostSummaryByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	return &port.CostSummaryView{
		Totals: lo.Map(result.Totals, func(t *modelcall.CostTotal, _ int) *dto.AuditCostTotalItem {
			return &dto.AuditCostTotalItem{
				Currency: enum.Currency(t.Currency),
				Cost:     dto.PriceDisplayFromMicro(t.CostMicro),
			}
		}),
		Series: lo.Map(result.Series, func(p *modelcall.CostPoint, _ int) *dto.AuditCostSeriesPoint {
			return &dto.AuditCostSeriesPoint{
				BucketTime: p.Time.UTC(),
				Currency:   enum.Currency(p.Currency),
				Cost:       dto.PriceDisplayFromMicro(p.CostMicro),
			}
		}),
	}, nil
}

// CostDistribution 估算费用分布：admin/demo 全量视角，user 限定名下 key（user 维度仅 admin）。
func (s *auditService) CostDistribution(ctx context.Context, permission enum.Permission, userID uint, groupBy string, startTime, endTime time.Time, limit int) (*port.CostDistributionView, error) {
	group := enum.CostGroupBy(groupBy)
	var result *modelcall.CostDistributionResult
	var err error
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		result, err = s.costDistribution.Handle(ctx, CostDistributionQuery{GroupBy: group, StartTime: startTime, EndTime: endTime, Limit: limit})
	case enum.PermissionUser:
		result, err = s.costDistributionByUser.Handle(ctx, CostDistributionByUserQuery{UserID: userID, GroupBy: group, StartTime: startTime, EndTime: endTime, Limit: limit})
	default:
		return nil, ierr.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}
	return &port.CostDistributionView{
		GroupBy: result.GroupBy,
		Items: lo.Map(result.Items, func(p *modelcall.CostDistributionPoint, _ int) *dto.AuditCostDistributionItem {
			return &dto.AuditCostDistributionItem{
				ID:       p.ID,
				Name:     p.Name,
				Currency: enum.Currency(p.Currency),
				Cost:     dto.PriceDisplayFromMicro(p.CostMicro),
			}
		}),
	}, nil
}

func (s *auditService) ListLogs(ctx context.Context, permission enum.Permission, userID uint, p port.ListAuditLogsParams) ([]*port.AuditLogView, *model.PageInfo, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		views, pageInfo, err := s.listAll.Handle(ctx, ListAllAuditLogsQuery{
			Page:      p.Page,
			PageSize:  p.PageSize,
			Query:     p.Query,
			Sort:      p.Sort,
			SortField: p.SortField,
			StartTime: p.StartTime,
			EndTime:   p.EndTime,
			Filter:    p.Filter,
			IsDemo:    permission == enum.PermissionDemo,
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
			CostMicro:                v.CostMicro,
			PricingCurrency:          v.PricingCurrency,
			APIKeyName:               v.APIKeyName,
			UserName:                 v.UserName,
			UserEmail:                v.UserEmail,
		}
	})
}

func (s *auditService) ListAuditOption(ctx context.Context, permission enum.Permission, userID uint, field, keyword string, startTime, endTime time.Time) ([]string, error) {
	query := ListAuditOptionQuery{
		Field:     field,
		Keyword:   keyword,
		StartTime: startTime,
		EndTime:   endTime,
	}
	// user 视角按名下 key 范围过滤选项（admin/demo 全量，demo 的身份脱敏在下方统一处理）
	if permission == enum.PermissionUser {
		query.UserID = &userID
	}
	items, err := s.listAuditOption.Handle(ctx, query)
	if err != nil {
		return nil, err
	}
	if permission == enum.PermissionDemo && field == constant.AuditFilterFieldUser {
		return lo.Uniq(lo.Map(items, func(item string, _ int) string {
			return commonutil.MaskIdentity(item)
		})), nil
	}
	return items, nil
}

func (s *auditService) ModelTrend(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		return s.modelTrend.Handle(ctx, ModelTrendQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.modelTrendByUser.Handle(ctx, ModelTrendByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) RequestRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.RequestRatePoint, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		return s.requestRate.Handle(ctx, RequestRateQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.requestRateByUser.Handle(ctx, RequestRateByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) TokenThroughput(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.TokenThroughputPoint, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		return s.tokenThroughput.Handle(ctx, TokenThroughputQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.tokenThroughputByUser.Handle(ctx, TokenThroughputByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) TokenRate(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.TokenRateItem, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		return s.tokenRate.Handle(ctx, TokenRateQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.tokenRateByUser.Handle(ctx, TokenRateByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) ModelUsage(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.ModelUsageItem, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		return s.modelUsage.Handle(ctx, ModelUsageQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.modelUsageByUser.Handle(ctx, ModelUsageByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}

func (s *auditService) FirstTokenLatency(ctx context.Context, permission enum.Permission, userID uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*dto.FirstTokenLatencyItem, error) {
	switch permission {
	case enum.PermissionAdmin, enum.PermissionDemo:
		return s.firstTokenLatency.Handle(ctx, FirstTokenLatencyQuery{StartTime: startTime, EndTime: endTime, Granularity: granularity})
	case enum.PermissionUser:
		return s.firstTokenLatencyByUser.Handle(ctx, FirstTokenLatencyByUserQuery{UserID: userID, StartTime: startTime, EndTime: endTime, Granularity: granularity})
	default:
		return nil, ierr.ErrUnauthorized
	}
}
