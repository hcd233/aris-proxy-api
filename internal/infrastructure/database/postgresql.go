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

// ManualMigrations 执行 GORM AutoMigrate 无法覆盖的删列/改列，幂等可重入。
//
// 必须在 AutoMigrate 之前执行：旧 model_call_audits.model_id 为 uint 类型，
// 先删旧列再 rename model→model_id，AutoMigrate 才能正确新建 text 列。
// 注意：dbmodel 结构已更新（无旧列字段），HasColumn 检查的是真实数据库表列。
func ManualMigrations(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	migrator := db.Migrator()

	// model_call_audits：旧库存在 model 列（存 alias）与 model_id 列（uint 主键）。
	// 先删 uint model_id 列，再把 model 列改名为 model_id。
	if migrator.HasColumn(&model.ModelCallAudit{}, "model") {
		if err := migrator.DropColumn(&model.ModelCallAudit{}, "model_id"); err != nil {
			return err
		}
		if err := migrator.RenameColumn(&model.ModelCallAudit{}, "model", "model_id"); err != nil {
			return err
		}
	}

	// sessions：models 列改名为 model_ids
	if migrator.HasColumn(&model.Session{}, "models") {
		if err := migrator.RenameColumn(&model.Session{}, "models", "model_ids"); err != nil {
			return err
		}
	}

	// messages：model 列改名为 model_id
	if migrator.HasColumn(&model.Message{}, "model") {
		if err := migrator.RenameColumn(&model.Message{}, "model", "model_id"); err != nil {
			return err
		}
	}
	return nil
}

// BackfillModelIDs 回填 models.model_id = alias，幂等（仅空值行）。
//
// 必须在 AutoMigrate 之后执行（依赖新列已存在）。
func BackfillModelIDs(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	if !db.Migrator().HasColumn(&model.Model{}, "model_id") {
		return nil
	}
	return db.Model(&model.Model{}).
		Where("model_id IS NULL OR model_id = ''").
		Update("model_id", gorm.Expr("alias")).Error
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
