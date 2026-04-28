# lint-conv Go Analyzer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace regex-based `make lint-conv` implementation with a repository-local Go AST static checker while preserving the existing command entrypoint and failure behavior.

**Architecture:** Add a focused `internal/tool/lintconv` package that parses Go source files into ASTs, runs project convention rules, and returns structured diagnostics. Add `cmd/lintconv` as a thin CLI over that package, and keep `script/lint-conventions.sh` as a compatibility wrapper that executes `go run ./cmd/lintconv ./...`.

**Tech Stack:** Go 1.25.1, standard library `go/ast`, `go/parser`, `go/token`, `go/scanner`-style positions via `token.FileSet`, standard `testing`, existing `make lint-conv` workflow.

---

## File Structure

- Create: `internal/tool/lintconv/diagnostic.go`
- Responsibility: diagnostic severity, rule metadata, result aggregation, stable text output.
- Create: `internal/tool/lintconv/source.go`
- Responsibility: discover Go files from package/path arguments, parse files with comments, normalize slash paths, expose parsed file metadata.
- Create: `internal/tool/lintconv/runner.go`
- Responsibility: define `Run(args []string) Result`, wire all rule groups, and provide shared helpers for path allowlists and AST traversal.
- Create: `internal/tool/lintconv/rules_error.go`
- Responsibility: migrate error-handling and constant-forwarding checks.
- Create: `internal/tool/lintconv/rules_logging.go`
- Responsibility: migrate logger prefix and sensitive `zap.String` checks.
- Create: `internal/tool/lintconv/rules_testing.go`
- Responsibility: migrate test layout, testify import, and `time.Sleep` checks.
- Create: `internal/tool/lintconv/rules_architecture.go`
- Responsibility: migrate handler/db, root context, domain/application dependency, passthrough wrapper checks.
- Create: `internal/tool/lintconv/rules_magic.go`
- Responsibility: migrate magic number, magic duration, magic string, and anonymous struct checks.
- Create: `internal/tool/lintconv/rules_style.go`
- Responsibility: migrate commented-out code and implementation-detail naming warnings.
- Create: `cmd/lintconv/main.go`
- Responsibility: CLI entrypoint that prints diagnostics and exits `1` when error diagnostics exist.
- Modify: `script/lint-conventions.sh`
- Responsibility: compatibility wrapper around `go run ./cmd/lintconv ./...`.
- Create: `test/unit/lintconv/lintconv_test.go`
- Responsibility: unit tests using temporary fixture modules/files to exercise the checker API.
- Create: `test/unit/lintconv/fixtures/cases.json`
- Responsibility: declarative cases with file paths, file content, expected diagnostic rule IDs, and expected severities.

## Task 1: Diagnostic Model And Fixture Test Harness

**Files:**
- Create: `internal/tool/lintconv/diagnostic.go`
- Create: `internal/tool/lintconv/source.go`
- Create: `internal/tool/lintconv/runner.go`
- Create: `test/unit/lintconv/lintconv_test.go`
- Create: `test/unit/lintconv/fixtures/cases.json`

- [ ] **Step 1: Write fixture cases for the first end-to-end checker behavior**

Create `test/unit/lintconv/fixtures/cases.json` with this content:

```json
[
  {
    "name": "logger missing module prefix",
    "files": [
      {
        "path": "internal/application/demo/demo.go",
        "content": "package demo\n\nimport \"go.uber.org/zap\"\n\nvar logger demoLogger\n\ntype demoLogger struct{}\n\nfunc (demoLogger) Info(msg string, fields ...zap.Field) {}\n\nfunc Run() {\n\tlogger.Info(\"missing prefix\")\n}\n"
      }
    ],
    "want": [
      {"rule": "logging.prefix", "severity": "warning"}
    ]
  }
]
```

- [ ] **Step 2: Write failing unit test harness**

Create `test/unit/lintconv/lintconv_test.go` with this content:

```go
package lintconv_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
)

type fixtureCase struct {
	Name  string            `json:"name"`
	Files []fixtureFile     `json:"files"`
	Want  []fixtureExpected `json:"want"`
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
	cases := loadFixtureCases(t)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			root := t.TempDir()
			writeModule(t, root)
			for _, file := range tc.Files {
				writeFixtureFile(t, root, file)
			}

			result := lintconv.Run([]string{root})
			for _, want := range tc.Want {
				if !hasDiagnostic(result.Diagnostics, want.Rule, lintconv.Severity(want.Severity)) {
					t.Fatalf("missing diagnostic rule=%s severity=%s in %#v", want.Rule, want.Severity, result.Diagnostics)
				}
			}
		})
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

func hasDiagnostic(diagnostics []lintconv.Diagnostic, rule string, severity lintconv.Severity) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Rule == rule && diagnostic.Severity == severity {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: FAIL because `internal/tool/lintconv` does not exist.

- [ ] **Step 4: Implement diagnostic and source loading core**

Create `internal/tool/lintconv/diagnostic.go` with this content:

```go
package lintconv

import (
	"fmt"
	"io"
	"sort"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Diagnostic struct {
	Rule     string
	Severity Severity
	Path     string
	Line     int
	Message  string
}

type Result struct {
	Diagnostics []Diagnostic
}

func (r Result) ErrorCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityError {
			count++
		}
	}
	return count
}

func (r Result) WarningCount() int {
	count := 0
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.Severity == SeverityWarning {
			count++
		}
	}
	return count
}

func (r Result) Print(w io.Writer) {
	diagnostics := append([]Diagnostic(nil), r.Diagnostics...)
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path != diagnostics[j].Path {
			return diagnostics[i].Path < diagnostics[j].Path
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Rule < diagnostics[j].Rule
	})
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(w, "%s:%d: [%s] %s: %s\n", diagnostic.Path, diagnostic.Line, diagnostic.Severity, diagnostic.Rule, diagnostic.Message)
	}
	if r.ErrorCount() == 0 && r.WarningCount() == 0 {
		fmt.Fprintln(w, "All convention checks passed!")
		return
	}
	fmt.Fprintf(w, "%d error(s), %d warning(s)\n", r.ErrorCount(), r.WarningCount())
}
```

Create `internal/tool/lintconv/source.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type SourceFile struct {
	Path string
	File *ast.File
	Set  *token.FileSet
}

func loadSourceFiles(args []string) ([]SourceFile, []Diagnostic) {
	if len(args) == 0 {
		args = []string{"."}
	}
	var files []SourceFile
	var diagnostics []Diagnostic
	for _, arg := range args {
		paths, err := expandGoFiles(arg)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Rule: "source.load", Severity: SeverityError, Path: slashPath(arg), Line: 1, Message: err.Error()})
			continue
		}
		for _, path := range paths {
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, path, nil, parser.ParseComments)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{Rule: "source.parse", Severity: SeverityError, Path: slashPath(path), Line: 1, Message: err.Error()})
				continue
			}
			files = append(files, SourceFile{Path: slashPath(path), File: file, Set: set})
		}
	}
	return files, diagnostics
}

func expandGoFiles(arg string) ([]string, error) {
	clean := filepath.Clean(arg)
	info, err := os.Stat(clean)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(clean, ".go") {
			return []string{clean}, nil
		}
		return nil, nil
	}
	var paths []string
	err = filepath.WalkDir(clean, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == ".worktrees" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			paths = append(paths, path)
		}
		return nil
	})
	return paths, err
}

func slashPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func (file SourceFile) line(pos token.Pos) int {
	if !pos.IsValid() {
		return 1
	}
	return file.Set.Position(pos).Line
}
```

Create `internal/tool/lintconv/runner.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"strings"
)

type checker struct {
	files       []SourceFile
	diagnostics []Diagnostic
}

func Run(args []string) Result {
	files, diagnostics := loadSourceFiles(args)
	c := &checker{files: files, diagnostics: diagnostics}
	c.checkLogging()
	return Result{Diagnostics: c.diagnostics}
}

func (c *checker) report(file SourceFile, node ast.Node, severity Severity, rule string, message string) {
	c.diagnostics = append(c.diagnostics, Diagnostic{Rule: rule, Severity: severity, Path: file.Path, Line: file.line(node.Pos()), Message: message})
}

func inspectFile(file SourceFile, fn func(ast.Node)) {
	ast.Inspect(file.File, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		fn(node)
		return true
	})
}

func selectorName(expr ast.Expr) (string, string, bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	return ident.Name, selector.Sel.Name, true
}

func isUnder(path string, prefix string) bool {
	path = slashPath(path)
	prefix = strings.TrimSuffix(slashPath(prefix), "/") + "/"
	return strings.HasPrefix(path, prefix)
}
```

Create `internal/tool/lintconv/rules_logging.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func (c *checker) checkLogging() {
	for _, file := range c.files {
		if !isUnder(file.Path, "internal") {
			continue
		}
		inspectFile(file, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			receiver, method, ok := selectorName(call.Fun)
			if !ok || receiver != "logger" || !isLoggerMethod(method) || len(call.Args) == 0 {
				return
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return
			}
			message, err := strconv.Unquote(literal.Value)
			if err != nil {
				return
			}
			if !strings.HasPrefix(message, "[") {
				c.report(file, literal, SeverityWarning, "logging.prefix", "日志消息应使用 [ModuleName] 前缀")
			}
		})
	}
}

func isLoggerMethod(method string) bool {
	switch method {
	case "Info", "Error", "Warn", "Debug":
		return true
	default:
		return false
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

## Task 2: Error Handling And Constant Rules

**Files:**
- Modify: `internal/tool/lintconv/runner.go`
- Create: `internal/tool/lintconv/rules_error.go`
- Modify: `test/unit/lintconv/fixtures/cases.json`

- [ ] **Step 1: Add fixture cases for deprecated errors and forwarding constants**

Replace `test/unit/lintconv/fixtures/cases.json` with this content:

```json
[
  {
    "name": "logger missing module prefix",
    "files": [
      {
        "path": "internal/application/demo/demo.go",
        "content": "package demo\n\nimport \"go.uber.org/zap\"\n\nvar logger demoLogger\n\ntype demoLogger struct{}\n\nfunc (demoLogger) Info(msg string, fields ...zap.Field) {}\n\nfunc Run() {\n\tlogger.Info(\"missing prefix\")\n}\n"
      }
    ],
    "want": [
      {"rule": "logging.prefix", "severity": "warning"}
    ]
  },
  {
    "name": "deprecated constant error selector",
    "files": [
      {
        "path": "internal/application/demo/error.go",
        "content": "package demo\n\nfunc Run() string {\n\treturn constant.ErrInvalidRequest\n}\n"
      }
    ],
    "want": [
      {"rule": "error.deprecated_constant", "severity": "error"}
    ]
  },
  {
    "name": "forwarding constant in enum package",
    "files": [
      {
        "path": "internal/common/enum/demo.go",
        "content": "package enum\n\nconst DemoStatus = upstream.StatusReady\n"
      }
    ],
    "want": [
      {"rule": "constant.forwarding", "severity": "error"}
    ]
  }
]
```

- [ ] **Step 2: Run tests to verify new cases fail**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: FAIL with missing `error.deprecated_constant` and `constant.forwarding` diagnostics.

- [ ] **Step 3: Wire error rule group**

In `internal/tool/lintconv/runner.go`, replace `Run` with:

```go
func Run(args []string) Result {
	files, diagnostics := loadSourceFiles(args)
	c := &checker{files: files, diagnostics: diagnostics}
	c.checkErrorHandling()
	c.checkLogging()
	return Result{Diagnostics: c.diagnostics}
}
```

- [ ] **Step 4: Implement error and constant rules**

Create `internal/tool/lintconv/rules_error.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"strings"
)

func (c *checker) checkErrorHandling() {
	for _, file := range c.files {
		c.checkDeprecatedConstantErrors(file)
		c.checkForwardingConstants(file)
	}
}

func (c *checker) checkDeprecatedConstantErrors(file SourceFile) {
	if !isUnder(file.Path, "internal") || strings.HasSuffix(file.Path, "internal/common/constant/error.go") {
		return
	}
	inspectFile(file, func(node ast.Node) {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name != "constant" || !strings.HasPrefix(selector.Sel.Name, "Err") {
			return
		}
		c.report(file, selector, SeverityError, "error.deprecated_constant", "禁止使用 constant.ErrXxx，使用 ierr.ErrXxx.BizError()")
	})
}

func (c *checker) checkForwardingConstants(file SourceFile) {
	if !isConstantOrEnumPath(file.Path) || strings.HasSuffix(file.Path, "_test.go") {
		return
	}
	for _, decl := range file.File.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok.String() != "const" {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, value := range valueSpec.Values {
				selector, ok := value.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				if _, ok := selector.X.(*ast.Ident); ok {
					c.report(file, valueSpec, SeverityError, "constant.forwarding", "禁止在 constant/enum 中定义 const X = pkg.Y 转发常量，直接使用原始常量")
				}
			}
		}
	}
}

func isConstantOrEnumPath(path string) bool {
	return isUnder(path, "internal/common/constant") || isUnder(path, "internal/common/enum") || isUnder(path, "internal/enum")
}
```

- [ ] **Step 5: Run tests to verify pass**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

## Task 3: Logging Sensitive Field Rule

**Files:**
- Modify: `internal/tool/lintconv/rules_logging.go`
- Modify: `test/unit/lintconv/fixtures/cases.json`

- [ ] **Step 1: Add fixture for unmasked sensitive zap field**

Append this case to the JSON array in `test/unit/lintconv/fixtures/cases.json` before the closing `]`:

```json
,
  {
    "name": "zap string sensitive field without mask",
    "files": [
      {
        "path": "internal/application/demo/log_secret.go",
        "content": "package demo\n\nimport \"go.uber.org/zap\"\n\nfunc Run(apiKey string) zap.Field {\n\treturn zap.String(\"apiKey\", apiKey)\n}\n"
      }
    ],
    "want": [
      {"rule": "logging.sensitive", "severity": "warning"}
    ]
  }
```

- [ ] **Step 2: Run tests to verify new case fails**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: FAIL with missing `logging.sensitive` diagnostic.

- [ ] **Step 3: Replace logging rules implementation with prefix plus sensitive checks**

Replace `internal/tool/lintconv/rules_logging.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func (c *checker) checkLogging() {
	for _, file := range c.files {
		if !isUnder(file.Path, "internal") {
			continue
		}
		inspectFile(file, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			c.checkLoggerPrefix(file, call)
			c.checkSensitiveZapString(file, call)
		})
	}
}

func (c *checker) checkLoggerPrefix(file SourceFile, call *ast.CallExpr) {
	receiver, method, ok := selectorName(call.Fun)
	if !ok || receiver != "logger" || !isLoggerMethod(method) || len(call.Args) == 0 {
		return
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return
	}
	message, err := strconv.Unquote(literal.Value)
	if err != nil {
		return
	}
	if !strings.HasPrefix(message, "[") {
		c.report(file, literal, SeverityWarning, "logging.prefix", "日志消息应使用 [ModuleName] 前缀")
	}
}

func (c *checker) checkSensitiveZapString(file SourceFile, call *ast.CallExpr) {
	receiver, method, ok := selectorName(call.Fun)
	if !ok || receiver != "zap" || method != "String" || len(call.Args) < 2 {
		return
	}
	name := stringLiteral(call.Args[0])
	if !isSensitiveFieldName(name) || isAllowedSensitiveFieldName(name) {
		return
	}
	if containsMaskSecret(call.Args[1]) {
		return
	}
	c.report(file, call, SeverityWarning, "logging.sensitive", "日志中记录 Key/Token/Secret/Password 应使用 util.MaskSecret()")
}

func isLoggerMethod(method string) bool {
	switch method {
	case "Info", "Error", "Warn", "Debug":
		return true
	default:
		return false
	}
}

func stringLiteral(expr ast.Expr) string {
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return ""
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return ""
	}
	return value
}

func isSensitiveFieldName(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "key") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password")
}

func isAllowedSensitiveFieldName(name string) bool {
	lower := strings.ToLower(name)
	allowed := []string{"ctxkey", "apikeyname", "keyname", "lockkey", "cachekey", "configkey", "routekey", "sortkey", "tokentype", "tokenexpir", "sessionapikeyname"}
	for _, item := range allowed {
		if strings.Contains(lower, item) {
			return true
		}
	}
	return false
}

func containsMaskSecret(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		_, method, ok := selectorName(call.Fun)
		if ok && method == "MaskSecret" {
			found = true
			return false
		}
		return true
	})
	return found
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

## Task 4: Testing Convention Rules

**Files:**
- Modify: `internal/tool/lintconv/runner.go`
- Create: `internal/tool/lintconv/rules_testing.go`
- Modify: `test/unit/lintconv/fixtures/cases.json`

- [ ] **Step 1: Add fixture cases for test conventions**

Append these cases to the JSON array in `test/unit/lintconv/fixtures/cases.json` before the closing `]`:

```json
,
  {
    "name": "internal test file is rejected",
    "files": [
      {
        "path": "internal/application/demo/demo_test.go",
        "content": "package demo\n\nimport \"testing\"\n\nfunc TestDemo(t *testing.T) {}\n"
      }
    ],
    "want": [
      {"rule": "testing.internal_file", "severity": "error"}
    ]
  },
  {
    "name": "test root file is rejected",
    "files": [
      {
        "path": "test/demo_test.go",
        "content": "package test\n\nimport \"testing\"\n\nfunc TestDemo(t *testing.T) {}\n"
      }
    ],
    "want": [
      {"rule": "testing.root_file", "severity": "error"}
    ]
  },
  {
    "name": "testify import is rejected",
    "files": [
      {
        "path": "test/unit/demo/demo_test.go",
        "content": "package demo\n\nimport \"github.com/stretchr/testify/require\"\n\nfunc TestDemo() {\n\trequire.True(nil, true)\n}\n"
      }
    ],
    "want": [
      {"rule": "testing.testify", "severity": "error"}
    ]
  },
  {
    "name": "time sleep in test is rejected",
    "files": [
      {
        "path": "test/unit/demo/sleep_test.go",
        "content": "package demo\n\nimport \"time\"\n\nfunc TestDemo() {\n\ttime.Sleep(time.Second)\n}\n"
      }
    ],
    "want": [
      {"rule": "testing.sleep", "severity": "error"}
    ]
  }
```

- [ ] **Step 2: Run tests to verify new cases fail**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: FAIL with missing testing rule diagnostics.

- [ ] **Step 3: Wire testing rule group**

In `internal/tool/lintconv/runner.go`, replace `Run` with:

```go
func Run(args []string) Result {
	files, diagnostics := loadSourceFiles(args)
	c := &checker{files: files, diagnostics: diagnostics}
	c.checkErrorHandling()
	c.checkLogging()
	c.checkTesting()
	return Result{Diagnostics: c.diagnostics}
}
```

- [ ] **Step 4: Implement testing rules**

Create `internal/tool/lintconv/rules_testing.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"strconv"
	"strings"
)

func (c *checker) checkTesting() {
	for _, file := range c.files {
		if strings.HasSuffix(file.Path, "_test.go") && isUnder(file.Path, "internal") {
			c.report(file, file.File, SeverityError, "testing.internal_file", "禁止在 internal/ 目录中存放 *_test.go 文件，所有测试必须放在 test/ 目录")
		}
		if strings.HasSuffix(file.Path, "_test.go") && strings.HasPrefix(file.Path, "test/") && strings.Count(strings.TrimPrefix(file.Path, "test/"), "/") == 0 {
			c.report(file, file.File, SeverityError, "testing.root_file", "禁止在 test/ 根目录直接放 *_test.go，必须放入主题子目录")
		}
		for _, imp := range file.File.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if strings.HasPrefix(path, "github.com/stretchr/testify") {
				c.report(file, imp, SeverityError, "testing.testify", "禁止使用 testify 等第三方断言库，使用标准库 testing 包")
			}
		}
		if !strings.HasPrefix(file.Path, "test/") {
			continue
		}
		inspectFile(file, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			receiver, method, ok := selectorName(call.Fun)
			if ok && receiver == "time" && method == "Sleep" {
				c.report(file, call, SeverityError, "testing.sleep", "禁止在测试中使用 time.Sleep() 做同步，使用 channel/WaitGroup/deadline")
			}
		})
	}
}
```

- [ ] **Step 5: Run tests to verify pass**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

## Task 5: Architecture Rules And Passthrough Wrapper

**Files:**
- Modify: `internal/tool/lintconv/runner.go`
- Create: `internal/tool/lintconv/rules_architecture.go`
- Modify: `test/unit/lintconv/fixtures/cases.json`

- [ ] **Step 1: Add fixture cases for architecture rules**

Append these cases to the JSON array in `test/unit/lintconv/fixtures/cases.json` before the closing `]`:

```json
,
  {
    "name": "handler direct dao call is rejected",
    "files": [
      {
        "path": "internal/handler/demo.go",
        "content": "package handler\n\nfunc Run() {\n\tdao.Find()\n}\n"
      }
    ],
    "want": [
      {"rule": "architecture.handler_db", "severity": "error"}
    ]
  },
  {
    "name": "interface layer root context is rejected",
    "files": [
      {
        "path": "internal/handler/context.go",
        "content": "package handler\n\nimport \"context\"\n\nfunc Run() {\n\t_ = context.Background()\n}\n"
      }
    ],
    "want": [
      {"rule": "architecture.root_context", "severity": "error"}
    ]
  },
  {
    "name": "domain infrastructure import is rejected",
    "files": [
      {
        "path": "internal/domain/demo/domain.go",
        "content": "package demo\n\nimport _ \"github.com/hcd233/aris-proxy-api/internal/infrastructure/database\"\n"
      }
    ],
    "want": [
      {"rule": "architecture.domain_dependency", "severity": "error"}
    ]
  },
  {
    "name": "application deprecated import is rejected",
    "files": [
      {
        "path": "internal/application/demo/app.go",
        "content": "package demo\n\nimport _ \"github.com/hcd233/aris-proxy-api/internal/service\"\n"
      }
    ],
    "want": [
      {"rule": "architecture.deprecated_application_import", "severity": "error"}
    ]
  },
  {
    "name": "passthrough wrapper warns",
    "files": [
      {
        "path": "internal/application/demo/wrapper.go",
        "content": "package demo\n\ntype Service struct{}\n\nfunc (s Service) Run() string {\n\treturn s.Other()\n}\n\nfunc (s Service) Other() string {\n\treturn \"ok\"\n}\n"
      }
    ],
    "want": [
      {"rule": "architecture.passthrough", "severity": "warning"}
    ]
  }
```

- [ ] **Step 2: Run tests to verify new cases fail**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: FAIL with missing architecture diagnostics.

- [ ] **Step 3: Wire architecture rule group**

In `internal/tool/lintconv/runner.go`, replace `Run` with:

```go
func Run(args []string) Result {
	files, diagnostics := loadSourceFiles(args)
	c := &checker{files: files, diagnostics: diagnostics}
	c.checkErrorHandling()
	c.checkLogging()
	c.checkTesting()
	c.checkArchitecture()
	return Result{Diagnostics: c.diagnostics}
}
```

- [ ] **Step 4: Implement architecture rules**

Create `internal/tool/lintconv/rules_architecture.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"strconv"
	"strings"
)

func (c *checker) checkArchitecture() {
	for _, file := range c.files {
		c.checkArchitectureImports(file)
		c.checkArchitectureCalls(file)
		c.checkPassthroughWrappers(file)
	}
}

func (c *checker) checkArchitectureImports(file SourceFile) {
	for _, imp := range file.File.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if isUnder(file.Path, "internal/domain") && strings.HasPrefix(path, "github.com/hcd233/aris-proxy-api/internal/infrastructure/") {
			c.report(file, imp, SeverityError, "architecture.domain_dependency", "Domain 层禁止依赖 Infrastructure 层")
		}
		if isUnder(file.Path, "internal/domain") && path == "github.com/hcd233/aris-proxy-api/internal/dto" {
			c.report(file, imp, SeverityError, "architecture.domain_dependency", "Domain 层禁止依赖 DTO")
		}
		if isUnder(file.Path, "internal/domain") && path == "github.com/hcd233/aris-proxy-api/internal/util" {
			c.report(file, imp, SeverityError, "architecture.domain_dependency", "Domain 层禁止依赖 internal/util，请改用 internal/common/util")
		}
		if isUnder(file.Path, "internal/application") && isDeprecatedApplicationImport(path) {
			c.report(file, imp, SeverityError, "architecture.deprecated_application_import", "Application 层禁止引用已废弃 internal/service/converter/proxy/agent/jwt/oauth2 包")
		}
	}
}

func (c *checker) checkArchitectureCalls(file SourceFile) {
	inspectFile(file, func(node ast.Node) {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return
		}
		receiver, method, ok := selectorName(call.Fun)
		if !ok {
			return
		}
		if isUnder(file.Path, "internal/handler") && isHandlerDBCall(receiver, method) {
			c.report(file, call, SeverityError, "architecture.handler_db", "Handler 层禁止直接操作 DAO/DB，业务逻辑应放在 Service 层")
		}
		if isInterfaceLayerPath(file.Path) && receiver == "context" && (method == "Background" || method == "TODO") {
			c.report(file, call, SeverityError, "architecture.root_context", "接口逻辑层禁止使用 context.Background()/context.TODO()，应从上层传递 context")
		}
	})
}

func (c *checker) checkPassthroughWrappers(file SourceFile) {
	if !isUnder(file.Path, "internal") || isUnder(file.Path, "internal/handler") {
		return
	}
	for _, decl := range file.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil || fn.Name.Name == "init" || len(fn.Body.List) != 1 {
			continue
		}
		ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		receiverName := receiverIdentName(fn)
		callReceiver, callMethod, ok := selectorName(call.Fun)
		if ok && receiverName != "" && callReceiver == receiverName && callMethod != "" {
			c.report(file, fn, SeverityWarning, "architecture.passthrough", "发现透传封装函数，应将逻辑内联或合并方法")
		}
	}
}

func isDeprecatedApplicationImport(path string) bool {
	deprecated := []string{
		"github.com/hcd233/aris-proxy-api/internal/service",
		"github.com/hcd233/aris-proxy-api/internal/converter",
		"github.com/hcd233/aris-proxy-api/internal/proxy",
		"github.com/hcd233/aris-proxy-api/internal/agent/",
		"github.com/hcd233/aris-proxy-api/internal/jwt/",
		"github.com/hcd233/aris-proxy-api/internal/oauth2/",
	}
	for _, item := range deprecated {
		if strings.HasSuffix(item, "/") {
			if strings.HasPrefix(path, item) {
				return true
			}
			continue
		}
		if path == item || strings.HasPrefix(path, item+"/") {
			return true
		}
	}
	return false
}

func isHandlerDBCall(receiver string, method string) bool {
	if receiver == "dao" || receiver == "database" {
		return true
	}
	switch method {
	case "Where", "Find", "Create", "Save":
		return true
	default:
		return false
	}
}

func isInterfaceLayerPath(path string) bool {
	return isUnder(path, "internal/handler") || isUnder(path, "internal/middleware") || isUnder(path, "internal/router") || isUnder(path, "internal/dto") || isUnder(path, "internal/application")
}

func receiverIdentName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 || len(fn.Recv.List[0].Names) == 0 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
}
```

- [ ] **Step 5: Run tests to verify pass**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

## Task 6: Style Rules For Commented Code And Naming

**Files:**
- Modify: `internal/tool/lintconv/runner.go`
- Create: `internal/tool/lintconv/rules_style.go`
- Modify: `test/unit/lintconv/fixtures/cases.json`

- [ ] **Step 1: Add fixture cases for style warnings**

Append these cases to the JSON array in `test/unit/lintconv/fixtures/cases.json` before the closing `]`:

```json
,
  {
    "name": "commented out code warns",
    "files": [
      {
        "path": "internal/application/demo/comment.go",
        "content": "package demo\n\nfunc Run() {\n\t// if enabled {\n}\n"
      }
    ],
    "want": [
      {"rule": "style.commented_code", "severity": "warning"}
    ]
  },
  {
    "name": "implementation detail variable name warns",
    "files": [
      {
        "path": "internal/application/demo/name.go",
        "content": "package demo\n\nfunc Run() {\n\tuserList := []string{}\n\t_ = userList\n}\n"
      }
    ],
    "want": [
      {"rule": "style.implementation_name", "severity": "warning"}
    ]
  }
```

- [ ] **Step 2: Run tests to verify new cases fail**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: FAIL with missing style diagnostics.

- [ ] **Step 3: Wire style rule group**

In `internal/tool/lintconv/runner.go`, replace `Run` with:

```go
func Run(args []string) Result {
	files, diagnostics := loadSourceFiles(args)
	c := &checker{files: files, diagnostics: diagnostics}
	c.checkErrorHandling()
	c.checkLogging()
	c.checkTesting()
	c.checkArchitecture()
	c.checkStyle()
	return Result{Diagnostics: c.diagnostics}
}
```

- [ ] **Step 4: Implement style rules**

Create `internal/tool/lintconv/rules_style.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"regexp"
	"strings"
)

var implementationNamePattern = regexp.MustCompile(`[a-z](List|Map|Slice|Array)$`)

func (c *checker) checkStyle() {
	for _, file := range c.files {
		c.checkCommentedCode(file)
		c.checkImplementationDetailNames(file)
	}
}

func (c *checker) checkCommentedCode(file SourceFile) {
	if !isUnder(file.Path, "internal") {
		return
	}
	for _, group := range file.File.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			if strings.HasPrefix(text, "@") || strings.HasPrefix(text, "Package ") || strings.HasPrefix(text, "go:") || strings.HasPrefix(text, "nolint") {
				continue
			}
			if looksLikeCommentedCode(text) {
				c.report(file, comment, SeverityWarning, "style.commented_code", "可能存在被注释掉的死代码，请确认是否需要删除")
			}
		}
	}
}

func (c *checker) checkImplementationDetailNames(file SourceFile) {
	if !isUnder(file.Path, "internal") || strings.HasSuffix(file.Path, "_test.go") {
		return
	}
	inspectFile(file, func(node ast.Node) {
		switch stmt := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if ok && isImplementationDetailName(ident.Name) {
					c.report(file, ident, SeverityWarning, "style.implementation_name", "变量命名可能暴露实现细节，建议使用复数形式")
				}
			}
		case *ast.ValueSpec:
			for _, ident := range stmt.Names {
				if isImplementationDetailName(ident.Name) {
					c.report(file, ident, SeverityWarning, "style.implementation_name", "变量命名可能暴露实现细节，建议使用复数形式")
				}
			}
		}
	})
}

func looksLikeCommentedCode(text string) bool {
	prefixes := []string{"func ", "if ", "for ", "var ", "type ", "const ", "switch ", "case ", "return ", "err :=", "err =", "ctx.", "req.", "rsp."}
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func isImplementationDetailName(name string) bool {
	if !implementationNamePattern.MatchString(name) {
		return false
	}
	allowed := []string{"stateMap", "choiceMap", "toolCallMap", "blockMap", "blackList", "whiteList", "allowList", "denyList", "bodyMap", "dataMap", "msgMap", "messageMap", "toolMap", "existingMap", "SchemaMap"}
	for _, item := range allowed {
		if name == item {
			return false
		}
	}
	return true
}
```

- [ ] **Step 5: Run tests to verify pass**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

## Task 7: Magic Value And Anonymous Struct Rules

**Files:**
- Modify: `internal/tool/lintconv/runner.go`
- Create: `internal/tool/lintconv/rules_magic.go`
- Modify: `test/unit/lintconv/fixtures/cases.json`

- [ ] **Step 1: Add fixture cases for magic values and anonymous struct**

Append these cases to the JSON array in `test/unit/lintconv/fixtures/cases.json` before the closing `]`:

```json
,
  {
    "name": "magic number is rejected",
    "files": [
      {
        "path": "internal/application/demo/number.go",
        "content": "package demo\n\nfunc Run() int {\n\treturn 128\n}\n"
      }
    ],
    "want": [
      {"rule": "magic.number", "severity": "error"}
    ]
  },
  {
    "name": "magic duration is rejected",
    "files": [
      {
        "path": "internal/application/demo/duration.go",
        "content": "package demo\n\nimport \"time\"\n\nfunc Run() time.Duration {\n\treturn 5 * time.Second\n}\n"
      }
    ],
    "want": [
      {"rule": "magic.duration", "severity": "error"}
    ]
  },
  {
    "name": "magic string is rejected",
    "files": [
      {
        "path": "internal/application/demo/string.go",
        "content": "package demo\n\nfunc Run() string {\n\treturn \"hello\"\n}\n"
      }
    ],
    "want": [
      {"rule": "magic.string", "severity": "error"}
    ]
  },
  {
    "name": "anonymous struct is rejected",
    "files": [
      {
        "path": "internal/application/demo/struct.go",
        "content": "package demo\n\nfunc Run() {\n\tvalue := struct {\n\t\tName string\n\t}{}\n\t_ = value\n}\n"
      }
    ],
    "want": [
      {"rule": "anonymous_struct", "severity": "error"}
    ]
  }
```

- [ ] **Step 2: Run tests to verify new cases fail**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: FAIL with missing magic and anonymous struct diagnostics.

- [ ] **Step 3: Wire magic rule group**

In `internal/tool/lintconv/runner.go`, replace `Run` with:

```go
func Run(args []string) Result {
	files, diagnostics := loadSourceFiles(args)
	c := &checker{files: files, diagnostics: diagnostics}
	c.checkErrorHandling()
	c.checkLogging()
	c.checkTesting()
	c.checkArchitecture()
	c.checkStyle()
	c.checkMagicValues()
	return Result{Diagnostics: c.diagnostics}
}
```

- [ ] **Step 4: Implement magic and anonymous struct rules**

Create `internal/tool/lintconv/rules_magic.go` with this content:

```go
package lintconv

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

func (c *checker) checkMagicValues() {
	for _, file := range c.files {
		if isMagicExcludedPath(file.Path) {
			continue
		}
		c.checkMagicLiterals(file)
		c.checkAnonymousStructs(file)
	}
}

func (c *checker) checkMagicLiterals(file SourceFile) {
	inspectFile(file, func(node ast.Node) {
		switch current := node.(type) {
		case *ast.BasicLit:
			c.checkMagicBasicLit(file, current)
		case *ast.BinaryExpr:
			c.checkMagicDuration(file, current)
		}
	})
}

func (c *checker) checkMagicBasicLit(file SourceFile, lit *ast.BasicLit) {
	if lit.Kind == token.INT {
		value, err := strconv.Atoi(lit.Value)
		if err == nil && value >= 30 {
			c.report(file, lit, SeverityError, "magic.number", "发现魔法数字，应提取为具名常量")
		}
		return
	}
	if lit.Kind != token.STRING || !isUnder(file.Path, "internal") {
		return
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil || len(value) < 2 || strings.HasPrefix(value, "[") {
		return
	}
	c.report(file, lit, SeverityError, "magic.string", "发现魔法字符串，应提取为具名常量")
}

func (c *checker) checkMagicDuration(file SourceFile, expr *ast.BinaryExpr) {
	if expr.Op != token.MUL {
		return
	}
	if _, ok := expr.X.(*ast.BasicLit); !ok {
		return
	}
	receiver, _, ok := selectorName(expr.Y)
	if ok && receiver == "time" {
		c.report(file, expr, SeverityError, "magic.duration", "发现魔法 duration 乘数，应提取为具名常量")
	}
}

func (c *checker) checkAnonymousStructs(file SourceFile) {
	if !(isUnder(file.Path, "internal") || isUnder(file.Path, "cmd")) || strings.HasSuffix(file.Path, "_test.go") {
		return
	}
	inspectFile(file, func(node ast.Node) {
		typeSpec, ok := node.(*ast.TypeSpec)
		if ok {
			return
		}
		_ = typeSpec
		structType, ok := node.(*ast.StructType)
		if ok {
			c.report(file, structType, SeverityError, "anonymous_struct", "禁止使用匿名 struct，请提取为包内命名类型")
		}
	})
}

func isMagicExcludedPath(path string) bool {
	excluded := []string{
		"internal/common/constant",
		"internal/common/enum",
		"internal/common/ierr",
		"internal/common/model",
		"internal/enum",
		"internal/config",
		"internal/router",
	}
	for _, prefix := range excluded {
		if isUnder(path, prefix) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 5: Run tests to verify pass or expose anonymous struct parent issue**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS for magic rules, but anonymous struct may falsely report named `type X struct` in future full-repo runs because this first implementation cannot see parents.

- [ ] **Step 6: Fix anonymous struct parent handling before full-repo run**

Modify `internal/tool/lintconv/rules_magic.go` by replacing `checkAnonymousStructs` with this implementation:

```go
func (c *checker) checkAnonymousStructs(file SourceFile) {
	if !(isUnder(file.Path, "internal") || isUnder(file.Path, "cmd")) || strings.HasSuffix(file.Path, "_test.go") {
		return
	}
	ast.Inspect(file.File, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.TypeSpec:
			return false
		case *ast.StructType:
			c.report(file, current, SeverityError, "anonymous_struct", "禁止使用匿名 struct，请提取为包内命名类型")
		}
		return true
	})
}
```

- [ ] **Step 7: Run tests to verify pass**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

## Task 8: CLI Entrypoint And Shell Wrapper

**Files:**
- Create: `cmd/lintconv/main.go`
- Modify: `script/lint-conventions.sh`
- Modify: `test/unit/lintconv/lintconv_test.go`

- [ ] **Step 1: Add unit test for Result exit semantics**

Append this test to `test/unit/lintconv/lintconv_test.go`:

```go
func TestResultCountsBySeverity(t *testing.T) {
	result := lintconv.Result{Diagnostics: []lintconv.Diagnostic{
		{Rule: "a", Severity: lintconv.SeverityError},
		{Rule: "b", Severity: lintconv.SeverityWarning},
		{Rule: "c", Severity: lintconv.SeverityWarning},
	}}
	if result.ErrorCount() != 1 {
		t.Fatalf("ErrorCount() = %d, want 1", result.ErrorCount())
	}
	if result.WarningCount() != 2 {
		t.Fatalf("WarningCount() = %d, want 2", result.WarningCount())
	}
}
```

- [ ] **Step 2: Run tests to verify pass before wrapper edit**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

- [ ] **Step 3: Create CLI entrypoint**

Create `cmd/lintconv/main.go` with this content:

```go
package main

import (
	"os"

	"github.com/hcd233/aris-proxy-api/internal/tool/lintconv"
)

func main() {
	result := lintconv.Run(os.Args[1:])
	result.Print(os.Stdout)
	if result.ErrorCount() > 0 {
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Replace shell script with compatibility wrapper**

Replace `script/lint-conventions.sh` with this content:

```bash
#!/usr/bin/env bash
# lint-conventions.sh — compatibility wrapper for the Go AST convention checker.
set -euo pipefail

go run ./cmd/lintconv ./...
```

- [ ] **Step 5: Run CLI against fixture directory**

Run: `go run ./cmd/lintconv ./test/unit/lintconv`

Expected: exits 0 and prints `All convention checks passed!`, because fixture JSON content is not parsed as Go source.

## Task 9: Full Verification And Rule Tuning

**Files:**
- Modify as needed: `internal/tool/lintconv/*.go`
- Modify as needed: production files only when the new analyzer reports a true existing convention violation.

- [ ] **Step 1: Run focused lintconv unit tests**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

- [ ] **Step 2: Run make lint-conv**

Run: `make lint-conv`

Expected: Either PASS or structured diagnostics from the new Go analyzer.

- [ ] **Step 3: If make lint-conv reports false positives, tune analyzer narrowly**

For each false positive, update the specific rule with an AST-aware exemption. Example for magic strings in logger calls: add helper in `internal/tool/lintconv/rules_magic.go`:

```go
func isLoggerMessageLiteral(file SourceFile, lit *ast.BasicLit) bool {
	found := false
	ast.Inspect(file.File, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		_, method, ok := selectorName(call.Fun)
		if !ok || !isLoggerMethod(method) || len(call.Args) == 0 {
			return true
		}
		if call.Args[0] == lit {
			found = true
			return false
		}
		return true
	})
	return found
}
```

Then call it before reporting `magic.string`:

```go
if isLoggerMessageLiteral(file, lit) {
	return
}
```

- [ ] **Step 4: If make lint-conv reports true violations, fix the smallest affected code**

Use the diagnostic path and line. Do not weaken the rule for true violations. Examples:

```go
const requestTimeout = 30 * time.Second
```

or:

```go
type responsePayload struct {
	ID string
}
```

- [ ] **Step 5: Run focused lintconv unit tests again**

Run: `go test -count=1 ./test/unit/lintconv/`

Expected: PASS.

- [ ] **Step 6: Run make lint-conv again**

Run: `make lint-conv`

Expected: PASS, or only warnings with exit code 0 if existing behavior allows warnings.

- [ ] **Step 7: Run full test suite**

Run: `go test -count=1 ./...`

Expected: PASS. E2E tests should skip when `BASE_URL` and `API_KEY` are not set.

## Self-Review

- Spec coverage: Plan covers `make lint-conv` entrypoint preservation, shell wrapper compatibility, Go AST checker, all existing shell rule groups, structured diagnostics, unit tests, and full verification commands.
- Placeholder scan: No `TBD`, `TODO`, or open-ended implementation placeholders are present. Task 9 contains conditional tuning steps with concrete helper examples because full-repo diagnostics are not known until the analyzer runs.
- Type consistency: The plan consistently uses `lintconv.Run([]string) Result`, `Diagnostic`, `Severity`, `SeverityError`, `SeverityWarning`, `checker.report`, and rule IDs used by fixture expectations.
