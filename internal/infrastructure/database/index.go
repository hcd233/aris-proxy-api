// Package database 存储中间件
//
//	update 2024-06-22 09:04:46
package database

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"
)

// EnsureSearchIndexes 幂等创建 keyword 检索依赖的 PG 索引。
//
// 背景（feature/session-keyword-trgm-perf-2026-06-07）：
//   - GET /api/v1/session/list?keyword=xxx 走
//     "EXISTS (SELECT 1 FROM messages WHERE messages.message::text ILIKE '%kw%' ...)"
//     ，没有 trigram 索引时是 messages 表顺序扫描；线上体量下一次请求秒级。
//   - 通过本函数在 database migrate 阶段建好 pg_trgm + 两类 GIN 索引，
//     ILIKE 退化为 trigram bitmap 扫描，jsonb_exists 反查走 sessions GIN。
//
// 全部 SQL 使用 IF NOT EXISTS，重入安全；任何一条失败立即返回并 wrap
// ierr.ErrDBCreate，由调用方决定是否降级（生产环境应视为启动失败）。
//
//	@param ctx context.Context
//	@return error
//	@author centonhuang
//	@update 2026-06-07 02:00:00
func EnsureSearchIndexes(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	log := logger.WithCtx(ctx)

	for i, ddl := range constant.SessionKeywordIndexSQLs {
		if err := db.Exec(ddl).Error; err != nil {
			log.Error("[Database] Failed to ensure search index",
				zap.Int("step", i),
				zap.String("ddl", ddl),
				zap.Error(err))
			return ierr.Wrap(ierr.ErrDBCreate, err, fmt.Sprintf("ensure search index step %d", i))
		}
		log.Info("[Database] Ensured search index step", zap.Int("step", i), zap.String("ddl", ddl))
	}
	return nil
}
