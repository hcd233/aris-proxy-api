package modelcall_status

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/domain/modelcall/vo"
)

// TestCallStatusIsError 覆盖触发词标注与真实失败的判定边界：
// capture 命中写入的 errorMessage 只是标注，不算失败；200 的 in-band 失败仍算失败。
func TestCallStatusIsError(t *testing.T) {
	t.Parallel()

	captureRemark := constant.TriggerCaptureAuditRemark + ": `save`"
	tests := []struct {
		name      string
		status    int
		message   string
		wantError bool
	}{
		{name: "success", status: 200, message: "", wantError: false},
		{name: "capture annotation", status: 200, message: captureRemark, wantError: false},
		{name: "upstream http failure", status: 500, message: "upstream error", wantError: true},
		{name: "unknown failure", status: 0, message: "deny by trigger word: `bad`", wantError: true},
		{name: "in band failure on 200", status: 200, message: "response incomplete: max_tokens", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status := vo.NewCallStatus(tt.status, tt.message)
			if got := status.IsError(); got != tt.wantError {
				t.Fatalf("IsError() = %v, want %v", got, tt.wantError)
			}
			if got := status.IsSuccess(); got == tt.wantError {
				t.Fatalf("IsSuccess() = %v, want %v", got, !tt.wantError)
			}
		})
	}
}
