package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// releaseRequests 记录本地 release 服务的请求次数
type releaseRequests struct {
	head atomic.Int32
	get  atomic.Int32
}

// tarGz 将单个 aris 二进制打包为 release 资产
func tarGz(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{
		Name: constant.ArisClientBinaryFileName,
		Mode: constant.ArisClientUpdateBinaryMode,
		Size: int64(len(binary)),
	}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// sha256Hex 计算归档摘要
func sha256Hex(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// newReleaseServer 本地 release 服务：HEAD 资产地址返回 302（Location 含 tag），GET 返回归档与摘要
func newReleaseServer(t *testing.T, latest string, archive []byte, checksum string) (*httptest.Server, *releaseRequests) {
	t.Helper()
	counts := &releaseRequests{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			counts.head.Add(1)
			writer.Header().Set("Location", "/releases/download/"+latest+request.URL.Path)
			writer.WriteHeader(http.StatusFound)
			return
		}
		counts.get.Add(1)
		if strings.HasSuffix(request.URL.Path, constant.ArisClientReleaseChecksumSuffix) {
			_, _ = writer.Write([]byte(checksum))
			return
		}
		_, _ = writer.Write(archive)
	}))
	t.Cleanup(server.Close)
	return server, counts
}

// writeTarget 在临时目录里放置一个"当前二进制"
func writeTarget(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aris")
	if err := os.WriteFile(path, []byte(content), constant.ArisClientUpdateBinaryMode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunReplacesBinaryWithLatestRelease(t *testing.T) {
	t.Parallel()
	newBinary := []byte("new-binary-content")
	archive := tarGz(t, newBinary)
	server, counts := newReleaseServer(t, "v0.3.0", archive, sha256Hex(t, archive))
	target := writeTarget(t, "old-binary-content")

	if err := update.Run(t.Context(), update.Options{
		Current:    "v0.2.2",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, newBinary) {
		t.Fatalf("installed binary = %q, want %q", installed, newBinary)
	}
	if counts.head.Load() != 1 || counts.get.Load() != 2 {
		t.Fatalf("requests = head %d, get %d, want head 1, get 2", counts.head.Load(), counts.get.Load())
	}
}

func TestRunSkipsDownloadWhenUpToDate(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("new-binary-content"))
	server, counts := newReleaseServer(t, "v9.9.9", archive, sha256Hex(t, archive))
	target := writeTarget(t, "current-binary-content")
	var out bytes.Buffer

	if err := update.Run(t.Context(), update.Options{
		Current:    "v9.9.9",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        &out,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if counts.get.Load() != 0 {
		t.Fatalf("download requests = %d, want 0", counts.get.Load())
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Fatalf("output = %q, want up-to-date notice", out.String())
	}
	installed, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != "current-binary-content" {
		t.Fatalf("binary changed to %q", installed)
	}
}

func TestRunKeepsBinaryWhenChecksumMismatch(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("new-binary-content"))
	otherDigest := sha256Hex(t, []byte("other-content"))
	server, _ := newReleaseServer(t, "v9.9.9", archive, otherDigest)
	target := writeTarget(t, "current-binary-content")

	err := update.Run(t.Context(), update.Options{
		Current:    "v0.1.0",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want checksum failure")
	}
	installed, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(installed) != "current-binary-content" {
		t.Fatalf("binary changed to %q", installed)
	}
}

func TestRunKeepsBinaryWhenArchiveBroken(t *testing.T) {
	t.Parallel()
	archive := []byte("not-a-gzip-archive")
	server, _ := newReleaseServer(t, "v9.9.9", archive, sha256Hex(t, archive))
	target := writeTarget(t, "current-binary-content")

	err := update.Run(t.Context(), update.Options{
		Current:    "v0.1.0",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want archive failure")
	}
	installed, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(installed) != "current-binary-content" {
		t.Fatalf("binary changed to %q", installed)
	}
}

func TestLatestRejectsResponseWithoutRedirect(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	if _, err := update.Latest(t.Context(), server.URL, server.Client()); err == nil {
		t.Fatal("Latest() error = nil, want version resolution failure")
	}
}

func TestLatestReadsTagFromRedirect(t *testing.T) {
	t.Parallel()
	server, _ := newReleaseServer(t, "v9.9.9", nil, "")
	latest, err := update.Latest(t.Context(), server.URL, server.Client())
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if latest != "v9.9.9" {
		t.Fatalf("Latest() = %q, want v9.9.9", latest)
	}
}
