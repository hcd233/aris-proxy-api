package bootstrap

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"

	appbootstrap "github.com/hcd233/aris-proxy-api/internal/bootstrap"
)

func TestBuildServer(t *testing.T) {
	t.Parallel()
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if server == nil {
		t.Fatal("BuildServer() returned nil server")
		return
	}
	if server.App == nil {
		t.Fatal("BuildServer() returned nil Fiber app")
		return
	}
	if server.HumaAPI == nil {
		t.Fatal("BuildServer() returned nil Huma API")
		return
	}
}

func TestRegisterRoutes(t *testing.T) {
	t.Parallel()
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if err := appbootstrap.RegisterRoutes(server); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}
}

func TestRegisterRoutes_RegistersHealthRoute(t *testing.T) {
	t.Parallel()
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if err := appbootstrap.RegisterRoutes(server); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	rsp, err := server.App.Test(req)
	if err != nil {
		t.Fatalf("App.Test() error = %v", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health status = %d, want %d", rsp.StatusCode, http.StatusOK)
	}
}

func TestServerDoesNotExposeDigContainer(t *testing.T) {
	t.Parallel()
	content := readFile(t, "../../../internal/bootstrap/container.go")
	if strings.Contains(content, "Container *dig.Container") {
		t.Fatal("Server must not expose dig.Container as an exported field")
	}
}

func TestBootstrapDoesNotUseAnyProviderList(t *testing.T) {
	t.Parallel()
	content := readFile(t, "../../../internal/bootstrap/container.go")
	if strings.Contains(content, "[]any{") || strings.Contains(content, "[]interface{}{") {
		t.Fatal("bootstrap providers must be registered without any/interface{} provider lists")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

func TestWebRouter_FallbackAndNotFound(t *testing.T) {
	t.Parallel()
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if err := appbootstrap.RegisterRoutes(server); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	// 1. 测试首页加载，应返回 200 (即 index.html)
	req1, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/web/", http.NoBody)
	rsp1, err := server.App.Test(req1)
	if err != nil {
		t.Fatalf("App.Test() error = %v", err)
	}
	defer rsp1.Body.Close()
	if rsp1.StatusCode != http.StatusOK {
		t.Errorf("GET /web/ status = %d, want %d", rsp1.StatusCode, http.StatusOK)
	}

	// 2. 测试不存在的页面路由，应 Fallback 到 index.html 返回 200
	req2, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/web/some-non-existent-page-route", http.NoBody)
	rsp2, err := server.App.Test(req2)
	if err != nil {
		t.Fatalf("App.Test() error = %v", err)
	}
	defer rsp2.Body.Close()
	if rsp2.StatusCode != http.StatusOK {
		t.Errorf("GET /web/some-non-existent-page-route status = %d, want %d", rsp2.StatusCode, http.StatusOK)
	}

	// 3. 测试不存在的静态资源（带有 js 后缀），应返回 404
	req3, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/web/_next/static/chunks/non-existent-file.js", http.NoBody)
	rsp3, err := server.App.Test(req3)
	if err != nil {
		t.Fatalf("App.Test() error = %v", err)
	}
	defer rsp3.Body.Close()
	if rsp3.StatusCode != http.StatusNotFound {
		t.Errorf("GET /web/_next/static/chunks/non-existent-file.js status = %d, want %d", rsp3.StatusCode, http.StatusNotFound)
	}
}
