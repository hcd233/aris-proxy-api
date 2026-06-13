package compression

import (
	"testing"

	compression "github.com/hcd233/aris-proxy-api/internal/application/llmproxy/compression"
)

func TestDetectJSONArray(t *testing.T) {
	t.Parallel()
	content := `[{"id":1,"name":"a"},{"id":2,"name":"b"},{"id":3,"name":"c"}]`
	ct, conf := compression.DetectContentType(content)
	if ct != compression.ContentTypeJSONArray {
		t.Errorf("expected JSON_ARRAY, got %v", ct)
	}
	if conf < 0.9 {
		t.Errorf("confidence too low: %f", conf)
	}
}

func TestDetectJSONArrayNotObject(t *testing.T) {
	t.Parallel()
	content := `[1,2,3,4,5]`
	ct, conf := compression.DetectContentType(content)
	if ct != compression.ContentTypeJSONArray {
		t.Errorf("expected JSON_ARRAY, got %v", ct)
	}
	if conf >= 1.0 {
		t.Errorf("non-dict array confidence should be < 1.0: %f", conf)
	}
}

func TestDetectSearchResults(t *testing.T) {
	t.Parallel()
	content := "src/main.go:42:func process()\nsrc/main.go:58:return result\nsrc/utils.go:12:import \"fmt\"\nsrc/utils.go:33:func helper()"
	ct, conf := compression.DetectContentType(content)
	if ct != compression.ContentTypeSearchResults {
		t.Errorf("expected SEARCH_RESULTS, got %v", ct)
	}
	if conf < 0.6 {
		t.Errorf("confidence too low: %f", conf)
	}
}

func TestDetectBuildOutput(t *testing.T) {
	t.Parallel()
	content := "2024-01-01 12:00:00 ERROR connection failed\n2024-01-01 12:00:01 WARN retrying\n2024-01-01 12:00:02 ERROR timeout\n2024-01-01 12:00:03 INFO recovered\n2024-01-01 12:00:04 FATAL shutdown"
	ct, conf := compression.DetectContentType(content)
	if ct != compression.ContentTypeBuildOutput {
		t.Errorf("expected BUILD_OUTPUT, got %v", ct)
	}
	if conf < 0.5 {
		t.Errorf("confidence too low: %f", conf)
	}
}

func TestDetectCodePython(t *testing.T) {
	t.Parallel()
	content := "def process():\n    return True\n\nclass Handler:\n    def __init__(self):\n        pass\n\nimport os\n\ndef main():\n    result = process()\n    return result"
	ct, _ := compression.DetectContentType(content)
	if ct != compression.ContentTypeSourceCode {
		t.Errorf("expected SOURCE_CODE, got %v", ct)
	}
}

func TestDetectCodeGo(t *testing.T) {
	t.Parallel()
	content := "package main\n\nimport \"fmt\"\n\nfunc main() {\n    fmt.Println(\"hello\")\n}\n\ntype Server struct {\n    port int\n}"
	ct, _ := compression.DetectContentType(content)
	if ct != compression.ContentTypeSourceCode {
		t.Errorf("expected SOURCE_CODE, got %v", ct)
	}
}

func TestDetectPlainText(t *testing.T) {
	t.Parallel()
	content := "This is just a normal sentence with no special formatting at all."
	ct, _ := compression.DetectContentType(content)
	if ct != compression.ContentTypePlainText {
		t.Errorf("expected PLAIN_TEXT, got %v", ct)
	}
}

func TestDetectEmptyContent(t *testing.T) {
	t.Parallel()
	ct, conf := compression.DetectContentType("")
	if ct != compression.ContentTypePlainText {
		t.Errorf("expected PLAIN_TEXT for empty, got %v", ct)
	}
	if conf != 0.0 {
		t.Errorf("expected 0.0 confidence for empty, got %f", conf)
	}
}
