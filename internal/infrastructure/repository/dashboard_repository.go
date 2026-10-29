package repository

import (
	"context"
	"fmt"
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

// QueryUptimeBuckets 可用性状态条的定长桶（恰好 buckets 个，无请求的桶补零）。
//
// 桶边界以 start 为原点按桶宽等分；补桶在仓储内完成，因为读缓存按 cacheTimeBucket
// 对齐的时间窗共享结果，桶边界必须随缓存内容一起固化，否则命中缓存时桶会错位。
func (r *dashboardRepository) QueryUptimeBuckets(ctx context.Context, apiKeyIDs []uint, start, end time.Time, buckets int) ([]*modelcall.DashboardUptimePoint, error) {
	if buckets <= 0 {
		return []*modelcall.DashboardUptimePoint{}, nil
	}
	if isEmptyScope(apiKeyIDs) {
		return modelcall.BuildUptimeBuckets(nil, start, end, buckets), nil
	}
	widthSeconds := int64(modelcall.UptimeBucketWidth(start, end, buckets) / time.Second)
	key := readCacheKey(constant.ReadCachePurposeDashboardUptime, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end), buckets)
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardUptimePoint, error) {
		var results []*modelcall.DashboardUptimePoint
		bucketExpr := fmt.Sprintf(constant.DateBinUptimeTemplate, widthSeconds)
		if err := r.windowQuery(loadCtx, apiKeyIDs, start, end).
			Select(bucketExpr+constant.DashboardUptimeSelect, start).
			Group(constant.DashboardGroupByTime).
			Order(constant.DashboardGroupByTime).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard uptime buckets")
		}
		return modelcall.BuildUptimeBuckets(results, start, end, buckets), nil
	})
}

func (r *dashboardRepository) QueryModelSeries(ctx context.Context, apiKeyIDs []uint, start, end time.Time, granularity enum.Granularity) ([]*modelcall.DashboardModelPoint, error) {
	if isEmptyScope(apiKeyIDs) {
		return []*modelcall.DashboardModelPoint{}, nil
	}
	key := readCacheKey(constant.ReadCachePurposeDashboardModel, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end), granularity)
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardModelPoint, error) {
		var results []*modelcall.DashboardModelPoint
		if err := r.windowQuery(loadCtx, apiKeyIDs, start, end).
			Select(dateTruncSQL(granularity) + constant.DashboardModelSeriesSelect).
			Group(constant.DashboardModelSeriesGroup).
			Order(constant.DashboardModelSeriesGroup).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard model series")
		}
		return results, nil
	})
}

func (r *dashboardRepository) QueryTopErrors(ctx context.Context, apiKeyIDs []uint, start, end time.Time, limit int) ([]*modelcall.DashboardErrorRow, error) {
	if isEmptyScope(apiKeyIDs) {
		return []*modelcall.DashboardErrorRow{}, nil
	}
	key := readCacheKey(constant.ReadCachePurposeDashboardErrors, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end), limit)
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardErrorRow, error) {
		var results []*modelcall.DashboardErrorRow
		if err := r.windowQuery(loadCtx, apiKeyIDs, start, end).
			Where(constant.DashboardConditionFailure).
			Select(constant.DashboardTopErrorsSelect).
			Group(constant.DashboardTopErrorsGroup).
			Order(constant.DashboardOrderCountDesc).
			Limit(limit).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard top errors")
		}
		return results, nil
	})
}

func (r *dashboardRepository) QueryEndpointHealth(ctx context.Context, apiKeyIDs []uint, start, end time.Time, limit int) ([]*modelcall.DashboardEndpointRow, error) {
	if isEmptyScope(apiKeyIDs) {
		return []*modelcall.DashboardEndpointRow{}, nil
	}
	key := readCacheKey(constant.ReadCachePurposeDashboardEndpoint, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end), limit)
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardEndpointRow, error) {
		var results []*modelcall.DashboardEndpointRow
		if err := r.windowQuery(loadCtx, apiKeyIDs, start, end).
			Select(constant.DashboardEndpointSelect).
			Group(constant.DashboardEndpointGroup).
			Order(constant.DashboardOrderRequestsDesc).
			Limit(limit).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard endpoint health")
		}
		return results, nil
	})
}

// groupKeyExpr 维度 → 分组键 SQL 表达式（白名单；user 维度由应用层按 apikey 聚合）
func groupKeyExpr(dimension enum.DashboardDimension) (string, error) {
	switch dimension {
	case enum.DashboardDimensionModel:
		return constant.DashboardGroupKeyModel, nil
	case enum.DashboardDimensionAPIKey:
		return constant.DashboardGroupKeyAPIKey, nil
	case enum.DashboardDimensionHarness:
		return constant.AuditHarnessGroupSQL, nil
	default:
		return "", ierr.New(ierr.ErrValidation, "unsupported dashboard dimension")
	}
}

func (r *dashboardRepository) QueryGroupUsage(ctx context.Context, apiKeyIDs []uint, start, end time.Time, dimension enum.DashboardDimension, rulesFingerprint string) ([]*modelcall.DashboardGroupRow, error) {
	keyExpr, err := groupKeyExpr(dimension)
	if err != nil {
		return nil, err
	}
	if isEmptyScope(apiKeyIDs) {
		return []*modelcall.DashboardGroupRow{}, nil
	}
	key := readCacheKey(constant.ReadCachePurposeDashboardGroup, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end), dimension, rulesFingerprint)
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardGroupRow, error) {
		var results []*modelcall.DashboardGroupRow
		query := r.windowQuery(loadCtx, apiKeyIDs, start, end).Select(keyExpr + constant.DashboardGroupUsageSelect)
		if dimension == enum.DashboardDimensionHarness {
			inner := r.windowQuery(loadCtx, apiKeyIDs, start, end).
				Select(constant.DashboardUAUsageSelect).
				Group(constant.DashboardUAUsageGroup)
			query = r.db.WithContext(loadCtx).
				Table(constant.DashboardUAUsageFrom, inner).
				Select(keyExpr + constant.DashboardUAUsageOuterSelect)
		}
		if err := query.
			Group(constant.DashboardGroupUsageGroup).
			Order(constant.DashboardGroupUsageOrder).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard group usage")
		}
		return results, nil
	})
}

func (r *dashboardRepository) QueryHeatmap(ctx context.Context, apiKeyIDs []uint, start, end time.Time, location string) ([]*modelcall.DashboardHeatmapCell, error) {
	if isEmptyScope(apiKeyIDs) {
		return []*modelcall.DashboardHeatmapCell{}, nil
	}
	key := readCacheKey(constant.ReadCachePurposeDashboardHeatmap, cacheScopeIDs(apiKeyIDs), cacheTimeBucket(start), cacheTimeBucket(end), location)
	return cachedSlice(ctx, r.cache, key, func(loadCtx context.Context) ([]*modelcall.DashboardHeatmapCell, error) {
		var results []*modelcall.DashboardHeatmapCell
		if err := r.windowQuery(loadCtx, apiKeyIDs, start, end).
			Select(constant.DashboardHeatmapSelect, location, location).
			Group(constant.DashboardHeatmapGroup).
			Find(&results).Error; err != nil {
			return nil, ierr.Wrap(ierr.ErrDBQuery, err, "query dashboard heatmap")
		}
		return results, nil
	})
}
