package handler

import (
	"context"

	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	auditquery "github.com/hcd233/aris-proxy-api/internal/application/audit/query"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// AuditHandler 审计处理器
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type AuditHandler interface {
	HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error)
}

// AuditDependencies AuditHandler 依赖项
//
//	@author centonhuang
//	@update 2026-05-29 14:00:00
type AuditDependencies struct {
	ListAll    auditquery.ListAllAuditLogsHandler
	ListByUser auditquery.ListAuditLogsByUserHandler
	APIKeyDAO  *dao.ProxyAPIKeyDAO
	UserDAO    *dao.UserDAO
	DB         *gorm.DB
}

type auditHandler struct {
	listAll    auditquery.ListAllAuditLogsHandler
	listByUser auditquery.ListAuditLogsByUserHandler
	apiKeyDAO  *dao.ProxyAPIKeyDAO
	userDAO    *dao.UserDAO
	db         *gorm.DB
}

// NewAuditHandler 创建审计处理器
//
//	@param deps AuditDependencies
//	@return AuditHandler
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func NewAuditHandler(deps AuditDependencies) AuditHandler {
	return &auditHandler{
		listAll:    deps.ListAll,
		listByUser: deps.ListByUser,
		apiKeyDAO:  deps.APIKeyDAO,
		userDAO:    deps.UserDAO,
		db:         deps.DB,
	}
}

// HandleListAuditLogs 分页获取审计日志列表，按当前 JWT 用户权限分级返回数据范围
//
//	@receiver h *auditHandler
//	@param ctx context.Context
//	@param req *dto.ListAuditLogsReq
//	@return *dto.HTTPResponse[*dto.ListAuditLogsRsp]
//	@return error
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (h *auditHandler) HandleListAuditLogs(ctx context.Context, req *dto.ListAuditLogsReq) (*dto.HTTPResponse[*dto.ListAuditLogsRsp], error) {
	rsp := &dto.ListAuditLogsRsp{}
	userID := util.CtxValueUint(ctx, constant.CtxKeyUserID)
	permission := util.CtxValuePermission(ctx)

	var (
		audits   []*aggregate.ModelCallAudit
		pageInfo *model.PageInfo
		err      error
	)

	switch permission {
	case enum.PermissionAdmin:
		audits, pageInfo, err = h.listAll.Handle(ctx, auditquery.ListAllAuditLogsQuery{
			Page:      req.Page,
			PageSize:  req.PageSize,
			Query:     req.Query,
			Sort:      req.Sort,
			SortField: req.SortField,
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
		})
	case enum.PermissionUser:
		audits, pageInfo, err = h.listByUser.Handle(ctx, auditquery.ListAuditLogsByUserQuery{
			UserID:    userID,
			Page:      req.Page,
			PageSize:  req.PageSize,
			Query:     req.Query,
			Sort:      req.Sort,
			SortField: req.SortField,
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
		})
	default:
		rsp.Error = ierr.ErrUnauthorized.BizError()
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] List audit logs failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	keyByID, userByID, err := h.fetchRelations(ctx, audits)
	if err != nil {
		logger.WithCtx(ctx).Error("[AuditHandler] Fetch audit relations failed", zap.Error(err))
		rsp.Error = ierr.ToBizError(err, ierr.ErrInternal.BizError())
		return apiutil.WrapHTTPResponse(rsp, nil)
	}

	rsp.Logs = lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) *dto.AuditLogItem {
		item := &dto.AuditLogItem{
			ID:                       a.AggregateID(),
			CreatedAt:                a.CreatedAt(),
			Model:                    a.Model(),
			UpstreamProvider:         a.UpstreamProvider(),
			APIProvider:              a.APIProvider(),
			InputTokens:              a.Tokens().Input(),
			OutputTokens:             a.Tokens().Output(),
			CacheCreationInputTokens: a.Tokens().CacheCreation(),
			CacheReadInputTokens:     a.Tokens().CacheRead(),
			FirstTokenLatencyMs:      a.Latency().FirstTokenMs(),
			StreamDurationMs:         a.Latency().StreamMs(),
			UserAgent:                a.UserAgent(),
			UpstreamStatusCode:       a.Status().UpstreamStatusCode(),
			ErrorMessage:             a.Status().ErrorMessage(),
			TraceID:                  a.TraceID(),
		}
		if k, ok := keyByID[a.APIKeyID()]; ok {
			item.APIKeyName = k.Name
			if u, ok := userByID[k.UserID]; ok {
				item.UserName = u.Name
				item.UserEmail = u.Email
			}
		}
		return item
	})
	rsp.PageInfo = pageInfo
	return apiutil.WrapHTTPResponse(rsp, nil)
}

// fetchRelations 批量拉取 audit 涉及的 ProxyAPIKey 和 User，返回按 ID 索引的 map
func (h *auditHandler) fetchRelations(ctx context.Context, audits []*aggregate.ModelCallAudit) (map[uint]*dbmodel.ProxyAPIKey, map[uint]*dbmodel.User, error) {
	if len(audits) == 0 {
		return map[uint]*dbmodel.ProxyAPIKey{}, map[uint]*dbmodel.User{}, nil
	}
	db := h.db.WithContext(ctx)

	apiKeyIDs := lo.Uniq(lo.Map(audits, func(a *aggregate.ModelCallAudit, _ int) uint { return a.APIKeyID() }))
	keys, err := h.apiKeyDAO.BatchGetByField(db, constant.FieldID, apiKeyIDs, []string{constant.FieldID, constant.FieldName, constant.FieldUserID})
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get proxy api keys")
	}
	keyByID := lo.SliceToMap(keys, func(k *dbmodel.ProxyAPIKey) (uint, *dbmodel.ProxyAPIKey) { return k.ID, k })

	userIDs := lo.Uniq(lo.Map(keys, func(k *dbmodel.ProxyAPIKey, _ int) uint { return k.UserID }))
	users, err := h.userDAO.BatchGetByField(db, constant.FieldID, userIDs, []string{constant.FieldID, constant.FieldName, constant.FieldEmail})
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get users")
	}
	userByID := lo.SliceToMap(users, func(u *dbmodel.User) (uint, *dbmodel.User) { return u.ID, u })

	return keyByID, userByID, nil
}
