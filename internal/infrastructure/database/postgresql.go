// Package database 存储中间件
//
//	update 2024-06-22 09:04:46
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/config"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
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

// AutoMigrate 自动迁移数据库表
//
//	return *gorm.DB
//	author centonhuang
//	update 2024-09-22 10:04:36
//	@param ctx
//	@return error
//	@author centonhuang
//	@update 2026-06-15 21:52:16
func AutoMigrate(ctx context.Context) error {
	return InitDatabase().WithContext(ctx).AutoMigrate(model.Models...)
}

// toolChecksumBackfillStats 工具 checksum 回填统计
//
//	@author centonhuang
//	@update 2026-08-10 10:00:00
type toolChecksumBackfillStats struct {
	total     int
	unchanged int
	updated   int
	conflict  int
}

// BackfillToolChecksums 将存量 tool 的 check_sum 回填为当前算法结果
//
//	背景（bugfix/tool-checksum-dedup-2026-08-10）：ComputeToolChecksum 纳入了工具级
//	description，存量记录的 check_sum 由旧算法生成。不回填会导致同一工具被当成新工具
//	重复插入，存量记录退化为永不再命中的孤儿。
//
//	正确性：新算法输入 (name, description, parameters) 是旧算法输入 (name, parameters)
//	的超集，因此原本互不相同的 checksum 重算后仍互不相同——回填是一对一 UPDATE，
//	不产生记录合并，无需 remap sessions.tool_ids。
//
//	幂等：重算值等于现值时跳过，可安全重复执行。
//
//	容错：逐行独立 UPDATE 而非包在单一事务内，使单行唯一冲突不会中断整批。唯一冲突意味着
//	已存在同 checksum 的记录（新版本在回填前就已按新算法写入），此时保留旧行并计数跳过；
//	非冲突错误直接返回中断，避免把真实故障当成冲突静默忽略。
//
//	@param ctx context.Context
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func BackfillToolChecksums(ctx context.Context) error {
	return BackfillToolChecksumsWithDB(InitDatabase().WithContext(ctx))
}

// BackfillToolChecksumsWithDB 在指定 db 上执行工具 checksum 回填
//
//	与 BackfillToolChecksums 的唯一区别是 db 由外部注入，便于测试使用内存库。
//	语义与容错策略见 BackfillToolChecksums 的说明。
//
//	注意：调用方的 db 必须开启 gorm.Config.TranslateError，否则唯一冲突无法被识别为
//	gorm.ErrDuplicatedKey，会被当作未知错误中断回填。
//
//	@param db *gorm.DB
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func BackfillToolChecksumsWithDB(db *gorm.DB) error {
	log := logger.Logger()

	var stats toolChecksumBackfillStats
	var batch []*model.Tool
	err := db.Model(&model.Tool{}).FindInBatches(&batch, config.SQLBatchSize, func(_ *gorm.DB, _ int) error {
		for _, record := range batch {
			if err := backfillToolChecksum(db, record, &stats); err != nil {
				return err
			}
		}
		return nil
	}).Error
	if err != nil {
		return ierr.Wrap(ierr.ErrDBQuery, err, "backfill tool checksums")
	}

	log.Info("[Database] Tool checksum backfill completed",
		zap.Int("total", stats.total),
		zap.Int("unchanged", stats.unchanged),
		zap.Int("updated", stats.updated),
		zap.Int("conflict", stats.conflict))
	return nil
}

// backfillToolChecksum 回填单条 tool 记录的 check_sum
//
//	@param db *gorm.DB
//	@param record *model.Tool
//	@param stats *toolChecksumBackfillStats
//	@return error
//	@author centonhuang
//	@update 2026-08-10 10:00:00
func backfillToolChecksum(db *gorm.DB, record *model.Tool, stats *toolChecksumBackfillStats) error {
	stats.total++
	if record.Tool == nil {
		stats.unchanged++
		return nil
	}

	want := vo.ComputeToolChecksum(record.Tool)
	if want == record.CheckSum {
		stats.unchanged++
		return nil
	}

	err := db.Model(&model.Tool{ID: record.ID}).
		Select(constant.FieldCheckSum, constant.FieldUpdatedAt).
		Updates(map[string]any{
			constant.FieldCheckSum:  want,
			constant.FieldUpdatedAt: time.Now().UTC(),
		}).Error
	switch {
	case err == nil:
		stats.updated++
	case errors.Is(err, gorm.ErrDuplicatedKey):
		stats.conflict++
		logger.Logger().Warn("[Database] Tool checksum backfill conflict, keeping legacy row",
			zap.Uint("toolID", record.ID),
			zap.String("wantChecksum", want))
	default:
		return err
	}
	return nil
}

// InitDatabase 初始化数据库
//
//	@return *gorm.DB
//	@author centonhuang
//	@update 2026-06-15 21:52:09
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
