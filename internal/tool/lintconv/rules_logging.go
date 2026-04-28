package lintconv

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"
	"unicode"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

func (c *checker) checkLogging() {
	for _, file := range c.files {
		if !isUnder(file.Path, constant.ConvCheckPathInternal) {
			continue
		}
		inspectFile(file, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			c.checkLoggerPrefix(file, call)
			c.checkLogMessageFormat(file, call)
			c.checkLogMessageChinese(file, call)
			c.checkSensitiveZapString(file, call)
		})
	}
}

func isLoggerCall(call *ast.CallExpr) (string, bool) {
	// Direct: logger.Info(...)
	receiver, method, ok := selectorName(call.Fun)
	if ok && receiver == constant.ConvCheckRecvLogger && isLoggerMethod(method) && len(call.Args) > 0 {
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return "", false
		}
		message, err := strconv.Unquote(literal.Value)
		if err != nil {
			return "", false
		}
		return message, true
	}
	// Chained: logger.WithCtx(...).Info(...) or logger.WithFCtx(...).Info(...)
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !isLoggerMethod(sel.Sel.Name) || len(call.Args) == 0 {
		return "", false
	}
	inner, ok := sel.X.(*ast.CallExpr)
	if !ok {
		return "", false
	}
	innerRecv, innerMethod, ok := selectorName(inner.Fun)
	if !ok || innerRecv != constant.ConvCheckRecvLogger || (innerMethod != constant.ConvCheckLogWithCtx && innerMethod != constant.ConvCheckLogWithFCtx) {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	message, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return message, true
}

func (c *checker) checkLoggerPrefix(file SourceFile, call *ast.CallExpr) {
	message, ok := isLoggerCall(call)
	if !ok {
		return
	}
	if !strings.HasPrefix(message, constant.ConvCheckPrefixBracket) {
		c.report(file, call.Args[0].(*ast.BasicLit), enum.SeverityWarning, constant.RuleLoggingPrefix, constant.ConvCheckMsgShouldPrefix)
	}
}

func (c *checker) checkLogMessageFormat(file SourceFile, call *ast.CallExpr) {
	message, ok := isLoggerCall(call)
	if !ok {
		return
	}
	if !strings.HasPrefix(message, constant.ConvCheckPrefixBracket) {
		return
	}
	idx := strings.IndexByte(message, ']')
	if idx < 0 {
		return
	}
	after := message[idx+1:]
	if len(after) == 0 {
		c.report(file, call.Args[0].(*ast.BasicLit), enum.SeverityWarning, constant.RuleLoggingFormat, constant.ConvCheckMsgAfterModuleName)
		return
	}
	if after[0] != ' ' {
		c.report(file, call.Args[0].(*ast.BasicLit), enum.SeverityWarning, constant.RuleLoggingFormat, constant.ConvCheckMsgAfterModuleSpace)
		return
	}
	rest := strings.TrimLeft(after, " ")
	if len(rest) == 0 {
		c.report(file, call.Args[0].(*ast.BasicLit), enum.SeverityWarning, constant.RuleLoggingFormat, constant.ConvCheckMsgAfterModuleName)
		return
	}
	if !unicode.IsUpper(rune(rest[0])) {
		c.report(file, call.Args[0].(*ast.BasicLit), enum.SeverityWarning, constant.RuleLoggingFormat, constant.ConvCheckMsgMustStartUppercase)
	}
}

func (c *checker) checkLogMessageChinese(file SourceFile, call *ast.CallExpr) {
	message, ok := isLoggerCall(call)
	if !ok {
		return
	}
	if containsChinese(message) {
		c.report(file, call.Args[0].(*ast.BasicLit), enum.SeverityWarning, constant.RuleLoggingChinese, constant.ConvCheckMsgMustNotChinese)
	}
}

func containsChinese(s string) bool {
	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) ||
			(r >= 0x3400 && r <= 0x4DBF) ||
			(r >= 0xF900 && r <= 0xFAFF) ||
			(r >= 0x2E80 && r <= 0x2FDF) ||
			(r >= 0x3000 && r <= 0x303F) ||
			(r >= 0xFF00 && r <= 0xFFEF) {
			return true
		}
	}
	return false
}

func (c *checker) checkSensitiveZapString(file SourceFile, call *ast.CallExpr) {
	receiver, method, ok := selectorName(call.Fun)
	if !ok || receiver != constant.ConvCheckRecvZap || method != constant.ConvCheckMethodString || len(call.Args) < 2 {
		return
	}
	name := stringLiteral(call.Args[0])
	if !isSensitiveFieldName(name) || isAllowedSensitiveFieldName(name) {
		return
	}
	if containsMaskSecret(call.Args[1]) {
		return
	}
	c.report(file, call, enum.SeverityWarning, constant.RuleLoggingSensitive, constant.ConvCheckMsgUseMaskSecret)
}

func isLoggerMethod(method string) bool {
	switch method {
	case constant.ConvCheckLogInfo, constant.ConvCheckLogError, constant.ConvCheckLogWarn, constant.ConvCheckLogDebug:
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
	return strings.Contains(lower, constant.ConvCheckSensitiveAPIKey) ||
		strings.Contains(lower, constant.ConvCheckSensitiveToken) ||
		strings.Contains(lower, constant.ConvCheckSensitiveSecret) ||
		strings.Contains(lower, constant.ConvCheckSensitivePassword)
}

func isAllowedSensitiveFieldName(name string) bool {
	lower := strings.ToLower(name)
	for _, item := range []string{constant.ConvCheckAllowedAPIKeyName, constant.ConvCheckAllowedTokenType, constant.ConvCheckAllowedTokenExpir, constant.ConvCheckAllowedSessionAPIKeyName} {
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
		if ok && method == constant.ConvCheckMethodMaskSecret {
			found = true
			return false
		}
		return true
	})
	return found
}
