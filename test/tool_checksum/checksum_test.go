package tool_checksum

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/dto"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

// toolChecksumCase 工具 checksum 测试用例
type toolChecksumCase struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Tools       []*dto.UnifiedTool `json:"tools"`
	ExpectEqual bool               `json:"expect_equal"`
}

// loadToolCases 从 fixtures/cases.json 加载测试用例
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

func TestComputeToolChecksum_SameToolSameParams(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "same_tool_same_params")

	t.Run("identical tools should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: name=%s, checksum=%s", tc.Tools[0].Name, checksum1)
		t.Logf("tool2: name=%s, checksum=%s", tc.Tools[1].Name, checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeToolChecksum() mismatch for identical tools: got %s and %s", checksum1, checksum2)
		}
	})
}

func TestComputeToolChecksum_SameNameDifferentDescription(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "same_name_different_description")

	t.Run("same name and params but different description should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: name=%s, desc=%s, checksum=%s", tc.Tools[0].Name, tc.Tools[0].Description, checksum1)
		t.Logf("tool2: name=%s, desc=%s, checksum=%s", tc.Tools[1].Name, tc.Tools[1].Description, checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeToolChecksum() should ignore description: got %s and %s", checksum1, checksum2)
		}
	})
}

func TestComputeToolChecksum_DifferentProviderSameSchema(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "different_provider_same_schema")

	t.Run("different provider but same name and params should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: provider=%s, name=%s, checksum=%s", tc.Tools[0].Provider, tc.Tools[0].Name, checksum1)
		t.Logf("tool2: provider=%s, name=%s, checksum=%s", tc.Tools[1].Provider, tc.Tools[1].Name, checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeToolChecksum() should ignore provider: got %s and %s", checksum1, checksum2)
		}
	})
}

func TestComputeToolChecksum_DifferentName(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "different_name")

	t.Run("different tool names should produce different checksums", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: name=%s, checksum=%s", tc.Tools[0].Name, checksum1)
		t.Logf("tool2: name=%s, checksum=%s", tc.Tools[1].Name, checksum2)

		if checksum1 == checksum2 {
			t.Errorf("ComputeToolChecksum() should produce different checksums for different names, both got %s", checksum1)
		}
	})
}

func TestComputeToolChecksum_DifferentParamKeys(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "different_param_keys")

	t.Run("different parameter keys should produce different checksums", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: name=%s, checksum=%s", tc.Tools[0].Name, checksum1)
		t.Logf("tool2: name=%s, checksum=%s", tc.Tools[1].Name, checksum2)

		if checksum1 == checksum2 {
			t.Errorf("ComputeToolChecksum() should produce different checksums for different param keys, both got %s", checksum1)
		}
	})
}

func TestComputeToolChecksum_NilParameters(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "nil_parameters")

	t.Run("tool without parameters should compute checksum normally", func(t *testing.T) {
		checksum := util.ComputeToolChecksum(tc.Tools[0])
		t.Logf("tool: name=%s, checksum=%s", tc.Tools[0].Name, checksum)

		if checksum == "" {
			t.Errorf("ComputeToolChecksum() returned empty string for tool without parameters")
		}
	})
}

func TestComputeToolChecksum_EmptyProperties(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "empty_properties")

	t.Run("tool with empty properties should compute checksum normally", func(t *testing.T) {
		checksum := util.ComputeToolChecksum(tc.Tools[0])
		t.Logf("tool: name=%s, checksum=%s", tc.Tools[0].Name, checksum)

		if checksum == "" {
			t.Errorf("ComputeToolChecksum() returned empty string for tool with empty properties")
		}
	})
}

func TestComputeToolChecksum_SameParamsDifferentKeyOrder(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "same_params_different_key_order")

	t.Run("same param keys in different order should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: name=%s, checksum=%s", tc.Tools[0].Name, checksum1)
		t.Logf("tool2: name=%s, checksum=%s", tc.Tools[1].Name, checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeToolChecksum() should produce same checksum regardless of param key order: got %s and %s", checksum1, checksum2)
		}
	})
}

func TestComputeToolChecksum_ManyParamsSameTool(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "many_params_same_tool")

	t.Run("many-param tools with same name and param keys should produce same checksum", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: provider=%s, name=%s, checksum=%s", tc.Tools[0].Provider, tc.Tools[0].Name, checksum1)
		t.Logf("tool2: provider=%s, name=%s, checksum=%s", tc.Tools[1].Provider, tc.Tools[1].Name, checksum2)

		if checksum1 != checksum2 {
			t.Errorf("ComputeToolChecksum() mismatch for many-param tools: got %s and %s", checksum1, checksum2)
		}
	})
}

func TestComputeToolChecksum_SameTopLevelKeysDifferentNestedParams(t *testing.T) {
	cases := loadToolCases(t)
	tc := findCase(t, cases, "same_top_level_keys_different_nested_params")

	t.Run("same top-level keys but different nested properties should produce different checksum", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(tc.Tools[0])
		checksum2 := util.ComputeToolChecksum(tc.Tools[1])

		t.Logf("tool1: name=%s, nested keys=[timeout,retries], checksum=%s", tc.Tools[0].Name, checksum1)
		t.Logf("tool2: name=%s, nested keys=[endpoint,apiKey,region], checksum=%s", tc.Tools[1].Name, checksum2)

		if checksum1 == checksum2 {
			t.Errorf("ComputeToolChecksum() should produce different checksums when nested params differ, both got %s", checksum1)
		}
	})
}

func TestComputeToolChecksum_Deterministic(t *testing.T) {
	tool := &dto.UnifiedTool{
		Provider:    "openai",
		Name:        "Bash",
		Description: "Execute a bash command",
		Parameters: &dto.JSONSchemaProperty{
			Type: "object",
			Properties: map[string]*dto.JSONSchemaProperty{
				"command": {Type: "string", Description: "The bash command"},
				"timeout": {Type: "integer", Description: "Timeout in seconds"},
			},
			Required: []string{"command"},
		},
	}

	t.Run("same tool should always produce same checksum across multiple calls", func(t *testing.T) {
		checksums := make(map[string]bool)
		for i := 0; i < 100; i++ {
			checksum := util.ComputeToolChecksum(tool)
			checksums[checksum] = true
		}

		t.Logf("unique checksums from 100 calls: %d", len(checksums))

		if len(checksums) != 1 {
			t.Errorf("ComputeToolChecksum() is not deterministic: got %d unique checksums from 100 calls", len(checksums))
		}
	})
}

func TestComputeToolChecksum_NilVsEmptyProperties(t *testing.T) {
	toolNilParams := &dto.UnifiedTool{
		Provider:    "openai",
		Name:        "GetTime",
		Description: "Get the current time",
	}

	toolEmptyProps := &dto.UnifiedTool{
		Provider:    "openai",
		Name:        "GetTime",
		Description: "Get the current time",
		Parameters: &dto.JSONSchemaProperty{
			Type: "object",
		},
	}

	t.Run("nil parameters and empty properties should produce different checksum", func(t *testing.T) {
		checksum1 := util.ComputeToolChecksum(toolNilParams)
		checksum2 := util.ComputeToolChecksum(toolEmptyProps)

		t.Logf("nil params checksum: %s", checksum1)
		t.Logf("empty props checksum: %s", checksum2)

		if checksum1 == checksum2 {
			t.Errorf("ComputeToolChecksum() should distinguish nil parameters from empty schema, both got %s", checksum1)
		}
	})
}

// findCase 根据名称查找测试用例
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
