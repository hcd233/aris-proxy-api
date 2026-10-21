// Package endpoint_visibility 验证端点可见性枚举的合法性、默认值与公开判定。
package endpoint_visibility

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func TestIsValid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   enum.EndpointVisibility
		want bool
	}{
		{"private", enum.EndpointVisibilityPrivate, true},
		{"public", enum.EndpointVisibilityPublic, true},
		{"empty", "", false},
		{"unknown", "shared", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.IsValid(); got != tc.want {
				t.Fatalf("IsValid(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()
	if got := enum.EndpointVisibility("").Normalize(); got != enum.EndpointVisibilityPrivate {
		t.Fatalf("empty must normalize to private, got %q", got)
	}
	if got := enum.EndpointVisibilityPublic.Normalize(); got != enum.EndpointVisibilityPublic {
		t.Fatalf("public must stay public, got %q", got)
	}
}

func TestIsPublic(t *testing.T) {
	t.Parallel()
	if !enum.EndpointVisibilityPublic.IsPublic() {
		t.Fatal("public must report public")
	}
	if enum.EndpointVisibilityPrivate.IsPublic() || enum.EndpointVisibility("").IsPublic() {
		t.Fatal("private/empty must not report public")
	}
}
