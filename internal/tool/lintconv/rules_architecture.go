package lintconv

import (
	"go/ast"
	"strconv"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/enum"
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
		if isUnder(file.Path, constant.ConvCheckPathDomain) && strings.HasPrefix(path, constant.ConvCheckImportInfra) {
			c.report(file, imp, enum.SeverityError, constant.RuleDomainDependency, constant.ConvCheckMsgDomainInfra)
		}
		if isUnder(file.Path, constant.ConvCheckPathDomain) && path == constant.ConvCheckImportDTO {
			c.report(file, imp, enum.SeverityError, constant.RuleDomainDependency, constant.ConvCheckMsgDomainDTO)
		}
		if isUnder(file.Path, constant.ConvCheckPathDomain) && path == constant.ConvCheckImportUtil {
			c.report(file, imp, enum.SeverityError, constant.RuleDomainDependency, constant.ConvCheckMsgDomainUtil)
		}
		if isUnder(file.Path, constant.ConvCheckPathApp) && isDeprecatedApplicationImport(path) {
			c.report(file, imp, enum.SeverityError, constant.RuleDeprecatedApplicationImport, constant.ConvCheckMsgDeprecatedAppImport)
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
		if isUnder(file.Path, constant.ConvCheckPathHandler) && isHandlerDBCall(receiver, method) {
			c.report(file, call, enum.SeverityError, constant.RuleHandlerDB, constant.ConvCheckMsgHandlerDB)
		}
		if isInterfaceLayerPath(file.Path) && receiver == constant.ConvCheckRecvContext && (method == constant.ConvCheckMethodBackground || method == constant.ConvCheckMethodTODO) {
			c.report(file, call, enum.SeverityError, constant.RuleRootContext, constant.ConvCheckMsgRootContext)
		}
		if receiver == constant.ConvCheckRecvDB && method == constant.ConvCheckMethodGetDBInstance && hasRootContextArg(call) {
			c.report(file, call, enum.SeverityError, constant.RuleDBRootContext, constant.ConvCheckMsgDBRootContext)
		}
	})
}

func (c *checker) checkPassthroughWrappers(file SourceFile) {
	if !isUnder(file.Path, constant.ConvCheckPathInternal) || isUnder(file.Path, constant.ConvCheckPathHandler) {
		return
	}
	for _, decl := range file.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Body == nil || fn.Name.Name == constant.ConvCheckFuncInit || len(fn.Body.List) != 1 {
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
			c.report(file, fn, enum.SeverityWarning, constant.RulePassthrough, constant.ConvCheckMsgPassthrough)
		}
	}
}

func hasRootContextArg(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	argCall, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	receiver, method, ok := selectorName(argCall.Fun)
	return ok && receiver == constant.ConvCheckRecvContext && (method == constant.ConvCheckMethodBackground || method == constant.ConvCheckMethodTODO)
}

func isDeprecatedApplicationImport(path string) bool {
	deprecated := []string{
		constant.ConvCheckDeprecatedImportService,
		constant.ConvCheckDeprecatedImportConverter,
		constant.ConvCheckDeprecatedImportProxy,
		constant.ConvCheckDeprecatedImportAgent,
		constant.ConvCheckDeprecatedImportJWT,
		constant.ConvCheckDeprecatedImportOAuth2,
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
	if receiver == constant.ConvCheckRecvDAO || receiver == constant.ConvCheckRecvDB {
		return true
	}
	switch method {
	case constant.ConvCheckMethodWhere, constant.ConvCheckMethodFind, constant.ConvCheckMethodCreate, constant.ConvCheckMethodSave:
		return true
	default:
		return false
	}
}

func isInterfaceLayerPath(path string) bool {
	return isUnder(path, constant.ConvCheckPathHandler) || isUnder(path, constant.ConvCheckPathMiddleware) || isUnder(path, constant.ConvCheckPathRouter) || isUnder(path, constant.ConvCheckPathDTO) || isUnder(path, constant.ConvCheckPathApp)
}

func receiverIdentName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 || len(fn.Recv.List[0].Names) == 0 {
		return ""
	}
	return fn.Recv.List[0].Names[0].Name
}
