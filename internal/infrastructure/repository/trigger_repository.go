package repository

import (
	"context"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger"
	"github.com/hcd233/aris-proxy-api/internal/domain/trigger/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

type triggerRepository struct {
	dao *dao.TriggerDAO
	db  *gorm.DB
}

func NewTriggerRepository(db *gorm.DB) trigger.TriggerRepository {
	return &triggerRepository{dao: dao.GetTriggerDAO(), db: db}
}

func (r *triggerRepository) FindByID(ctx context.Context, id uint) (*aggregate.Trigger, error) {
	db := r.db.WithContext(ctx)
	m, err := r.dao.Get(db, &dbmodel.Trigger{BaseModel: dbmodel.BaseModel{ID: id}}, constant.TriggerRepoFieldsFull)
	if err != nil {
		return nil, err
	}
	return toTriggerAggregate(m), nil
}

func (r *triggerRepository) Create(ctx context.Context, word *aggregate.Trigger) (uint, error) {
	db := r.db.WithContext(ctx)
	m := toTriggerDBModel(word)
	err := r.dao.Create(db, m)
	return m.ID, err
}

func (r *triggerRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	return r.dao.Delete(db, &dbmodel.Trigger{BaseModel: dbmodel.BaseModel{ID: id}})
}

// DeleteBatch 批量软删触发词（单条 UPDATE deleted_at，原子）
func (r *triggerRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	db := r.db.WithContext(ctx)
	return r.dao.BatchDeleteByField(db, constant.FieldID, ids)
}

func (r *triggerRepository) UpdateAction(ctx context.Context, id uint, action string) error {
	db := r.db.WithContext(ctx)
	result := db.Model(&dbmodel.Trigger{}).
		Where(constant.WhereIDEquals, id).
		Where(constant.DBConditionDeletedAtZero).
		Updates(map[string]any{
			constant.FieldAction:    action,
			constant.FieldUpdatedAt: time.Now().UTC(),
		})
	if result.Error != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, result.Error, "update trigger action")
	}
	// 目标不存在或已软删：不静默成功，返回明确错误（与 FindByID 语义一致）
	if result.RowsAffected == 0 {
		return ierr.New(ierr.ErrDataNotExists, "trigger word not found")
	}
	return nil
}

func (r *triggerRepository) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Trigger, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Trigger{},
		constant.TriggerRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldWord}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, err
	}
	items := lo.Map(records, func(m *dbmodel.Trigger, _ int) *aggregate.Trigger {
		return toTriggerAggregate(m)
	})
	return items, pageInfo, nil
}

func (r *triggerRepository) ListAll(ctx context.Context) ([]*aggregate.Trigger, error) {
	db := r.db.WithContext(ctx)
	records, err := r.dao.FindAll(db)
	if err != nil {
		return nil, err
	}
	return lo.Map(records, func(m *dbmodel.Trigger, _ int) *aggregate.Trigger {
		return toTriggerAggregate(m)
	}), nil
}

func (r *triggerRepository) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	db := r.db.WithContext(ctx)
	for id, count := range idHits {
		err := db.Model(&dbmodel.Trigger{}).
			Where(constant.WhereIDEquals, id).
			UpdateColumn(constant.FieldHitCount, gorm.Expr(constant.FieldHitCount+" + ?", count)).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func toTriggerAggregate(m *dbmodel.Trigger) *aggregate.Trigger {
	b, err := aggregate.CreateTrigger(m.ID, m.Word, m.Action)
	if err != nil {
		return nil
	}
	b.SetHitCount(m.HitCount)
	b.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return b
}

func toTriggerDBModel(b *aggregate.Trigger) *dbmodel.Trigger {
	return &dbmodel.Trigger{
		Word:   b.Word(),
		Action: b.Action(),
	}
}
