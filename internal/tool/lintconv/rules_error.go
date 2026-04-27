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
	if !isUnder(file.Path, "internal") || file.Path == "internal/common/constant/error.go" {
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
