package blobstore

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-mass/internal/common/enum"
	"github.com/hcd233/aris-mass/internal/common/vo"
	"github.com/hcd233/aris-mass/internal/domain/blob"
	"github.com/hcd233/aris-mass/internal/infrastructure/blobstore"
	dbmodel "github.com/hcd233/aris-mass/internal/infrastructure/database/model"
)

type readerEnv struct {
	*migrateEnv
	reader blob.ImageReader
}

func newReaderEnv(t *testing.T, withStore bool) *readerEnv {
	t.Helper()
	m := newMigrateEnv(t)
	var store blob.ObjectStore
	if withStore {
		store = m.store
	}
	return &readerEnv{migrateEnv: m, reader: blobstore.NewReader(m.db, store)}
}

func uploadedBlob(sha string) *dbmodel.Blob {
	return &dbmodel.Blob{SHA256: sha, Status: enum.BlobStatusUploaded, StorageKey: "images/" + sha + ".png", Mime: "image/png", Size: 5, Width: 2, Height: 3}
}

func TestLocateUploadedReturnsURL(t *testing.T) {
	t.Parallel()
	e := newReaderEnv(t, true)
	insertBlob(t, e.db, uploadedBlob(shaOf('a')))
	view, err := e.reader.Locate(t.Context(), shaOf('a'))
	if err != nil || view == nil || !strings.Contains(view.URL, "images/") || view.Data != nil || view.Width != 2 {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	if view.ExpiresAt == nil || view.ExpiresAt.Before(time.Now().Add(9*time.Minute)) || view.ExpiresAt.After(time.Now().Add(11*time.Minute)) {
		t.Fatalf("expiresAt=%v", view.ExpiresAt)
	}
}

func TestLocatePendingReturnsData(t *testing.T) {
	t.Parallel()
	e := newReaderEnv(t, true)
	insertBlob(t, e.db, pendingBlob(shaOf('a'), time.Now()))
	view, err := e.reader.Locate(t.Context(), shaOf('a'))
	if err != nil || view == nil || view.URL != "" || !bytes.Equal(view.Data, []byte("img")) || view.Mime != "image/png" {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestLocateMissingReturnsNil(t *testing.T) {
	t.Parallel()
	e := newReaderEnv(t, true)
	if view, err := e.reader.Locate(t.Context(), shaOf('z')); view != nil || err != nil {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestLocateUploadedNoStoreErrors(t *testing.T) {
	t.Parallel()
	e := newReaderEnv(t, false)
	insertBlob(t, e.db, uploadedBlob(shaOf('a')))
	if _, err := e.reader.Locate(t.Context(), shaOf('a')); err == nil {
		t.Fatal("expected error when object storage is not configured")
	}
}

func TestIsReferenced(t *testing.T) {
	t.Parallel()
	e := newReaderEnv(t, true)
	sha := vo.SHA256Hex([]byte("x"))
	mine := seedMessage(t, e.db, imageMsg(vo.ImageRefOf(sha)), "a", 0)
	other := seedMessage(t, e.db, imageMsg(vo.ImageRefOf(vo.SHA256Hex([]byte("y")))), "b", 0)
	stranger := seedMessage(t, e.db, imageMsg(vo.ImageRefOf(vo.SHA256Hex([]byte("z")))), "c", 0)

	if ok, err := e.reader.IsReferenced(t.Context(), []uint{mine.ID, other.ID}, sha); !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if ok, _ := e.reader.IsReferenced(t.Context(), []uint{other.ID}, sha); ok {
		t.Fatal("must not match other message")
	}
	if ok, _ := e.reader.IsReferenced(t.Context(), []uint{mine.ID}, vo.SHA256Hex([]byte("z"))); ok {
		t.Fatalf("image of message %d must not leak through message %d", stranger.ID, mine.ID)
	}
	if ok, _ := e.reader.IsReferenced(t.Context(), nil, sha); ok {
		t.Fatal("empty ids must not match")
	}
}

func TestReadBytes(t *testing.T) {
	t.Parallel()
	e := newReaderEnv(t, true)
	up := uploadedBlob(shaOf('a'))
	insertBlob(t, e.db, up)
	e.store.objects[up.StorageKey] = []byte("from-store")
	insertBlob(t, e.db, pendingBlob(shaOf('b'), time.Now()))

	if mime, data, err := e.reader.ReadBytes(t.Context(), shaOf('a')); err != nil || string(data) != "from-store" || mime != "image/png" {
		t.Fatalf("uploaded: mime=%s data=%q err=%v", mime, data, err)
	}
	if _, data, err := e.reader.ReadBytes(t.Context(), shaOf('b')); err != nil || string(data) != "img" {
		t.Fatalf("pending: data=%q err=%v", data, err)
	}
	if _, _, err := e.reader.ReadBytes(t.Context(), shaOf('z')); err == nil {
		t.Fatal("missing must error")
	}
}
