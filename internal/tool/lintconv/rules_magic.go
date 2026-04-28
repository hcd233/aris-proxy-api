package lintconv

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

func (c *checker) checkMagicValues() {
	for _, file := range c.files {
		if !isMagicScanPath(file.Path) || isMagicExcludedPath(file.Path) {
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
	if isConstLiteral(file, lit) || isImportLiteral(file, lit) || isLoggerMessageLiteral(file, lit) || isStructTagLiteral(file, lit) {
		return
	}
	if lit.Kind == token.INT {
		value, err := strconv.Atoi(lit.Value)
		if err == nil && value >= constant.ConvCheckMagicNumberThreshold {
			c.report(file, lit, enum.SeverityError, constant.RuleMagicNumber, constant.ConvCheckMsgMagicNumber)
		}
		return
	}
	if lit.Kind != token.STRING || !isUnder(file.Path, constant.ConvCheckPathInternal) || strings.HasPrefix(lit.Value, constant.ConvCheckBacktickPrefix) {
		return
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil || value == constant.ConvCheckEmptyString || len(value) < 2 || strings.HasPrefix(value, constant.ConvCheckPrefixBracket) || !isMagicStringContext(file, lit) {
		return
	}
	c.report(file, lit, enum.SeverityError, constant.RuleMagicString, constant.ConvCheckMsgMagicString)
}

func (c *checker) checkMagicDuration(file SourceFile, expr *ast.BinaryExpr) {
	if expr.Op != token.MUL || isConstExpr(file, expr) {
		return
	}
	if _, ok := expr.X.(*ast.BasicLit); !ok {
		return
	}
	receiver, _, ok := selectorName(expr.Y)
	if ok && receiver == constant.ConvCheckRecvTime {
		c.report(file, expr, enum.SeverityError, constant.RuleMagicDuration, constant.ConvCheckMsgMagicDuration)
	}
}

func (c *checker) checkAnonymousStructs(file SourceFile) {
	if !(isUnder(file.Path, constant.ConvCheckPathInternal) || isUnder(file.Path, constant.ConvCheckPathCMD)) || strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) {
		return
	}
	ast.Inspect(file.File, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.TypeSpec:
			return false
		case *ast.StructType:
			if current.Fields == nil || len(current.Fields.List) == 0 {
				return true
			}
			c.report(file, current, enum.SeverityError, constant.RuleAnonymousStruct, constant.ConvCheckMsgAnonymousStruct)
		}
		return true
	})
}

func isMagicScanPath(path string) bool {
	return isUnder(path, constant.ConvCheckPathInternal) || isUnder(path, constant.ConvCheckPathCMD)
}

func isMagicExcludedPath(path string) bool {
	excluded := []string{
		constant.ConvCheckPathConstant,
		constant.ConvCheckPathCommonEnum,
		constant.ConvCheckPathIerr,
		constant.ConvCheckPathEnum,
		constant.ConvCheckPathConfig,
		constant.ConvCheckPathRouter,
	}
	for _, prefix := range excluded {
		if isUnder(path, prefix) {
			return true
		}
	}
	return false
}

func isConstLiteral(file SourceFile, lit *ast.BasicLit) bool {
	return hasAncestor(file, lit, func(node ast.Node) bool {
		decl, ok := node.(*ast.GenDecl)
		return ok && decl.Tok == token.CONST
	})
}

func isConstExpr(file SourceFile, expr ast.Expr) bool {
	return hasAncestor(file, expr, func(node ast.Node) bool {
		decl, ok := node.(*ast.GenDecl)
		return ok && decl.Tok == token.CONST
	})
}

func isImportLiteral(file SourceFile, lit *ast.BasicLit) bool {
	return hasAncestor(file, lit, func(node ast.Node) bool {
		_, ok := node.(*ast.ImportSpec)
		return ok
	})
}

func isStructTagLiteral(file SourceFile, lit *ast.BasicLit) bool {
	return hasAncestor(file, lit, func(node ast.Node) bool {
		field, ok := node.(*ast.Field)
		return ok && field.Tag == lit
	})
}

func isLoggerMessageLiteral(file SourceFile, lit *ast.BasicLit) bool {
	return hasAncestor(file, lit, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || call.Args[0] != lit {
			return false
		}
		_, method, ok := selectorName(call.Fun)
		return ok && isLoggerMethod(method)
	})
}

func hasAncestor(file SourceFile, target ast.Node, match func(ast.Node) bool) bool {
	found := false
	var stack []ast.Node
	ast.Inspect(file.File, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		for _, ancestor := range stack {
			if node == target && match(ancestor) {
				found = true
				return false
			}
		}
		stack = append(stack, node)
		return !found
	})
	return found
}

func parentOf(file SourceFile, target ast.Node) ast.Node {
	var parent ast.Node
	var stack []ast.Node
	ast.Inspect(file.File, func(node ast.Node) bool {
		if node == nil {
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			return true
		}
		if node == target {
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			return false
		}
		stack = append(stack, node)
		return true
	})
	return parent
}

func isMagicStringContext(file SourceFile, lit *ast.BasicLit) bool {
	if isIgnoredMagicStringLiteral(file, lit) {
		return false
	}
	parent := parentOf(file, lit)
	switch current := parent.(type) {
	case *ast.AssignStmt:
		return true
	case *ast.ReturnStmt:
		return true
	case *ast.CaseClause:
		return true
	case *ast.BinaryExpr:
		return current.Op == token.EQL || current.Op == token.NEQ
	case *ast.CompositeLit:
		return true
	case *ast.KeyValueExpr:
		return true
	case *ast.CallExpr:
		return true
	default:
		return false
	}
}

func isIgnoredMagicStringLiteral(file SourceFile, lit *ast.BasicLit) bool {
	return hasAncestor(file, lit, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return false
		}
		return isIgnoredMagicStringCall(call) || isHumaSchemaNameArg(call, lit)
	})
}

func isIgnoredMagicStringCall(call *ast.CallExpr) bool {
	receiver, method, ok := selectorName(call.Fun)
	if ok {
		if receiver == constant.ConvCheckRecvIerr && (method == constant.ConvCheckIerrWrap || method == constant.ConvCheckIerrWrapf) {
			return true
		}
		if receiver == constant.ConvCheckRecvIerr && (method == constant.ConvCheckIerrNew || method == constant.ConvCheckIerrNewf) {
			return true
		}
		if (receiver == constant.ConvCheckRecvLogger || receiver == constant.ConvCheckRecvLog) && isLoggerMethod(method) {
			return true
		}
		if receiver == constant.ConvCheckRecvZap {
			return true
		}
		if receiver == constant.ConvCheckRecvReflect && method == constant.ConvCheckMethodTypeFor {
			return true
		}
	}
	method, ok = selectorMethodName(call.Fun)
	return ok && isLoggerMethod(method)
}

func isHumaSchemaNameArg(call *ast.CallExpr, lit *ast.BasicLit) bool {
	method, ok := selectorMethodName(call.Fun)
	if !ok || method != constant.ConvCheckMethodSchema || len(call.Args) < 3 {
		return false
	}
	return call.Args[2] == lit
}
