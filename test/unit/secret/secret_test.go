package secret

import (
	"testing"

	commonutil "github.com/hcd233/aris-proxy-api/internal/common/util"
)

func TestMaskIdentity(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "非空身份返回固定占位符", in: "alice@example.com", want: "***"},
		{name: "空身份返回空", in: "", want: ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := commonutil.MaskIdentity(tc.in); got != tc.want {
				t.Fatalf("MaskIdentity(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMaskSecret(t *testing.T) {
	t.Parallel()
	const in = "sk-1234567890abcd"
	const want = "sk-1***abcd"
	if got := commonutil.MaskSecret(in); got != want {
		t.Fatalf("MaskSecret(%q) = %q, want %q", in, got, want)
	}
}
