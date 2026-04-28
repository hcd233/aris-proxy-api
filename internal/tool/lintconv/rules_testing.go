package lintconv

import (
	"go/ast"
	"strconv"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/enum"
)

func (c *checker) checkTesting() {
	for _, file := range c.files {
		if strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) && isUnder(file.Path, constant.ConvCheckPathInternal) {
			c.report(file, file.File, enum.SeverityError, constant.RuleTestingInternalFile, constant.ConvCheckMsgTestingInternal)
		}
		if strings.HasSuffix(file.Path, constant.ConvCheckSuffixTestGo) && strings.HasPrefix(file.Path, constant.ConvCheckPrefixTest) && strings.Count(strings.TrimPrefix(file.Path, constant.ConvCheckPrefixTest), constant.ConvCheckSeparatorSlash) == 0 {
			c.report(file, file.File, enum.SeverityError, constant.RuleTestingRootFile, constant.ConvCheckMsgTestingRoot)
		}
		for _, imp := range file.File.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if strings.HasPrefix(path, constant.ConvCheckPrefixTestify) {
				c.report(file, imp, enum.SeverityError, constant.RuleTestingTestify, constant.ConvCheckMsgTestingTestify)
			}
		}
		if !strings.HasPrefix(file.Path, constant.ConvCheckPrefixTest) {
			continue
		}
		inspectFile(file, func(node ast.Node) {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return
			}
			receiver, method, ok := selectorName(call.Fun)
			if ok && receiver == constant.ConvCheckRecvTime && method == constant.ConvCheckMethodSleep {
				c.report(file, call, enum.SeverityError, constant.RuleTestingSleep, constant.ConvCheckMsgTestingSleep)
			}
		})
	}
}
