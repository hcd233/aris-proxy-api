package database

import (
	"context"
	"errors"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/gorm"
)

// MigrateUserScopeData 多租户化存量数据迁移入口（database migrate-data 命令调用）。
//
//	@author centonhuang
//	@update 2026-08-26
func MigrateUserScopeData(ctx context.Context) error {
	return MigrateUserScopeDataWith(ctx, InitDatabase())
}

// MigrateUserScopeDataWith 可注入 DB 的迁移实现：
//
//  1. 回填 user_id=0 的 endpoints/models 到主 admin（permission=admin 中 ID 最小者），幂等；
//
//  2. 重建两个复合唯一索引（GORM AutoMigrate 不会改已有同名索引的列组合）。
//
//     @param db *gorm.DB
//     @param ctx context.Context
//     @return error
//     @author centonhuang
//     @update 2026-08-26
func MigrateUserScopeDataWith(ctx context.Context, db *gorm.DB) error {
	db = db.WithContext(ctx)
	return db.Transaction(func(tx *gorm.DB) error {
		var admin dbmodel.User
		err := tx.Where(constant.DBConditionPermissionAdmin, enum.PermissionAdmin).Order(constant.DBOrderByIDAsc).First(&admin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ierr.New(ierr.ErrDataNotExists, "no admin user found for user-scope backfill")
		}
		if err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "find primary admin")
		}
		if err := tx.Model(&dbmodel.Endpoint{}).Where(constant.DBConditionUserIDZero).Update(constant.FieldUserID, admin.ID).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, err, "backfill endpoint user_id")
		}
		if err := tx.Model(&dbmodel.Model{}).Where(constant.DBConditionUserIDZero).Update(constant.FieldUserID, admin.ID).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBUpdate, err, "backfill model user_id")
		}
		if err := rebuildUniqueIndex(tx, constant.TableEndpoints, constant.IndexEndpointName, constant.ColsEndpointName); err != nil {
			return err
		}
		return rebuildUniqueIndex(tx, constant.TableModels, constant.IndexModelAliasEp, constant.ColsModelAliasEp)
	})
}

// rebuildUniqueIndex 幂等重建唯一索引：DROP IF EXISTS 后按新列组合重建。
//
// PG DDL 按语句自动提交，IF EXISTS / IF NOT EXISTS 守卫保证重跑不卡死。
//
//	@param tx *gorm.DB
//	@param table string 表名
//	@param indexName string 索引名
//	@param columns string 列组合（逗号分隔）
//	@return error
//	@author centonhuang
//	@update 2026-08-26
func rebuildUniqueIndex(tx *gorm.DB, table, indexName, columns string) error {
	if err := tx.Exec(constant.SQLDropIndex + indexName).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "drop index "+indexName)
	}
	if err := tx.Exec(constant.SQLCreateUniqueIndex + indexName + constant.SQLIndexOn + table + "(" + columns + ")").Error; err != nil {
		return ierr.Wrap(ierr.ErrDBUpdate, err, "create index "+indexName)
	}
	return nil
}
