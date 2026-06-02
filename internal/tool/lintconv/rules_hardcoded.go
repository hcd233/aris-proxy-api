package lintconv

import (
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func (c *checker) checkHardcodedValues() {
	for _, file := range c.files {
		if isHardcodedExcludedPath(file.Path) {
			continue
		}
		c.checkHardcodedURLs(file)
		c.checkHardcodedErrorCodes(file)
	}
}

func (c *checker) checkHardcodedURLs(file SourceFile) {
	if strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) {
		return
	}
	inspectFile(file, func(node ast.Node) {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return
		}
		if isConstLiteral(file, lit) || isImportLiteral(file, lit) || isStructTagLiteral(file, lit) || isLoggerMessageLiteral(file, lit) {
			return
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return
		}
		if !looksLikeURL(value) || looksLikeURLPatternOnly(value) {
			return
		}
		c.report(file, lit, enum.SeverityWarning, constant.RuleHardcodedURL, constant.ConvCheckMsgHardcodedURL)
	})
}

func (c *checker) checkHardcodedErrorCodes(file SourceFile) {
	if isUnder(file.Path, constant.ConvCheckPathIerr) {
		return
	}
	inspectFile(file, func(node ast.Node) {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return
		}
		receiver, method, ok := selectorName(call.Fun)
		if !ok || receiver != constant.ConvCheckRecvModel || method != constant.ConvCheckMethodNewError {
			return
		}
		if len(call.Args) == 0 {
			return
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return
		}
		c.report(file, lit, enum.SeverityError, constant.RuleHardcodedErrorCode, constant.ConvCheckMsgHardcodedErrorCode)
	})
}

func isHardcodedExcludedPath(path string) bool {
	excluded := []string{
		constant.ConvCheckPathConfig,
		constant.ConvCheckPathIerr,
		constant.ConvCheckPathConstant,
		constant.ConvCheckPathCommonEnum,
		constant.ConvCheckPathRouter,
		constant.ConvCheckPathTool,
		constant.ConvCheckPathLintconv,
	}
	for _, prefix := range excluded {
		if isUnder(path, prefix) {
			return true
		}
	}
	return false
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, constant.ConvCheckURLPrefixHTTPS) || strings.HasPrefix(value, constant.ConvCheckURLPrefixHTTP)
}

// looksLikeURLPatternOnly 排除只写协议前缀的占位字符串（如 "https://"），
// 这些通常是规则/常量定义本身，而不是业务侧硬编码。
func looksLikeURLPatternOnly(value string) bool {
	rest := strings.TrimPrefix(value, constant.ConvCheckURLPrefixHTTPS)
	rest = strings.TrimPrefix(rest, constant.ConvCheckURLPrefixHTTP)
	return rest == ""
}
