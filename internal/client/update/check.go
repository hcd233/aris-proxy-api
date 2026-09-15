package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// CheckOptions 使用中更新检查参数；Out/BaseURL/HTTPClient 为空时使用默认实现
type CheckOptions struct {
	Current    string
	Out        io.Writer
	BaseURL    string
	HTTPClient *http.Client
}

// StartCheck 后台解析最新版本，返回等待并打印提示的函数；任何错误静默，等待上限 ArisClientUpdateNoticeWait
func StartCheck(ctx context.Context, opts CheckOptions) func() {
	out := opts.Out
	if out == nil {
		out = os.Stderr
	}
	hc := opts.HTTPClient
	if hc == nil {
		hc = newHTTPClient(constant.ArisClientUpdateCheckTimeout)
	}
	base, err := resolveBaseURL(opts.BaseURL)
	if err != nil {
		// 使用中检查不打扰用户：更新源非法时直接放弃本轮检查
		return func() {}
	}
	checkCtx, cancel := context.WithTimeout(ctx, constant.ArisClientUpdateCheckTimeout)

	result := make(chan string, 1)
	go func() {
		latest, err := Latest(checkCtx, base, hc)
		if err != nil {
			result <- ""
			return
		}
		result <- latest
	}()

	return func() {
		defer cancel()
		timer := time.NewTimer(constant.ArisClientUpdateNoticeWait)
		defer timer.Stop()
		select {
		case latest := <-result:
			if IsNewer(latest, opts.Current) {
				_, _ = fmt.Fprintf(out, constant.ArisClientUpdateAvailableFormat+"\n", latest, opts.Current) //nolint:errcheck // best-effort notice
			}
		case <-timer.C:
		}
	}
}
