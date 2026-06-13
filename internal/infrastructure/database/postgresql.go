// Package database 存储中间件
//
//	update 2024-06-22 09:04:46
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/sonic"
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

// InitDatabase 初始化数据库
//
//	return *gorm.DB
//	author centonhuang
//	update 2024-09-22 10:04:36
func AutoMigrate(ctx context.Context) error {
	db := InitDatabase().WithContext(ctx)
	if err := db.AutoMigrate(model.Models...); err != nil {
		return err
	}
	return migrateMessageChecksums(db.WithContext(ctx))
}

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

// migrateMessageChecksums 三阶段数据迁移
//
// Phase 1: 刷新有 reasoning_content 的消息 check_sum 为 content-only
// Phase 2: 合并 checksum 重复的消息，更新会话引用，删除冗余
//
//	@param db *gorm.DB
//	@return error
//	@author centonhuang
//	@update 2026-06-13 10:00:00
func migrateMessageChecksums(db *gorm.DB) error {
	log := logger.Logger()
	const batchSize = 1000

	// ── Phase 1: 刷新 checksum ──
	log.Info("[Database] Phase 1: refreshing message checksums for reasoning_content")
	offset := 0
	for {
		var messages []*model.Message
		if err := db.Where("message::jsonb->>'reasoning_content' IS NOT NULL").
			Select("id, message").
			Order("id").
			Limit(batchSize).
			Offset(offset).
			Find(&messages).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "phase 1: select messages with reasoning")
		}
		if len(messages) == 0 {
			break
		}
		for _, m := range messages {
			newCS := vo.ComputeMessageChecksum(m.Message, nil)
			if err := db.Model(&model.Message{ID: m.ID}).
				Where("check_sum != ?", newCS).
				Update("check_sum", newCS).Error; err != nil {
				return ierr.Wrap(ierr.ErrDBUpdate, err, "phase 1: update checksum")
			}
		}
		offset += len(messages)
		log.Info("[Database] Phase 1 progress", zap.Int("processed", offset))
	}
	log.Info("[Database] Phase 1 complete")

	// ── Phase 2: 合并重复消息 ──
	log.Info("[Database] Phase 2: merging duplicate messages by content checksum")
	phase2Offset := 0
	for {
		type dupRow struct {
			CheckSum string `gorm:"column:check_sum"`
			IDs      string `gorm:"column:ids"`
		}
		var groups []dupRow
		if err := db.Raw(`
			SELECT check_sum, json_agg(id ORDER BY
				CASE WHEN message::jsonb->>'reasoning_content' IS NOT NULL THEN 0 ELSE 1 END,
				id DESC
			) AS ids
			FROM messages
			WHERE check_sum != ''
			GROUP BY check_sum
			HAVING count(*) > 1
			LIMIT ? OFFSET ?
		`, batchSize, phase2Offset).Scan(&groups).Error; err != nil {
			return ierr.Wrap(ierr.ErrDBQuery, err, "phase 2: find duplicate groups")
		}
		if len(groups) == 0 {
			break
		}
		for _, g := range groups {
			var ids []uint
			if err := sonic.UnmarshalString(g.IDs, &ids); err != nil {
				return ierr.Wrap(ierr.ErrDBQuery, err, "phase 2: unmarshal ids")
			}
			if len(ids) < 2 {
				continue
			}
			keepID := ids[0]
			for _, oldID := range ids[1:] {
				if err := db.Exec(`
					UPDATE sessions SET message_ids = (
						SELECT COALESCE(jsonb_agg(
							CASE WHEN value = ?::jsonb THEN ?::jsonb ELSE value END
						), '[]'::jsonb)
						FROM jsonb_array_elements(COALESCE(message_ids::jsonb, '[]'::jsonb)) AS t(value)
					)
					WHERE message_ids::jsonb @> ?::jsonb
				`, oldID, keepID, oldID).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBUpdate, err, "phase 2: update session message_ids")
				}
				if err := db.Exec(`
					UPDATE sessions SET questions = (
						SELECT COALESCE(jsonb_agg(
							CASE WHEN value = ?::jsonb THEN ?::jsonb ELSE value END
						), '[]'::jsonb)
						FROM jsonb_array_elements(COALESCE(questions::jsonb, '[]'::jsonb)) AS t(value)
					)
					WHERE questions IS NOT NULL AND questions::jsonb @> ?::jsonb
				`, oldID, keepID, oldID).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBUpdate, err, "phase 2: update session questions")
				}
				if err := db.Delete(&model.Message{ID: oldID}).Error; err != nil {
					return ierr.Wrap(ierr.ErrDBDelete, err, "phase 2: delete redundant message")
				}
			}
		}
		phase2Offset += len(groups)
		log.Info("[Database] Phase 2 progress", zap.Int("groups_processed", phase2Offset))
	}
	log.Info("[Database] Phase 2 complete")

	return nil
}
