package llmproxy_usecase

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/usecase"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
)

func TestCanSwitchEndpoint(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"连接错误", &model.UpstreamConnectionError{Cause: ierr.New(ierr.ErrProxyRequest, "dial")}, true},
		{"上游 5xx", &model.UpstreamError{StatusCode: 502}, true},
		{"上游 429", &model.UpstreamError{StatusCode: 429}, true},
		{"上游 400 不可切换", &model.UpstreamError{StatusCode: 400}, false},
		{"熔断打开", &model.CircuitOpenError{}, true},
		{"信号量满载", &model.BulkheadFullError{}, true},
		{"nil 不可切换", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := usecase.CanSwitchEndpoint(tc.err); got != tc.want {
				t.Fatalf("CanSwitchEndpoint = %v, want %v", got, tc.want)
			}
		})
	}
}
