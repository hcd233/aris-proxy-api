// Package cron 定时任务模块
//
//	update 2024-12-09 15:55:25
package cron

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// Cron 定时任务接口
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
type Cron interface {
	Start() error
	Stop()
}

var cronInstances []Cron

// InitCronJobs 初始化定时任务
//
//	author centonhuang
//	update 2026-04-02 10:00:00
func InitCronJobs() {
	sessionDeduplicateCron := NewSessionDeduplicateCron()
	lo.Must0(sessionDeduplicateCron.Start())
	cronInstances = append(cronInstances, sessionDeduplicateCron)

	sessionSummarizeCron := NewSessionSummarizeCron()
	lo.Must0(sessionSummarizeCron.Start())
	cronInstances = append(cronInstances, sessionSummarizeCron)

	sessionScoreCron := NewSessionScoreCron()
	lo.Must0(sessionScoreCron.Start())
	cronInstances = append(cronInstances, sessionScoreCron)

	softDeletePurgeCron := NewSoftDeletePurgeCron()
	lo.Must0(softDeletePurgeCron.Start())
	cronInstances = append(cronInstances, softDeletePurgeCron)

	logger.Logger().Info("[Cron] Init cron jobs")
}

// StopCronJobs 停止所有定时任务，用于优雅关闭
//
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func StopCronJobs() {
	for _, c := range cronInstances {
		c.Stop()
	}
	logger.Logger().Info("[Cron] All cron jobs stopped")
}

type cronLoggerAdapter struct {
	module string
	logger *zap.Logger
}

func newCronLoggerAdapter(module string, logger *zap.Logger) *cronLoggerAdapter {
	if module == "" {
		module = "Cron"
	}
	module = strings.TrimSpace(strings.TrimRight(strings.TrimLeft(strings.TrimSpace(module), "["), "]"))
	return &cronLoggerAdapter{module: module, logger: logger}
}

func (l *cronLoggerAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	zapKeyValues := []zap.Field{zap.Error(err)}
	zapKeyValues = append(zapKeyValues, convertZapKeyValues(keysAndValues...)...)
	l.logger.Error(fmt.Sprintf("[%s] %s", l.module, capitalizeFirst(msg)), zapKeyValues...)
}

func (l *cronLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	zapKeyValues := convertZapKeyValues(keysAndValues...)
	l.logger.Info(fmt.Sprintf("[%s] %s", l.module, capitalizeFirst(msg)), zapKeyValues...)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+utf8.RuneLen(r):]
	}
	return s
}

func convertZapKeyValues(keysAndValues ...interface{}) []zap.Field {
	if len(keysAndValues)%2 != 0 {
		return []zap.Field{zap.String("error", "keysAndValues must be a slice of key-value pairs")}
	}
	kvLen := len(keysAndValues) / 2
	zapKeyValues := make([]zap.Field, 0, kvLen)
	for i := 0; i < kvLen; i++ {
		key, ok := keysAndValues[i*2].(string)
		if !ok {
			key = "invalid_key"
		}
		value := keysAndValues[i*2+1]
		zapKeyValues = append(zapKeyValues, zap.Any(key, value))
	}
	return zapKeyValues
}
