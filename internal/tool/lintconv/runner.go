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
	c.checkErrorHandling()
	c.checkLogging()
	c.checkTesting()
	c.checkArchitecture()
	c.checkStyle()
	c.checkMagicValues()
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
