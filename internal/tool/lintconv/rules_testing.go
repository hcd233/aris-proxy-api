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
