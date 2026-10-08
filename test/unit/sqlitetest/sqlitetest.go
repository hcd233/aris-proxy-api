// Package sqlitetest 单元测试共用的 sqlite 内存库构造
package sqlitetest

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// OpenPrivateMemoryDB 每个用例独立的内存库（以 t.Name() 命名、cache=private，避免用例间串扰）。
//
// cache=private 的内存库按连接隔离：database/sql 连接池一旦开出第二条连接就会读写到另一个空库
// （全量并行下表现为偶发读到空表），故限单连接保证读写共库。
//
//	@param t *testing.T
//	@return *gorm.DB
//	@author centonhuang
//	@update 2026-10-08 10:00:00
func OpenPrivateMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"),
		&gorm.Config{TranslateError: true, Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	return db
}
