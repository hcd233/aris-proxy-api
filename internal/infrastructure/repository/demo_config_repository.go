package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// demoConfigRepository Demo 配置单行表的 GORM 实现
type demoConfigRepository struct {
	db *gorm.DB
}

// NewDemoConfigRepository 构造
//
//	@param db *gorm.DB
//	@return port.DemoConfigRepository
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func NewDemoConfigRepository(db *gorm.DB) port.DemoConfigRepository {
	return &demoConfigRepository{db: db}
}

// Get 读取单例配置行；表为空时返回默认配置（不落库，零值登录入口关闭）
//
//	@receiver r *demoConfigRepository
//	@param ctx context.Context
//	@return *port.DemoConfigEntity
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (r *demoConfigRepository) Get(ctx context.Context) (*port.DemoConfigEntity, error) {
	db := r.db.WithContext(ctx)
	record := &dbmodel.DemoConfig{}
	err := db.First(record, constant.DemoConfigSingletonID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &port.DemoConfigEntity{
				LoginEnabled: false,
				Modules:      []string{},
			}, nil
		}
		return nil, ierr.Wrap(ierr.ErrDBQuery, err, "get demo config")
	}
	return toDemoConfigEntity(record), nil
}

// Save 保存单例配置行（主键固定，全字段覆盖；serializer:json 需 struct Save 生效）
//
//	@receiver r *demoConfigRepository
//	@param ctx context.Context
//	@param entity *port.DemoConfigEntity
//	@return error
//	@author centonhuang
//	@update 2026-08-16 10:00:00
func (r *demoConfigRepository) Save(ctx context.Context, entity *port.DemoConfigEntity) error {
	db := r.db.WithContext(ctx)
	record := &dbmodel.DemoConfig{
		ID:           constant.DemoConfigSingletonID,
		LoginEnabled: entity.LoginEnabled,
		Modules:      entity.Modules,
		UpdatedAt:    time.Now().UTC(),
	}
	if err := db.Save(record).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "save demo config")
	}
	return nil
}

func toDemoConfigEntity(m *dbmodel.DemoConfig) *port.DemoConfigEntity {
	return &port.DemoConfigEntity{
		LoginEnabled: m.LoginEnabled,
		Modules:      m.Modules,
		UpdatedAt:    m.UpdatedAt,
	}
}
