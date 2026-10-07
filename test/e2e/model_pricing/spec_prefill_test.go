package model_pricing

import "testing"

// TestSpecPrefill 模型规格导入：命中返回规格+定价、未命中 found=false（需 BASE_URL/WEB_JWT；离线 skip）。
// wire 为 huma unwrap 后的扁平结构（无 data 外层）。
func TestSpecPrefill(t *testing.T) {
	t.Parallel()
	env := mustEnv(t, "BASE_URL", "WEB_JWT")
	obj := getJSON(t, env["BASE_URL"], env["WEB_JWT"],
		"/api/web/v1/model/spec/prefill?upstreamModel=claude-sonnet-4-5")
	if obj["found"] != true {
		t.Fatalf("claude-sonnet-4-5 must be found: %v", obj)
	}
	if _, ok := obj["contextLength"].(float64); !ok {
		t.Fatalf("contextLength missing: %v", obj)
	}
	if _, ok := obj["maxOutputTokens"].(float64); !ok {
		t.Fatalf("maxOutputTokens missing: %v", obj)
	}
	if caps, ok := obj["capabilities"].([]any); !ok || len(caps) == 0 {
		t.Fatalf("capabilities missing: %v", obj)
	}
	miss := getJSON(t, env["BASE_URL"], env["WEB_JWT"],
		"/api/web/v1/model/spec/prefill?upstreamModel=__no_such_model__")
	if miss["found"] != false {
		t.Fatalf("unknown model must miss: %v", miss)
	}
}
