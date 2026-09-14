package clientcmd_e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// releaseArchive 将二进制打包为 release 资产并返回归档与摘要
func releaseArchive(t *testing.T, binary []byte) (archive []byte, checksum string) {
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
	archive = buffer.Bytes()
	sum := sha256.Sum256(archive)
	return archive, hex.EncodeToString(sum[:])
}

// newReleaseServer 本地 release 服务：HEAD 资产地址返回 302（Location 含 tag），GET 返回归档与摘要
func newReleaseServer(t *testing.T, latest string, archive []byte, checksum string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			writer.Header().Set("Location", "/releases/download/"+latest+request.URL.Path)
			writer.WriteHeader(http.StatusFound)
			return
		}
		if strings.HasSuffix(request.URL.Path, constant.ArisClientReleaseChecksumSuffix) {
			_, _ = writer.Write([]byte(checksum))
			return
		}
		_, _ = writer.Write(archive)
	}))
	t.Cleanup(server.Close)
	return server
}

// runClient 以指定更新源执行 aris 子命令，返回 stdout+stderr 与错误
func runClient(t *testing.T, binary, baseURL string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), binary, args...)
	cmd.Env = append(os.Environ(), constant.ArisClientUpdateBaseURLEnv+"="+baseURL)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestUpdateCommand_InstallsLatestRelease(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	binary := buildClient(t, root, "-X main.version=v0.1.0")
	newBinary, err := os.ReadFile(buildClient(t, root, "-X main.version=v9.9.9"))
	if err != nil {
		t.Fatal(err)
	}
	archive, checksum := releaseArchive(t, newBinary)
	server := newReleaseServer(t, "v9.9.9", archive, checksum)

	output, err := runClient(t, binary, server.URL, constant.ArisClientCommandUpdate)
	if err != nil {
		t.Fatalf("aris update: %v\n%s", err, output)
	}
	if !strings.Contains(output, "v9.9.9") {
		t.Fatalf("aris update output = %q, want new version", output)
	}

	versionOutput, err := runClient(t, binary, server.URL, constant.ArisClientCommandVersion)
	if err != nil {
		t.Fatalf("aris version: %v\n%s", err, versionOutput)
	}
	if got := strings.TrimSpace(versionOutput); got != "v9.9.9" {
		t.Fatalf("aris version = %q, want v9.9.9", got)
	}
}

func TestUpdateCommand_KeepsBinaryOnChecksumMismatch(t *testing.T) {
	t.Parallel()
	root := projectRoot(t)
	binary := buildClient(t, root, "-X main.version=v0.1.0")
	brokenArchive, _ := releaseArchive(t, []byte("broken-binary-content"))
	server := newReleaseServer(t, "v9.9.9", brokenArchive, strings.Repeat("0", sha256.Size*2))

	output, err := runClient(t, binary, server.URL, constant.ArisClientCommandUpdate)
	if err == nil {
		t.Fatalf("aris update should fail on checksum mismatch, output = %q", output)
	}

	versionOutput, err := runClient(t, binary, server.URL, constant.ArisClientCommandVersion)
	if err != nil {
		t.Fatalf("aris version: %v\n%s", err, versionOutput)
	}
	if got := strings.TrimSpace(versionOutput); got != "v0.1.0" {
		t.Fatalf("aris version = %q, want unchanged v0.1.0", got)
	}
}
