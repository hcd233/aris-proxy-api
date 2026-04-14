package truncate_field

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type fieldCase struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	MaxLen int    `json:"max_len"`
	Want   string `json:"want"`
}

type mapCase struct {
	Name   string         `json:"name"`
	Input  map[string]any `json:"input"`
	MaxLen int            `json:"max_len"`
	Want   map[string]any `json:"want"`
}

func loadFieldCases(t *testing.T) []fieldCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []fieldCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

func loadMapCases(t *testing.T) []mapCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/map_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/map_cases.json: %v", err)
	}
	var cases []mapCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/map_cases.json: %v", err)
	}
	return cases
}

func findFieldCase(t *testing.T, cases []fieldCase, name string) fieldCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("field case %q not found", name)
	return fieldCase{}
}

func findMapCase(t *testing.T, cases []mapCase, name string) mapCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("map case %q not found", name)
	return mapCase{}
}

func TestTruncateFieldValue_ShortString(t *testing.T) {
	cases := loadFieldCases(t)
	tc := findFieldCase(t, cases, "short_string_unchanged")

	got := util.TruncateFieldValue(tc.Input, tc.MaxLen)
	if got != tc.Want {
		t.Errorf("TruncateFieldValue(%q, %d) = %q, want %q", tc.Input, tc.MaxLen, got, tc.Want)
	}
}

func TestTruncateFieldValue_ExactLength(t *testing.T) {
	cases := loadFieldCases(t)
	tc := findFieldCase(t, cases, "exact_length_unchanged")

	got := util.TruncateFieldValue(tc.Input, tc.MaxLen)
	if got != tc.Want {
		t.Errorf("TruncateFieldValue(%q, %d) = %q, want %q", tc.Input, tc.MaxLen, got, tc.Want)
	}
}

func TestTruncateFieldValue_LongString(t *testing.T) {
	cases := loadFieldCases(t)
	tc := findFieldCase(t, cases, "long_string_truncated")

	got := util.TruncateFieldValue(tc.Input, tc.MaxLen)
	if got != tc.Want {
		t.Errorf("TruncateFieldValue(%q, %d) = %q, want %q", tc.Input, tc.MaxLen, got, tc.Want)
	}
}

func TestTruncateFieldValue_EmptyString(t *testing.T) {
	cases := loadFieldCases(t)
	tc := findFieldCase(t, cases, "empty_string")

	got := util.TruncateFieldValue(tc.Input, tc.MaxLen)
	if got != tc.Want {
		t.Errorf("TruncateFieldValue(%q, %d) = %q, want %q", tc.Input, tc.MaxLen, got, tc.Want)
	}
}

func TestTruncateFieldValue_SingleCharOver(t *testing.T) {
	cases := loadFieldCases(t)
	tc := findFieldCase(t, cases, "single_char_over")

	got := util.TruncateFieldValue(tc.Input, tc.MaxLen)
	if got != tc.Want {
		t.Errorf("TruncateFieldValue(%q, %d) = %q, want %q", tc.Input, tc.MaxLen, got, tc.Want)
	}
}

func TestTruncateFieldValue_UnicodeString(t *testing.T) {
	cases := loadFieldCases(t)
	tc := findFieldCase(t, cases, "unicode_string_truncated")

	got := util.TruncateFieldValue(tc.Input, tc.MaxLen)
	if got != tc.Want {
		t.Errorf("TruncateFieldValue(%q, %d) = %q, want %q", tc.Input, tc.MaxLen, got, tc.Want)
	}
}

func TestTruncateMapValues_FlatMapShortValues(t *testing.T) {
	cases := loadMapCases(t)
	tc := findMapCase(t, cases, "flat_map_short_values")

	got := util.TruncateMapValues(tc.Input, tc.MaxLen)
	if got["key"] != tc.Want["key"] {
		t.Errorf("TruncateMapValues() key=%q, got %v, want %v", "key", got["key"], tc.Want["key"])
	}
	if got["num"] != tc.Want["num"] {
		t.Errorf("TruncateMapValues() key=%q, got %v, want %v", "num", got["num"], tc.Want["num"])
	}
}

func TestTruncateMapValues_FlatMapLongString(t *testing.T) {
	cases := loadMapCases(t)
	tc := findMapCase(t, cases, "flat_map_long_string")

	got := util.TruncateMapValues(tc.Input, tc.MaxLen)
	if got["content"] != tc.Want["content"] {
		t.Errorf("TruncateMapValues() content mismatch:\n  got:  %q\n  want: %q", got["content"], tc.Want["content"])
	}
}

func TestTruncateMapValues_NestedMap(t *testing.T) {
	cases := loadMapCases(t)
	tc := findMapCase(t, cases, "nested_map_truncation")

	got := util.TruncateMapValues(tc.Input, tc.MaxLen)
	outer, ok := got["outer"].(map[string]any)
	if !ok {
		t.Fatalf("TruncateMapValues() outer is not map[string]any, got %T", got["outer"])
	}
	if outer["inner"] != "short" {
		t.Errorf("TruncateMapValues() inner=%q, want %q", outer["inner"], "short")
	}
	wantOuter := tc.Want["outer"].(map[string]any)
	if outer["long"] != wantOuter["long"] {
		t.Errorf("TruncateMapValues() long mismatch:\n  got:  %q\n  want: %q", outer["long"], wantOuter["long"])
	}
}

func TestTruncateMapValues_ArrayOfStrings(t *testing.T) {
	cases := loadMapCases(t)
	tc := findMapCase(t, cases, "array_of_strings_truncation")

	got := util.TruncateMapValues(tc.Input, tc.MaxLen)
	messages, ok := got["messages"].([]any)
	if !ok {
		t.Fatalf("TruncateMapValues() messages is not []any, got %T", got["messages"])
	}
	wantMessages := tc.Want["messages"].([]any)
	if messages[0] != wantMessages[0] {
		t.Errorf("TruncateMapValues() messages[0]=%q, want %q", messages[0], wantMessages[0])
	}
	if messages[1] != wantMessages[1] {
		t.Errorf("TruncateMapValues() messages[1]=%q, want %q", messages[1], wantMessages[1])
	}
}

func TestTruncateMapValues_MixedTypes(t *testing.T) {
	cases := loadMapCases(t)
	tc := findMapCase(t, cases, "mixed_types_preserved")

	got := util.TruncateMapValues(tc.Input, tc.MaxLen)
	if got["str"] != tc.Want["str"] {
		t.Errorf("TruncateMapValues() str=%q, want %q", got["str"], tc.Want["str"])
	}
	if got["num"] != tc.Want["num"] {
		t.Errorf("TruncateMapValues() num=%v, want %v", got["num"], tc.Want["num"])
	}
	if got["bool"] != tc.Want["bool"] {
		t.Errorf("TruncateMapValues() bool=%v, want %v", got["bool"], tc.Want["bool"])
	}
	if got["nil"] != tc.Want["nil"] {
		t.Errorf("TruncateMapValues() nil=%v, want %v", got["nil"], tc.Want["nil"])
	}
}

func TestTruncateMapValues_EmptyMap(t *testing.T) {
	cases := loadMapCases(t)
	tc := findMapCase(t, cases, "empty_map")

	got := util.TruncateMapValues(tc.Input, tc.MaxLen)
	if len(got) != 0 {
		t.Errorf("TruncateMapValues() on empty map returned %d entries, want 0", len(got))
	}
}
