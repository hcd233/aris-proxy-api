package repository

import (
	"context"
	"errors"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/application/cronmgmt/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// cronRepository CronJobRepository 的 GORM 实现
//
//	@author centonhuang
//	@update 2026-06-17 10:00:00
type cronRepository struct {
	db  *gorm.DB
	dao *dao.CronJobDAO
}

// NewCronRepository 构造 CronJob 仓储
//
//	@param db *gorm.DB
//	@return port.CronJobRepository
func NewCronRepository(db *gorm.DB) port.CronJobRepository {
	return &cronRepository{db: db, dao: dao.GetCronJobDAO()}
}

// Sync 同步定时任务元数据（只插入不存在记录，或更新 type/description）。
//
// 注意：不会覆盖已有记录的 spec，因为 spec 是用户通过 API 可配置的字段。
// 代码升级导致的 default spec 变更只影响新插入的记录。
//
//	@receiver r *cronRepository
//	@param ctx context.Context
//	@param jobs []*port.CronJobView
//	@return error
func (r *cronRepository) Sync(ctx context.Context, jobs []*port.CronJobView) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, job := range jobs {
			existing := dbmodel.CronJob{}
			err := tx.Where(constant.CronJobWhereNameEquals, job.Name).First(&existing).Error
			if err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					return ierr.Wrap(ierr.ErrDBQuery, err, "query cron job")
				}
				if err := tx.Create(&dbmodel.CronJob{
					Name:        job.Name,
					Type:        job.Type,
					Spec:        job.Spec,
					Description: job.Description,
					Enabled:     true,
				}).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBCreate, err, "create cron job")
				}
				continue
			}
			// 已有记录仅同步 type/description，不覆盖用户通过 API 设置的 spec
			if existing.Type != job.Type || existing.Description != job.Description {
				if err := tx.Model(&existing).Updates(map[string]any{
					constant.FieldCronType:    job.Type,
					constant.FieldDescription: job.Description,
				}).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBQuery, err, "update cron job")
				}
			}
		}
		return nil
	})
}

// List 分页列出 CronJob
//
//	@receiver r *cronRepository
//	@param ctx context.Context
//	@param param model.CommonParam
//	@return []*port.CronJobView
//	@return *model.PageInfo
//	@return error
func (r *cronRepository) List(ctx context.Context, param model.CommonParam) ([]*port.CronJobView, *model.PageInfo, error) {
	daoParam := dao.CommonParam{
		PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
		QueryParam: dao.QueryParam{Query: param.Query, QueryFields: param.QueryFields},
		SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
	}
	rows, pageInfo, err := r.dao.Paginate(r.db.WithContext(ctx), &dbmodel.CronJob{}, nil, &daoParam)
	if err != nil {
		return nil, nil, ierr.Wrap(ierr.ErrDBQuery, err, "list cron jobs")
	}
	views := lo.Map(rows, func(row *dbmodel.CronJob, _ int) *port.CronJobView {
		return &port.CronJobView{
			Name:        row.Name,
			Type:        row.Type,
			Spec:        row.Spec,
			Description: row.Description,
			Enabled:     row.Enabled,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		}
	})
	return views, pageInfo, nil
}

// Update 更新 CronJob（部分更新，非 nil 字段才更新）
//
//	@receiver r *cronRepository
//	@param ctx context.Context
//	@param name string
//	@param params port.UpdateCronJobParams
//	@return error
func (r *cronRepository) Update(ctx context.Context, name string, params port.UpdateCronJobParams) error {
	updates := map[string]any{}
	if params.Enabled != nil {
		updates[constant.FieldEnabled] = *params.Enabled
	}
	if params.Spec != nil {
		updates[constant.FieldSpec] = *params.Spec
	}
	if len(updates) == 0 {
		return nil
	}

	result := r.db.WithContext(ctx).Model(&dbmodel.CronJob{}).
		Where(constant.CronJobWhereNameEquals, name).
		Updates(updates)
	if result.Error != nil {
		return ierr.Wrap(ierr.ErrDBQuery, result.Error, "update cron job")
	}
	if result.RowsAffected == 0 {
		return ierr.New(ierr.ErrDataNotExists, constant.CronJobNotFoundMessage+name)
	}
	return nil
}

// Get 获取单个 CronJob
//
//	@receiver r *cronRepository
//	@param ctx context.Context
//	@param name string
//	@return *port.CronJobView
//	@return error
func (r *cronRepository) Get(ctx context.Context, name string) (*port.CronJobView, error) {
	row := dbmodel.CronJob{}
	err := r.db.WithContext(ctx).Where(constant.CronJobWhereNameEquals, name).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ierr.New(ierr.ErrDataNotExists, constant.CronJobNotFoundMessage+name)
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get cron job")
	}
	return &port.CronJobView{
		Name:        row.Name,
		Type:        row.Type,
		Spec:        row.Spec,
		Description: row.Description,
		Enabled:     row.Enabled,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}
