package context_util

import (
	"context"
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type ctxValueStringCase struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Function     string `json:"function"`
	Key          string `json:"key"`
	CtxValueType string `json:"ctxValueType"`
	CtxValue     any    `json:"ctxValue"`
	Expected     string `json:"expected"`
}

type extractMetadataCase struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Function    string                 `json:"function"`
	Metadata    *dto.AnthropicMetadata `json:"metadata"`
	ExpectedMap map[string]string      `json:"expectedMap"`
}

type rawCase struct {
	Name     string `json:"name"`
	Function string `json:"function"`
}

func loadRawCases(t *testing.T) []rawCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []rawCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func loadCtxValueStringCases(t *testing.T) []ctxValueStringCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []ctxValueStringCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	var filtered []ctxValueStringCase
	for _, c := range cases {
		if c.Function == "CtxValueString" {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func loadExtractMetadataCases(t *testing.T) []extractMetadataCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []extractMetadataCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	var filtered []extractMetadataCase
	for _, c := range cases {
		if c.Function == "ExtractAnthropicMetadata" {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func findCtxCase(t *testing.T, cases []ctxValueStringCase, name string) ctxValueStringCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return ctxValueStringCase{}
}

func findMetaCase(t *testing.T, cases []extractMetadataCase, name string) extractMetadataCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found", name)
	return extractMetadataCase{}
}

func TestCtxValueString(t *testing.T) {
	allCases := loadCtxValueStringCases(t)

	t.Run("existing string value", func(t *testing.T) {
		tc := findCtxCase(t, allCases, "ctx_value_string_exists")
		ctx := context.WithValue(context.Background(), tc.Key, "Mozilla/5.0")
		got := util.CtxValueString(ctx, tc.Key)
		if got != tc.Expected {
			t.Errorf("CtxValueString() = %q, want %q", got, tc.Expected)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		tc := findCtxCase(t, allCases, "ctx_value_string_missing_key")
		ctx := context.Background()
		got := util.CtxValueString(ctx, tc.Key)
		if got != tc.Expected {
			t.Errorf("CtxValueString() = %q, want %q", got, tc.Expected)
		}
	})

	t.Run("non-string value", func(t *testing.T) {
		tc := findCtxCase(t, allCases, "ctx_value_string_non_string_value")
		ctx := context.WithValue(context.Background(), tc.Key, 12345)
		got := util.CtxValueString(ctx, tc.Key)
		if got != tc.Expected {
			t.Errorf("CtxValueString() = %q, want %q", got, tc.Expected)
		}
	})

	t.Run("empty string value", func(t *testing.T) {
		tc := findCtxCase(t, allCases, "ctx_value_string_empty_value")
		ctx := context.WithValue(context.Background(), tc.Key, "")
		got := util.CtxValueString(ctx, tc.Key)
		if got != tc.Expected {
			t.Errorf("CtxValueString() = %q, want %q", got, tc.Expected)
		}
	})
}

func TestExtractAnthropicMetadata(t *testing.T) {
	allCases := loadExtractMetadataCases(t)

	t.Run("nil metadata", func(t *testing.T) {
		tc := findMetaCase(t, allCases, "extract_anthropic_metadata_nil")
		got := util.ExtractAnthropicMetadata(tc.Metadata)
		if got != nil {
			t.Errorf("ExtractAnthropicMetadata(nil) = %v, want nil", got)
		}
	})

	t.Run("metadata with user_id", func(t *testing.T) {
		tc := findMetaCase(t, allCases, "extract_anthropic_metadata_with_user_id")
		got := util.ExtractAnthropicMetadata(tc.Metadata)
		if got == nil {
			t.Fatalf("ExtractAnthropicMetadata() returned nil, want non-nil")
		}
		for k, v := range tc.ExpectedMap {
			if got[k] != v {
				t.Errorf("ExtractAnthropicMetadata()[%q] = %q, want %q", k, got[k], v)
			}
		}
		if len(got) != len(tc.ExpectedMap) {
			t.Errorf("ExtractAnthropicMetadata() length = %d, want %d", len(got), len(tc.ExpectedMap))
		}
	})

	t.Run("metadata with empty user_id", func(t *testing.T) {
		tc := findMetaCase(t, allCases, "extract_anthropic_metadata_empty_user_id")
		got := util.ExtractAnthropicMetadata(tc.Metadata)
		if got != nil {
			t.Errorf("ExtractAnthropicMetadata() = %v, want nil", got)
		}
	})
}
