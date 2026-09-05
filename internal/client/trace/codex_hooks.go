package trace

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type hookSpec struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type hookGroup struct {
	Matcher string     `json:"matcher"`
	Hooks   []hookSpec `json:"hooks"`
}

// ingestCommand 返回指定 agent 的完整 ingest hook 命令（显式携带 --agent）
func ingestCommand(binPath, agent string) string {
	return binPath + constant.ArisClientIngestCommandSuffix + " --agent " + agent
}

// InstallCodexHooks 将 aris hook 幂等写入 ~/.codex/hooks.json（写前备份 .bak），返回注册事件数
func InstallCodexHooks(paths Paths, binPath string) (int, error) {
	return installAgentHooks(
		paths.CodexHooksFile(),
		paths.CodexHooksBackupFile(),
		ingestCommand(binPath, constant.TraceAgentCodex),
		constant.ArisClientCodexHookEvents,
	)
}

// InspectCodexHooks 检测 ~/.codex/hooks.json 中 aris hook 的注册情况；文件缺失或损坏时全部视为未注册
func InspectCodexHooks(paths Paths, binPath string) (found int, missing []string) {
	return inspectAgentHooks(
		paths.CodexHooksFile(),
		ingestCommand(binPath, constant.TraceAgentCodex),
		constant.ArisClientCodexHookEvents,
	)
}

// installAgentHooks 将 aris hook 组幂等写入目标 settings 文件：
// 保留非 aris hook 组与顶层其他键；文件已存在时先写 .bak 备份；解析失败中止且不改原文件。
func installAgentHooks(settingsFile, backupFile, command string, events []string) (int, error) {
	root, hooks, existed, err := readHooksFile(settingsFile)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		kept := make([]hookGroup, 0, len(hooks[event])+1)
		for _, group := range hooks[event] {
			// 按 " trace ingest" 子串去重：清理任意旧路径/旧格式的 aris hook，保证幂等
			if !groupHasIngestHook(group) {
				kept = append(kept, group)
			}
		}
		hooks[event] = append(kept, hookGroup{
			Matcher: "",
			Hooks: []hookSpec{{
				Type:    constant.ArisClientHookTypeCommand,
				Command: command,
				Timeout: constant.ArisClientHookTimeout,
			}},
		})
	}
	// 清理已从注册列表移除的事件：删除其下残留的 aris hook 组（保留非 aris 用户组）
	stripRemovedEvents(hooks, events)
	hooksData, err := sonic.Marshal(hooks)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDTOMarshal, err, "encode hooks field")
	}
	root[constant.ArisClientHooksField] = hooksData

	if existed {
		raw, err := os.ReadFile(settingsFile)
		if err != nil {
			return 0, ierr.Wrap(ierr.ErrInternal, err, "read hooks file for backup")
		}
		if err := writePrivateFile(backupFile, raw); err != nil {
			return 0, err
		}
	}

	data, err := sonic.MarshalIndent(root, "", constant.ArisClientJSONIndent)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDTOMarshal, err, "encode hooks file")
	}
	if err := writePrivateFile(settingsFile, data); err != nil {
		return 0, err
	}
	return len(events), nil
}

// inspectAgentHooks 检测目标 settings 文件中 aris hook 的注册情况；文件缺失或损坏时全部视为未注册
func inspectAgentHooks(settingsFile, command string, events []string) (found int, missing []string) {
	_, hooks, existed, err := readHooksFile(settingsFile)
	if err != nil || !existed {
		return 0, append([]string{}, events...)
	}
	for _, event := range events {
		registered := false
		for _, group := range hooks[event] {
			if groupHasCommand(group, command) {
				registered = true
				break
			}
		}
		if registered {
			found++
		} else {
			missing = append(missing, event)
		}
	}
	return found, missing
}

func readHooksFile(settingsFile string) (root map[string]sonic.NoCopyRawMessage, hooks map[string][]hookGroup, existed bool, err error) {
	data, err := os.ReadFile(settingsFile)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]sonic.NoCopyRawMessage{}, map[string][]hookGroup{}, false, nil
	}
	if err != nil {
		return nil, nil, false, ierr.Wrap(ierr.ErrInternal, err, "read hooks file")
	}
	root = map[string]sonic.NoCopyRawMessage{}
	if err := sonic.Unmarshal(data, &root); err != nil {
		return nil, nil, false, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode hooks file")
	}
	hooks = map[string][]hookGroup{}
	if raw, ok := root[constant.ArisClientHooksField]; ok && len(raw) > 0 {
		if err := sonic.Unmarshal(raw, &hooks); err != nil {
			return nil, nil, false, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode hooks field")
		}
	}
	return root, hooks, true, nil
}

// stripRemovedEvents 删除不在 events 列表中的事件下残留的 aris hook 组（保留非 aris 用户组）。
func stripRemovedEvents(hooks map[string][]hookGroup, events []string) {
	for event, groups := range hooks {
		if slices.Contains(events, event) {
			continue
		}
		kept := make([]hookGroup, 0, len(groups))
		for _, group := range groups {
			if !groupHasIngestHook(group) {
				kept = append(kept, group)
			}
		}
		if len(kept) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = kept
		}
	}
}

func groupHasCommand(group hookGroup, command string) bool {
	for _, hook := range group.Hooks {
		if hook.Command == command {
			return true
		}
	}
	return false
}

func groupHasIngestHook(group hookGroup) bool {
	for _, hook := range group.Hooks {
		if strings.Contains(hook.Command, constant.ArisClientIngestCommandSuffix) {
			return true
		}
	}
	return false
}
