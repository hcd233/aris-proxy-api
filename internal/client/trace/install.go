package trace

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// InstallOptions aris trace install 运行参数
type InstallOptions struct {
	Paths Paths
	In    io.Reader
	Out   io.Writer
}

// RunInstall 加载配置后交互式注册 agent hooks；未初始化时引导先运行 aris init
func RunInstall(ctx context.Context, opts InstallOptions) error {
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	paths := opts.Paths
	if paths.Root == "" {
		resolved, err := DefaultPaths()
		if err != nil {
			return err
		}
		paths = resolved
	}

	store := NewConfigStore(paths)
	cfg, loadErr := store.Load(ctx) //nolint:errcheck // 读不到按未初始化处理
	if loadErr != nil || cfg.Host == "" || cfg.APIKey == "" {
		return ierr.New(ierr.ErrValidation, constant.TraceClientInstallNeedInitMessage)
	}

	ttyIn, ttyOut, cleanup, err := terminalIO(in)
	if err != nil {
		return err
	}
	defer cleanup()

	agents, err := selectAgents(ttyIn, ttyOut)
	if err != nil {
		return err
	}

	binPath, err := ExecutablePath()
	if err != nil {
		return err
	}

	installCodex := slices.Contains(agents, constant.TraceAgentCodex)
	installClaude := slices.Contains(agents, constant.TraceAgentClaude)
	var codexRegistered, claudeRegistered int
	err = ui.RunWithSpinner(ttyIn, ttyOut, constant.TraceClientInitInstallingHooks, func() error {
		var installErr error
		if installCodex {
			if codexRegistered, installErr = InstallCodexHooks(paths, binPath); installErr != nil {
				return installErr
			}
		}
		if installClaude {
			if claudeRegistered, installErr = InstallClaudeHooks(paths, binPath); installErr != nil {
				return installErr
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	summary := []string{constant.TraceClientInstallDone}
	if installCodex {
		summary = append(summary,
			fmt.Sprintf(constant.TraceClientInitHooksFormat, constant.TraceClientInitAgentOptionCodex, codexRegistered, len(constant.TraceClientCodexHookEvents)),
			constant.TraceClientInitApprovalHint,
		)
	}
	if installClaude {
		summary = append(summary,
			fmt.Sprintf(constant.TraceClientInitHooksFormat, constant.TraceClientInitAgentOptionClaude, claudeRegistered, len(constant.TraceClientClaudeHookEvents)),
			constant.TraceClientInitClaudeApprovalHint,
		)
	}
	_, _ = fmt.Fprintln(out, ui.SummaryPanel(summary...)) //nolint:errcheck // best-effort stdout
	return nil
}

// selectAgents 多选要注册 hook 的 agent：Space 勾选/取消，Enter 确认
func selectAgents(in io.Reader, out io.Writer) ([]string, error) {
	agents := []string{constant.TraceAgentCodex, constant.TraceAgentClaude}
	field := huh.NewMultiSelect[string]().
		Title(constant.TraceClientInitAgentSelectTitle).
		Options(
			huh.NewOption(constant.TraceClientInitAgentOptionCodex, constant.TraceAgentCodex),
			huh.NewOption(constant.TraceClientInitAgentOptionClaude, constant.TraceAgentClaude),
		).
		Validate(func(selected []string) error {
			if len(selected) == 0 {
				return ierr.New(ierr.ErrValidation, constant.TraceClientInitAgentRequired)
			}
			return nil
		}).
		Value(&agents)
	form := huh.NewForm(huh.NewGroup(field)).
		WithInput(in).
		WithOutput(out).
		WithTheme(ui.HuhTheme()).
		WithShowHelp(false)
	if err := form.Run(); err != nil {
		return nil, err
	}
	return agents, nil
}

// terminalIO 解析交互终端：stdin 是 TTY 直接用；否则打开 /dev/tty（curl|sh 场景）
func terminalIO(in io.Reader) (io.Reader, io.Writer, func(), error) {
	if file, ok := in.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		return in, os.Stdout, func() {}, nil
	}
	tty, err := os.OpenFile(constant.TraceClientDevTTYPath, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, nil, ierr.New(ierr.ErrValidation, constant.TraceClientInitNonInteractiveMessage)
	}
	return tty, tty, func() { _ = tty.Close() }, nil //nolint:errcheck // best-effort close
}

// ExecutablePath 返回当前可执行文件的绝对路径（解析符号链接）
func ExecutablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "resolve executable path")
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "resolve executable symlinks")
	}
	return resolved, nil
}
