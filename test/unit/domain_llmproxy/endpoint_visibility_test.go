package domain_llmproxy

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/domain/llmproxy/aggregate"
)

// mustEndpoint 构造一个合法的最小端点聚合根（可见性走默认值）。
func mustEndpoint(t *testing.T) *aggregate.Endpoint {
	t.Helper()
	return aggregate.RehydrateEndpoint(1, "ep", "https://o.example.com", "", "k", true, false, false, false, enum.EndpointAuthAPIKey)
}

func TestEndpoint_DefaultsToPrivate(t *testing.T) {
	t.Parallel()
	ep, err := aggregate.CreateEndpoint(0, "ep", "https://o.example.com", "", "k", true, false, false, false, enum.EndpointAuthAPIKey)
	if err != nil {
		t.Fatal(err)
	}
	if ep.Visibility() != enum.EndpointVisibilityPrivate {
		t.Fatalf("new endpoint visibility = %q, want private", ep.Visibility())
	}
}

func TestEndpoint_SetVisibilityRejectsUnknown(t *testing.T) {
	t.Parallel()
	ep := mustEndpoint(t)
	if err := ep.SetVisibility("shared"); err == nil {
		t.Fatal("unknown visibility must be rejected")
	}
	if err := ep.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatalf("set public: %v", err)
	}
	if ep.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatalf("visibility = %q, want public", ep.Visibility())
	}
}

func TestEndpoint_RestoreVisibilityNormalizesInvalid(t *testing.T) {
	t.Parallel()
	ep := mustEndpoint(t)
	ep.RestoreVisibility("garbage")
	if ep.Visibility() != enum.EndpointVisibilityPrivate {
		t.Fatalf("invalid restored visibility = %q, want private", ep.Visibility())
	}
	ep.RestoreVisibility(enum.EndpointVisibilityPublic)
	if ep.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatal("restore must keep valid value")
	}
}

func TestEndpoint_UpdatePreservesVisibility(t *testing.T) {
	t.Parallel()
	ep := mustEndpoint(t)
	if err := ep.SetVisibility(enum.EndpointVisibilityPublic); err != nil {
		t.Fatal(err)
	}
	name := "renamed"
	if err := ep.Update(&name, nil, nil, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if ep.Visibility() != enum.EndpointVisibilityPublic {
		t.Fatal("Update must not reset visibility")
	}
}
