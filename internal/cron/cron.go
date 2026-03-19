// Package cron 定时任务模块
//
//	update 2024-12-09 15:55:25
package cron

import (
	"fmt"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// Cron 定时任务接口
//
//	@author centonhuang
//	@update 2025-09-30 16:08:18
type Cron interface {
	Start() error
}

// InitCronJobs 初始化定时任务
//
//	author centonhuang
//	update 2024-12-09 15:55:20
func InitCronJobs() {
	sessionDeduplicateCron := NewSessionDeduplicateCron()
	lo.Must0(sessionDeduplicateCron.Start())

	logger.Logger().Info("[Cron] Init cron jobs")
}

type cronLoggerAdapter struct {
	module string
	logger *zap.Logger
}

func newCronLoggerAdapter(module string, logger *zap.Logger) cronLoggerAdapter {
	if module == "" {
		module = "Cron"
	}
	module = strings.TrimSpace(strings.TrimRight(strings.TrimLeft(strings.TrimSpace(module), "["), "]"))
	return cronLoggerAdapter{module: module, logger: logger}
}

func (l cronLoggerAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	zapKeyValues := []zap.Field{zap.Error(err)}
	zapKeyValues = append(zapKeyValues, convertZapKeyValues(keysAndValues...)...)
	l.logger.Error(fmt.Sprintf("[%s] %s", l.module, msg), zapKeyValues...)
}

func (l cronLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	zapKeyValues := convertZapKeyValues(keysAndValues...)
	l.logger.Info(fmt.Sprintf("[%s] %s", l.module, msg), zapKeyValues...)
}

func convertZapKeyValues(keysAndValues ...interface{}) []zap.Field {
	if len(keysAndValues)%2 != 0 {
		panic("keysAndValues must be a slice of key-value pairs")
	}
	kvLen := len(keysAndValues) / 2
	zapKeyValues := make([]zap.Field, 0, kvLen)
	for i := 0; i < kvLen; i++ {
		key, value := keysAndValues[i*2].(string), keysAndValues[i*2+1]
		zapKeyValues = append(zapKeyValues, zap.Any(key, value))
	}
	return zapKeyValues
}
