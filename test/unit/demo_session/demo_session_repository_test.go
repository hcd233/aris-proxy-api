// Package demo_session 验证 demo 会话白名单仓储的 Add 去重、List 排序与 Remove。
package demo_session

import (
	"context"
	"reflect"
	"strings"
	"testing"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.DemoSession{}); err != nil {
		t.Fatalf("failed to migrate demo_sessions: %v", err)
	}
	return db
}

func TestAddDeduplicatesAndListSorts(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := repository.NewDemoSessionRepository(db)

	if err := repo.Add(context.Background(), []uint{3, 1, 2, 3, 1}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	want := []uint{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}

	var count int64
	if err := db.Model(&dbmodel.DemoSession{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 3 {
		t.Fatalf("row count = %d, want 3 (dedup)", count)
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := repository.NewDemoSessionRepository(db)

	if err := repo.Add(context.Background(), []uint{1, 2, 3}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := repo.Remove(context.Background(), []uint{2, 3, 3}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	want := []uint{1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %v, want %v", got, want)
	}
}

func TestEmptyIDsNoOp(t *testing.T) {
	t.Parallel()
	db := newTestDB(t)
	repo := repository.NewDemoSessionRepository(db)

	if err := repo.Add(context.Background(), nil); err != nil {
		t.Fatalf("Add(empty) failed: %v", err)
	}
	if err := repo.Remove(context.Background(), nil); err != nil {
		t.Fatalf("Remove(empty) failed: %v", err)
	}

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List = %v, want empty", got)
	}
}
