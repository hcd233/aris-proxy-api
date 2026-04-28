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
	// Only flag actual sensitive-value semantics, not generic "key" identifiers.
	return strings.Contains(lower, "apikey") ||
		strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password")
}

func isAllowedSensitiveFieldName(name string) bool {
	lower := strings.ToLower(name)
	allowed := []string{"apikeyname", "tokentype", "tokenexpir", "sessionapikeyname"}
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
