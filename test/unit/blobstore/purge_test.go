package blobstore

import (
	"slices"
	"testing"

	"gorm.io/gorm"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
)

type purgeEnv struct {
	db               *gorm.DB
	store            *fakeStore
	doomed, survivor *dbmodel.Message
	// a 只被 doomed 引用（已上传）；b 被两条消息引用；c 只被 doomed 引用（pending）
	a, b, c *dbmodel.Blob
}

func newPurgeEnv(t *testing.T) *purgeEnv {
	t.Helper()
	db := openBlobDB(t)
	if err := db.AutoMigrate(&dbmodel.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	e := &purgeEnv{db: db, store: newFakeStore()}
	e.a = insertBlob(t, db, &dbmodel.Blob{SHA256: shaOf('a'), Status: enum.BlobStatusUploaded, StorageKey: "images/a.png", Mime: "image/png"})
	e.b = insertBlob(t, db, &dbmodel.Blob{SHA256: shaOf('b'), Status: enum.BlobStatusUploaded, StorageKey: "images/b.png", Mime: "image/png"})
	e.c = insertBlob(t, db, pendingBlob(shaOf('c'), e.a.CreatedAt))
	e.store.objects["images/a.png"] = []byte("a")
	e.store.objects["images/b.png"] = []byte("b")

	e.doomed = &dbmodel.Message{CheckSum: "doomed", Message: imageMsg(vo.ImageRefOf(e.a.SHA256), vo.ImageRefOf(e.b.SHA256), vo.ImageRefOf(e.c.SHA256))}
	e.survivor = &dbmodel.Message{CheckSum: "survivor", Message: imageMsg(vo.ImageRefOf(e.b.SHA256))}
	for _, m := range []*dbmodel.Message{e.doomed, e.survivor} {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create message: %v", err)
		}
	}
	return e
}

// collectAndDeleteDoomed 按软删清理 cron 的顺序：先收集引用，再硬删 message
func (e *purgeEnv) collectAndDeleteDoomed(t *testing.T) []string {
	t.Helper()
	shas, err := blobstore.CollectImageRefs(t.Context(), e.db, []uint{e.doomed.ID})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	slices.Sort(shas)
	if !slices.Equal(shas, []string{e.a.SHA256, e.b.SHA256, e.c.SHA256}) {
		t.Fatalf("collected=%v", shas)
	}
	if _, err := dao.GetMessageDAO().HardDeleteByIDs(e.db, []uint{e.doomed.ID}); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	return shas
}

func (e *purgeEnv) exists(t *testing.T, b *dbmodel.Blob) bool {
	t.Helper()
	row, err := dao.GetBlobDAO().GetBySHA256(e.db, b.SHA256)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return row != nil
}

// TestDeleteUnreferencedKeepsSharedImages 只删不再被引用的图片；已上传的同时删对象
func TestDeleteUnreferencedKeepsSharedImages(t *testing.T) {
	t.Parallel()
	e := newPurgeEnv(t)
	shas := e.collectAndDeleteDoomed(t)

	n, err := blobstore.DeleteUnreferenced(t.Context(), e.db, e.store, shas)
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if e.exists(t, e.a) || e.exists(t, e.c) || !e.exists(t, e.b) {
		t.Fatalf("a=%v b=%v c=%v", e.exists(t, e.a), e.exists(t, e.b), e.exists(t, e.c))
	}
	if !slices.Equal(e.store.deletes, []string{"images/a.png"}) || e.store.objects["images/b.png"] == nil {
		t.Fatalf("deletes=%v", e.store.deletes)
	}
}

// TestDeleteUnreferencedKeepsRowWhenObjectDeleteFails 对象删除失败时保留行，避免留下无从追踪的孤儿对象
func TestDeleteUnreferencedKeepsRowWhenObjectDeleteFails(t *testing.T) {
	t.Parallel()
	e := newPurgeEnv(t)
	e.store.failDelete = true
	shas := e.collectAndDeleteDoomed(t)

	n, err := blobstore.DeleteUnreferenced(t.Context(), e.db, e.store, shas)
	if err != nil || n != 1 || !e.exists(t, e.a) || e.exists(t, e.c) {
		t.Fatalf("n=%d err=%v a=%v c=%v", n, err, e.exists(t, e.a), e.exists(t, e.c))
	}
}

// TestDeleteUnreferencedWithoutStoreKeepsUploaded 对象存储未配置时 uploaded 行保留
func TestDeleteUnreferencedWithoutStoreKeepsUploaded(t *testing.T) {
	t.Parallel()
	e := newPurgeEnv(t)
	shas := e.collectAndDeleteDoomed(t)

	n, err := blobstore.DeleteUnreferenced(t.Context(), e.db, nil, shas)
	if err != nil || n != 1 || !e.exists(t, e.a) || e.exists(t, e.c) {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

// TestCollectImageRefsIgnoresTextRefs 文本中出现的引用字符串不算引用
func TestCollectImageRefsIgnoresTextRefs(t *testing.T) {
	t.Parallel()
	db := openBlobDB(t)
	if err := db.AutoMigrate(&dbmodel.Message{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ref := vo.ImageRefOf(shaOf('d'))
	m := &dbmodel.Message{CheckSum: "text", Message: &vo.UnifiedMessage{Role: enum.RoleUser, Content: &vo.UnifiedContent{
		Parts: []*vo.UnifiedContentPart{{Type: enum.ContentPartTypeText, Text: `"image_url":"` + ref + `"`}},
	}}}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	shas, err := blobstore.CollectImageRefs(t.Context(), db, []uint{m.ID})
	if err != nil || len(shas) != 0 {
		t.Fatalf("shas=%v err=%v", shas, err)
	}
}
