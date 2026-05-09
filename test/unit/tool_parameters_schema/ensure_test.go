package toolparametersschema

import (
	"os"
	"testing"

	"github.com/bytedance/sonic"
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
