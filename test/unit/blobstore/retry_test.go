package blobstore

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	"github.com/hcd233/aris-mass/internal/infrastructure/database/dao"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
)

func dueBlob(sha string, attempts int, now time.Time) *dbmodel.Blob {
	b := pendingBlob(sha, now.Add(-time.Minute))
	b.Attempts = attempts
	return b
}

func reload(t *testing.T, e *testEnv, id uint) *dbmodel.Blob {
	t.Helper()
	got, err := dao.GetBlobDAO().GetByID(e.db, id)
	if err != nil || got == nil {
		t.Fatalf("reload: %v", err)
	}
	return got
}

func TestRetrySuccessMarksUploaded(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	now := time.Now().UTC()
	b := insertBlob(t, e.db, dueBlob(shaOf('a'), 1, now))
	stats, err := blobstore.RetryUploads(t.Context(), e.db, e.store, now)
	got := reload(t, e, b.ID)
	if err != nil || stats.Uploaded != 1 || got.Status != enum.BlobStatusUploaded || got.Data != nil {
		t.Fatalf("err=%v stats=%+v row=%+v", err, stats, got)
	}
	if !bytes.Equal(e.store.objects[got.StorageKey], []byte("img")) {
		t.Fatal("object content mismatch")
	}
}

// TestRetryCompressesRawPending 上传池来不及处理而直接落库的原图：补传前压缩，并写回存储版本元数据
func TestRetryCompressesRawPending(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	now := time.Now().UTC()
	raw := encodePNG(t, noiseRGBA(1200, 1200))
	if len(raw) <= 2<<20 {
		t.Fatalf("fixture must exceed 2MB, got %d", len(raw))
	}
	sha := vo.SHA256Hex(raw)
	b := dueBlob(sha, 0, now)
	b.Data, b.Mime, b.Size, b.StorageKey = raw, "image/png", int64(len(raw)), blobstore.StorageKey(sha, "image/png")
	insertBlob(t, e.db, b)

	stats, err := blobstore.RetryUploads(t.Context(), e.db, e.store, now)
	got := reload(t, e, b.ID)
	if err != nil || stats.Uploaded != 1 || got.Status != enum.BlobStatusUploaded {
		t.Fatalf("err=%v stats=%+v row=%+v", err, stats, got)
	}
	obj := e.store.objects[got.StorageKey]
	if len(obj) == 0 || len(obj) > 2<<20 || got.Size != int64(len(obj)) || got.Width == 0 || got.Height == 0 {
		t.Fatalf("object=%d row=%+v", len(obj), got)
	}
}

// TestRetryUnprocessableRawStopsRetrying 无法压缩到上限内的原图转 unprocessable，不再上传
func TestRetryUnprocessableRawStopsRetrying(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	now := time.Now().UTC()
	b := dueBlob(shaOf('c'), 0, now)
	b.Data = bytes.Repeat([]byte{0x01, 0x02, 0x03}, 1<<20)
	insertBlob(t, e.db, b)

	if _, err := blobstore.RetryUploads(t.Context(), e.db, e.store, now); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got := reload(t, e, b.ID); got.Status != enum.BlobStatusUnprocessable || len(got.Data) == 0 || e.store.totalPuts() != 0 {
		t.Fatalf("row=%+v puts=%d", got, e.store.totalPuts())
	}
}

// TestTruncateErrorKeepsValidUTF8 多字节字符按字符截断，不能切出非法 UTF-8（PG varchar 会拒绝写入）
func TestTruncateErrorKeepsValidUTF8(t *testing.T) {
	t.Parallel()
	got := blobstore.TruncateError(&longError{msg: strings.Repeat("上传失败", 200)})
	if !utf8.ValidString(got) || utf8.RuneCountInString(got) != 512 {
		t.Fatalf("valid=%v runes=%d", utf8.ValidString(got), utf8.RuneCountInString(got))
	}
}

func TestRetryFailureBacksOff(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.store.failPut = true
	now := time.Now().UTC().Truncate(time.Second)
	b := insertBlob(t, e.db, dueBlob(shaOf('a'), 1, now))
	if _, err := blobstore.RetryUploads(t.Context(), e.db, e.store, now); err != nil {
		t.Fatalf("retry: %v", err)
	}
	got := reload(t, e, b.ID)
	if got.Attempts != 2 || !got.NextRetryAt.Equal(now.Add(15*time.Minute)) || got.LastError == "" || got.Status != enum.BlobStatusPending {
		t.Fatalf("row=%+v", got)
	}
}

func TestRetryBackoffCapped(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.store.failPut = true
	now := time.Now().UTC().Truncate(time.Second)
	b := insertBlob(t, e.db, dueBlob(shaOf('a'), 8, now))
	_, _ = blobstore.RetryUploads(t.Context(), e.db, e.store, now)
	got := reload(t, e, b.ID)
	if got.Attempts != 9 || !got.NextRetryAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("row=%+v", got)
	}
}

func TestRetryReachesMaxGoesFailed(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	e.store.failPut = true
	now := time.Now().UTC()
	b := insertBlob(t, e.db, dueBlob(shaOf('a'), 9, now))
	stats, _ := blobstore.RetryUploads(t.Context(), e.db, e.store, now)
	got := reload(t, e, b.ID)
	if got.Status != enum.BlobStatusFailed || got.Attempts != 10 || got.Data == nil || stats.Failed != 1 {
		t.Fatalf("stats=%+v row=%+v", stats, got)
	}
}

func TestRetrySkipsNotDue(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	now := time.Now().UTC()
	insertBlob(t, e.db, pendingBlob(shaOf('a'), now.Add(time.Hour)))
	stats, _ := blobstore.RetryUploads(t.Context(), e.db, e.store, now)
	if e.store.putCalls != 0 || stats.Pending != 1 {
		t.Fatalf("putCalls=%d stats=%+v", e.store.putCalls, stats)
	}
}

func TestRetrySkipsUnprocessableAndFailed(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	now := time.Now().UTC()
	for i, status := range []string{enum.BlobStatusUnprocessable, enum.BlobStatusFailed} {
		b := dueBlob(shaOf(byte('a'+i)), 1, now)
		b.Status = status
		insertBlob(t, e.db, b)
	}
	_, _ = blobstore.RetryUploads(t.Context(), e.db, e.store, now)
	if e.store.putCalls != 0 {
		t.Fatalf("putCalls=%d", e.store.putCalls)
	}
}

func TestRetryNilStoreNoop(t *testing.T) {
	t.Parallel()
	e := newEnv(t, false)
	now := time.Now().UTC()
	b := insertBlob(t, e.db, dueBlob(shaOf('a'), 1, now))
	if _, err := blobstore.RetryUploads(t.Context(), e.db, nil, now); err != nil {
		t.Fatalf("err=%v", err)
	}
	if got := reload(t, e, b.ID); got.Status != enum.BlobStatusPending || got.Attempts != 1 {
		t.Fatalf("row changed: %+v", got)
	}
}

func TestRetryRowWithoutDataSkipped(t *testing.T) {
	t.Parallel()
	e := newEnv(t, true)
	now := time.Now().UTC()
	broken := dueBlob(shaOf('a'), 1, now.Add(-time.Hour))
	broken.Data = nil
	b1 := insertBlob(t, e.db, broken)
	b2 := insertBlob(t, e.db, dueBlob(shaOf('b'), 1, now))
	stats, err := blobstore.RetryUploads(t.Context(), e.db, e.store, now)
	if err != nil || stats.Uploaded != 1 || reload(t, e, b2.ID).Status != enum.BlobStatusUploaded {
		t.Fatalf("err=%v stats=%+v", err, stats)
	}
	if got := reload(t, e, b1.ID); got.Status == enum.BlobStatusUploaded || got.LastError == "" {
		t.Fatalf("broken row: %+v", got)
	}
}

func TestRetryLastErrorTruncated(t *testing.T) {
	t.Parallel()
	err := strings.Repeat("x", 2000)
	if got := blobstore.TruncateError(&longError{msg: err}); len(got) != 512 {
		t.Fatalf("len=%d", len(got))
	}
}

type longError struct{ msg string }

func (e *longError) Error() string { return e.msg }
