package enum

// DemoAccessAction Demo 访问审计动作类型
//
//	@author centonhuang
//	@update 2026-08-23 10:00:00
type DemoAccessAction = string

const (
	// DemoAccessActionLogin demo 登录成功
	DemoAccessActionLogin DemoAccessAction = "login"
	// DemoAccessActionLoginDenied demo 登录被拒
	DemoAccessActionLoginDenied DemoAccessAction = "login_denied"
	// DemoAccessActionModuleAccess 模块访问放行
	DemoAccessActionModuleAccess DemoAccessAction = "module_access"
	// DemoAccessActionModuleDenied 模块访问被白名单拒绝
	DemoAccessActionModuleDenied DemoAccessAction = "module_denied"
)
