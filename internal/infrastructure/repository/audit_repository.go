package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// auditRepository AuditRepository 的 GORM 实现
type auditRepository struct {
	dao *dao.ModelCallAuditDAO
	db  *gorm.DB
}

// NewAuditRepository 构造审计仓储
//
//	@return modelcall.AuditRepository
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewAuditRepository(db *gorm.DB) modelcall.AuditRepository {
	return &auditRepository{dao: dao.GetModelCallAuditDAO(), db: db}
}

// Save 持久化审计聚合
//
//	@receiver r *auditRepository
//	@param ctx context.Context
//	@param audit *aggregate.ModelCallAudit
//	@return error
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func (r *auditRepository) Save(ctx context.Context, audit *aggregate.ModelCallAudit) error {
	db := r.db.WithContext(ctx)
	record := &dbmodel.ModelCallAudit{
		APIKeyID:                 audit.APIKeyID(),
		ModelID:                  audit.ModelID(),
		Model:                    audit.Model(),
		UpstreamProvider:         audit.UpstreamProvider(),
		APIProvider:              audit.APIProvider(),
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
	}
	if err := r.dao.Create(db, record); err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "create model call audit")
	}
	audit.SetID(record.ID)
	return nil
}

// ListByAPIKeyID 按 APIKeyID 分页查询审计记录
//
//	@receiver r *auditRepository
//	@param ctx context.Context
//	@param apiKeyID uint
//	@param param model.CommonParam
//	@param startTime time.Time
//	@param endTime time.Time
//	@return []*aggregate.ModelCallAudit
//	@return *model.PageInfo
//	@return error
//	@author centonhuang
//	@update 2026-05-11 10:00:00
func (r *auditRepository) ListByAPIKeyID(ctx context.Context, apiKeyID uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)

	if !startTime.IsZero() {
		db = db.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		db = db.Where(constant.FieldCreatedAt+" <= ?", endTime)
	}

	where := &dbmodel.ModelCallAudit{APIKeyID: apiKeyID}
	records, pageInfo, err := r.dao.Paginate(
		db,
		where,
		constant.AuditRepoFields,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: constant.AuditQueryFields},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate audit logs")
	}

	audits := make([]*aggregate.ModelCallAudit, 0, len(records))
	for _, r := range records {
		a := aggregate.ReconstructAudit(aggregate.ReconstructAuditInput{
			APIKeyID:         r.APIKeyID,
			ModelID:          r.ModelID,
			Model:            r.Model,
			UpstreamProvider: r.UpstreamProvider,
			APIProvider:      r.APIProvider,
			Tokens:           vo.NewTokenBreakdown(r.InputTokens, r.OutputTokens, r.CacheCreationInputTokens, r.CacheReadInputTokens),
			Latency:          vo.NewCallLatency(time.Duration(r.FirstTokenLatencyMs)*time.Millisecond, time.Duration(r.StreamDurationMs)*time.Millisecond),
			Status:           vo.NewCallStatus(r.UpstreamStatusCode, r.ErrorMessage),
			UserAgent:        r.UserAgent,
			TraceID:          r.TraceID,
			CreatedAt:        r.CreatedAt,
		})
		a.SetID(r.ID)
		audits = append(audits, a)
	}
	return audits, pageInfo, nil
}
