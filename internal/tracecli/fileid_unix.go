//go:build darwin || linux

package tracecli

import (
	"fmt"
	"os"
	"syscall"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

func fileIdentity(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		constant.TraceClientFileIdentityFormat,
		uint64(uint32(stat.Dev)), //nolint:gosec // device number is always non-negative
		stat.Ino,
	)
}
