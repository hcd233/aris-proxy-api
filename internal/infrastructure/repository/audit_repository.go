package repository

import (
	"context"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// auditRepository AuditRepository 的 GORM 实现
type auditRepository struct {
	dao *dao.ModelCallAuditDAO
}

// NewAuditRepository 构造审计仓储
//
//	@return modelcall.AuditRepository
//	@author centonhuang
//	@update 2026-04-22 17:00:00
func NewAuditRepository() modelcall.AuditRepository {
	return &auditRepository{dao: dao.GetModelCallAuditDAO()}
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
	db := database.GetDBInstance(ctx)
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
