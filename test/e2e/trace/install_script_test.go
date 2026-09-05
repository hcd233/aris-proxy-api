package trace_e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humafiber"
	"github.com/gofiber/fiber/v3"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/handler"
)

func TestInstallScript_ReturnsScriptWithHost(t *testing.T) {
	t.Parallel()
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Install Script Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Tags: []string{constant.TagTrace},
	}, traceHandler.HandleInstallScript)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/install.sh", http.NoBody)
	req.Host = "aris.example.com"
	req.Header.Set(constant.HTTPHeaderXForwardedProto, "https")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get(constant.HTTPHeaderContentType); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain, got %s", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	script := string(body)
	if !strings.Contains(script, "https://aris.example.com") {
		t.Fatalf("script must contain embedded host, got:\n%s", script)
	}
	if !strings.Contains(script, "aris-$os-$arch.tar.gz.sha256") {
		t.Fatalf("script must download an independent checksum file")
	}
	if !strings.Contains(script, "sha256sum") || !strings.Contains(script, "shasum -a 256") {
		t.Fatalf("script must support sha256sum and shasum")
	}
	if strings.Index(script, "tar -xzf \"$tmp\"") < strings.Index(script, "Checksum verification failed") {
		t.Fatalf("archive must be extracted only after checksum verification")
	}
	if !strings.HasPrefix(script, "#!/bin/sh") {
		t.Fatalf("script must start with #!/bin/sh")
	}
	if !strings.Contains(script, "init --host") {
		t.Fatalf("script must exec aris init --host after install, got:\n%s", script)
	}
	// PATH 写入段：带标记注释、按 $SHELL 分派 rc、幂等、位置在安装消息与 exec 之间
	pathMarker := "# aris (added by installer)"
	if !strings.Contains(script, pathMarker) {
		t.Fatalf("script must append a marked PATH block, got:\n%s", script)
	}
	for _, rc := range []string{".zshrc", ".bashrc", ".bash_profile", ".profile"} {
		if !strings.Contains(script, rc) {
			t.Fatalf("script must dispatch PATH setup to %s by $SHELL", rc)
		}
	}
	if !strings.Contains(script, "PATH already configured") {
		t.Fatalf("script must skip PATH setup when already configured (idempotent)")
	}
	installedIdx := strings.Index(script, "Installed to")
	pathIdx := strings.Index(script, pathMarker)
	execIdx := strings.Index(script, `exec "$aris_bin" init`)
	if installedIdx < 0 || pathIdx < 0 || execIdx < 0 || pathIdx < installedIdx || pathIdx > execIdx {
		t.Fatalf("PATH block must sit between install message and exec init, got:\n%s", script)
	}
	for _, removed := range []string{"jq", "stty", "[1/4]", "/dev/tty"} {
		if strings.Contains(script, removed) {
			t.Fatalf("script must not contain %q (wizard moved into the binary), got:\n%s", removed, script)
		}
	}
}

func TestInstallScript_InvalidSchemeReturnsErrorScript(t *testing.T) {
	t.Parallel()
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Install Script Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Tags: []string{constant.TagTrace},
	}, traceHandler.HandleInstallScript)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/install.sh", http.NoBody)
	req.Host = ""
	req.Header.Set(constant.HTTPHeaderXForwardedProto, "ftp")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "exit 1") {
		t.Fatalf("invalid origin should return error script, got:\n%s", body)
	}
}

func TestInstallScript_ShellSyntaxValid(t *testing.T) {
	t.Parallel()
	traceHandler := handler.NewTraceHandler(handler.TraceDependencies{})

	app := fiber.New()
	api := humafiber.New(app, huma.DefaultConfig("Install Script Test", "1.0"))
	huma.Register(api, huma.Operation{
		OperationID: "installTraceScript", Method: http.MethodGet, Path: "/install.sh",
		Tags: []string{constant.TagTrace},
	}, traceHandler.HandleInstallScript)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/install.sh", http.NoBody)
	req.Host = "aris.example.com"
	req.Header.Set(constant.HTTPHeaderXForwardedProto, "https")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	scriptPath := filepath.Join(t.TempDir(), "install.sh")
	if err := os.WriteFile(scriptPath, body, 0o700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "sh", "-n", scriptPath).CombinedOutput(); err != nil {
		t.Fatalf("install script has shell syntax errors: %v\n%s", err, out)
	}
}
