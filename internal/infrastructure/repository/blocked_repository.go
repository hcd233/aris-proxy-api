package repository

import (
	"context"
	"time"

	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked"
	"github.com/hcd233/aris-proxy-api/internal/domain/blocked/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

type blockedRepository struct {
	dao *dao.BlockedDAO
	db  *gorm.DB
}

func NewBlockedRepository(db *gorm.DB) blocked.BlockedRepository {
	return &blockedRepository{dao: dao.GetBlockedDAO(), db: db}
}

func (r *blockedRepository) FindByID(ctx context.Context, id uint) (*aggregate.Blocked, error) {
	db := r.db.WithContext(ctx)
	m, err := r.dao.Get(db, &dbmodel.Blocked{BaseModel: dbmodel.BaseModel{ID: id}}, constant.BlockedRepoFieldsFull)
	if err != nil {
		return nil, err
	}
	return toBlockedAggregate(m), nil
}

func (r *blockedRepository) Create(ctx context.Context, word *aggregate.Blocked) (uint, error) {
	db := r.db.WithContext(ctx)
	m := toBlockedDBModel(word)
	err := r.dao.Create(db, m)
	return m.ID, err
}

func (r *blockedRepository) Delete(ctx context.Context, id uint) error {
	db := r.db.WithContext(ctx)
	return r.dao.Delete(db, &dbmodel.Blocked{BaseModel: dbmodel.BaseModel{ID: id}})
}

// DeleteBatch 批量软删敏感词（单条 UPDATE deleted_at，原子）
func (r *blockedRepository) DeleteBatch(ctx context.Context, ids []uint) error {
	db := r.db.WithContext(ctx)
	return r.dao.BatchDeleteByField(db, constant.FieldID, ids)
}

func (r *blockedRepository) UpdateAction(ctx context.Context, id uint, action string) error {
	db := r.db.WithContext(ctx)
	result := db.Model(&dbmodel.Blocked{}).
		Where(constant.WhereIDEquals, id).
		Where(constant.DBConditionDeletedAtZero).
		Updates(map[string]any{
			constant.FieldAction:    action,
			constant.FieldUpdatedAt: time.Now().UTC(),
		})
	if result.Error != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, result.Error, "update blocked action")
	}
	// 目标不存在或已软删：不静默成功，返回明确错误（与 FindByID 语义一致）
	if result.RowsAffected == 0 {
		return ierr.New(ierr.ErrDataNotExists, "blocked word not found")
	}
	return nil
}

func (r *blockedRepository) Paginate(ctx context.Context, param model.CommonParam) ([]*aggregate.Blocked, *model.PageInfo, error) {
	db := r.db.WithContext(ctx)
	records, pageInfo, err := r.dao.Paginate(
		db,
		&dbmodel.Blocked{},
		constant.BlockedRepoFieldsFull,
		&dao.CommonParam{
			PageParam:  dao.PageParam{Page: param.Page, PageSize: param.PageSize},
			QueryParam: dao.QueryParam{Query: param.Query, QueryFields: []string{constant.FieldWord}},
			SortParam:  dao.SortParam{Sort: param.Sort, SortField: param.SortField},
		},
	)
	if err != nil {
		return nil, nil, err
	}
	items := lo.Map(records, func(m *dbmodel.Blocked, _ int) *aggregate.Blocked {
		return toBlockedAggregate(m)
	})
	return items, pageInfo, nil
}

func (r *blockedRepository) ListAll(ctx context.Context) ([]*aggregate.Blocked, error) {
	db := r.db.WithContext(ctx)
	records, err := r.dao.FindAll(db)
	if err != nil {
		return nil, err
	}
	return lo.Map(records, func(m *dbmodel.Blocked, _ int) *aggregate.Blocked {
		return toBlockedAggregate(m)
	}), nil
}

func (r *blockedRepository) BatchIncrementHitCount(ctx context.Context, idHits map[uint]uint) error {
	db := r.db.WithContext(ctx)
	for id, count := range idHits {
		err := db.Model(&dbmodel.Blocked{}).
			Where(constant.WhereIDEquals, id).
			UpdateColumn(constant.FieldHitCount, gorm.Expr(constant.FieldHitCount+" + ?", count)).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func toBlockedAggregate(m *dbmodel.Blocked) *aggregate.Blocked {
	b, err := aggregate.CreateBlocked(m.ID, m.Word, m.Action)
	if err != nil {
		return nil
	}
	b.SetHitCount(m.HitCount)
	b.SetTimestamps(m.CreatedAt, m.UpdatedAt)
	return b
}

func toBlockedDBModel(b *aggregate.Blocked) *dbmodel.Blocked {
	return &dbmodel.Blocked{
		Word:   b.Word(),
		Action: b.Action(),
	}
}
