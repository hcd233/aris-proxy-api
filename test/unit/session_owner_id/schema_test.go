// Package session_owner_id 会话/Trace 归属改按 API Key ID 判定的回归测试。
//
// 背景：归属原依赖 api_key_name，而该名称在 (user_id, name) 维度唯一、
// 可跨用户重复，导致普通用户建同名 Key 即可越权操作他人会话。
// 改为 api_key_id 后名称降级为纯展示字段。
//
//	@author centonhuang
//	@update 2026-09-27
package session_owner_id

import (
	"testing"

	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/test/unit/sqlitetest"
)

// TestSessionAPIKeyIDColumnAndIndex sessions 表必须有 api_key_id 列与其索引。
//
// 归属过滤是热路径（列表/详情/删除/评分/导出都要过），无索引会退化全表扫描。
func TestSessionAPIKeyIDColumnAndIndex(t *testing.T) {
	t.Parallel()
	db := sqlitetest.OpenPrivateMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.Session{}); err != nil {
		t.Fatalf("automigrate sessions: %v", err)
	}
	if !db.Migrator().HasColumn(&dbmodel.Session{}, "api_key_id") {
		t.Error("sessions.api_key_id column missing")
	}
	if !db.Migrator().HasIndex(&dbmodel.Session{}, "idx_sessions_api_key_id") {
		t.Error("index idx_sessions_api_key_id missing")
	}
}

// TestTraceAPIKeyIDColumnAndIndex traces 表同理。
func TestTraceAPIKeyIDColumnAndIndex(t *testing.T) {
	t.Parallel()
	db := sqlitetest.OpenPrivateMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.Trace{}); err != nil {
		t.Fatalf("automigrate traces: %v", err)
	}
	if !db.Migrator().HasColumn(&dbmodel.Trace{}, "api_key_id") {
		t.Error("traces.api_key_id column missing")
	}
	if !db.Migrator().HasIndex(&dbmodel.Trace{}, "idx_traces_api_key_id") {
		t.Error("index idx_traces_api_key_id missing")
	}
}

// TestAPIKeyIDDefaultsToZero 未显式赋值时归属为 0（存量空归属语义）。
//
// 自增主键从 1 起，故 0 天然不匹配任何真实 Key——这是「空归属仅 admin 可见」
// 得以成立的前提，不需要额外守卫。
func TestAPIKeyIDDefaultsToZero(t *testing.T) {
	t.Parallel()
	db := sqlitetest.OpenPrivateMemoryDB(t)
	if err := db.AutoMigrate(&dbmodel.Session{}); err != nil {
		t.Fatalf("automigrate sessions: %v", err)
	}
	row := &dbmodel.Session{APIKeyName: "legacy", MessageIDs: []uint{1}, ToolIDs: []uint{}}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("create session: %v", err)
	}
	var got dbmodel.Session
	if err := db.First(&got, row.ID).Error; err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if got.APIKeyID != 0 {
		t.Errorf("APIKeyID default = %d, want 0", got.APIKeyID)
	}
}
