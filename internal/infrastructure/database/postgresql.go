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
	"gorm.io/gorm/clause"
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
// 由调用方传入连接，使迁移与其后的数据回填共用同一连接池。
// AutoMigrate 之后再补齐它表达不了的表达式索引（keyword 搜索依赖）。
//
//	@param ctx context.Context
//	@param db *gorm.DB
//	@return error
//	@author centonhuang
//	@update 2026-09-27 10:00:00
func AutoMigrate(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(model.Models...); err != nil {
		return err
	}
	if err := seedAgentHarnessRules(ctx, db); err != nil {
		// 种子失败不阻塞启动：规则可由 admin 后台补建，缺失时全部 UA 归 Other。
		logger.WithCtx(ctx).Warn("[Database] Seed harness rules failed", zap.Error(err))
	}
	return ensureKeywordSearchIndex(ctx, db)
}

// seedAgentHarnessRules 首次启动时写入内置 harness 规则种子。
//
// 只在表为空时插入；之后完全由 admin 后台维护（删除全部规则时重启不会复活，
// 因为这里以「表为空」为触发条件而非「缺某条」）。
func seedAgentHarnessRules(ctx context.Context, db *gorm.DB) error {
	var count int64
	if err := db.WithContext(ctx).Model(&model.AgentHarnessRule{}).Count(&count).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBMigrate, err, "count harness rules")
	}
	if count > 0 {
		return nil
	}
	seeds := lo.Map(constant.AgentHarnessRuleSeeds, func(seed constant.AgentHarnessRuleSeedRule, _ int) model.AgentHarnessRule {
		return model.AgentHarnessRule{Pattern: seed.Pattern, Name: seed.Name}
	})
	if err := db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: constant.FieldPattern}}, DoNothing: true}).
		Create(&seeds).Error; err != nil {
		return ierr.Wrap(ierr.ErrDBMigrate, err, "seed harness rules")
	}
	return nil
}

// ensureKeywordSearchIndex 建立 keyword 搜索依赖的 pg_trgm 表达式索引。
//
// 该索引带「表达式 + 操作符类 + 部分谓词」，GORM tag 无法表达，历史上只手工在生产建过；
// 新环境缺索引时 keyword 搜索退化为全表扫描。DDL 幂等（IF NOT EXISTS），已建环境为 no-op。
// 用非 CONCURRENTLY 版本：新环境 messages 为空、生产索引已存在，启动期不会被大表重建拖住。
//
// 审计列表的 idx_mca_active_created 部分索引不在此处：按运维约定手工执行
// constant.AuditActiveCreatedIndexDDL（CREATE INDEX CONCURRENTLY，避免部署期锁表）。
func ensureKeywordSearchIndex(ctx context.Context, db *gorm.DB) error {
	for _, statement := range []string{constant.PgTrgmExtensionDDL, constant.MessageContentTrigramIndexDDL} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBMigrate, err, "ensure keyword search index")
		}
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
