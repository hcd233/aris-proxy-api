package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	demoport "github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/samber/lo"
)

type demoSessionRepository struct {
	db *gorm.DB
}

func NewDemoSessionRepository(db *gorm.DB) demoport.DemoSessionRepository {
	return &demoSessionRepository{db: db}
}

func (r *demoSessionRepository) List(ctx context.Context) ([]uint, error) {
	var rows []dbmodel.DemoSession
	if err := r.db.WithContext(ctx).
		Order(clause.OrderByColumn{Column: clause.Column{Name: constant.FieldSessionID}, Desc: false}).
		Find(&rows).Error; err != nil {
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "list demo sessions")
	}
	return lo.Map(rows, func(m dbmodel.DemoSession, _ int) uint { return m.SessionID }), nil
}

func (r *demoSessionRepository) Add(ctx context.Context, ids []uint) error {
	ids = lo.Uniq(ids)
	if len(ids) == 0 {
		return nil
	}
	rows := lo.Map(ids, func(id uint, _ int) dbmodel.DemoSession {
		return dbmodel.DemoSession{SessionID: id}
	})
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBCreate, err, "add demo sessions")
	}
	return nil
}

func (r *demoSessionRepository) Remove(ctx context.Context, ids []uint) error {
	ids = lo.Uniq(ids)
	if len(ids) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Where(constant.FieldSessionID+" IN ?", ids).
		Delete(&dbmodel.DemoSession{}).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBDelete, err, "remove demo sessions")
	}
	return nil
}
