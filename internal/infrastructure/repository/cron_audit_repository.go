package repository

import (
	"context"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hcd233/aris-proxy-api/internal/application/cronaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// cronCallAuditRepository CronCallAuditRepository 的 GORM 实现
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type cronCallAuditRepository struct {
	db  *gorm.DB
	dao *dao.CronCallAuditDAO
}

// NewCronCallAuditRepository 构造 CronCallAudit 仓储
//
//	@param db *gorm.DB
//	@return port.CronCallAuditRepository
func NewCronCallAuditRepository(db *gorm.DB) port.CronCallAuditRepository {
	return &cronCallAuditRepository{db: db, dao: dao.GetCronCallAuditDAO()}
}

// Save 保存 CronCallAudit
//
//	@receiver r *cronCallAuditRepository
//	@param ctx context.Context
//	@param audit *port.CronCallAuditView
//	@return error
func (r *cronCallAuditRepository) Save(ctx context.Context, audit *port.CronCallAuditView) error {
	record := &dbmodel.CronCallAudit{
		CronName:   audit.CronName,
		TraceID:    audit.TraceID,
		StartedAt:  audit.StartedAt,
		EndedAt:    audit.EndedAt,
		DurationMs: audit.DurationMs,
		Status:     audit.Status,
		Message:    audit.Message,
	}
	if err := r.dao.Create(r.db.WithContext(ctx), record); err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "create cron call audit")
	}
	audit.ID = record.ID
	return nil
}

// List 分页列出 CronCallAudit
//
//	@receiver r *cronCallAuditRepository
//	@param ctx context.Context
//	@param param dao.CommonParam
//	@param startTime time.Time
//	@param endTime time.Time
//	@param filterExp string
//	@return []*port.CronCallAuditView
//	@return *model.PageInfo
//	@return error
func (r *cronCallAuditRepository) List(ctx context.Context, param dao.CommonParam, startTime, endTime time.Time, filterExp string) ([]*port.CronCallAuditView, *model.PageInfo, error) {
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	sql := r.db.WithContext(ctx).Model(&dbmodel.CronCallAudit{}).
		Select(constant.CronCallAuditRepoFields).
		Where(constant.CronAuditPaginateWhereDeletedAtZero)

	if !startTime.IsZero() {
		sql = sql.Where(constant.CronAuditPaginateWhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		sql = sql.Where(constant.CronAuditPaginateWhereCreatedAtLTE, endTime)
	}

	sql, filterErr := r.applyFilter(sql, filterExp)
	if filterErr != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, filterErr, "build filter SQL")
	}
	sql = r.applyKeywordSearch(r.db.WithContext(ctx), sql, param.Query)
	sql = r.applySort(sql, param.Sort, param.SortField)

	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if err := sql.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count cron call audits")
	}

	limit, offset := param.PageSize, (param.Page-1)*param.PageSize
	var records []*dbmodel.CronCallAudit
	if err := sql.Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate cron call audits")
	}

	views := lo.Map(records, func(rec *dbmodel.CronCallAudit, _ int) *port.CronCallAuditView {
		return &port.CronCallAuditView{
			ID:         rec.ID,
			CronName:   rec.CronName,
			TraceID:    rec.TraceID,
			StartedAt:  rec.StartedAt,
			EndedAt:    rec.EndedAt,
			DurationMs: rec.DurationMs,
			Status:     rec.Status,
			Message:    rec.Message,
			CreatedAt:  rec.CreatedAt,
		}
	})
	return views, pageInfo, nil
}

// ListDistinctTypes 列出 distinct cron_name
//
//	@receiver r *cronCallAuditRepository
//	@param ctx context.Context
//	@param keyword string
//	@param startTime time.Time
//	@param endTime time.Time
//	@return []string
//	@return error
func (r *cronCallAuditRepository) ListDistinctTypes(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	var types []string
	query := r.db.WithContext(ctx).Model(&dbmodel.CronCallAudit{}).
		Select(constant.CronAuditDistinctSelectType).
		Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtLTE, endTime)
	}
	if keyword != "" {
		query = query.Where(constant.CronAuditDistinctWhereType, "%"+keyword+"%")
	}

	if err := query.Order(constant.CronAuditDistinctOrderType).Limit(constant.CronAuditDistinctLimit).Scan(&types).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list distinct cron types")
	}
	return types, nil
}

// ListDistinctStatuses 列出 distinct status
//
//	@receiver r *cronCallAuditRepository
//	@param ctx context.Context
//	@param keyword string
//	@param startTime time.Time
//	@param endTime time.Time
//	@return []string
//	@return error
func (r *cronCallAuditRepository) ListDistinctStatuses(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	var statuses []string
	query := r.db.WithContext(ctx).Model(&dbmodel.CronCallAudit{}).
		Select(constant.CronAuditDistinctSelectStatus).
		Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtLTE, endTime)
	}
	if keyword != "" {
		query = query.Where(constant.CronAuditDistinctWhereStatus, "%"+keyword+"%")
	}

	if err := query.Order(constant.CronAuditDistinctOrderStatus).Limit(constant.CronAuditDistinctLimit).Scan(&statuses).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list distinct cron statuses")
	}
	return statuses, nil
}

// applyFilter 注入 filter 条件
func (r *cronCallAuditRepository) applyFilter(db *gorm.DB, filterExp string) (*gorm.DB, error) {
	if filterExp == "" {
		return db, nil
	}
	filters, err := filter.Parse(filterExp)
	if err != nil {
		return nil, err
	}
	if len(filters) == 0 {
		return db, nil
	}
	fieldConfigs := map[string]filter.FieldConfig{
		constant.CronAuditFilterFieldType:   {SQLColumn: constant.CronAuditFilterTypeSQLColumn},
		constant.CronAuditFilterFieldStatus: {SQLColumn: constant.CronAuditFilterStatusSQLColumn},
	}
	criteria := &filter.FilterCriteria{Filters: filters, FieldConfigs: fieldConfigs}
	filterSQL, filterArgs, err := filter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		return nil, err
	}
	if filterSQL != "" {
		db = db.Where(filterSQL, filterArgs...)
	}
	return db, nil
}

// applyKeywordSearch 注入关键词搜索条件
func (r *cronCallAuditRepository) applyKeywordSearch(db, sql *gorm.DB, query string) *gorm.DB {
	if query == "" {
		return sql
	}
	like := "%" + query + "%"
	fields := []string{constant.FieldCronName, constant.FieldTraceID}
	expressions := lo.FilterMap(fields, func(field string, _ int) (clause.Expression, bool) {
		if field == "" {
			return nil, false
		}
		return clause.Like{Column: clause.Column{Name: field}, Value: like}, true
	})
	if len(expressions) > 0 {
		sub := db.Session(&gorm.Session{NewDB: true}).Where(expressions[0])
		for _, expr := range expressions[1:] {
			sub = sub.Or(expr)
		}
		sql = sql.Where(sub)
	}
	return sql
}

// applySort 注入排序条件
func (r *cronCallAuditRepository) applySort(db *gorm.DB, sort enum.Sort, sortField string) *gorm.DB {
	if sort == "" || sortField == "" {
		return db
	}
	sortField = safeSortField(sortField)
	return db.Order(clause.OrderByColumn{Column: clause.Column{Name: sortField}, Desc: sort == enum.SortDesc})
}
