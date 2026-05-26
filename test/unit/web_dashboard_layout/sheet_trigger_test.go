package web_dashboard_layout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMobileSheetTriggerIsInsideSheetRoot(t *testing.T) {
	layoutPath := filepath.Join("..", "..", "..", "web", "src", "app", "(dashboard)", "layout.tsx")
	data, err := os.ReadFile(layoutPath)
	if err != nil {
		t.Fatalf("failed to read dashboard layout: %v", err)
	}

	source := string(data)
	sheetStart := strings.Index(source, "<Sheet open={sidebarOpen}")
	if sheetStart < 0 {
		t.Fatalf("dashboard layout must contain the mobile Sheet root")
	}

	sheetEndOffset := strings.Index(source[sheetStart:], "</Sheet>")
	if sheetEndOffset < 0 {
		t.Fatalf("dashboard layout must close the mobile Sheet root")
	}
	sheetEnd := sheetStart + sheetEndOffset

	trigger := strings.Index(source, "<SheetTrigger")
	if trigger < 0 {
		t.Fatalf("dashboard layout must contain the mobile Sheet trigger")
	}

	if trigger < sheetStart || trigger > sheetEnd {
		t.Fatalf("SheetTrigger must be rendered inside Sheet root to satisfy Base UI Dialog context")
	}
}
