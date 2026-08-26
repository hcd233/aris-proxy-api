// Package setup 实现 aris init 交互式配置向导（huh 表单 + spinner 异步反馈）。
package setup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/hcd233/aris-proxy-api/internal/client/api"
	trace "github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// InitOptions aris init 运行参数
type InitOptions struct {
	Host       string
	Paths      trace.Paths
	In         io.Reader
	Out        io.Writer
	HTTPClient *http.Client
}

// RunInit 执行四步配置向导：连接检查 → 选择 agent → API Key → 配置 hooks
func RunInit(ctx context.Context, opts InitOptions) error {
	paths := opts.Paths
	if paths.Root == "" {
		resolved, err := trace.DefaultPaths()
		if err != nil {
			return err
		}
		paths = resolved
	}
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	ttyIn, ttyOut, cleanup, err := terminalIO(in)
	if err != nil {
		return err
	}
	defer cleanup()

	store := trace.NewConfigStore(paths)
	existing, _ := store.Load(ctx) //nolint:errcheck // best-effort：读不到按未初始化处理

	host := opts.Host
	if host == "" {
		host = existing.Host
	}
	if host == "" {
		if host, err = promptHost(ttyIn, ttyOut); err != nil {
			return err
		}
	}
	if host, err = NormalizeHost(host); err != nil {
		return err
	}

	printStep(out, 1, constant.TraceClientInitTitleConnect)
	if err := checkHealthWithRetry(ctx, api.New(host, "", opts.HTTPClient), ttyIn, ttyOut, out, host); err != nil {
		return err
	}

	printStep(out, 2, constant.TraceClientInitTitleAPIKey)
	apiKey := os.Getenv(constant.TraceClientAPIKeyEnv)
	if apiKey == "" {
		if apiKey, err = promptAPIKey(ttyIn, ttyOut, existing.APIKey); err != nil {
			return err
		}
		apiKey = ResolveAPIKey(apiKey, existing.APIKey)
	}
	if err := checkAPIKeyWithRetry(ctx, api.New(host, apiKey, opts.HTTPClient), ttyIn, ttyOut, out); err != nil {
		return err
	}

	printStep(out, 3, constant.TraceClientInitTitleSave)
	if err := ui.RunWithSpinner(ttyIn, ttyOut, constant.TraceClientInitSavingConfig, func() error {
		return store.Save(ctx, trace.Config{Host: host, APIKey: apiKey})
	}); err != nil {
		return err
	}

	printLine(out, "", ui.SummaryPanel(
		constant.TraceClientInitDone,
		fmt.Sprintf(constant.TraceClientInitConfigFormat, paths.ConfigFile()),
		constant.TraceClientNextHintTraceInstall,
		constant.TraceClientNextHintModelExport,
	))
	return nil
}

func promptHost(in io.Reader, out io.Writer) (string, error) {
	var host string
	field := huh.NewInput().
		Title(constant.TraceClientInitHostPrompt).
		Placeholder(constant.TraceClientInitHostPlaceholder).
		Value(&host)
	if err := newForm(in, out, field).Run(); err != nil {
		return "", err
	}
	return host, nil
}

func promptAPIKey(in io.Reader, out io.Writer, existing string) (string, error) {
	key := ""
	input := huh.NewInput().
		Title(constant.TraceClientInitAPIKeyTitle).
		EchoMode(huh.EchoModePassword).
		Value(&key).
		Validate(func(s string) error {
			if s == "" && existing == "" {
				return ierr.New(ierr.ErrValidation, constant.TraceClientInitMissingAPIKeyMessage)
			}
			return nil
		})
	if existing != "" {
		input.Description(constant.TraceClientInitKeepAPIKeyHint)
	}
	if err := newForm(in, out, input).Run(); err != nil {
		return "", err
	}
	return key, nil
}

func checkHealthWithRetry(ctx context.Context, client *api.Client, ttyIn io.Reader, ttyOut, out io.Writer, host string) error {
	for {
		var latency time.Duration
		err := ui.RunWithSpinner(ttyIn, ttyOut, fmt.Sprintf(constant.TraceClientInitConnectingFormat, host), func() error {
			var checkErr error
			latency, checkErr = client.CheckHealth(ctx)
			return checkErr
		})
		if err == nil {
			printLine(out, ui.CheckRowOK(host, fmt.Sprintf(constant.TraceClientReachableFormat, latency.Round(time.Millisecond))))
			return nil
		}
		printLine(out, ui.CheckRowFail(host, err.Error()))
		retry, confirmErr := confirm(ttyIn, ttyOut, constant.TraceClientInitRetryPrompt)
		if confirmErr != nil {
			return confirmErr
		}
		if !retry {
			return err
		}
	}
}

func checkAPIKeyWithRetry(ctx context.Context, client *api.Client, ttyIn io.Reader, ttyOut, out io.Writer) error {
	for {
		err := ui.RunWithSpinner(ttyIn, ttyOut, constant.TraceClientInitValidatingKey, func() error {
			return client.CheckAPIKey(ctx)
		})
		if err == nil {
			printLine(out, ui.CheckRowOK(constant.TraceClientInitAPIKeyTitle, ""))
			return nil
		}
		printLine(out, ui.CheckRowFail(constant.TraceClientInitAPIKeyFailed, err.Error()))
		retry, confirmErr := confirm(ttyIn, ttyOut, constant.TraceClientInitAPIKeyRetryPrompt)
		if confirmErr != nil {
			return confirmErr
		}
		if !retry {
			return err
		}
	}
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

// NormalizeHost 归一化服务器地址：去空白与尾部斜杠，要求 http(s):// 前缀
func NormalizeHost(raw string) (string, error) {
	host := strings.TrimSpace(raw)
	if host == "" {
		return "", ierr.New(ierr.ErrValidation, constant.TraceClientInitHostRequiredMessage)
	}
	httpPrefix := constant.TraceClientSchemeHTTP + "://"
	httpsPrefix := constant.TraceClientSchemeHTTPS + "://"
	if !strings.HasPrefix(host, httpPrefix) && !strings.HasPrefix(host, httpsPrefix) {
		return "", ierr.New(ierr.ErrValidation, constant.TraceClientInitHostSchemeMessage)
	}
	return strings.TrimRight(host, "/"), nil
}

// ResolveAPIKey 解析最终 API Key：新输入优先，空输入保留存量
func ResolveAPIKey(input, existing string) string {
	if input != "" {
		return input
	}
	return existing
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

func printStep(out io.Writer, step int, title string) {
	printLine(out, "", ui.StepHeader(step, constant.TraceClientInitSteps, title))
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

func confirm(in io.Reader, out io.Writer, title string) (bool, error) {
	value := true
	field := huh.NewConfirm().
		Title(title).
		Affirmative(constant.TraceClientInitContinueLabel).
		Negative(constant.TraceClientInitCancelLabel).
		Value(&value)
	if err := newForm(in, out, field).Run(); err != nil {
		return false, err
	}
	return value, nil
}
