package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// BackfillSessionAPIKeyID 按 api_key_name 幂等回填 sessions/traces 的 api_key_id。
//
// 仅回填能唯一确定归属的存量行，规则见 constant.APIKeyIDBackfillSQLTemplate。
// 空归属、跨用户同名（含历史已删同名）、名称无对应 key 的行保持 0（仅 admin 可见），
// 不做任何推断——错误回填会把他人会话划给当前同名 key 的持有者，危害大于不可见。
//
// 必须在新版本滚动部署之前执行：新版本按 api_key_id 过滤，未回填会让存量
// 会话对普通用户短时全不可见。
//
//	@param ctx context.Context
//	@param db *gorm.DB
//	@return sessions int64 本次回填的 sessions 行数
//	@return traces int64 本次回填的 traces 行数
//	@return err error
//	@author centonhuang
//	@update 2026-09-27 10:00:00
func BackfillSessionAPIKeyID(ctx context.Context, db *gorm.DB) (sessions, traces int64, err error) {
	db = db.WithContext(ctx)
	sessions, err = backfillTable(db, constant.SessionTableName)
	if err != nil {
		return 0, 0, err
	}
	traces, err = backfillTable(db, constant.TraceTableName)
	if err != nil {
		return sessions, 0, err
	}
	return sessions, traces, nil
}

// backfillTable 对单表执行回填。table 只接受包内常量，不接受外部输入。
func backfillTable(db *gorm.DB, table string) (int64, error) {
	sql := fmt.Sprintf(constant.APIKeyIDBackfillSQLTemplate, table, table, table, table)
	result := db.Exec(sql)
	if result.Error != nil {
		return 0, ierr.Wrap(ierr.ErrDBUpdate, result.Error, "backfill "+table+" api_key_id")
	}
	return result.RowsAffected, nil
}
