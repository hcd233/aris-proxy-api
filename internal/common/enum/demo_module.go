package enum

import "slices"

// DemoModule Demo 演示账户可配置开放的数据模块
type DemoModule = string

const (
	DemoModuleDashboard DemoModule = "dashboard"  // 首页概览（统计卡片 + 审计图表）
	DemoModuleSessions  DemoModule = "sessions"   // 会话列表与详情（行为数据，取模抽样）
	DemoModuleAudit     DemoModule = "audit"      // 模型调用审计（行为数据，取模抽样）
	DemoModuleUpstream  DemoModule = "upstream"   // 上游配置（原 endpoints+models 合并）
	DemoModuleTrigger   DemoModule = "trigger"    // 触发词配置
	DemoModuleMonitor   DemoModule = "monitor"    // 运行时指标
	DemoModuleCron      DemoModule = "cron"       // 定时任务配置
	DemoModuleCronAudit DemoModule = "cron_audit" // 定时任务执行审计
)

// DemoModules 全部合法 Demo 模块
var DemoModules = []DemoModule{
	DemoModuleDashboard,
	DemoModuleSessions,
	DemoModuleAudit,
	DemoModuleUpstream,
	DemoModuleTrigger,
	DemoModuleMonitor,
	DemoModuleCron,
	DemoModuleCronAudit,
}

// IsValidDemoModule 校验模块 key 是否合法
func IsValidDemoModule(module DemoModule) bool {
	return slices.Contains(DemoModules, module)
}
