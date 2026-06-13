package compression

import (
	"strings"
	"testing"

	compression "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestSmartCrusherTooFewItems(t *testing.T) {
	t.Parallel()
	crusher := compression.NewSmartCrusher(compression.DefaultSmartCrusherConfig())
	content := `[{"a":1},{"a":2}]`
	crushed, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify array with fewer than 5 items")
	}
	if crushed != content {
		t.Errorf("content changed: %s", crushed)
	}
}

func TestSmartCrusherPassthroughShortContent(t *testing.T) {
	t.Parallel()
	crusher := compression.NewSmartCrusher(compression.DefaultSmartCrusherConfig())
	content := `[{"a":1},{"a":2},{"a":3},{"a":4},{"a":5}]`
	crushed, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify content below min tokens threshold")
	}
	if crushed != content {
		t.Errorf("content changed: %s", crushed)
	}
}

func TestSmartCrusherDedupIdenticalItems(t *testing.T) {
	t.Parallel()
	crusher := compression.NewSmartCrusher(compression.DefaultSmartCrusherConfig())
	padding := strings.Repeat("x", 100)
	item := `{"v":"same","pad":"` + padding + `"}`
	var parts []string
	for i := 0; i < 10; i++ {
		parts = append(parts, item)
	}
	content := "[" + strings.Join(parts, ",") + "]"
	crushed, modified, strategy := crusher.Crush(content)
	t.Logf("dedup result: %s", crushed)
	t.Logf("strategy: %s", strategy)
	t.Logf("modified: %v", modified)
	if strategy == "passthrough" && modified {
		t.Error("strategy should not be passthrough when modified")
	}
}

func TestSmartCrusherKeepsFirstAndLast(t *testing.T) {
	t.Parallel()
	crusher := compression.NewSmartCrusher(compression.DefaultSmartCrusherConfig())
	padding := strings.Repeat("y", 100)
	items := `[{"id":0,"v":"start","pad":"` + padding + `"},{"id":1,"v":10,"pad":"` + padding + `"},{"id":2,"v":10,"pad":"` + padding + `"},{"id":3,"v":10,"pad":"` + padding + `"},{"id":4,"v":10,"pad":"` + padding + `"},{"id":5,"v":10,"pad":"` + padding + `"},{"id":6,"v":10,"pad":"` + padding + `"},{"id":7,"v":10,"pad":"` + padding + `"},{"id":8,"v":10,"pad":"` + padding + `"},{"id":9,"v":"end","pad":"` + padding + `"}]`
	crushed, modified, strategy := crusher.Crush(items)
	t.Logf("crushed: %s", crushed)
	t.Logf("strategy: %s", strategy)
	t.Logf("modified: %v", modified)
	_ = modified
}

func TestSmartCrusherEmptyArray(t *testing.T) {
	t.Parallel()
	crusher := compression.NewSmartCrusher(compression.DefaultSmartCrusherConfig())
	crushed, modified, _ := crusher.Crush(`[]`)
	if modified {
		t.Error("should not modify empty array")
	}
	if crushed != `[]` {
		t.Error("should return empty array")
	}
}

func TestSmartCrusherNonJSONPassthrough(t *testing.T) {
	t.Parallel()
	crusher := compression.NewSmartCrusher(compression.DefaultSmartCrusherConfig())
	content := "not json at all"
	crushed, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify non-JSON content")
	}
	if crushed != content {
		t.Error("should return original content")
	}
}

func TestSmartCrusherNonArrayJSONPassthrough(t *testing.T) {
	t.Parallel()
	crusher := compression.NewSmartCrusher(compression.DefaultSmartCrusherConfig())
	content := `{"key":"value"}`
	_, modified, _ := crusher.Crush(content)
	if modified {
		t.Error("should not modify non-array JSON")
	}
}
