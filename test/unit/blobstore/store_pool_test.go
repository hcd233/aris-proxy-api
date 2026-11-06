package blobstore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/dto"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
	"github.com/hcd233/aris-mass/internal/infrastructure/pool"
	"github.com/hcd233/aris-mass/internal/util"
)

// openStorePoolDB 建好 blobs/session/message/tool 表，并返回 session 写入完成的通知通道
func openStorePoolDB(t *testing.T) (db *gorm.DB, persisted chan struct{}) {
	t.Helper()
	db = openBlobDB(t)
	if err := db.AutoMigrate(&dbmodel.Session{}, &dbmodel.Message{}, &dbmodel.Tool{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	persisted = make(chan struct{}, 1)
	if err := db.Callback().Create().After("gorm:create").Register("test:session_created", func(tx *gorm.DB) {
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
	return db, persisted
}

// storeMessages 走 PoolManager 落库一轮消息，等 session 写入后停止所有池（图片上传随之完成）
func storeMessages(t *testing.T, db *gorm.DB, persisted chan struct{}, msgs []*vo.UnifiedMessage) {
	t.Helper()
	ext := blobstore.NewExternalizer(db, blobstore.NewAsyncUploader(db, newFakeStore(), 1, 10))
	pm := pool.NewPoolManager(db, nil, nil, nil, ext)
	if err := pm.SubmitMessageStoreTask(&dto.MessageStoreTask{
		Ctx:      util.CopyContextValues(context.Background()),
		Messages: msgs,
	}); err != nil {
		t.Fatalf("submit: %v", err)
	}
	select {
	case <-persisted:
	case <-time.After(5 * time.Second):
		t.Fatal("session not persisted within timeout")
	}
	pm.Stop()
}

// TestStoreTaskPersistsReference 落库路径：message 行只存引用，图片进 blobs
func TestStoreTaskPersistsReference(t *testing.T) {
	t.Parallel()
	db, persisted := openStorePoolDB(t)
	raw := smallPNG(t, 42)
	storeMessages(t, db, persisted, []*vo.UnifiedMessage{imageMsg(dataURI(raw))})

	var stored []string
	if err := db.Table("messages").Pluck("message", &stored).Error; err != nil || len(stored) != 1 {
		t.Fatalf("messages=%v err=%v", stored, err)
	}
	if strings.Contains(stored[0], "base64") || !strings.Contains(stored[0], vo.ImageRefOf(vo.SHA256Hex(raw))) {
		t.Fatalf("message not externalized: %s", stored[0])
	}
	var blobs int64
	db.Model(&dbmodel.Blob{}).Count(&blobs)
	if blobs != 1 {
		t.Fatalf("blobs=%d", blobs)
	}
}

// TestStoreTaskDowngradesClientImageRef 客户端自带的 aris-blob 引用落库时降级为文本，不能成为取图凭据
func TestStoreTaskDowngradesClientImageRef(t *testing.T) {
	t.Parallel()
	db, persisted := openStorePoolDB(t)
	forged := vo.ImageRefOf(shaOf('f'))
	storeMessages(t, db, persisted, []*vo.UnifiedMessage{imageMsg(forged)})

	var stored []string
	if err := db.Table("messages").Pluck("message", &stored).Error; err != nil || len(stored) != 1 {
		t.Fatalf("messages=%v err=%v", stored, err)
	}
	var msg vo.UnifiedMessage
	if err := sonic.UnmarshalString(stored[0], &msg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if refs := vo.ImageRefSHAs(&msg); len(refs) != 0 {
		t.Fatalf("forged ref stored as image: %s", stored[0])
	}
	if p := msg.Content.Parts[1]; p.Type != enum.ContentPartTypeText || p.Text != forged {
		t.Fatalf("forged ref not kept as text: %+v", p)
	}
}
