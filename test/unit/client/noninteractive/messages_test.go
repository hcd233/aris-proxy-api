package noninteractive

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// TestMessagesNameTheirCommand 回归：三条交互命令曾共用同一个常量，导致 aris init 无 TTY
// 时报出 "trace init"，与用户实际执行的命令不符。
func TestMessagesNameTheirCommand(t *testing.T) {
	t.Parallel()
	cases := []struct {
		command string
		message string
	}{
		{command: "aris init", message: constant.ArisClientInitNonInteractiveMessage},
		{command: "aris trace install", message: constant.ArisClientInstallNonInteractiveMessage},
		{command: "aris model export", message: constant.ClientModelExportNonInteractiveMessage},
	}
	seen := make(map[string]string, len(cases))
	for _, tc := range cases {
		if !strings.Contains(tc.message, tc.command) {
			t.Errorf("%q should name command %q", tc.message, tc.command)
		}
		if prev, ok := seen[tc.message]; ok {
			t.Errorf("%q and %q share the same message", prev, tc.command)
		}
		seen[tc.message] = tc.command
	}
}
