package web_dashboard_layout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readWebFile(t *testing.T, rel ...string) string {
	t.Helper()
	parts := append([]string{"..", "..", "..", "web"}, rel...)
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatalf("failed to read %s: %v", filepath.Join(rel...), err)
	}
	return string(data)
}

// 移动端抽屉由 shadcn Sidebar 组件内置降级（isMobile → Sheet）。
// 历史坑：手写 SheetTrigger 若不在 Sheet root 内会脱离 Base UI Dialog context
// 导致运行时崩溃，因此 layout.tsx 禁止手写 SheetTrigger，触发一律走
// useSidebar().toggleSidebar（context 驱动，无 context 依赖）。
func TestMobileTriggerUsesSidebarContext(t *testing.T) {
	t.Parallel()
	source := readWebFile(t, "src", "app", "(dashboard)", "layout.tsx")

	if !strings.Contains(source, "<Sidebar") {
		t.Fatal("dashboard layout must use shadcn Sidebar (its isMobile branch renders the mobile Sheet)")
	}
	if strings.Contains(source, "<SheetTrigger") {
		t.Fatal("layout must not hand-write SheetTrigger: the Sheet root lives inside ui/sidebar.tsx, a bare trigger loses the Base UI Dialog context")
	}
	if !strings.Contains(source, "toggleSidebar") {
		t.Fatal("mobile trigger must open the drawer via useSidebar().toggleSidebar")
	}
}

// Sidebar 组件必须内置移动端 Sheet 分支，抽屉才有 Base UI Dialog context。
func TestSidebarProvidesMobileSheetFallback(t *testing.T) {
	t.Parallel()
	source := readWebFile(t, "src", "components", "ui", "sidebar.tsx")

	if !strings.Contains(source, "if (isMobile)") || !strings.Contains(source, "<Sheet open={openMobile}") {
		t.Fatal("ui/sidebar.tsx must render the mobile Sheet branch so the drawer keeps its Base UI Dialog context")
	}
}
