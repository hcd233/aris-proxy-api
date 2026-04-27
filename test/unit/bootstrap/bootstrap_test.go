package bootstrap

import (
	"net/http"
	"os"
	"strings"
	"testing"

	appbootstrap "github.com/hcd233/aris-proxy-api/internal/bootstrap"
)

func TestBuildServer(t *testing.T) {
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if server == nil {
		t.Fatal("BuildServer() returned nil server")
	}
	if server.App == nil {
		t.Fatal("BuildServer() returned nil Fiber app")
	}
	if server.HumaAPI == nil {
		t.Fatal("BuildServer() returned nil Huma API")
	}
}

func TestRegisterRoutes(t *testing.T) {
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if err := appbootstrap.RegisterRoutes(server); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}
}

func TestRegisterRoutes_RegistersHealthRoute(t *testing.T) {
	server, err := appbootstrap.BuildServer()
	if err != nil {
		t.Fatalf("BuildServer() error = %v", err)
	}
	if err := appbootstrap.RegisterRoutes(server); err != nil {
		t.Fatalf("RegisterRoutes() error = %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
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
	content := readFile(t, "../../../internal/bootstrap/container.go")
	if strings.Contains(content, "Container *dig.Container") {
		t.Fatal("Server must not expose dig.Container as an exported field")
	}
}

func TestBootstrapDoesNotUseAnyProviderList(t *testing.T) {
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
