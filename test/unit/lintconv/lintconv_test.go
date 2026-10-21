package lintconv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
)

type fixtureCase struct {
	Name    string            `json:"name"`
	Files   []fixtureFile     `json:"files"`
	Want    []fixtureExpected `json:"want"`
	NotWant []fixtureExpected `json:"notWant"`
}

type fixtureFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type fixtureExpected struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
}

func TestRunReportsExpectedDiagnostics(t *testing.T) {
	t.Parallel()
	cases := loadFixtureCases(t)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeModule(t, root)
			for _, file := range tc.Files {
				writeFixtureFile(t, root, file)
			}

			result := lintconv.Run([]string{root})
			for _, want := range tc.Want {
				if !hasDiagnostic(result.Diagnostics, want.Rule, enum.Severity(want.Severity)) {
					t.Fatalf("missing diagnostic rule=%s severity=%s in %#v", want.Rule, want.Severity, result.Diagnostics)
				}
			}
			for _, nw := range tc.NotWant {
				if hasDiagnostic(result.Diagnostics, nw.Rule, enum.Severity(nw.Severity)) {
					t.Fatalf("unexpected diagnostic rule=%s severity=%s in %#v", nw.Rule, nw.Severity, result.Diagnostics)
				}
			}
		})
	}
}

func TestResultCountsBySeverity(t *testing.T) {
	t.Parallel()
	result := lintconv.Result{Diagnostics: []lintconv.Diagnostic{
		{Rule: "a", Severity: enum.SeverityError},
		{Rule: "b", Severity: enum.SeverityWarning},
		{Rule: "c", Severity: enum.SeverityWarning},
	}}
	if result.ErrorCount() != 1 {
		t.Fatalf("ErrorCount() = %d, want 1", result.ErrorCount())
	}
	if result.WarningCount() != 2 {
		t.Fatalf("WarningCount() = %d, want 2", result.WarningCount())
	}
}

func loadFixtureCases(t *testing.T) []fixtureCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("fixtures", "cases.json"))
	if err != nil {
		t.Fatalf("read cases fixture: %v", err)
	}
	var cases []fixtureCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("unmarshal cases fixture: %v", err)
	}
	return cases
}

func writeModule(t *testing.T, root string) {
	t.Helper()
	content := "module github.com/hcd233/aris-proxy-api\n\ngo 1.25.1\n\nrequire go.uber.org/zap v1.27.0\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(content), 0o600); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
}

func writeFixtureFile(t *testing.T, root string, file fixtureFile) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(file.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
}

func hasDiagnostic(diagnostics []lintconv.Diagnostic, rule string, severity enum.Severity) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Rule == rule && diagnostic.Severity == severity {
			return true
		}
	}
	return false
}

// TestRunWithPackageDirArgResolvesRepoRelativePaths 回归：pre-commit hook 传「包目录」
// （如 ./internal/infrastructure/repository）时，诊断路径必须相对仓库根归一；
// 否则 internal/infrastructure 下的文件会被误判成「非 infrastructure 层导入 dbmodel」，
// 让所有触及仓储目录的提交被 hook 拦下。
//
//nolint:paralleltest // 需要切换工作目录到临时仓库根，不能并行
func TestRunWithPackageDirArgResolvesRepoRelativePaths(t *testing.T) {
	root := t.TempDir()
	writeModule(t, root)
	writeFixtureFile(t, root, fixtureFile{
		Path: "internal/infrastructure/repository/endpoint_repository.go",
		Content: `package repository

import _ "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
`,
	})
	t.Chdir(root)

	result := lintconv.Run([]string{"internal/infrastructure/repository"})
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Rule == constant.RuleDatabaseModelDependency {
			t.Fatalf("scoped package dir arg must resolve repo-relative paths, got %#v", diagnostic)
		}
	}
}
