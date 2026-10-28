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
)

type auditService struct {
	listAll         ListAllAuditLogsHandler
	listByUser      ListAuditLogsByUserHandler
	listAuditOption ListAuditOptionHandler
}

// NewAuditService 构造权限派发服务。
func NewAuditService(
	listAll ListAllAuditLogsHandler,
	listByUser ListAuditLogsByUserHandler,
	listAuditOption ListAuditOptionHandler,
) port.AuditService {
	return &auditService{
		listAll:         listAll,
		listByUser:      listByUser,
		listAuditOption: listAuditOption,
	}
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
