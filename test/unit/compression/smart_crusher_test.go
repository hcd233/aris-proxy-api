package compression

import (
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	"github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestSmartCrusherLosslessCSV(t *testing.T) {
	t.Parallel()
	sc := compression.NewSmartCrusher()
	content := `[{"name":"error","code":500},{"name":"warn","code":0},{"name":"info","code":200}]`
	result := sc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied")
	}
	if !strings.HasPrefix(result.Output, "code,name\n") {
		t.Errorf("expected CSV header, got: %s", result.Output[:20])
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
}

func TestSmartCrusherLossySampling(t *testing.T) {
	t.Parallel()
	sc := compression.NewSmartCrusher()
	// 构造 50 个对象的数组
	var items []map[string]any
	for i := 0; i < 50; i++ {
		items = append(items, map[string]any{
			"id":    i,
			"name":  "item_" + strings.Repeat("x", 20),
			"value": "some long value that makes the json verbose",
		})
	}
	content := mustJSON(items)
	result := sc.Compress(content)
	if !result.Applied {
		t.Fatal("expected compression to be applied for large array")
	}
	if result.BytesAfter >= result.BytesBefore {
		t.Errorf("expected bytes to decrease: before=%d after=%d", result.BytesBefore, result.BytesAfter)
	}
	if !strings.Contains(result.Output, "省略") {
		t.Error("expected sampling summary in output")
	}
}

func TestSmartCrusherNonArrayPassthrough(t *testing.T) {
	t.Parallel()
	sc := compression.NewSmartCrusher()
	content := `{"key": "value"}`
	result := sc.Compress(content)
	if result.Applied {
		t.Error("should not compress non-array JSON")
	}
}

func TestSmartCrusherEmptyArray(t *testing.T) {
	t.Parallel()
	sc := compression.NewSmartCrusher()
	result := sc.Compress("[]")
	if result.Applied {
		t.Error("should not compress empty array")
	}
}

func mustJSON(v any) string {
	out, err := sonic.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(out)
}
