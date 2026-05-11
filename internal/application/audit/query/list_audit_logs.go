package query

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

var validSortFields = map[string]bool{
	constant.FieldCreatedAt:            true,
	constant.FieldInputTokens:          true,
	constant.FieldOutputTokens:         true,
	constant.FieldFirstTokenLatencyMs:  true,
	constant.FieldStreamDurationMs:     true,
}

// ListAuditLogsQuery 审计日志列表查询
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsQuery struct {
	APIKeyID  uint
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
}

// ListAuditLogsHandler 审计日志列表查询处理器
//
//	@author centonhuang
//	@update 2026-05-11 10:00:00
type ListAuditLogsHandler interface {
	Handle(ctx context.Context, q ListAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

type listAuditLogsHandler struct {
	repo modelcall.AuditRepository
}

// NewListAuditLogsHandler 构造审计日志列表查询处理器
//
//	@param repo modelcall.AuditRepository
//	@return ListAuditLogsHandler
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func NewListAuditLogsHandler(repo modelcall.AuditRepository) ListAuditLogsHandler {
	return &listAuditLogsHandler{repo: repo}
}

// Handle 执行审计日志分页查询
//
//	@receiver h *listAuditLogsHandler
//	@param ctx context.Context
//	@param q ListAuditLogsQuery
//	@return []*aggregate.ModelCallAudit
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func (h *listAuditLogsHandler) Handle(ctx context.Context, q ListAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	log := logger.WithCtx(ctx)

	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > constant.AuditMaxPageSize {
		q.PageSize = constant.AuditMaxPageSize
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.SortField != "" && !validSortFields[q.SortField] {
		log.Warn("[AuditQuery] Invalid sort field", zap.String("sortField", q.SortField))
		return nil, nil, ierr.New(ierr.ErrValidation, "invalid sort field: "+q.SortField)
	}
	if q.Sort == "" {
		q.Sort = enum.SortDesc
	}
	if q.SortField == "" {
		q.SortField = constant.FieldCreatedAt
	}

	param := model.CommonParam{
		PageParam:  model.PageParam{Page: q.Page, PageSize: q.PageSize},
		QueryParam: model.QueryParam{Query: q.Query},
		SortParam:  model.SortParam{Sort: q.Sort, SortField: q.SortField},
	}

	return h.repo.ListByAPIKeyID(ctx, q.APIKeyID, param, q.StartTime, q.EndTime)
}
