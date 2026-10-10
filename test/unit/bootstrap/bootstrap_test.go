package bootstrap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"go.uber.org/fx"

	appbootstrap "github.com/hcd233/aris-proxy-api/internal/bootstrap"
)

func TestBuildFxAppOptions(t *testing.T) {
	t.Parallel()
	opts := appbootstrap.BuildFxAppOptions("localhost", "0")
	if len(opts) == 0 {
		t.Fatal("BuildFxAppOptions() returned empty options")
	}
}

func TestServerDoesNotExposeDigContainer(t *testing.T) {
	t.Parallel()
	content := readFile(t, "../../../internal/bootstrap/container.go")
	if strings.Contains(content, "Container *dig.Container") {
		t.Fatal("Server must not expose dig.Container as an exported field")
	}
}

func TestBootstrapDoesNotUseAnyProviderList(t *testing.T) {
	t.Parallel()
	content := readFile(t, "../../../internal/bootstrap/container.go")
	if strings.Contains(content, "[]any{") || strings.Contains(content, "[]interface{}{") {
		t.Fatal("bootstrap providers must be registered without any/interface{} provider lists")
	}
}

// TestContainerDoesNotUseInterfaceType 验证 container.go 不使用 interface{} 和未导出 fx.Container
func TestContainerDoesNotUseInterfaceType(t *testing.T) {
	t.Parallel()
	content := readFile(t, "../../../internal/bootstrap/container.go")
	if strings.Contains(content, "interface{}") {
		t.Fatal("container.go should not use interface{} type — use concrete types")
	}
	if strings.Contains(content, "Container *fx.Container") {
		t.Fatal("container.go must not expose fx.Container as an exported field")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

// TestRouteParamsCoverAPIRouterDependencies 守护 bootstrap 路由装配的字段同步。
//
// 背景：#189 新增 APIRouterDependencies.PlaygroundHandler 但 routeParams 与
// registerRoutes 漏同步，生产启动时 initPlaygroundRouter 对 nil handler 解引用
// panic（CrashLoopBackOff）。fx.ValidateApp 不执行 Invoke，编译期也发现不了
// （结构体字面量允许缺字段），故按源码 AST 断言：APIRouterDependencies 的每个
// 字段都必须同时出现在 routeParams 字段与 registerRoutes 的字面量键中。
func TestRouteParamsCoverAPIRouterDependencies(t *testing.T) {
	t.Parallel()
	depFields := structFieldNames(t, "../../../internal/router/router.go", "APIRouterDependencies")
	paramFields := structFieldNames(t, "../../../internal/bootstrap/router.go", "routeParams")
	literalKeys := compositeLiteralKeys(t, "../../../internal/bootstrap/router.go", "APIRouterDependencies")
	for name := range depFields {
		if !paramFields[name] {
			t.Errorf("routeParams 缺少字段 %s（fx 不会注入，路由注册时为 nil）", name)
		}
		if !literalKeys[name] {
			t.Errorf("registerRoutes 未把 %s 传入 router.APIRouterDependencies", name)
		}
	}
}

func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("ParseFile(%s) error = %v", path, err)
	}
	return f
}

func structFieldNames(t *testing.T, path, typeName string) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	ast.Inspect(parseGoFile(t, path), func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != typeName {
			return true
		}
		st, ok := spec.Type.(*ast.StructType)
		if !ok {
			return false
		}
		for _, field := range st.Fields.List {
			for _, ident := range field.Names {
				names[ident.Name] = true
			}
		}
		return false
	})
	if len(names) == 0 {
		t.Fatalf("struct %s not found in %s", typeName, path)
	}
	return names
}

func compositeLiteralKeys(t *testing.T, path, typeName string) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	ast.Inspect(parseGoFile(t, path), func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != typeName {
			return true
		}
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if ident, ok := kv.Key.(*ast.Ident); ok {
					keys[ident.Name] = true
				}
			}
		}
		return false
	})
	if len(keys) == 0 {
		t.Fatalf("composite literal %s not found in %s", typeName, path)
	}
	return keys
}

// TestFxAppDependencyGraphValidates 静态校验 fx 依赖图（CR I4/R2）。
//
// 背景：#161 漏写 fx.As(new(port.ListClientModelsHandler)) 导致生产
// CrashLoopBackOff——编译期与普通单测均无法发现此类装配错误；
// fx.ValidateApp 只做依赖图解析（不执行构造函数/不连数据库），
// 把 DI 装配校验纳入 CI。
func TestFxAppDependencyGraphValidates(t *testing.T) {
	t.Parallel()
	if err := fx.ValidateApp(appbootstrap.BuildFxAppOptions("localhost", "0")...); err != nil {
		t.Fatalf("fx dependency graph validation failed: %v", err)
	}
}
