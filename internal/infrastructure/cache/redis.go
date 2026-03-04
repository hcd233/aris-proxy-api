// Package cache Redis缓存模块
//
//	update 2024-12-09 15:56:25
package cache

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var rdb *redis.Client

const redisDB = 0

// GetRedisClient 获取Redis客户端
//
//	return *redis.Client
//	author centonhuang
//	update 2024-12-09 15:56:40
func GetRedisClient() *redis.Client {
	return rdb
}

// InitCache 初始化Redis客户端
//
//	author centonhuang
//	update 2024-12-09 15:56:36
func InitCache() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort),
		Password: config.RedisPassword,
		DB:       redisDB,
	})

	_ = lo.Must1(rdb.Ping(context.Background()).Result())

	logger.Logger().Info("[Cache] Connected to Redis database", zap.String("host", config.RedisHost), zap.String("port", config.RedisPort), zap.Int("db", redisDB))
}
