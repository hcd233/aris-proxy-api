package update

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/client/update"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// roundTripFunc 用函数实现 http.RoundTripper：测试里观察请求 ctx 并伪造响应
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

// aris update 是显式命令，探测预算不能用使用中检查的 1.5s：慢网络下命令必然失败。
func TestRunProbeBudgetUsesUpdateTimeout(t *testing.T) {
	t.Parallel()
	remaining := time.Duration(0)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Error("probe request carries no deadline")
		} else {
			remaining = time.Until(deadline)
		}
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"/releases/download/v0.2.0" + request.URL.Path}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})}

	// Current 与 latest 相同：Latest 仍会被调用（这是被测路径），之后直接走"已是最新"提前返回
	if err := update.Run(t.Context(), update.Options{
		Current:    "v0.2.0",
		BaseURL:    "https://releases.example.com",
		Out:        io.Discard,
		HTTPClient: client,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if remaining <= constant.ArisClientUpdateCheckTimeout {
		t.Fatalf("probe budget %v must exceed the in-use check timeout %v", remaining, constant.ArisClientUpdateCheckTimeout)
	}
}

// 明文更新源等于用 http 下发可执行文件，必须直接拒绝（http 只放行 loopback 镜像/测试）。
func TestRunRejectsPlaintextBaseURL(t *testing.T) {
	t.Parallel()
	err := update.Run(t.Context(), update.Options{
		Current: "v0.1.0",
		BaseURL: "http://mirror.example.invalid",
		Out:     io.Discard,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want insecure source rejection")
	}
	// 断言拒绝理由本身，避免"DNS 失败也算过"的假阳性
	want := fmt.Sprintf(constant.ArisClientUpdateInsecureSourceFormat, "http://mirror.example.invalid")
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Run() error = %v, want insecure source rejection", err)
	}
}

// 归档 GET 被重定向到明文第三方时，不能静默跟随：校验文件与归档同源，跟随等于一起被替换。
func TestRunRejectsInsecureAssetRedirect(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead {
			writer.Header().Set("Location", "/releases/download/v9.9.9"+request.URL.Path)
			writer.WriteHeader(http.StatusFound)
			return
		}
		http.Redirect(writer, request, "http://mirror.example.invalid"+request.URL.Path, http.StatusFound)
	}))
	t.Cleanup(server.Close)
	target := writeTarget(t, "current-binary-content")

	err := update.Run(t.Context(), update.Options{
		Current:    "v0.1.0",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want redirect rejection")
	}
	want := fmt.Sprintf(constant.ArisClientUpdateInsecureSourceFormat, "http://mirror.example.invalid")
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Run() error = %v, want insecure redirect rejection", err)
	}
	installed, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(installed) != "current-binary-content" {
		t.Fatalf("binary changed to %q", installed)
	}
}

// sha256 文件口径与 install_aris_client.sh.tmpl 一致：恰好一行、64 位 hex、带文件名时须匹配资产名。
func TestRunValidatesChecksumFileFormat(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("new-binary-content"))
	digest := sha256Hex(t, archive)
	asset := fmt.Sprintf(constant.ArisClientReleaseAssetFormat, runtime.GOOS, runtime.GOARCH)

	cases := []struct {
		name     string
		checksum string
		wantOK   bool
	}{
		{name: "single digest", checksum: digest, wantOK: true},
		{name: "sha256sum two columns", checksum: digest + "  " + asset, wantOK: true},
		{name: "binary mode marker", checksum: digest + " *" + asset, wantOK: true},
		{name: "trailing newline", checksum: digest + "\n", wantOK: true},
		{name: "two digests", checksum: digest + "\n" + digest, wantOK: false},
		{name: "digest of another file", checksum: digest + "  other-" + asset, wantOK: false},
		{name: "short digest", checksum: digest[:32], wantOK: false},
		{name: "non hex digest", checksum: strings.Repeat("z", 64), wantOK: false},
		{name: "three columns", checksum: digest + " " + asset + " extra", wantOK: false},
		{name: "empty file", checksum: "", wantOK: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			server, _ := newReleaseServer(t, "v9.9.9", archive, testCase.checksum)
			target := writeTarget(t, "current-binary-content")

			err := update.Run(t.Context(), update.Options{
				Current:    "v0.1.0",
				BaseURL:    server.URL,
				BinaryPath: target,
				Out:        io.Discard,
			})
			if testCase.wantOK {
				if err != nil {
					t.Fatalf("Run() error = %v, want success", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Run() error = nil, want checksum format failure")
			}
			installed, readErr := os.ReadFile(target)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(installed) != "current-binary-content" {
				t.Fatalf("binary changed to %q", installed)
			}
		})
	}
}

// 安装后的二进制权限位是实现契约（同目录临时文件 0700 + rename），必须固定住。
func TestRunInstallsBinaryWithOwnerOnlyMode(t *testing.T) {
	t.Parallel()
	archive := tarGz(t, []byte("new-binary-content"))
	server, _ := newReleaseServer(t, "v9.9.9", archive, sha256Hex(t, archive))
	target := filepath.Join(t.TempDir(), "aris")
	if err := os.WriteFile(target, []byte("current-binary-content"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := update.Run(t.Context(), update.Options{
		Current:    "v0.1.0",
		BaseURL:    server.URL,
		BinaryPath: target,
		Out:        io.Discard,
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != constant.ArisClientUpdateBinaryMode {
		t.Fatalf("installed mode = %v, want %v", perm, constant.ArisClientUpdateBinaryMode)
	}
}
