package repository

import (
	"context"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	demoauditport "github.com/hcd233/aris-proxy-api/internal/application/demoaccessaudit/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type demoAccessAuditRepository struct {
	db *gorm.DB
}

// NewDemoAccessAuditRepository 构造 DemoAccessAudit 仓储
//
//	@param db *gorm.DB
//	@return demoauditport.DemoAccessAuditRepository
//	@author centonhuang
//	@update 2026-08-23 10:00:00
func NewDemoAccessAuditRepository(db *gorm.DB) demoauditport.DemoAccessAuditRepository {
	return &demoAccessAuditRepository{db: db}
}

// Save 保存 Demo 访问审计记录
func (r *demoAccessAuditRepository) Save(ctx context.Context, view *demoauditport.DemoAccessAuditView) error {
	record := &dbmodel.DemoAccessAudit{
		Action:    view.Action,
		Module:    view.Module,
		Path:      view.Path,
		IP:        view.IP,
		UserAgent: view.UserAgent,
		Reason:    view.Reason,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "create demo access audit")
	}
	view.ID = record.ID
	return nil
}

// List 分页列出 Demo 访问审计记录
func (r *demoAccessAuditRepository) List(ctx context.Context, param model.CommonParam, startTime, endTime time.Time, filterExp string) ([]*demoauditport.DemoAccessAuditView, *model.PageInfo, error) {
	if param.Page < 1 {
		param.Page = 1
	}
	if param.PageSize < 1 {
		param.PageSize = 20
	}

	sql := r.db.WithContext(ctx).Model(&dbmodel.DemoAccessAudit{}).
		Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		sql = sql.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		sql = sql.Where(constant.WhereCreatedAtLTE, endTime)
	}

	sql, filterErr := r.applyFilter(sql, filterExp)
	if filterErr != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, filterErr, "build demo access audit filter SQL")
	}
	sql = applySort(sql, param.Sort, param.SortField)

	pageInfo := &model.PageInfo{Page: param.Page, PageSize: param.PageSize}
	if err := sql.Count(&pageInfo.Total).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "count demo access audits")
	}

	limit, offset := param.PageSize, (param.Page-1)*param.PageSize
	var records []*dbmodel.DemoAccessAudit
	if err := sql.Order(clause.OrderByColumn{Column: clause.Column{Name: constant.FieldID}, Desc: true}).
		Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "paginate demo access audits")
	}

	views := lo.Map(records, func(rec *dbmodel.DemoAccessAudit, _ int) *demoauditport.DemoAccessAuditView {
		return &demoauditport.DemoAccessAuditView{
			ID:        rec.ID,
			Action:    rec.Action,
			Module:    rec.Module,
			Path:      rec.Path,
			IP:        rec.IP,
			UserAgent: rec.UserAgent,
			Reason:    rec.Reason,
			CreatedAt: rec.CreatedAt,
		}
	})
	return views, pageInfo, nil
}

// ListDistinctActions 列出 distinct action
func (r *demoAccessAuditRepository) ListDistinctActions(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	return r.listDistinct(ctx, constant.FieldAction, keyword, startTime, endTime)
}

// ListDistinctModules 列出 distinct module
func (r *demoAccessAuditRepository) ListDistinctModules(ctx context.Context, keyword string, startTime, endTime time.Time) ([]string, error) {
	return r.listDistinct(ctx, constant.FieldModule, keyword, startTime, endTime)
}

func (r *demoAccessAuditRepository) listDistinct(ctx context.Context, column, keyword string, startTime, endTime time.Time) ([]string, error) {
	query := r.db.WithContext(ctx).Model(&dbmodel.DemoAccessAudit{}).
		Distinct(column).
		Where(constant.DBConditionDeletedAtZero)

	if !startTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtGTE, startTime)
	}
	if !endTime.IsZero() {
		query = query.Where(constant.WhereCreatedAtLTE, endTime)
	}
	if keyword != "" {
		query = query.Where(column+" LIKE ?", "%"+keyword+"%")
	}

	var values []string
	if err := query.Order(column + " ASC").Limit(constant.CronAuditDistinctLimit).Scan(&values).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list distinct demo access audit "+column)
	}
	return values, nil
}

// applyFilter 注入 filter 条件（action / module）
func (r *demoAccessAuditRepository) applyFilter(db *gorm.DB, filterExp string) (*gorm.DB, error) {
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
		constant.DemoAccessAuditFilterFieldAction: {SQLColumn: constant.FieldAction},
		constant.DemoAccessAuditFilterFieldModule: {SQLColumn: constant.FieldModule},
	}
	filterSQL, filterArgs, err := filter.ToSQL(filters, fieldConfigs)
	if err != nil {
		return nil, err
	}
	if filterSQL != "" {
		db = db.Where(filterSQL, filterArgs...)
	}
	return db, nil
}

func applySort(db *gorm.DB, sort enum.Sort, sortField string) *gorm.DB {
	if sort == "" || sortField == "" {
		return db
	}
	sortField = util.SafeSortField(sortField)
	return db.Order(clause.OrderByColumn{Column: clause.Column{Name: sortField}, Desc: sort == enum.SortDesc})
}
