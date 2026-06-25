package cache

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/redis/go-redis/v9"
)

// RuntimeMetricsCache 运行时指标的 Redis 共享时序存储。
//
// 用两类 ZSET：实例注册表（心跳）+ 每实例快照时序（score=unix秒）。
// 实现 metrics.SnapshotStore 的写入能力，并提供聚合查询所需的读取能力。
//
//	@author centonhuang
//	@update 2026-06-25 10:00:00
type RuntimeMetricsCache struct {
	client *redis.Client
}

// NewRuntimeMetricsCache 创建运行时指标缓存
//
//	@param client *redis.Client
//	@return *RuntimeMetricsCache
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func NewRuntimeMetricsCache(client *redis.Client) *RuntimeMetricsCache {
	return &RuntimeMetricsCache{client: client}
}

func runtimeMetricsDataKey(instanceID string) string {
	return fmt.Sprintf(constant.RuntimeMetricsDataKeyTemplate, instanceID)
}

// WriteSnapshot 写入一份快照并滚动裁剪过期数据（实现 metrics.SnapshotStore）。
//
//	@receiver c *RuntimeMetricsCache
//	@param instanceID string
//	@param score int64 快照 unix 秒
//	@param payload []byte 快照 JSON
//	@param retentionCutoff int64 保留窗口下界 unix 秒（早于此的快照/死实例被清理）
//	@return error
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func (c *RuntimeMetricsCache) WriteSnapshot(instanceID string, score int64, payload []byte, retentionCutoff int64) error {
	ctx := context.Background()
	dataKey := runtimeMetricsDataKey(instanceID)
	cutoff := strconv.FormatInt(retentionCutoff, constant.DecimalBase)

	pipe := c.client.Pipeline()
	pipe.ZAdd(ctx, dataKey, redis.Z{Score: float64(score), Member: string(payload)})
	pipe.ZRemRangeByScore(ctx, dataKey, "0", "("+cutoff)
	pipe.ZAdd(ctx, constant.RuntimeMetricsInstancesKey, redis.Z{Score: float64(score), Member: instanceID})
	pipe.ZRemRangeByScore(ctx, constant.RuntimeMetricsInstancesKey, "0", "("+cutoff)
	_, err := pipe.Exec(ctx)
	return err
}

// ListInstances 列出 since（unix 秒）之后有过心跳的活跃实例。
//
//	@receiver c *RuntimeMetricsCache
//	@param ctx context.Context
//	@param sinceUnix int64
//	@return []string
//	@return error
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func (c *RuntimeMetricsCache) ListInstances(ctx context.Context, sinceUnix int64) ([]string, error) {
	return c.client.ZRangeByScore(ctx, constant.RuntimeMetricsInstancesKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(sinceUnix, constant.DecimalBase),
		Max: constant.RedisZRangePositiveInfinity,
	}).Result()
}

// ReadSnapshots 读取单个实例在 [startUnix, endUnix] 窗口内、按时间升序的快照 payload。
//
//	@receiver c *RuntimeMetricsCache
//	@param ctx context.Context
//	@param instanceID string
//	@param startUnix int64
//	@param endUnix int64
//	@return [][]byte
//	@return error
//	@author centonhuang
//	@update 2026-06-25 10:00:00
func (c *RuntimeMetricsCache) ReadSnapshots(ctx context.Context, instanceID string, startUnix, endUnix int64) ([][]byte, error) {
	raw, err := c.client.ZRangeByScore(ctx, runtimeMetricsDataKey(instanceID), &redis.ZRangeBy{
		Min: strconv.FormatInt(startUnix, constant.DecimalBase),
		Max: strconv.FormatInt(endUnix, constant.DecimalBase),
	}).Result()
	if err != nil {
		return nil, err
	}
	payloads := make([][]byte, len(raw))
	for i, s := range raw {
		payloads[i] = []byte(s)
	}
	return payloads, nil
}
