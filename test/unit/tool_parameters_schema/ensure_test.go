package tool_parameters_schema

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/hcd233/aris-proxy-api/internal/util"
)

type ensureCase struct {
	Name                      string `json:"name"`
	Description               string `json:"description"`
	Input                     string `json:"input"`
	ExpectModified            bool   `json:"expectModified"`
	ExpectedPropertiesPresent bool   `json:"expectedPropertiesPresent,omitempty"`
	ExpectedPropertiesEmpty   bool   `json:"expectedPropertiesEmpty,omitempty"`
}

func loadEnsureCases(t *testing.T) []ensureCase {
	t.Helper()
	data, err := os.ReadFile("./fixtures/ensure_cases.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	var cases []ensureCase
	if err := sonic.Unmarshal(data, &cases); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}
	return cases
}

// TestEnsureToolParametersSchema 覆盖 util.EnsureToolParametersSchema 的所有分支，
// 并回归上游 400 的 bug：object 类型 parameters 缺失 properties 必须补位为 {}。
func TestEnsureToolParametersSchema(t *testing.T) {
	for _, tc := range loadEnsureCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			got := util.EnsureToolParametersSchema([]byte(tc.Input))
			bodyStr := string(got)

			if !tc.ExpectModified {
				if bodyStr != tc.Input {
					t.Errorf("expected body unchanged\nInput:  %s\nOutput: %s", tc.Input, bodyStr)
				}
				return
			}

			if bodyStr == tc.Input {
				t.Fatalf("expected body to be modified\nInput:  %s\nOutput: %s", tc.Input, bodyStr)
			}

			var parsed map[string]any
			if err := sonic.Unmarshal(got, &parsed); err != nil {
				t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, bodyStr)
			}

			tools, ok := parsed["tools"].([]any)
			if !ok {
				t.Fatalf("tools field missing or not array in output: %s", bodyStr)
			}

			for i, toolRaw := range tools {
				tool, ok := toolRaw.(map[string]any)
				if !ok {
					continue
				}
				fn, hasFn := tool["function"].(map[string]any)
				if !hasFn {
					continue
				}
				params, hasParams := fn["parameters"].(map[string]any)
				if !hasParams {
					continue
				}

				_, hasProps := params["properties"]
				if tc.ExpectedPropertiesPresent {
					if !hasProps {
						t.Fatalf("tools[%d].function.parameters.properties should be present, body=%s", i, bodyStr)
					}
					if tc.ExpectedPropertiesEmpty {
						propsMap, isMap := params["properties"].(map[string]any)
						if !isMap {
							t.Fatalf("tools[%d].function.parameters.properties should be a map, body=%s", i, bodyStr)
						}
						if len(propsMap) != 0 {
							t.Fatalf("tools[%d].function.parameters.properties should be empty, body=%s", i, bodyStr)
						}
					}
				} else {
					if hasProps {
						t.Fatalf("tools[%d].function.parameters.properties should NOT be present, body=%s", i, bodyStr)
					}
				}
			}
		})
	}
}

// TestEnsureToolParametersSchemaRegression 直接复现上游错误场景：
// chrome-devtools_list_pages 的 parameters 为 {"type":"object"} 无 properties，
// 上游返回 400 "object schema missing properties"（traceID: 2a996b4c-305e-4852-a890-a2c37583295c）
func TestEnsureToolParametersSchemaRegression(t *testing.T) {
	input := `{"model":"gpt-5.5","tools":[{"type":"function","function":{"name":"chrome-devtools_list_pages","parameters":{"type":"object"}}}]}`

	got := util.EnsureToolParametersSchema([]byte(input))
	if string(got) == input {
		t.Fatalf("expected body to be modified for regression case")
	}

	var parsed map[string]any
	if err := sonic.Unmarshal(got, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	tools := parsed["tools"].([]any)
	tool := tools[0].(map[string]any)
	fn := tool["function"].(map[string]any)
	params := fn["parameters"].(map[string]any)
	props := params["properties"].(map[string]any)
	if len(props) != 0 {
		t.Fatalf("properties should be empty, got %v", props)
	}
}

// TestEnsureToolParametersSchemaInertInput 确保无效 JSON 等不会 panic
func TestEnsureToolParametersSchemaInertInput(t *testing.T) {
	cases := []string{
		"",
		"not json",
		`{"model":"test"}`,
		`{"model":"test","tools":null}`,
		`{"model":"test","tools":"not_array"}`,
	}
	for _, c := range cases {
		result := util.EnsureToolParametersSchema([]byte(c))
		if string(result) != c {
			t.Errorf("expected pass-through for inert input %q, got %s", c, result)
		}
	}
}
