package rate_limiter

import (
	"context"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

func TestContextKeyTypeMatch(t *testing.T) {
	key := enum.CtxKey("apiKeyID")
	expectedValue := uint(42)

	ctx := context.WithValue(context.Background(), key, expectedValue)

	t.Run("enum.CtxKey lookup finds value", func(t *testing.T) {
		got := ctx.Value(key)
		if got == nil {
			t.Fatal("context.Value(enum.CtxKey) returned nil, expected value to be found")
		}
		if v, ok := got.(uint); !ok || v != expectedValue {
			t.Fatalf("context.Value(enum.CtxKey) = %v, want %v", got, expectedValue)
		}
	})

	t.Run("string lookup does NOT find value (regression for rate limiter bug)", func(t *testing.T) {
		got := ctx.Value("apiKeyID")
		if got != nil {
			t.Fatalf("context.Value(string) = %v, expected nil (different type from enum.CtxKey)", got)
		}
	})
}
