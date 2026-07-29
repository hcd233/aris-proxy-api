package trace

import (
	"errors"
	"os"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

type codexHookSpec struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type codexHookGroup struct {
	Matcher string          `json:"matcher"`
	Hooks   []codexHookSpec `json:"hooks"`
}

// InstallCodexHooks 将 aris hook 幂等写入 ~/.codex/hooks.json（写前备份 .bak），返回注册事件数
func InstallCodexHooks(paths Paths, binPath string) (int, error) {
	root, hooks, existed, err := readCodexHooksFile(paths)
	if err != nil {
		return 0, err
	}
	command := binPath + constant.TraceClientIngestCommandSuffix
	for _, event := range constant.TraceClientCodexHookEvents {
		kept := make([]codexHookGroup, 0, len(hooks[event])+1)
		for _, group := range hooks[event] {
			// 按 " trace ingest" 后缀去重：清理任意旧路径的 aris hook，保证幂等
			if !groupHasIngestHook(group) {
				kept = append(kept, group)
			}
		}
		hooks[event] = append(kept, codexHookGroup{
			Matcher: "",
			Hooks: []codexHookSpec{{
				Type:    constant.TraceClientHookTypeCommand,
				Command: command,
				Timeout: constant.TraceClientHookTimeout,
			}},
		})
	}
	hooksData, err := sonic.Marshal(hooks)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDTOMarshal, err, "encode codex hooks field")
	}
	root[constant.TraceClientHooksField] = hooksData

	if existed {
		raw, err := os.ReadFile(paths.CodexHooksFile())
		if err != nil {
			return 0, ierr.Wrap(ierr.ErrInternal, err, "read codex hooks file for backup")
		}
		if err := writePrivateFile(paths.CodexHooksBackupFile(), raw); err != nil {
			return 0, err
		}
	}

	data, err := sonic.MarshalIndent(root, "", constant.TraceClientJSONIndent)
	if err != nil {
		return 0, ierr.Wrap(ierr.ErrDTOMarshal, err, "encode codex hooks file")
	}
	if err := writePrivateFile(paths.CodexHooksFile(), data); err != nil {
		return 0, err
	}
	return len(constant.TraceClientCodexHookEvents), nil
}

// InspectCodexHooks 检测 ~/.codex/hooks.json 中 aris hook 的注册情况；文件缺失或损坏时全部视为未注册
func InspectCodexHooks(paths Paths, binPath string) (found int, missing []string) {
	_, hooks, existed, err := readCodexHooksFile(paths)
	if err != nil || !existed {
		return 0, append([]string{}, constant.TraceClientCodexHookEvents...)
	}
	command := binPath + constant.TraceClientIngestCommandSuffix
	for _, event := range constant.TraceClientCodexHookEvents {
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

func readCodexHooksFile(paths Paths) (root map[string]sonic.NoCopyRawMessage, hooks map[string][]codexHookGroup, existed bool, err error) {
	data, err := os.ReadFile(paths.CodexHooksFile())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]sonic.NoCopyRawMessage{}, map[string][]codexHookGroup{}, false, nil
	}
	if err != nil {
		return nil, nil, false, ierr.Wrap(ierr.ErrInternal, err, "read codex hooks file")
	}
	root = map[string]sonic.NoCopyRawMessage{}
	if err := sonic.Unmarshal(data, &root); err != nil {
		return nil, nil, false, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode codex hooks file")
	}
	hooks = map[string][]codexHookGroup{}
	if raw, ok := root[constant.TraceClientHooksField]; ok && len(raw) > 0 {
		if err := sonic.Unmarshal(raw, &hooks); err != nil {
			return nil, nil, false, ierr.Wrap(ierr.ErrDTOUnmarshal, err, "decode codex hooks field")
		}
	}
	return root, hooks, true, nil
}

func groupHasCommand(group codexHookGroup, command string) bool {
	for _, hook := range group.Hooks {
		if hook.Command == command {
			return true
		}
	}
	return false
}

func groupHasIngestHook(group codexHookGroup) bool {
	for _, hook := range group.Hooks {
		if strings.HasSuffix(hook.Command, constant.TraceClientIngestCommandSuffix) {
			return true
		}
	}
	return false
}
