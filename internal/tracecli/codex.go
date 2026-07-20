package tracecli

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

var codexHookEvents = []string{
	constant.TraceEventSessionStart,
	constant.TraceEventUserPromptSubmit,
	constant.TraceEventPreToolUse,
	constant.TraceEventPermissionRequest,
	constant.TraceEventPostToolUse,
	constant.TraceEventStop,
	constant.TraceEventSubagentStart,
	constant.TraceEventSubagentStop,
	constant.TraceEventPreCompact,
	constant.TraceEventPostCompact,
}

type hookCommandView struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type hookGroupView struct {
	Matcher string            `json:"matcher"`
	Hooks   []hookCommandView `json:"hooks"`
}

type codexHookInstaller struct {
	paths Paths
}

func NewCodexHookInstaller(paths Paths) CodexInstaller {
	return &codexHookInstaller{paths: paths}
}

func CodexHookEvents() []string {
	return append([]string{}, codexHookEvents...)
}

func (i *codexHookInstaller) Install(
	ctx context.Context,
	commandPath string,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	absoluteCommand, err := filepath.Abs(commandPath)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrValidation, err, "resolve trace client command path")
	}
	targetCommand := absoluteCommand + constant.TraceClientIngestCommandSuffix
	root, hooks, original, err := i.readConfig()
	if err != nil {
		return "", err
	}
	group, err := sonic.Marshal(hookGroupView{
		Matcher: "",
		Hooks: []hookCommandView{{
			Type:    constant.TraceClientHookTypeCommand,
			Command: targetCommand,
			Timeout: constant.TraceClientHookTimeout,
		}},
	})
	if err != nil {
		return "", ierr.Wrap(ierr.ErrDTOMarshal, err, "encode Aris Codex hook")
	}

	for _, event := range codexHookEvents {
		groups := make([]sonic.NoCopyRawMessage, 0, len(hooks[event])+1)
		for _, raw := range hooks[event] {
			if !isArisHookGroup(raw, targetCommand) {
				groups = append(groups, raw)
			}
		}
		groups = append(groups, sonic.NoCopyRawMessage(group))
		hooks[event] = groups
	}
	hooksRaw, err := sonic.Marshal(hooks)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrDTOMarshal, err, "encode Codex hooks")
	}
	root[constant.TraceClientHooksField] = sonic.NoCopyRawMessage(hooksRaw)
	updated, err := sonic.MarshalIndent(root, "", constant.TraceClientJSONIndent)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrDTOMarshal, err, "encode Codex config")
	}

	backupPath := ""
	if len(original) > 0 {
		backupPath = i.paths.CodexHooksBackupFile()
		if err := writePrivateFile(backupPath, original); err != nil {
			return "", err
		}
	}
	if err := writePrivateFile(i.paths.CodexHooksFile(), updated); err != nil {
		return "", err
	}
	return backupPath, nil
}

func (i *codexHookInstaller) readConfig() (
	root map[string]sonic.NoCopyRawMessage,
	hooks map[string][]sonic.NoCopyRawMessage,
	original []byte,
	err error,
) {
	root = map[string]sonic.NoCopyRawMessage{}
	hooks = map[string][]sonic.NoCopyRawMessage{}
	original, err = os.ReadFile(i.paths.CodexHooksFile())
	if errors.Is(err, os.ErrNotExist) {
		return root, hooks, nil, nil
	}
	if err != nil {
		return nil, nil, nil, ierr.Wrap(ierr.ErrInternal, err, "read Codex config")
	}
	if unmarshalErr := sonic.Unmarshal(original, &root); unmarshalErr != nil {
		return nil, nil, nil, ierr.Wrap(ierr.ErrDTOUnmarshal, unmarshalErr, "decode Codex config")
	}
	if raw, ok := root[constant.TraceClientHooksField]; ok {
		if unmarshalErr := sonic.Unmarshal(raw, &hooks); unmarshalErr != nil {
			return nil, nil, nil, ierr.Wrap(ierr.ErrDTOUnmarshal, unmarshalErr, "decode Codex hooks")
		}
	}
	return root, hooks, original, nil
}

func isArisHookGroup(raw sonic.NoCopyRawMessage, targetCommand string) bool {
	var group hookGroupView
	if err := sonic.Unmarshal(raw, &group); err != nil {
		return false
	}
	for _, hook := range group.Hooks {
		if hook.Command == targetCommand {
			return true
		}
	}
	return false
}
