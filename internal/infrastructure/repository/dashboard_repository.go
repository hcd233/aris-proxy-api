package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/cache"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// dashboardRepository DashboardRepository 的 GORM(PostgreSQL) 实现
type dashboardRepository struct {
	db    *gorm.DB
	cache *cache.ReadCache
}

// NewDashboardRepository 构造 dashboard 聚合仓储
//
//	@param db *gorm.DB
//	@param rc *cache.ReadCache 为 nil 时不走读缓存
//	@return modelcall.DashboardRepository
//	@author centonhuang
//	@update 2026-10-28 10:00:00
func NewDashboardRepository(db *gorm.DB, rc *cache.ReadCache) modelcall.DashboardRepository {
	return &dashboardRepository{db: db, cache: rc}
}

// isEmptyScope 非 nil 空切片 = 名下无 Key，必须短路（nil = admin/demo 全量）
func isEmptyScope(apiKeyIDs []uint) bool {
	return apiKeyIDs != nil && len(apiKeyIDs) == 0
}

// windowQuery 时间窗 + 软删 + 范围过滤的基础查询
func (r *dashboardRepository) windowQuery(ctx context.Context, apiKeyIDs []uint, start, end time.Time) *gorm.DB {
	db := r.db.WithContext(ctx).Model(&dbmodel.ModelCallAudit{}).
		Where(constant.FieldCreatedAt+" >= ? AND "+constant.FieldCreatedAt+" <= ?", start, end).
		Where(constant.DBConditionDeletedAtZero)
	if len(apiKeyIDs) > 0 {
		db = db.Where(constant.FieldAPIKeyID+" IN ?", apiKeyIDs)
	}
	return db
}

func (r *dashboardRepository) QueryStats(ctx context.Context, apiKeyIDs []uint, start, end time.Time) (*modelcall.DashboardStats, error) {
	if isEmptyScope(apiKeyIDs) {
		return &modelcall.DashboardStats{}, nil
	}
	key := readCacheKey(constant.ReadCachePurposeDashboardStats, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end))
	rows, err := cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardStats, error) {
		var results []*modelcall.DashboardStats
		if err := r.windowQuery(loadCtx, apiKeyIDs, start, end).
			Select(constant.DashboardStatsSelect).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard stats")
		}
		return results, nil
	})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || rows[0] == nil {
		return &modelcall.DashboardStats{}, nil
	}
	return rows[0], nil
}

func (r *dashboardRepository) QueryStatsSeries(ctx context.Context, apiKeyIDs []uint, start, end time.Time, granularity enum.Granularity) ([]*modelcall.DashboardStatsPoint, error) {
	if isEmptyScope(apiKeyIDs) {
		return []*modelcall.DashboardStatsPoint{}, nil
	}
	key := readCacheKey(constant.ReadCachePurposeDashboardSeries, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end), granularity)
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardStatsPoint, error) {
		var results []*modelcall.DashboardStatsPoint
		if err := r.windowQuery(loadCtx, apiKeyIDs, start, end).
			Select(dateTruncSQL(granularity) + constant.DashboardTimeBucketAlias + constant.DashboardStatsSelect).
			Group(constant.DashboardGroupByTime).
			Order(constant.DashboardGroupByTime).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard stats series")
		}
		return results, nil
	})
}

func (r *dashboardRepository) QueryModelSeries(context.Context, []uint, time.Time, time.Time, enum.Granularity) ([]*modelcall.DashboardModelPoint, error) {
	return nil, ierr.New(ierr.ErrInternal, "not implemented")
}

func (r *dashboardRepository) QueryTopErrors(context.Context, []uint, time.Time, time.Time, int) ([]*modelcall.DashboardErrorRow, error) {
	return nil, ierr.New(ierr.ErrInternal, "not implemented")
}

func (r *dashboardRepository) QueryEndpointHealth(context.Context, []uint, time.Time, time.Time, int) ([]*modelcall.DashboardEndpointRow, error) {
	return nil, ierr.New(ierr.ErrInternal, "not implemented")
}

func (r *dashboardRepository) QueryGroupUsage(context.Context, []uint, time.Time, time.Time, enum.DashboardDimension, string) ([]*modelcall.DashboardGroupRow, error) {
	return nil, ierr.New(ierr.ErrInternal, "not implemented")
}

func (r *dashboardRepository) QueryHeatmap(context.Context, []uint, time.Time, time.Time, string) ([]*modelcall.DashboardHeatmapCell, error) {
	return nil, ierr.New(ierr.ErrInternal, "not implemented")
}
