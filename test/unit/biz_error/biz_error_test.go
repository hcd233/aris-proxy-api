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

// TestErrorStatusCode 验证业务错误码 → HTTP 状态码映射函数本身。
//
// 注意：该映射仅保留作语义参考（如中间件显式指定状态码），
// 管理 API 统一错误契约下 huma 错误响应恒为 HTTP 200，见 TestUnifiedHTTP200Contract。
//
//	@author centonhuang
//	@update 2026-08-16 15:00:00
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

// TestNewHumaBizError_JSONStructure 验证 handler 业务错误序列化为顶层 {"error":{code,message}}，
// 且 HTTP 状态码恒为 200（统一错误契约，错误语义由 error 体承载）。
//
//	@author centonhuang
//	@update 2026-08-16 15:00:00
func TestNewHumaBizError_JSONStructure(t *testing.T) {
	t.Parallel()
	err := apiutil.NewHumaBizError(context.Background(), ierr.New(ierr.ErrDataNotExists, "session missing"), ierr.ErrInternal.BizError())

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("NewHumaBizError result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusOK {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusOK)
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

// TestNewHumaBizError_Fallback 验证非 InternalError 时使用 fallback 业务错误，
// HTTP 状态码仍为 200（统一错误契约）。
//
//	@author centonhuang
//	@update 2026-08-16 15:00:00
func TestNewHumaBizError_Fallback(t *testing.T) {
	t.Parallel()
	err := apiutil.NewHumaBizError(context.Background(), context.DeadlineExceeded, ierr.ErrInternal.BizError())

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusOK {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusOK)
	}
}

// TestNewHumaBizError_Unauthorized 验证认证错误序列化结构正确，HTTP 状态码恒为 200。
//
//	@author centonhuang
//	@update 2026-08-16 15:00:00
func TestNewHumaBizError_Unauthorized(t *testing.T) {
	t.Parallel()
	err := apiutil.NewHumaBizError(context.Background(), ierr.New(ierr.ErrUnauthorized, "token invalid"), ierr.ErrInternal.BizError())
	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusOK {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusOK)
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

// TestFrameworkError 验证 huma 框架错误（校验失败 422 等）输出统一结构，
// HTTP 状态码恒为 200（统一错误契约）。
//
//	@author centonhuang
//	@update 2026-08-16 15:00:00
func TestFrameworkError(t *testing.T) {
	t.Parallel()
	err := apiutil.FrameworkError(http.StatusUnprocessableEntity, "validation failed")

	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("FrameworkError result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusOK {
		t.Errorf("GetStatus() = %d, want %d", got, http.StatusOK)
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

// TestUnifiedHTTP200Contract 验证统一错误契约：管理 API 所有错误路径
// （业务错误、框架错误）HTTP 状态码恒为 200，错误语义由 error 体承载。
//
//	@author centonhuang
//	@update 2026-08-16 15:00:00
func TestUnifiedHTTP200Contract(t *testing.T) {
	t.Parallel()
	// 业务错误：数据不存在 → 200 + error 体
	err := apiutil.NewHumaBizError(context.Background(), ierr.New(ierr.ErrDataNotExists, "missing"), ierr.ErrInternal.BizError())
	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("result does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusOK {
		t.Errorf("biz error GetStatus() = %d, want %d", got, http.StatusOK)
	}

	// 框架错误：路由未匹配 404 → 200 + error 体
	fwErr := huma.NewError(http.StatusNotFound, "route not found")
	if !errors.As(fwErr, &statusErr) {
		t.Fatalf("framework error does not implement huma.StatusError")
	}
	if got := statusErr.GetStatus(); got != http.StatusOK {
		t.Errorf("framework error GetStatus() = %d, want %d", got, http.StatusOK)
	}
}
