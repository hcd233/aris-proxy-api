// Package tool_dedup 验证工具去重写入的行为。
//
// 背景（bugfix/tool-checksum-dedup-2026-08-10）：
//   - ComputeToolChecksum 纳入工具级 description，同名同参数但描述不同的工具不再合并；
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

// newTestDB 建内存库并迁移 tools 表，开启 TranslateError 以与生产 GORM 配置对齐。
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

// countTools 返回 tools 表行数。
func countTools(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&dbmodel.Tool{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count tools: %v", err)
	}
	return count
}
