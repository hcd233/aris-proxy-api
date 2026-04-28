package tool_checksum

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	convvo "github.com/hcd233/aris-proxy-api/internal/domain/conversation/vo"
)

// toolChecksumCase represents a tool checksum test case loaded from fixtures
type toolChecksumCase struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Tools       []*convvo.UnifiedTool `json:"tools"`
	ExpectEqual bool                  `json:"expect_equal"`
}

// loadToolCases loads test cases from fixtures/cases.json
func loadToolCases(t *testing.T) []toolChecksumCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/cases.json")
	if err != nil {
		t.Fatalf("failed to read fixtures/cases.json: %v", err)
	}
	var cases []toolChecksumCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixtures/cases.json: %v", err)
	}
	return cases
}

// findCase finds a test case by name, fatals if not found
func findCase(t *testing.T, cases []toolChecksumCase, name string) toolChecksumCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("test case %q not found in fixtures", name)
	return toolChecksumCase{}
}

// TestComputeToolChecksum_PairComparison runs table-driven tests for all
// fixture cases that compare two tools and check checksum equality/inequality.
func TestComputeToolChecksum_PairComparison(t *testing.T) {
	allCases := loadToolCases(t)

	pairCases := []string{
		"same_tool_same_params",
		"same_name_different_description",
		"different_provider_same_schema",
		"different_name",
		"different_param_keys",
		"same_params_different_key_order",
		"many_params_same_tool",
		"same_top_level_keys_different_nested_params",
	}

	for _, caseName := range pairCases {
		tc := findCase(t, allCases, caseName)
		if len(tc.Tools) < 2 {
			t.Fatalf("case %q requires at least 2 tools, got %d", caseName, len(tc.Tools))
		}

		t.Run(caseName, func(t *testing.T) {
			checksum1 := convvo.ComputeToolChecksum(tc.Tools[0])
			checksum2 := convvo.ComputeToolChecksum(tc.Tools[1])

			t.Logf("description: %s", tc.Description)
			t.Logf("tool1: name=%s, checksum=%s", tc.Tools[0].Name, checksum1)
			t.Logf("tool2: name=%s, checksum=%s", tc.Tools[1].Name, checksum2)

			if tc.ExpectEqual && checksum1 != checksum2 {
				t.Errorf("ComputeToolChecksum() mismatch: got %s and %s, want same checksum", checksum1, checksum2)
			}
			if !tc.ExpectEqual && checksum1 == checksum2 {
				t.Errorf("ComputeToolChecksum() should produce different checksums, both got %s", checksum1)
			}
		})
	}
}

// TestComputeToolChecksum_SingleToolCases tests single-tool fixture cases
// that only need to verify the checksum is non-empty.
func TestComputeToolChecksum_SingleToolCases(t *testing.T) {
	allCases := loadToolCases(t)

	singleCases := []string{
		"nil_parameters",
		"empty_properties",
	}

	for _, caseName := range singleCases {
		tc := findCase(t, allCases, caseName)

		t.Run(caseName, func(t *testing.T) {
			checksum := convvo.ComputeToolChecksum(tc.Tools[0])
			t.Logf("tool: name=%s, checksum=%s", tc.Tools[0].Name, checksum)

			if checksum == "" {
				t.Errorf("ComputeToolChecksum() returned empty string for case %q", caseName)
			}
		})
	}
}

func TestComputeToolChecksum_Deterministic(t *testing.T) {
	allCases := loadToolCases(t)
	tc := findCase(t, allCases, "deterministic")

	checksums := make(map[string]bool)
	for i := range 100 {
		checksum := convvo.ComputeToolChecksum(tc.Tools[0])
		checksums[checksum] = true
		if i == 0 {
			t.Logf("first checksum: %s", checksum)
		}
	}

	t.Logf("unique checksums from 100 calls: %d", len(checksums))

	if len(checksums) != 1 {
		t.Errorf("ComputeToolChecksum() is not deterministic: got %d unique checksums from 100 calls", len(checksums))
	}
}

func TestComputeToolChecksum_NilVsEmptyProperties(t *testing.T) {
	allCases := loadToolCases(t)

	nilCase := findCase(t, allCases, "nil_vs_empty_properties_nil")
	emptyCase := findCase(t, allCases, "nil_vs_empty_properties_empty")

	checksum1 := convvo.ComputeToolChecksum(nilCase.Tools[0])
	checksum2 := convvo.ComputeToolChecksum(emptyCase.Tools[0])

	t.Logf("nil params checksum: %s", checksum1)
	t.Logf("empty schema checksum: %s", checksum2)

	if checksum1 == checksum2 {
		t.Errorf("ComputeToolChecksum() should distinguish nil parameters from empty schema, both got %s", checksum1)
	}
}
