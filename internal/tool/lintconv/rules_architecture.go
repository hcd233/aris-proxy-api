package lintconv

import (
	"go/ast"
	"strconv"
	"strings"
)

func (c *checker) checkArchitecture() {
	for _, file := range c.files {
		c.checkArchitectureImports(file)
		c.checkArchitectureCalls(file)
		c.checkPassthroughWrappers(file)
	}
}

func (c *checker) checkArchitectureImports(file SourceFile) {
	for _, imp := range file.File.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		if isUnder(file.Path, "internal/domain") && strings.HasPrefix(path, "github.com/hcd233/aris-proxy-api/internal/infrastructure/") {
			c.report(file, imp, SeverityError, "architecture.domain_dependency", "Domain 层禁止依赖 Infrastructure 层")
		}
		if isUnder(file.Path, "internal/domain") && path == "github.com/hcd233/aris-proxy-api/internal/dto" {
			c.report(file, imp, SeverityError, "architecture.domain_dependency", "Domain 层禁止依赖 DTO")
		}
		if isUnder(file.Path, "internal/domain") && path == "github.com/hcd233/aris-proxy-api/internal/util" {
			c.report(file, imp, SeverityError, "architecture.domain_dependency", "Domain 层禁止依赖 internal/util，请改用 internal/common/util")
		}
		if isUnder(file.Path, "internal/application") && isDeprecatedApplicationImport(path) {
			c.report(file, imp, SeverityError, "architecture.deprecated_application_import", "Application 层禁止引用已废弃 internal/service/converter/proxy/agent/jwt/oauth2 包")
		}
	}
}

func (c *checker) checkArchitectureCalls(file SourceFile) {
	inspectFile(file, func(node ast.Node) {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return
		}
		receiver, method, ok := selectorName(call.Fun)
		if !ok {
			return
		}
		if isUnder(file.Path, "internal/handler") && isHandlerDBCall(receiver, method) {
			c.report(file, call, SeverityError, "architecture.handler_db", "Handler 层禁止直接操作 DAO/DB，业务逻辑应放在 Service 层")
		}
		if isInterfaceLayerPath(file.Path) && receiver == "context" && (method == "Background" || method == "TODO") {
			c.report(file, call, SeverityError, "architecture.root_context", "接口逻辑层禁止使用 context.Background()/context.TODO()，应从上层传递 context")
		}
	})
}

func (c *checker) checkPassthroughWrappers(file SourceFile) {
	if !isUnder(file.Path, "internal") || isUnder(file.Path, "internal/handler") {
		return
	}
	for _, decl := range file.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil || fn.Name.Name == "init" || len(fn.Body.List) != 1 {
			continue
		}
		ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		receiverName := receiverIdentName(fn)
		callReceiver, callMethod, ok := selectorName(call.Fun)
		if ok && receiverName != "" && callReceiver == receiverName && callMethod != "" {
			c.report(file, fn, SeverityWarning, "architecture.passthrough", "发现透传封装函数，应将逻辑内联或合并方法")
		}
	}
}

func isDeprecatedApplicationImport(path string) bool {
	deprecated := []string{
		"github.com/hcd233/aris-proxy-api/internal/service",
		"github.com/hcd233/aris-proxy-api/internal/converter",
		"github.com/hcd233/aris-proxy-api/internal/proxy",
		"github.com/hcd233/aris-proxy-api/internal/agent/",
		"github.com/hcd233/aris-proxy-api/internal/jwt/",
		"github.com/hcd233/aris-proxy-api/internal/oauth2/",
	}
	for _, item := range deprecated {
		if strings.HasSuffix(item, "/") {
			if strings.HasPrefix(path, item) {
				return true
			}
			continue
		}
		if path == item || strings.HasPrefix(path, item+"/") {
			return true
		}
	}
	return false
}

func isHandlerDBCall(receiver string, method string) bool {
	if receiver == "dao" || receiver == "database" {
		return true
	}
	switch method {
	case "Where", "Find", "Create", "Save":
		return true
	default:
		return false
	}
}

func isInterfaceLayerPath(path string) bool {
	return isUnder(path, "internal/handler") || isUnder(path, "internal/middleware") || isUnder(path, "internal/router") || isUnder(path, "internal/dto") || isUnder(path, "internal/application")
}

func receiverIdentName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 || len(fn.Recv.List[0].Names) == 0 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
}
