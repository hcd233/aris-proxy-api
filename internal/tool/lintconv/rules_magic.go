package lintconv

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
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
		if err == nil && value >= 30 {
			c.report(file, lit, SeverityError, "magic.number", "发现魔法数字，应提取为具名常量")
		}
		return
	}
	if lit.Kind != token.STRING || !isUnder(file.Path, "internal") || strings.HasPrefix(lit.Value, "`") {
		return
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil || len(value) < 2 || strings.HasPrefix(value, "[") || !isMagicStringContext(file, lit, value) {
		return
	}
	c.report(file, lit, SeverityError, "magic.string", "发现魔法字符串，应提取为具名常量")
}

func (c *checker) checkMagicDuration(file SourceFile, expr *ast.BinaryExpr) {
	if expr.Op != token.MUL || isConstExpr(file, expr) {
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
	ast.Inspect(file.File, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.TypeSpec:
			return false
		case *ast.StructType:
			if current.Fields == nil || len(current.Fields.List) == 0 {
				return true
			}
			c.report(file, current, SeverityError, "anonymous_struct", "禁止使用匿名 struct，请提取为包内命名类型")
		}
		return true
	})
}

func isMagicScanPath(path string) bool {
	return isUnder(path, "internal") || isUnder(path, "cmd")
}

func isMagicExcludedPath(path string) bool {
	excluded := []string{
		"internal/common/constant",
		"internal/common/enum",
		"internal/common/ierr",
		"internal/common/model",
		"internal/tool/lintconv",
		"internal/enum",
		"internal/config",
		"internal/router",
		"cmd/lintconv",
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

func isMagicStringContext(file SourceFile, lit *ast.BasicLit, value string) bool {
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
		return strings.HasPrefix(value, "/")
	case *ast.KeyValueExpr:
		return strings.HasPrefix(value, "/")
	default:
		return false
	}
}
