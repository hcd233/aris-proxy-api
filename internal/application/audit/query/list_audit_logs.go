package query

import (
	"context"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

var validSortFields = map[string]bool{
	constant.FieldCreatedAt:           true,
	constant.FieldInputTokens:         true,
	constant.FieldOutputTokens:        true,
	constant.FieldFirstTokenLatencyMs: true,
	constant.FieldStreamDurationMs:    true,
}

// ─── 共享：参数清洗 ─────────────────────────────────────────

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
		in.Sort = enum.SortDesc
	}
	if in.SortField == "" {
		in.SortField = constant.FieldCreatedAt
	}
	return model.CommonParam{
		PageParam:  model.PageParam{Page: in.Page, PageSize: in.PageSize},
		QueryParam: model.QueryParam{Query: in.Query},
		SortParam:  model.SortParam{Sort: in.Sort, SortField: in.SortField},
	}, nil
}

// ─── ListAllAuditLogsHandler（admin） ─────────────────────────

// ListAllAuditLogsQuery admin 全量审计列表查询
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAllAuditLogsQuery struct {
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
}

// ListAllAuditLogsHandler 全量审计列表查询处理器
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAllAuditLogsHandler interface {
	Handle(ctx context.Context, q ListAllAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

type listAllAuditLogsHandler struct {
	repo modelcall.AuditRepository
}

// NewListAllAuditLogsHandler 构造 admin 全量审计查询处理器
//
//	@param repo modelcall.AuditRepository
//	@return ListAllAuditLogsHandler
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListAllAuditLogsHandler(repo modelcall.AuditRepository) ListAllAuditLogsHandler {
	return &listAllAuditLogsHandler{repo: repo}
}

// Handle 执行全量审计分页查询
func (h *listAllAuditLogsHandler) Handle(ctx context.Context, q ListAllAuditLogsQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	param, err := sanitizeListParam(ctx, listAuditLogsParam{
		Page: q.Page, PageSize: q.PageSize, Query: q.Query, Sort: q.Sort, SortField: q.SortField,
	})
	if err != nil {
		return nil, nil, err
	}
	return h.repo.ListAll(ctx, param, q.StartTime, q.EndTime)
}

// ─── ListAuditLogsByUserHandler（普通 user） ─────────────────

// ListAuditLogsByUserQuery 按 user 维度审计列表查询
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAuditLogsByUserQuery struct {
	UserID    uint
	Page      int
	PageSize  int
	Query     string
	Sort      enum.Sort
	SortField string
	StartTime time.Time
	EndTime   time.Time
}

// ListAuditLogsByUserHandler user 自己名下所有 key 的审计列表查询处理器
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type ListAuditLogsByUserHandler interface {
	Handle(ctx context.Context, q ListAuditLogsByUserQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error)
}

type listAuditLogsByUserHandler struct {
	repo      modelcall.AuditRepository
	apiKeyDAO *dao.ProxyAPIKeyDAO
	db        *gorm.DB
}

// NewListAuditLogsByUserHandler 构造 user 维度审计查询处理器
//
//	@param repo modelcall.AuditRepository
//	@param apiKeyDAO *dao.ProxyAPIKeyDAO
//	@param db *gorm.DB
//	@return ListAuditLogsByUserHandler
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewListAuditLogsByUserHandler(repo modelcall.AuditRepository, apiKeyDAO *dao.ProxyAPIKeyDAO, db *gorm.DB) ListAuditLogsByUserHandler {
	return &listAuditLogsByUserHandler{repo: repo, apiKeyDAO: apiKeyDAO, db: db}
}

// Handle 执行 user 维度审计分页查询
//
// 内部两步：
//  1. 用 ProxyAPIKeyDAO.BatchGetByField 按 user_id 查 user 名下所有 key 的 ID 列表
//  2. 调 repo.ListByAPIKeyIDs(keyIDs, ...)；空 keyIDs 时不打 SQL 直接返回空
func (h *listAuditLogsByUserHandler) Handle(ctx context.Context, q ListAuditLogsByUserQuery) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	param, err := sanitizeListParam(ctx, listAuditLogsParam{
		Page: q.Page, PageSize: q.PageSize, Query: q.Query, Sort: q.Sort, SortField: q.SortField,
	})
	if err != nil {
		return nil, nil, err
	}

	db := h.db.WithContext(ctx)
	keys, err := h.apiKeyDAO.BatchGetByField(db, constant.FieldUserID, []uint{q.UserID}, []string{constant.FieldID})
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list api keys by user id")
	}
	keyIDs := make([]uint, 0, len(keys))
	for _, k := range keys {
		keyIDs = append(keyIDs, k.ID)
	}
	return h.repo.ListByAPIKeyIDs(ctx, keyIDs, param, q.StartTime, q.EndTime)
}
