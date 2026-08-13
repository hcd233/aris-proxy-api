// Package model_handler 验证 model handler 层字段透传。
//
// 背景（Major，2026-08-12 review 发现）：
//   - HandleCreateModel 构造 CreateModelCommand 时漏传 Capabilities，
//     前端创建表单勾选的 image 能力被静默丢弃，创建出的模型始终是默认 [text]。
//   - 修复：补齐 Capabilities 字段透传，并清理 `_ = result.ModelID` 死代码。
package model_handler

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/model"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/handler"
)

// captureCreateHandler 捕获 CreateModelCommand 的 mock。
type captureCreateHandler struct {
	last port.CreateModelCommand
}

func (c *captureCreateHandler) Handle(_ context.Context, cmd port.CreateModelCommand) (*port.CreateModelResult, error) {
	c.last = cmd
	return &port.CreateModelResult{ModelID: 1}, nil
}

func newModelHandlerWithCapture(t *testing.T) (handler.ModelHandler, *captureCreateHandler) {
	t.Helper()
	c := &captureCreateHandler{}
	h := handler.NewModelHandler(handler.ModelDependencies{
		Create: c,
		Update: &captureUpdateHandler{},
		Delete: &captureDeleteHandler{},
		List:   &captureListHandler{},
	})
	return h, c
}

type captureUpdateHandler struct{}

func (c *captureUpdateHandler) Handle(_ context.Context, cmd port.UpdateModelCommand) error {
	return nil
}

type captureDeleteHandler struct{}

func (c *captureDeleteHandler) Handle(_ context.Context, cmd port.DeleteModelCommand) error {
	return nil
}

type captureListHandler struct{}

func (c *captureListHandler) Handle(_ context.Context, _ port.ListModelsQuery) ([]*port.ModelView, *model.PageInfo, error) {
	return nil, nil, nil
}

// TestHandleCreateModel_PassesCapabilities
// 创建模型时 Capabilities 必须透传到 command（回归：曾静默丢弃）。
func TestHandleCreateModel_PassesCapabilities(t *testing.T) {
	t.Parallel()
	h, c := newModelHandlerWithCapture(t)

	req := &dto.CreateModelReq{Body: &dto.CreateModelReqBody{
		Alias:           "gpt-test",
		UpstreamModel:   "gpt-4o",
		EndpointID:      1,
		ContextLength:   128000,
		MaxOutputTokens: 64000,
		Capabilities:    []enum.InputModality{enum.InputModalityText, enum.InputModalityImage},
	}}

	ctx := context.WithValue(context.Background(), constant.CtxKeyUserID, uint(1))
	_, err := h.HandleCreateModel(ctx, req)
	if err != nil {
		t.Fatalf("HandleCreateModel failed: %v", err)
	}

	if len(c.last.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities passed through, got %v", c.last.Capabilities)
	}
	if c.last.Capabilities[0] != enum.InputModalityText || c.last.Capabilities[1] != enum.InputModalityImage {
		t.Fatalf("capabilities mismatch: %v", c.last.Capabilities)
	}
	// 其余字段必须一并透传（防后续遗漏）
	if c.last.Alias != "gpt-test" || c.last.UpstreamModel != "gpt-4o" || c.last.EndpointID != 1 {
		t.Fatalf("base fields not passed: %+v", c.last)
	}
}

// TestHandleCreateModel_EmptyCapabilitiesDefaultsLater
// 未提供 Capabilities 时透传空切片（默认值由 command 层兜底 [text]）。
func TestHandleCreateModel_EmptyCapabilitiesPassed(t *testing.T) {
	t.Parallel()
	h, c := newModelHandlerWithCapture(t)

	req := &dto.CreateModelReq{Body: &dto.CreateModelReqBody{
		Alias:         "gpt-test",
		UpstreamModel: "gpt-4o",
		EndpointID:    1,
	}}

	ctx := context.WithValue(context.Background(), constant.CtxKeyUserID, uint(1))
	if _, err := h.HandleCreateModel(ctx, req); err != nil {
		t.Fatalf("HandleCreateModel failed: %v", err)
	}
	// 未提供时透传 nil（command 层以 DefaultModelCapabilities=[text] 兜底），
	// 不阻塞创建流程即视为通过（回归护栏）。
	if len(c.last.Capabilities) != 0 {
		t.Fatalf("expected no capabilities passed, got %v", c.last.Capabilities)
	}
}
