// Package database 存储中间件
//
//	update 2024-06-22 09:04:46
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"go.uber.org/zap"

	"github.com/samber/lo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// CloseDatabase 关闭数据库连接池，用于优雅关闭
//
//	@param db *gorm.DB
//	@return error
//	@author centonhuang
//	@update 2026-03-20 10:00:00
func CloseDatabase(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return ierr.Wrap(ierr.ErrDBClose, err, "get underlying sql.DB")
	}
	return sqlDB.Close()
}

// InitDatabase 初始化数据库
//
//	return *gorm.DB
//	author centonhuang
//	update 2024-09-22 10:04:36
func InitDatabase() *gorm.DB {
	var dialector gorm.Dialector
	var dbHost, dbPort, dbName string

	dsn := fmt.Sprintf(constant.PostgresDSNTemplate,
		config.PostgresHost, config.PostgresUser, config.PostgresPassword,
		config.PostgresDatabase, config.PostgresPort, config.PostgresSSLMode)
	dialector = postgres.Open(dsn)
	dbHost, dbPort, dbName = config.PostgresHost, config.PostgresPort, config.PostgresDatabase

	// 	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
	// 		config.MysqlUser, config.MysqlPassword, config.MysqlHost, config.MysqlPort, config.MysqlDatabase)
	// 	dialector = mysql.New(mysql.Config{
	// 		DSN:               dsn,
	// 		DefaultStringSize: 256,
	// 	})
	// 	dbHost, dbPort, dbName = config.MysqlHost, config.MysqlPort, config.MysqlDatabase

	db := lo.Must(gorm.Open(dialector, &gorm.Config{
		DryRun:         false, // 只生成SQL不运行
		TranslateError: true,
		Logger: &GormLoggerAdapter{
			LogLevel: gormlogger.Info, // Info级别
		},
	}))

	sqlDB := lo.Must(db.DB())

	sqlDB.SetMaxIdleConns(constant.PostgresMaxIdleConns)
	sqlDB.SetMaxOpenConns(constant.PostgresMaxOpenConns)
	sqlDB.SetConnMaxLifetime(constant.PostgresConnMaxLifetime)

	logger.Logger().Info("[Database] Connected to database",
		zap.String("host", dbHost),
		zap.String("port", dbPort),
		zap.String("database", dbName))
	return db
}

// GormLoggerAdapter 实现gorm的logger接口,使用zap输出SQL日志
//
//	author centonhuang
//	update 2025-01-05 21:10:18
type GormLoggerAdapter struct {
	LogLevel gormlogger.LogLevel
}

// LogMode 设置日志级别
//
//	receiver l *GormLogger
//	param level gormlogger.LogLevel
//	return gormlogger.Interface
//	author centonhuang
//	update 2025-01-05 21:10:15
func (l *GormLoggerAdapter) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 打印info级别的日志
//
//	receiver l *GormLogger
//	param _ context.Context
//	param msg string
//	param data ...any
//	author centonhuang
//	update 2025-01-05 21:11:07
func (l *GormLoggerAdapter) Info(ctx context.Context, msg string, data ...any) {
	logger.WithCtx(ctx).Info("[GORM] Info", zap.String("msg", fmt.Sprintf(msg, data...)))
}

// Warn 打印warn级别的日志
//
//	receiver l *GormLogger
//	param _ context.Context
//	param msg string
//	param data ...any
//	author centonhuang
//	update 2025-01-05 21:11:08
func (l *GormLoggerAdapter) Warn(ctx context.Context, msg string, data ...any) {
	logger.WithCtx(ctx).Warn("[GORM] Warn", zap.String("msg", fmt.Sprintf(msg, data...)))
}

// Error 打印error级别的日志
// π
//
//	receiver l *GormLogger
//	param _ context.Context
//	param msg string
//	param data ...any
//	author centonhuang
//	update 2025-01-05 21:11:10
func (l *GormLoggerAdapter) Error(ctx context.Context, msg string, data ...any) {
	logger.WithCtx(ctx).Error("[GORM] Error", zap.String("msg", fmt.Sprintf(msg, data...)))
}

// Trace 打印trace级别的日志
//
//	receiver l *GormLogger
//	param _ context.Context
//	param begin time.Time
//	param fc func() (string, int64)
//	param err error
//	author centonhuang
//	update 2025-01-05 21:11:11
func (l *GormLoggerAdapter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zap.Field{
		zap.String("sql", sql),
		zap.Int64("rows", rows),
		zap.String("elapsed", elapsed.String()),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
		logger.WithCtx(ctx).Error("[GORM] Trace", fields...)
		return
	}

	logger.WithCtx(ctx).Info("[GORM] Trace", fields...)
}
