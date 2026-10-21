package model_command

import (
	"errors"
	"testing"

	"github.com/samber/lo"

	"github.com/hcd233/aris-proxy-api/internal/application/model/command"
	"github.com/hcd233/aris-proxy-api/internal/application/model/port"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/vo"
)

// publicEndpointRepo 构造「归属 101 的公开端点」仓储。
func publicEndpointRepo(t *testing.T) *scopedEndpointRepo {
	t.Helper()
	ep, err := aggregate.CreateEndpoint(1, "ep-public", "https://o.example.com", "https://a.example.com", "k", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatalf("CreateEndpoint: %v", err)
	}
	ep.SetUserID(101)
	ep.RestoreVisibility(enum.EndpointVisibilityPublic)
	return &scopedEndpointRepo{byID: map[uint]*aggregate.Endpoint{1: ep}}
}

func modelOnEndpoint(t *testing.T, modelID, endpointID uint) *aggregate.Model {
	t.Helper()
	m, err := aggregate.CreateModel(modelID, vo.EndpointAlias("gpt-x"), "up-x", endpointID, true, 128000, 64000, []enum.InputModality{enum.InputModalityText})
	if err != nil {
		t.Fatalf("CreateModel: %v", err)
	}
	m.SetUserID(101)
	return m
}

func TestCreateModel_NonAdminCannotWritePublicEndpointModel(t *testing.T) {
	t.Parallel()
	endpointRepo := publicEndpointRepo(t)
	modelRepo := &recordingModelRepo{}
	handler := command.NewCreateModelHandler(endpointRepo, modelRepo)

	_, err := handler.Handle(t.Context(), port.CreateModelCommand{
		ScopeUserID:     lo.ToPtr(uint(101)),
		IsAdmin:         false,
		Alias:           "gpt-x",
		UpstreamModel:   "up-x",
		EndpointID:      1,
		Capabilities:    []enum.InputModality{enum.InputModalityText},
		ContextLength:   128000,
		MaxOutputTokens: 64000,
	})
	if !errors.Is(err, ierr.ErrUnauthorized) {
		t.Fatalf("non-admin create on public endpoint: err = %v, want ErrUnauthorized", err)
	}
	if modelRepo.gotModel != nil {
		t.Fatal("repo must not be written when guard rejects")
	}
}

func TestCreateModel_AdminCanWritePublicEndpointModel(t *testing.T) {
	t.Parallel()
	endpointRepo := publicEndpointRepo(t)
	modelRepo := &recordingModelRepo{}
	handler := command.NewCreateModelHandler(endpointRepo, modelRepo)

	if _, err := handler.Handle(t.Context(), port.CreateModelCommand{
		IsAdmin:         true,
		Alias:           "gpt-x",
		UpstreamModel:   "up-x",
		EndpointID:      1,
		Capabilities:    []enum.InputModality{enum.InputModalityText},
		ContextLength:   128000,
		MaxOutputTokens: 64000,
	}); err != nil {
		t.Fatalf("admin create on public endpoint: %v", err)
	}
	if modelRepo.gotModel == nil {
		t.Fatal("admin create must reach repo")
	}
}

func TestUpdateModel_NonAdminCannotUpdatePublicEndpointModel(t *testing.T) {
	t.Parallel()
	endpointRepo := publicEndpointRepo(t)
	modelRepo := &recordingModelRepo{byID: map[uint]*aggregate.Model{7: modelOnEndpoint(t, 7, 1)}}
	handler := command.NewUpdateModelHandler(endpointRepo, modelRepo)

	alias := "hijack"
	_, err := handler.Handle(t.Context(), port.UpdateModelCommand{
		ScopeUserID: lo.ToPtr(uint(101)),
		IsAdmin:     false,
		ID:          7,
		Alias:       &alias,
	})
	if !errors.Is(err, ierr.ErrUnauthorized) {
		t.Fatalf("non-admin update on public endpoint model: err = %v, want ErrUnauthorized", err)
	}
}

func TestDeleteModel_NonAdminCannotDeletePublicEndpointModel(t *testing.T) {
	t.Parallel()
	endpointRepo := publicEndpointRepo(t)
	modelRepo := &recordingModelRepo{byID: map[uint]*aggregate.Model{7: modelOnEndpoint(t, 7, 1)}}
	handler := command.NewDeleteModelHandler(modelRepo, endpointRepo)

	err := handler.Handle(t.Context(), port.DeleteModelCommand{
		ScopeUserID: lo.ToPtr(uint(101)),
		IsAdmin:     false,
		ModelID:     7,
	})
	if !errors.Is(err, ierr.ErrUnauthorized) {
		t.Fatalf("non-admin delete on public endpoint model: err = %v, want ErrUnauthorized", err)
	}
}
