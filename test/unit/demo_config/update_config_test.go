// Package demo_config Demo 配置应用层的单元测试
package demo_config

import (
	"context"
	"errors"
	"testing"

	democommand "github.com/hcd233/aris-proxy-api/internal/application/demo/command"
	"github.com/hcd233/aris-proxy-api/internal/application/demo/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// fakeConfigRepo 内存版 DemoConfigRepository
type fakeConfigRepo struct {
	entity *port.DemoConfigEntity
}

func (r *fakeConfigRepo) Get(ctx context.Context) (*port.DemoConfigEntity, error) {
	if r.entity == nil {
		return &port.DemoConfigEntity{LoginEnabled: false, Modules: []enum.DemoModule{}}, nil
	}
	return r.entity, nil
}

func (r *fakeConfigRepo) Save(ctx context.Context, entity *port.DemoConfigEntity) error {
	r.entity = entity
	return nil
}

func boolPtr(v bool) *bool { return &v }

func TestUpdateDemoConfig_MergesPartialFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeConfigRepo{entity: &port.DemoConfigEntity{
		LoginEnabled: false,
		Modules:      []enum.DemoModule{enum.DemoModuleSessions},
	}}
	handler := democommand.NewUpdateDemoConfigHandler(repo)

	view, err := handler.Handle(ctx, port.UpdateDemoConfigCommand{LoginEnabled: boolPtr(true)})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !view.LoginEnabled {
		t.Fatal("expected loginEnabled true")
	}
	if len(view.Modules) != 1 || view.Modules[0] != enum.DemoModuleSessions {
		t.Fatalf("expected modules unchanged, got %v", view.Modules)
	}
}

func TestUpdateDemoConfig_RejectsInvalidModule(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	handler := democommand.NewUpdateDemoConfigHandler(&fakeConfigRepo{})

	_, err := handler.Handle(ctx, port.UpdateDemoConfigCommand{Modules: []enum.DemoModule{"sessions", "nonexistent"}})
	if !errors.Is(err, ierr.ErrValidation) {
		t.Fatalf("expected ErrValidation for invalid module, got %v", err)
	}
}

func TestUpdateDemoConfig_DeduplicatesModules(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := &fakeConfigRepo{}
	handler := democommand.NewUpdateDemoConfigHandler(repo)

	view, err := handler.Handle(ctx, port.UpdateDemoConfigCommand{
		Modules: []enum.DemoModule{enum.DemoModuleSessions, enum.DemoModuleSessions, enum.DemoModuleAudit},
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if len(view.Modules) != 2 {
		t.Fatalf("expected 2 deduplicated modules, got %v", view.Modules)
	}
}
