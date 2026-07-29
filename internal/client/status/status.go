package status

import (
	"context"
	"io"
	"net/http"
	"os"

	"github.com/hcd233/aris-proxy-api/internal/client/trace"
	"github.com/hcd233/aris-proxy-api/internal/client/ui"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// StatusOptions aris status 运行参数
type StatusOptions struct {
	Paths      trace.Paths
	In         io.Reader
	Out        io.Writer
	JSON       bool
	HTTPClient *http.Client
}

// RunStatus 执行状态检查并渲染面板（--json 时跳过 spinner 直接输出）
func RunStatus(ctx context.Context, opts StatusOptions) error {
	paths := opts.Paths
	if paths.Root == "" {
		resolved, err := trace.DefaultPaths()
		if err != nil {
			return err
		}
		paths = resolved
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	in := opts.In
	if in == nil {
		in = os.Stdin
	}

	collect := func() *Report { return Collect(ctx, paths, opts.HTTPClient) }
	if opts.JSON {
		return RenderJSON(out, collect())
	}
	var report *Report
	if err := ui.RunWithSpinner(in, out, constant.ClientUIStatusChecking, func() error {
		report = collect()
		return nil
	}); err != nil {
		return err
	}
	return Render(out, report)
}
