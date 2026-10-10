package model_command

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/model/command"
	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func TestCreateModelWithPriorityWeight(t *testing.T) {
	t.Parallel()
	epRepo := newScopedEndpointRepo()
	modelRepo := &recordingModelRepo{}
	h := command.NewCreateModelHandler(epRepo, modelRepo)

	scope := uint(101)
	_, err := h.Handle(context.Background(), port.CreateModelCommand{
		ScopeUserID:   &scope,
		EndpointID:    1,
		Alias:         "gpt-4",
		UpstreamModel: "gpt-4-0613",
		Capabilities:  []enum.InputModality{enum.InputModalityText},
		Priority:      2,
		Weight:        10,
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if modelRepo.gotModel.Priority() != 2 {
		t.Fatalf("Priority = %d, want 2", modelRepo.gotModel.Priority())
	}
	if modelRepo.gotModel.Weight() != 10 {
		t.Fatalf("Weight = %d, want 10", modelRepo.gotModel.Weight())
	}
}

func TestCreateModelSchedulingDefaults(t *testing.T) {
	t.Parallel()
	epRepo := newScopedEndpointRepo()
	modelRepo := &recordingModelRepo{}
	h := command.NewCreateModelHandler(epRepo, modelRepo)
	scope := uint(101)

	_, err := h.Handle(context.Background(), port.CreateModelCommand{
		ScopeUserID:   &scope,
		EndpointID:    1,
		Alias:         "gpt-4",
		UpstreamModel: "gpt-4-0613",
		Capabilities:  []enum.InputModality{enum.InputModalityText},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if modelRepo.gotModel.Priority() != 0 || modelRepo.gotModel.Weight() != 1 {
		t.Fatalf("defaults = (%d,%d), want (0,1)", modelRepo.gotModel.Priority(), modelRepo.gotModel.Weight())
	}
}

func TestCreateModelNegativeWeightRejected(t *testing.T) {
	t.Parallel()
	epRepo := newScopedEndpointRepo()
	modelRepo := &recordingModelRepo{}
	h := command.NewCreateModelHandler(epRepo, modelRepo)
	scope := uint(101)

	_, err := h.Handle(context.Background(), port.CreateModelCommand{
		ScopeUserID:   &scope,
		EndpointID:    1,
		Alias:         "gpt-5",
		UpstreamModel: "gpt-5",
		Capabilities:  []enum.InputModality{enum.InputModalityText},
		Weight:        -1,
	})
	if err == nil {
		t.Fatal("weight=-1 应被拒绝")
	}
}
