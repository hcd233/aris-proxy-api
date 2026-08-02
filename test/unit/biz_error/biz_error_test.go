package biz_error

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/danielgtaylor/huma/v2"

	apiutil "github.com/hcd233/aris-proxy-api/internal/api/util"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/common/model"

	// 触发 internal/api 包的 init()，验证其中对 huma.NewError 的全局替换。
	_ "github.com/hcd233/aris-proxy-api/internal/api"
)

// TestErrorStatusCode 验证业务错误码 → HTTP 状态码映射。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestErrorStatusCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code int
		want int
	}{
		{10000, http.StatusInternalServerError}, // 内部错误兜底
		{10001, http.StatusUnauthorized},
		{10002, http.StatusForbidden},
		{10003, http.StatusNotFound},
		{10004, http.StatusConflict},
		{10005, http.StatusTooManyRequests},
		{10006, http.StatusBadRequest},
		{10007, http.StatusTooManyRequests}, // 配额不足
		{10008, http.StatusTooManyRequests}, // 配额超限
		{10009, http.StatusLocked},
		{10010, http.StatusBadRequest},          // 内容拦截
		{99999, http.StatusInternalServerError}, // 未识别码兜底
	}
	for _, c := range cases {
		got := model.NewError(c.code, "msg").StatusCode()
		if got != c.want {
			t.Errorf("code %d: StatusCode() = %d, want %d", c.code, got, c.want)
		}
	}
}

// TestNewHumaBizError_JSONStructure 验证 handler 业务错误序列化为顶层 {"error":{code,message}}。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestNewHumaBizError_JSONStructure(t *testing.T) {
	t.Parallel()
	err := apiutil.NewHumaBizError(context.Background(), ierr.New(ierr.ErrDataNotExists, "session missing"), ierr.ErrInternal.BizError())

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("NewHumaBizError result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusNotFound {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusNotFound)
	}

	raw, marshalErr := sonic.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}
	got := string(raw)
	want := `{"error":{"code":10003,"message":"Data Not Found"}}`
	if got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

// TestNewHumaBizError_Fallback 验证非 InternalError 时使用 fallback 业务错误。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestNewHumaBizError_Fallback(t *testing.T) {
	t.Parallel()
	err := apiutil.NewHumaBizError(context.Background(), context.DeadlineExceeded, ierr.ErrInternal.BizError())

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusInternalServerError {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusInternalServerError)
	}
}

// TestNewHumaBizError_Unauthorized 验证认证错误映射为 401。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestNewHumaBizError_Unauthorized(t *testing.T) {
	t.Parallel()
	err := apiutil.NewHumaBizError(context.Background(), ierr.New(ierr.ErrUnauthorized, "token invalid"), ierr.ErrInternal.BizError())
	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusUnauthorized {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusUnauthorized)
	}
}

// TestHumaNewErrorReplaced 验证 api 包 init 已全局替换 huma.NewError，
// 框架错误（404 路由未匹配、422 校验等）同样输出顶层 {"error":{code,message}}。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestHumaNewErrorReplaced(t *testing.T) {
	t.Parallel()
	// 导入 internal/api 触发其 init() 中对 huma.NewError 的替换。
	err := huma.NewError(http.StatusNotFound, "route not found")
	raw, marshalErr := sonic.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}
	got := string(raw)
	want := `{"error":{"code":10003,"message":"route not found"}}`
	if got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

// TestFrameworkError 验证 huma 框架错误（校验失败 422 等）也输出统一结构。
//
//	@author centonhuang
//	@update 2026-08-06 10:00:00
func TestFrameworkError(t *testing.T) {
	t.Parallel()
	err := apiutil.FrameworkError(http.StatusUnprocessableEntity, "validation failed")

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("FrameworkError result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusUnprocessableEntity {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusUnprocessableEntity)
	}

	raw, marshalErr := sonic.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("marshal error: %v", marshalErr)
	}
	got := string(raw)
	want := `{"error":{"code":10006,"message":"validation failed"}}`
	if got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}
