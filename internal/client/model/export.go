// Package model 实现 aris model export 交互式导出。
package model

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/hcd233/aris-proxy-api/internal/client/api"
	clienttrace "github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// ExportOptions aris model export 运行参数
type ExportOptions struct {
	In         io.Reader
	Out        io.Writer
	HTTPClient *http.Client
}

// RunExport 执行交互式模型导出：读配置 → 拉取模型 → 多选 → 选目标 → 写入
func RunExport(ctx context.Context, opts ExportOptions) error {
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	paths, pathErr := clienttrace.DefaultPaths()
	if pathErr != nil {
		return pathErr
	}
	store := clienttrace.NewConfigStore(paths)
	cfg, loadErr := store.Load(ctx) //nolint:errcheck // 读不到按未初始化处理
	if loadErr != nil || cfg.Host == "" || cfg.APIKey == "" {
		return ierr.New(ierr.ErrValidation, constant.ClientModelExportNeedInitMessage)
	}
	host := cfg.Host

	ttyIn, ttyOut, cleanup, ttyErr := terminalIO(in)
	if ttyErr != nil {
		return ttyErr
	}
	defer cleanup()

	var models []api.ClientModel
	fetchErr2 := ui.RunWithSpinner(ttyIn, ttyOut, constant.ClientModelExportFetchingMessage, func() error {
		var fetchErr error
		models, fetchErr = api.New(host, cfg.APIKey, opts.HTTPClient).ListModels(ctx)
		return fetchErr
	})
	if fetchErr2 != nil {
		return fetchErr2
	}
	if len(models) == 0 {
		return ierr.New(ierr.ErrValidation, constant.ClientModelExportEmptyModelsMessage)
	}

	selectedIdx := map[string]bool{}
	options := make([]huh.Option[string], 0, len(models))
	for _, m := range models {
		label := fmt.Sprintf(constant.ClientModelExportOptionFormat, m.Alias, m.UpstreamModel, m.ContextLength, m.MaxOutputTokens)
		options = append(options, huh.NewOption(label, m.Alias))
	}
	var selectedAliases []string
	field := huh.NewMultiSelect[string]().
		Title(constant.ClientModelExportSelectModelsTitle).
		Options(options...).
		Validate(func(v []string) error {
			if len(v) == 0 {
				return ierr.New(ierr.ErrValidation, constant.ClientModelExportModelsRequiredMessage)
			}
			return nil
		}).
		Value(&selectedAliases)
	if formErr := newForm(ttyIn, ttyOut, field).Run(); formErr != nil {
		return formErr
	}
	for _, a := range selectedAliases {
		selectedIdx[a] = true
	}

	var selected []TargetModel
	for _, m := range models {
		if selectedIdx[m.Alias] {
			selected = append(selected, TargetModel{
				Alias:           m.Alias,
				UpstreamModel:   m.UpstreamModel,
				ContextLength:   m.ContextLength,
				MaxOutputTokens: m.MaxOutputTokens,
				Capabilities:    m.Capabilities,
			})
		}
	}
	if formErr := newForm(ttyIn, ttyOut, field).Run(); formErr != nil {
		return formErr
	}

	targets := Targets()
	targetOptions := make([]huh.Option[string], 0, len(targets))
	for _, tgt := range targets {
		targetOptions = append(targetOptions, huh.NewOption(tgt.Label(), tgt.Key()))
	}
	var targetKey string
	selectField := huh.NewSelect[string]().
		Title(constant.ClientModelExportSelectTargetTitle).
		Options(targetOptions...).
		Value(&targetKey)
	if formErr := newForm(ttyIn, ttyOut, selectField).Run(); formErr != nil {
		return formErr
	}

	var target Target
	for _, tgt := range targets {
		if tgt.Key() == targetKey {
			target = tgt
			break
		}
	}

	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return ierr.Wrap(ierr.ErrInternal, homeErr, "resolve home directory")
	}
	path := target.ConfigPath(home)
	writeErr := ui.RunWithSpinner(ttyIn, ttyOut, fmt.Sprintf(constant.ClientModelExportWritingFormat, target.Label()), func() error {
		return target.Write(path, host, cfg.APIKey, selected)
	})
	if writeErr != nil {
		return writeErr
	}

	printLine(out, ui.SummaryPanel(
		fmt.Sprintf(constant.ClientModelExportDoneFormat, target.Label(), len(selected)),
		constant.ClientModelExportConfigPathPrefix+path,
		constant.ClientModelExportBackupHint,
	))
	return nil
}

func printLine(out io.Writer, lines ...string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(out, line) //nolint:errcheck // best-effort stdout
	}
}

func newForm(in io.Reader, out io.Writer, field huh.Field) *huh.Form {
	return huh.NewForm(huh.NewGroup(field)).
		WithInput(in).
		WithOutput(out).
		WithTheme(ui.HuhTheme()).
		WithShowHelp(false)
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
