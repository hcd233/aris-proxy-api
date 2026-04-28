// Package cache Redis缓存模块
//
//	update 2024-12-09 15:56:25
package cache

import (
	"context"
	"fmt"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

var rdb *redis.Client

// GetRedisClient 获取Redis客户端
//
//	return *redis.Client
//	author centonhuang
//	update 2024-12-09 15:56:40
func GetRedisClient() *redis.Client {
	return rdb
}

// CloseCache 关闭Redis客户端连接，用于优雅关闭
//
//	@return error
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func CloseCache() error {
	if rdb == nil {
		return nil
	}
	return rdb.Close()
}

// InitCache 初始化Redis客户端
//
//	author centonhuang
//	update 2024-12-09 15:56:36
func InitCache() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf(constant.HostPortTemplate, config.RedisHost, config.RedisPort),
		Password: config.RedisPassword,
		DB:       constant.RedisDB,
	})

	_ = lo.Must1(rdb.Ping(context.Background()).Result())

	logger.Logger().Info("[Cache] Connected to Redis database", zap.String("host", config.RedisHost), zap.String("port", config.RedisPort), zap.Int("db", constant.RedisDB))
}
