package query

import (
	"context"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/application/audit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

var validSortFields = map[string]bool{
	constant.FieldID:                  true,
	constant.FieldCreatedAt:           true,
	constant.FieldInputTokens:         true,
	constant.FieldOutputTokens:        true,
	constant.FieldFirstTokenLatencyMs: true,
	constant.FieldStreamDurationMs:    true,
}

// auditFieldConfigs Audit filter 字段配置
var auditFieldConfigs = map[string]filter.FieldConfig{
	constant.AuditFilterFieldUser: {
		SQLColumn: constant.AuditFilterUserSQLColumn,
		IsFuzzy:   true,
	},
	constant.AuditFilterFieldModel: {
		SQLColumn: constant.AuditFilterModelSQLColumn,
		IsFuzzy:   true,
	},
	constant.AuditFilterFieldStatus: {
		SQLColumn: constant.AuditFilterStatusSQLColumn,
		IsNumeric: true,
	},
}

// AuditLogView 审计日志列表视图。
type AuditLogView struct {
	ID                       uint
	CreatedAt                time.Time
	Model                    string
	UpstreamProtocol         string
	APIProtocol              string
	Endpoint                 string
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
	FirstTokenLatencyMs      int64
	StreamDurationMs         int64
	UserAgent                string
	UpstreamStatusCode       int
	ErrorMessage             string
	TraceID                  string
	APIKeyName               string
	UserName                 string
	UserEmail                string
	CompressionEnabled       bool
	CompressedTokens         int
	CompressionStrategies    []string
}

type listAuditLogsParam struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
}

// sanitizeListParam 校验并填充默认值；非法 SortField 返回 ErrValidation
func sanitizeListParam(ctx context.Context, in listAuditLogsParam) (model.CommonParam, error) {
	if in.PageSize < 1 {
		in.PageSize = 20
	}
	if in.PageSize > constant.AuditMaxPageSize {
		in.PageSize = constant.AuditMaxPageSize
	}
	if in.Page < 1 {
		in.Page = 1
	}
	if in.SortField != "" && !validSortFields[in.SortField] {
		logger.WithCtx(ctx).Warn("[AuditQuery] Invalid sort field", zap.String("sortField", in.SortField))
		return model.CommonParam{}, ierr.New(ierr.ErrValidation, "invalid sort field: "+in.SortField)
	}
	if in.Sort == "" {
		in.Sort = enum.SortAsc
	}
	if in.SortField == "" {
		in.SortField = constant.FieldID
	}
	return model.CommonParam{
		PageParam:  model.PageParam{Page: in.Page, PageSize: in.PageSize},
		QueryParam: model.QueryParam{Query: in.Query},
		SortParam:  model.SortParam{Sort: in.Sort, SortField: in.SortField},
	}, nil
}

// parseFilterCriteria 解析 filter 表达式为 FilterCriteria
func parseFilterCriteria(filterExpr string) (*filter.FilterCriteria, error) {
	if filterExpr == "" {
		return nil, nil
	}
	filters, err := filter.Parse(filterExpr)
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrValidation, err, "parse filter expression")
	}
	return &filter.FilterCriteria{
		Filters:      filters,
		FieldConfigs: auditFieldConfigs,
	}, nil
}

// ListAllAuditLogsQuery admin 全量审计列表查询
type ListAllAuditLogsQuery struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
	Filter    string
}

// ListAllAuditLogsHandler 全量审计列表查询处理器
type ListAllAuditLogsHandler interface {
	Handle(ctx context.Context, q ListAllAuditLogsQuery) ([]*AuditLogView, *model.PageInfo, error)
}

type listAllAuditLogsHandler struct {
	repo modelcall.AuditRepository
}

// NewListAllAuditLogsHandler 构造 admin 全量审计查询处理器
func NewListAllAuditLogsHandler(repo modelcall.AuditRepository) ListAllAuditLogsHandler {
	return &listAllAuditLogsHandler{repo: repo}
}

// Handle 执行全量审计分页查询
func (h *listAllAuditLogsHandler) Handle(ctx context.Context, q ListAllAuditLogsQuery) ([]*AuditLogView, *model.PageInfo, error) {
	param, err := sanitizeListParam(ctx, listAuditLogsParam{
		Page: q.Page, PageSize: q.PageSize, Query: q.Query, Sort: q.Sort, SortField: q.SortField,
	})
	if err != nil {
		return nil, nil, err
	}
	criteria, err := parseFilterCriteria(q.Filter)
	if err != nil {
		return nil, nil, err
	}
	audits, pageInfo, err := h.repo.ListAll(ctx, param, q.StartTime, q.EndTime, criteria)
	if err != nil {
		return nil, nil, err
	}
	views, err := buildAuditViews(ctx, h.repo, audits)
	if err != nil {
		return nil, nil, err
	}
	return views, pageInfo, nil
}

// ListAuditLogsByUserQuery 按 user 维度审计列表查询
type ListAuditLogsByUserQuery struct {
	UserID    uint
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
	Filter    string
}

// ListAuditLogsByUserHandler user 自己名下所有 key 的审计列表查询处理器
type ListAuditLogsByUserHandler interface {
	Handle(ctx context.Context, q ListAuditLogsByUserQuery) ([]*AuditLogView, *model.PageInfo, error)
}

type listAuditLogsByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyIDs port.APIKeyIDLookup
}

// NewListAuditLogsByUserHandler 构造 user 维度审计查询处理器
func NewListAuditLogsByUserHandler(repo modelcall.AuditRepository, apiKeyIDs port.APIKeyIDLookup) ListAuditLogsByUserHandler {
	return &listAuditLogsByUserHandler{repo: repo, apiKeyIDs: apiKeyIDs}
}

// Handle 执行 user 维度审计分页查询
func (h *listAuditLogsByUserHandler) Handle(ctx context.Context, q ListAuditLogsByUserQuery) ([]*AuditLogView, *model.PageInfo, error) {
	param, err := sanitizeListParam(ctx, listAuditLogsParam{
		Page: q.Page, PageSize: q.PageSize, Query: q.Query, Sort: q.Sort, SortField: q.SortField,
	})
	if err != nil {
		return nil, nil, err
	}

	keyIDs, err := h.apiKeyIDs.LookupIDsByUserID(ctx, q.UserID)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "lookup api key ids by user id")
	}
	criteria, err := parseFilterCriteria(q.Filter)
	if err != nil {
		return nil, nil, err
	}
	audits, pageInfo, err := h.repo.ListByAPIKeyIDs(ctx, keyIDs, param, q.StartTime, q.EndTime, criteria)
	if err != nil {
		return nil, nil, err
	}
	views, err := buildAuditViews(ctx, h.repo, audits)
	if err != nil {
		return nil, nil, err
	}
	return views, pageInfo, nil
}

func buildAuditViews(ctx context.Context, repo modelcall.AuditRepository, audits []*aggregate.ModelCallAudit) ([]*AuditLogView, error) {
	apiKeyIDs := lo.Uniq(lo.Map(audits, func(audit *aggregate.ModelCallAudit, _ int) uint { return audit.APIKeyID() }))
	relations, err := repo.BatchGetRelations(ctx, apiKeyIDs)
	if err != nil {
		return nil, err
	}

	views := lo.Map(audits, func(audit *aggregate.ModelCallAudit, _ int) *AuditLogView {
		view := &AuditLogView{
			ID:                       audit.AggregateID(),
			CreatedAt:                audit.CreatedAt(),
			Model:                    audit.Model(),
			UpstreamProtocol:         audit.UpstreamProtocol(),
			APIProtocol:              audit.APIProtocol(),
			Endpoint:                 audit.Endpoint(),
			InputTokens:              audit.Tokens().Input(),
			OutputTokens:             audit.Tokens().Output(),
			CacheCreationInputTokens: audit.Tokens().CacheCreation(),
			CacheReadInputTokens:     audit.Tokens().CacheRead(),
			FirstTokenLatencyMs:      audit.Latency().FirstTokenMs(),
			StreamDurationMs:         audit.Latency().StreamMs(),
			UserAgent:                audit.UserAgent(),
			UpstreamStatusCode:       audit.Status().UpstreamStatusCode(),
			ErrorMessage:             audit.Status().ErrorMessage(),
			TraceID:                  audit.TraceID(),
			CompressionEnabled:       audit.CompressionEnabled(),
			CompressedTokens:         audit.CompressedTokens(),
			CompressionStrategies:    audit.CompressionStrategies(),
		}
		if relation, ok := relations[audit.APIKeyID()]; ok {
			view.APIKeyName = relation.APIKeyName
			view.UserName = relation.UserName
			view.UserEmail = relation.UserEmail
		}
		return view
	})
	return views, nil
}
