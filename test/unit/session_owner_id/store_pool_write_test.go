package session_owner_id

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	commonvo "github.com/hcd233/aris-proxy-api/internal/common/vo"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
	"github.com/hcd233/aris-proxy-api/internal/infrastructure/pool"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// TestStorePoolPersistsAPIKeyID 协程池落库时必须写入 api_key_id。
//
// 这是新会话获得归属的**唯一**入口：store_pool 构造 dbmodel.Session 时若漏
// 写 APIKeyID，所有新会话的归属恒为 0，对普通用户永久不可见——正是缺陷 B
// 在 name 维度造成的事故，故 ID 维度必须有直接守护。
//
// 同步方式：提交任务后调 Stop() 排空协程池（pond 的 Stop 会等待在途任务），
// 不用 time.Sleep（项目 lint 在 AST 级禁止）。
func TestStorePoolPersistsAPIKeyID(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"),
		&gorm.Config{TranslateError: true, Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&dbmodel.Session{}, &dbmodel.Message{}, &dbmodel.Tool{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 落库完成信号：PoolManager.Stop() 丢弃了 pond 返回的 future（未 Wait），
	// 不能用它同步；改用 GORM after-create 回调把落库事件送进 channel，
	// 配合 select + time.After 等待（仓库先例见 test/unit/pool_manager）。
	// 按表名过滤而非按 Statement.Dest 类型断言：dao.Create 传入的是 **dbmodel.Session
	// （baseDAO.Create 对已是指针的入参又取了一次地址），依赖该内部形态很脆弱。
	persisted := make(chan struct{}, 1)
	if err := db.Callback().Create().After("gorm:create").Register("test:session_created",
		func(tx *gorm.DB) {
			if tx.Statement.Table != "sessions" {
				return
			}
			select {
			case persisted <- struct{}{}:
			default:
			}
		}); err != nil {
		t.Fatalf("register callback: %v", err)
	}

	// auditRepo / demoAccessAuditRepo 传 nil：本用例只走消息存储路径，不触达它们
	pm := pool.NewPoolManager(db, nil, nil)

	const wantAPIKeyID uint = 77
	ctx := context.WithValue(context.Background(), constant.CtxKeyAPIKeyID, wantAPIKeyID)
	ctx = context.WithValue(ctx, constant.CtxKeyAPIKeyName, "my-key")

	task := &dto.MessageStoreTask{
		Ctx:        util.CopyContextValues(ctx),
		APIKeyName: util.CtxValueString(ctx, constant.CtxKeyAPIKeyName),
		APIKeyID:   util.CtxValueUint(ctx, constant.CtxKeyAPIKeyID),
		ModelID:    "gpt-test",
		Messages: []*commonvo.UnifiedMessage{
			{Role: "user", Content: &commonvo.UnifiedContent{Text: "hello"}},
		},
	}
	if err := pm.SubmitMessageStoreTask(task); err != nil {
		t.Fatalf("submit store task: %v", err)
	}

	select {
	case <-persisted:
	case <-time.After(5 * time.Second):
		t.Fatal("session was not persisted by store pool within timeout")
	}

	var row dbmodel.Session
	if err := db.Order("id desc").First(&row).Error; err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if row.APIKeyID != wantAPIKeyID {
		t.Errorf("persisted api_key_id = %d, want %d", row.APIKeyID, wantAPIKeyID)
	}
	if row.APIKeyName != "my-key" {
		t.Errorf("persisted api_key_name = %q, want %q", row.APIKeyName, "my-key")
	}
}
