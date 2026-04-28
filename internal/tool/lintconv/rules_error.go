package lintconv

import (
	"go/ast"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

func (c *checker) checkErrorHandling() {
	for _, file := range c.files {
		c.checkDeprecatedConstantErrors(file)
		c.checkForwardingConstants(file)
	}
}

func (c *checker) checkDeprecatedConstantErrors(file SourceFile) {
	if !isUnder(file.Path, constant.ConvCheckPathInternal) || file.Path == constant.ConvCheckPathErrorGo {
		return
	}
	inspectFile(file, func(node ast.Node) {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name != constant.ConvCheckIdentConstant || !strings.HasPrefix(selector.Sel.Name, constant.ConvCheckPrefixErr) || strings.HasPrefix(selector.Sel.Name, constant.ConvCheckErrorPrefix) {
			return
		}
		c.report(file, selector, enum.SeverityError, constant.RuleErrorDeprecatedConstant, constant.ConvCheckMsgDeprecatedConstErr)
	})
}

func (c *checker) checkForwardingConstants(file SourceFile) {
	if !isConstantOrEnumPath(file.Path) || strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) {
		return
	}
	for _, decl := range file.File.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok.String() != constant.ConvCheckTokConst {
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
					c.report(file, valueSpec, enum.SeverityError, constant.RuleConstantForwarding, constant.ConvCheckMsgForwardingConst)
				}
			}
		}
	}
}

func isConstantOrEnumPath(path string) bool {
	return isUnder(path, constant.ConvCheckPathConstant) || isUnder(path, constant.ConvCheckPathCommonEnum) || isUnder(path, constant.ConvCheckPathEnum)
}
