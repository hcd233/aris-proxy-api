package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
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
//	@update 2026-05-29 14:00:00
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

// ListAll 全量分页查询审计记录（admin 用）
//
//	@receiver r *auditRepository
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (r *auditRepository) ListAll(ctx context.Context, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	return r.paginate(db, param, startTime, endTime)
}

// ListByAPIKeyIDs 按 api_key_id IN (...) 分页查询；apiKeyIDs 为空时返回空结果且不打 SQL
//
//	@receiver r *auditRepository
//	@author centonhuang
//	@update 2026-05-29 14:00:00
func (r *auditRepository) ListByAPIKeyIDs(ctx context.Context, apiKeyIDs []uint, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	if len(apiKeyIDs) == 0 {
		page, pageSize := param.Page, param.PageSize
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		}
		return nil, &model.PageInfo{Page: page, PageSize: pageSize, Total: 0}, nil
	}
	db := r.db.WithContext(ctx).Where(fmt.Sprintf(constant.DBConditionInTemplate, constant.FieldAPIKeyID), apiKeyIDs)
	return r.paginate(db, param, startTime, endTime)
}

// BatchGetRelations 批量查询审计列表所需的 API Key/User 展示信息。
func (r *auditRepository) BatchGetRelations(ctx context.Context, apiKeyIDs []uint) (map[uint]*modelcall.AuditRelation, error) {
	relations := make(map[uint]*modelcall.AuditRelation, len(apiKeyIDs))
	if len(apiKeyIDs) == 0 {
		return relations, nil
	}

	db := r.db.WithContext(ctx)
	keys, err := dao.GetProxyAPIKeyDAO().BatchGetByField(db, constant.FieldID, apiKeyIDs, []string{constant.FieldID, constant.FieldName, constant.FieldUserID})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get proxy api keys")
	}

	userIDs := make([]uint, 0, len(keys))
	seenUserIDs := make(map[uint]bool, len(keys))
	for _, key := range keys {
		relations[key.ID] = &modelcall.AuditRelation{APIKeyID: key.ID, APIKeyName: key.Name, UserID: key.UserID}
		if seenUserIDs[key.UserID] {
			continue
		}
		seenUserIDs[key.UserID] = true
		userIDs = append(userIDs, key.UserID)
	}
	if len(userIDs) == 0 {
		return relations, nil
	}

	users, err := dao.GetUserDAO().BatchGetByField(db, constant.FieldID, userIDs, []string{constant.FieldID, constant.FieldName, constant.FieldEmail})
	if err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "batch get users")
	}
	userByID := make(map[uint]*dbmodel.User, len(users))
	for _, user := range users {
		userByID[user.ID] = user
	}
	for _, relation := range relations {
		if user, ok := userByID[relation.UserID]; ok {
			relation.UserName = user.Name
			relation.UserEmail = user.Email
		}
	}
	return relations, nil
}

// paginate 通用分页：在调用方已附加范围过滤的 db 上做时间范围、模糊搜索、排序、count、limit/offset。
//
// 不复用 baseDAO.Paginate，因为后者只接受 *ModelT 等值 where 不支持 IN 条件。
func (r *auditRepository) paginate(db *gorm.DB, param model.CommonParam, startTime, endTime time.Time) ([]*aggregate.ModelCallAudit, *model.PageInfo, error) {
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	sql := db.Model(&dbmodel.ModelCallAudit{}).Select(constant.AuditRepoFields).Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" >= ?", startTime)
	}
	if !endTime.IsZero() {
		sql = sql.Where(constant.FieldCreatedAt+" <= ?", endTime)
	}

	if param.Query != "" && len(constant.AuditQueryFields) > 0 {
		like := "%" + param.Query + "%"
		expressions := make([]clause.Expression, 0, len(constant.AuditQueryFields))
		for _, field := range constant.AuditQueryFields {
			if field == "" {
				continue
			}
			expressions = append(expressions, clause.Like{Column: clause.Column{Name: field}, Value: like})
		}
		if len(expressions) > 0 {
			sub := db.Session(&gorm.Session{NewDB: true}).Where(expressions[0])
			for _, expr := range expressions[1:] {
				sub = sub.Or(expr)
			}
			sql = sql.Where(sub)
		}
	}

	if param.Sort != "" && param.SortField != "" {
		sql = sql.Order(clause.OrderByColumn{Column: clause.Column{Name: param.SortField}, Desc: param.Sort == enum.SortDesc})
	}

	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if err := sql.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count audit logs")
	}

	limit, offset := param.PageSize, (param.Page-1)*param.PageSize
	var records []*dbmodel.ModelCallAudit
	if err := sql.Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate audit logs")
	}

	audits := make([]*aggregate.ModelCallAudit, 0, len(records))
	for _, rec := range records {
		a := aggregate.ReconstructAudit(aggregate.ReconstructAuditInput{
			APIKeyID:         rec.APIKeyID,
			ModelID:          rec.ModelID,
			Model:            rec.Model,
			UpstreamProvider: rec.UpstreamProvider,
			APIProvider:      rec.APIProvider,
			Tokens:           vo.NewTokenBreakdown(rec.InputTokens, rec.OutputTokens, rec.CacheCreationInputTokens, rec.CacheReadInputTokens),
			Latency:          vo.NewCallLatency(time.Duration(rec.FirstTokenLatencyMs)*time.Millisecond, time.Duration(rec.StreamDurationMs)*time.Millisecond),
			Status:           vo.NewCallStatus(rec.UpstreamStatusCode, rec.ErrorMessage),
			UserAgent:        rec.UserAgent,
			TraceID:          rec.TraceID,
			CreatedAt:        rec.CreatedAt,
		})
		a.SetID(rec.ID)
		audits = append(audits, a)
	}
	return audits, pageInfo, nil
}

func dateTruncSQL(granularity string) string {
	switch granularity {
	case enum.GranularityMinute:
		return constant.DateTruncMinute
	case enum.GranularityHour:
		return constant.DateTruncHour
	case enum.GranularityDay:
		return constant.DateTruncDay
	case enum.GranularityWeek:
		return constant.DateTruncWeek
	default:
		return constant.DateTruncDay
	}
}

func (r *auditRepository) QueryModelTrend(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity enum.Granularity) ([]*modelcall.ModelTrendPoint, error) {
	db := r.db.WithContext(ctx).Model(&dbmodel.ModelCallAudit{}).
		Where(constant.FieldCreatedAt+" >= ? AND "+constant.FieldCreatedAt+" <= ?", startTime, endTime).
		Where(constant.SQLConditionDeletedAtZero)

	if len(apiKeyIDs) > 0 {
		db = db.Where(constant.FieldAPIKeyID+" IN ?", apiKeyIDs)
	}

	timeBucketExpr := dateTruncSQL(granularity)
	var results []*modelcall.ModelTrendPoint
	if err := db.Select(constant.FieldModel + ", " + timeBucketExpr + " AS time, COUNT(*) AS count").
		Group(constant.FieldModel + ", time").
		Order(constant.FieldModel + ", time").
		Scan(&results).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query model trend")
	}
	return results, nil
}

func (r *auditRepository) QueryRequestRate(ctx context.Context, apiKeyIDs []uint, startTime, endTime time.Time, granularity string) ([]*modelcall.RequestRatePoint, error) {
	db := r.db.WithContext(ctx).Model(&dbmodel.ModelCallAudit{}).
		Where(constant.FieldCreatedAt+" >= ? AND "+constant.FieldCreatedAt+" <= ?", startTime, endTime).
		Where(constant.SQLConditionDeletedAtZero)

	if len(apiKeyIDs) > 0 {
		db = db.Where(constant.FieldAPIKeyID+" IN ?", apiKeyIDs)
	}

	timeBucketExpr := dateTruncSQL(granularity)
	var results []*modelcall.RequestRatePoint
	if err := db.Select(constant.FieldModel + ", " + timeBucketExpr + " AS time, COUNT(*) AS total, COUNT(*) FILTER (WHERE " + constant.SQLConditionUpstreamSuccess + ") AS success").
		Group(constant.FieldModel + ", time").
		Order(constant.FieldModel + ", time").
		Scan(&results).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query request rate")
	}
	return results, nil
}
