package model_command

import (
	"context"
	"errors"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/command"
	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// TestCreateModel_RejectsMockUpstreamForNonAdmin 普通用户不得把模型指向 mock 上游。
//
// 回归：mock 装饰器直接在进程内生成响应，不经过 guard/bulkhead 并发隔离；
// 若任意用户都能用 upstream_model=mock-llm 命中，等于开放了一条绕过限流的通道。
func TestCreateModel_RejectsMockUpstreamForNonAdmin(t *testing.T) {
	t.Parallel()
	scope := uint(101)
	h := command.NewCreateModelHandler(newScopedEndpointRepo(), &recordingModelRepo{})

	_, err := h.Handle(context.Background(), port.CreateModelCommand{
		ScopeUserID:   &scope,
		EndpointID:    1,
		Alias:         "mock-llm",
		UpstreamModel: constant.MockUpstreamModel,
		Capabilities:  []enum.InputModality{enum.InputModalityText},
	})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
}

// TestCreateModel_AllowsMockUpstreamForAdmin admin（scope=nil）保留压测期配置 mock 的能力。
func TestCreateModel_AllowsMockUpstreamForAdmin(t *testing.T) {
	t.Parallel()
	repo := &recordingModelRepo{}
	h := command.NewCreateModelHandler(newScopedEndpointRepo(), repo)

	if _, err := h.Handle(context.Background(), port.CreateModelCommand{
		EndpointID:    2,
		Alias:         "mock-llm",
		UpstreamModel: constant.MockUpstreamModel,
		Capabilities:  []enum.InputModality{enum.InputModalityText},
	}); err != nil {
		t.Fatalf("admin create with mock upstream failed: %v", err)
	}
	if repo.gotModel == nil || repo.gotModel.UpstreamModel() != constant.MockUpstreamModel {
		t.Fatalf("mock upstream model not persisted: %+v", repo.gotModel)
	}
}

// staticModelRepo 返回固定 model 的仓储 fake（覆盖更新路径的 FindByID）。
type staticModelRepo struct {
	recordingModelRepo
	model *aggregate.Model
}

func (r *staticModelRepo) FindByID(context.Context, uint, *uint) (*aggregate.Model, error) {
	return r.model, nil
}

// TestUpdateModel_RejectsMockUpstreamForNonAdmin 更新路径同样不得开启 mock 上游。
func TestUpdateModel_RejectsMockUpstreamForNonAdmin(t *testing.T) {
	t.Parallel()
	model, err := aggregate.CreateModel(1, vo.EndpointAlias("cli-model"), "gpt-4", 1, true, 8192, 1024, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("create model aggregate: %v", err)
	}
	model.SetUserID(101)
	h := command.NewUpdateModelHandler(newScopedEndpointRepo(), &staticModelRepo{model: model})

	upstream := constant.MockUpstreamModel
	scope := uint(101)
	if _, err := h.Handle(context.Background(), port.UpdateModelCommand{
		ScopeUserID:   &scope,
		ID:            1,
		UpstreamModel: &upstream,
	}); !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	if model.UpstreamModel() == constant.MockUpstreamModel {
		t.Fatal("model must keep its upstream model after rejected update")
	}
}
