package bootstrap

import (
	"os"
	"strings"
	"testing"

	appbootstrap "github.com/hcd233/aris-proxy-api/internal/bootstrap"
)

func TestBuildFxAppOptions(t *testing.T) {
	t.Parallel()
	opts := appbootstrap.BuildFxAppOptions("localhost", "0")
	if len(opts) == 0 {
		t.Fatal("BuildFxAppOptions() returned empty options")
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

// TestContainerDoesNotUseInterfaceType 验证 container.go 不使用 interface{} 和未导出 fx.Container
func TestContainerDoesNotUseInterfaceType(t *testing.T) {
	t.Parallel()
	content := readFile(t, "../../../internal/bootstrap/container.go")
	if strings.Contains(content, "interface{}") {
		t.Fatal("container.go should not use interface{} type — use concrete types")
	}
	if strings.Contains(content, "Container *fx.Container") {
		t.Fatal("container.go must not expose fx.Container as an exported field")
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
