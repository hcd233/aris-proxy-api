// Package database 存储中间件
//
//	update 2026-06-07 21:50:00
package database

import (
	"context"

	"go.uber.org/zap"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/logger"
)

// PostMigrate 在 AutoMigrate 之后执行幂等的"补齐"动作，包括：
//
//  1. session 列表 baseline 路径的复合 BTREE 索引（CreatedAt 来自 BaseModel，
//     没法只在 Session 表上通过 GORM tag 表达，落在这里手动建）。
//  2. message_count / tool_count 物化冗余列的存量回填（新数据写入路径自维护）。
//
// 设计原则（参考 75658e5 / 11e4602 的回滚事故）：
//
//   - 全部 SQL 走标准 BTREE / UPDATE，不依赖 pg_trgm / 任何扩展，不需要 superuser；
//
//   - 全部 SQL 必须可重入（DDL 用 IF NOT EXISTS，DML 用 WHERE 限定到未回填行），
//     这样 migrate Job 反复跑不会出现首次成功、二次失败这种"骨牌效应"；
//
//   - 任意一条失败立即 wrap ierr.ErrDBQuery 返回，由调用方决定是否退出。
//     生产环境的 cmd database migrate 在主流程里 panic，让 K8s migrate Job
//     直接失败，避免半完成状态推到流量层。
//
//     @param ctx context.Context
//     @return error
//     @author centonhuang
//     @update 2026-06-07 21:50:00
func PostMigrate(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	log := logger.WithCtx(ctx)

	for i, ddl := range constant.SessionPerfPostMigrateSQLs {
		if err := db.Exec(ddl).Error; err != nil {
			log.Error("[Database] PostMigrate step failed",
				zap.Int("step", i),
				zap.String("sql", ddl),
				zap.Error(err))
			return ierr.Wrap(ierr.ErrDBQuery, err, "post-migrate sql")
		}
		log.Info("[Database] PostMigrate step OK",
			zap.Int("step", i),
			zap.String("sql", ddl))
	}
	return nil
}
