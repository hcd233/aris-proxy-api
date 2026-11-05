package objectstore

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hcd233/aris-mass/internal/config"
	"github.com/hcd233/aris-mass/internal/infrastructure/objectstore"
)

type recordedRequest struct {
	method, path, contentType string
	body                      []byte
}

// fakeS3 最小 S3：PUT 存对象，GET 取对象
type fakeS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
	last    recordedRequest
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.last = recordedRequest{method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type"), body: body}
	switch r.Method {
	case http.MethodPut:
		f.objects[r.URL.Path] = body
		w.Header().Set("ETag", `"etag"`)
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		data, ok := f.objects[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("ETag", `"etag"`)
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(data)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func configure(t *testing.T, endpoint string) {
	t.Helper()
	config.ObjectStorageEndpoint = endpoint
	config.ObjectStorageBucket = "aris"
	config.ObjectStorageRegion = "us-east-1"
	config.ObjectStorageAccessKey = "ak"
	config.ObjectStorageSecretKey = "sk"
	config.ObjectStorageTLS = false
	config.ObjectStoragePathStyle = true
	t.Cleanup(func() { config.ObjectStorageEndpoint = "" })
}

func newFake(t *testing.T) (fake *fakeS3, endpoint string) {
	t.Helper()
	fake = &fakeS3{objects: map[string][]byte{}}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	return fake, srv.URL
}

//nolint:paralleltest // 修改包级 config
func TestNewFromConfigUnconfiguredIsNil(t *testing.T) {
	configure(t, "")
	if store := objectstore.NewFromConfig(); store != nil {
		t.Fatalf("expected nil store, got %#v", store)
	}
}

//nolint:paralleltest // 修改包级 config
func TestNewFromConfigInvalidIsNil(t *testing.T) {
	configure(t, "http://bad host")
	if store := objectstore.NewFromConfig(); store != nil {
		t.Fatalf("expected nil store, got %#v", store)
	}
}

//nolint:paralleltest // 修改包级 config
func TestPutSendsKeyAndContentType(t *testing.T) {
	fake, url := newFake(t)
	configure(t, url)
	store := objectstore.NewFromConfig()
	data := []byte("png-bytes")
	if err := store.Put(t.Context(), "images/ab/cd/x.png", "image/png", data); err != nil {
		t.Fatalf("put: %v", err)
	}
	if fake.last.method != http.MethodPut || fake.last.path != "/aris/images/ab/cd/x.png" || fake.last.contentType != "image/png" {
		t.Fatalf("unexpected request: %+v", fake.last)
	}
	if !bytes.Equal(fake.objects["/aris/images/ab/cd/x.png"], data) {
		t.Fatalf("body mismatch: %q", fake.objects["/aris/images/ab/cd/x.png"])
	}
}

//nolint:paralleltest // 修改包级 config
func TestGetReturnsBody(t *testing.T) {
	fake, url := newFake(t)
	configure(t, url)
	fake.objects["/aris/k.png"] = []byte("stored")
	got, err := objectstore.NewFromConfig().Get(t.Context(), "k.png")
	if err != nil || string(got) != "stored" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := objectstore.NewFromConfig().Get(t.Context(), "missing.png"); err == nil {
		t.Fatal("expected error for missing object")
	}
}

//nolint:paralleltest // 修改包级 config
func TestPresignGet(t *testing.T) {
	_, url := newFake(t)
	configure(t, url)
	signed, err := objectstore.NewFromConfig().PresignGet(t.Context(), "images/ab/cd/x.png", 10*time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	for _, want := range []string{"/aris/images/ab/cd/x.png", "X-Amz-Signature=", "X-Amz-Expires=600"} {
		if !strings.Contains(signed, want) {
			t.Errorf("presigned url %q missing %q", signed, want)
		}
	}
}
