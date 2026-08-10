// Package tool_dedup 验证工具 checksum 回填迁移与去重写入的行为。
//
// 背景（bugfix/tool-checksum-dedup-2026-08-10）：
//   - ComputeToolChecksum 纳入工具级 description，存量记录需回填；
//   - tools.check_sum 新增 (check_sum, deleted_at) 复合唯一索引，去重写入改为
//     ON CONFLICT DO NOTHING + 补查，且严禁返回零值 ID。
package tool_dedup

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/vo"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestDB 建内存库并迁移 tools 表。
//
// 必须开启 TranslateError，否则唯一冲突不会被转换成 gorm.ErrDuplicatedKey，
// 回填就无法把冲突与真实故障区分开。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		TranslateError: true,
		Logger:         gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Tool{}); err != nil {
		t.Fatalf("failed to migrate tools: %v", err)
	}
	return db
}

// newTool 构造带描述的工具内容。
func newTool(name, description string) *vo.UnifiedTool {
	return &vo.UnifiedTool{Name: name, Description: description}
}

// legacyChecksum 复现旧算法（description 不参与）的结果，用于构造存量记录。
//
// 旧算法等价于新算法在 description 为空时的取值，因此对带非空描述的工具，
// 它必然不等于当前算法结果——正是回填要修正的状态。
func legacyChecksum(tool *vo.UnifiedTool) string {
	return vo.ComputeToolChecksum(&vo.UnifiedTool{Name: tool.Name, Parameters: tool.Parameters})
}

// insertTool 直接插入一条工具记录，绕过仓储以便构造任意 checksum 状态。
func insertTool(t *testing.T, db *gorm.DB, tool *vo.UnifiedTool, checksum string) *dbmodel.Tool {
	t.Helper()
	record := &dbmodel.Tool{Tool: tool, CheckSum: checksum}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("failed to insert tool %q: %v", tool.Name, err)
	}
	return record
}

// countTools 返回 tools 表行数。
func countTools(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&dbmodel.Tool{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count tools: %v", err)
	}
	return count
}

// findTool 按主键读取工具记录。
func findTool(t *testing.T, db *gorm.DB, id uint) *dbmodel.Tool {
	t.Helper()
	var record dbmodel.Tool
	if err := db.First(&record, id).Error; err != nil {
		t.Fatalf("failed to find tool %d: %v", id, err)
	}
	return &record
}
