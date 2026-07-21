package trace_e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(script, "github.com/hcd233/aris-proxy-api/releases/latest/download") {
		t.Fatalf("script must contain GitHub Releases URL")
	}
	if !strings.HasPrefix(script, "#!/bin/sh") {
		t.Fatalf("script must start with #!/bin/sh")
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
