package trace

import "github.com/hcd233/aris-proxy-api/internal/common/constant"

// InstallClaudeHooks 将 aris hook 幂等写入 ~/.claude/settings.json（写前备份 .bak），返回注册事件数。
// settings.json 顶层其他设置项原样保留；文件损坏时中止且不改原文件。
func InstallClaudeHooks(paths Paths, binPath string) (int, error) {
	return installAgentHooks(
		paths.ClaudeSettingsFile(),
		paths.ClaudeSettingsBackupFile(),
		ingestCommand(binPath, constant.TraceAgentClaude),
		constant.TraceClientClaudeHookEvents,
	)
}

// InspectClaudeHooks 检测 ~/.claude/settings.json 中 aris hook 的注册情况；文件缺失或损坏时全部视为未注册
func InspectClaudeHooks(paths Paths, binPath string) (found int, missing []string) {
	return inspectAgentHooks(
		paths.ClaudeSettingsFile(),
		ingestCommand(binPath, constant.TraceAgentClaude),
		constant.TraceClientClaudeHookEvents,
	)
}
