package json_schema

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/bytedance/sonic"
	dto "github.com/hcd233/aris-proxy-api/internal/dto/openai"
)

func TestToolParametersRoundTrip(t *testing.T) {
	t.Parallel()
	// Simulate MCP tool with empty properties - should survive round-trip
	cases := []struct {
		name    string
		rawBody string
	}{
		{
			name:    "empty properties + additionalProperties false",
			rawBody: `{"tools":[{"type":"function","function":{"name":"test_tool","parameters":{"type":"object","properties":{},"additionalProperties":false}}}]}`,
		},
		{
			name:    "with $schema",
			rawBody: `{"tools":[{"type":"function","function":{"name":"test_tool","parameters":{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"name":{"type":"string"}},"additionalProperties":false}}}]}`,
		},
		{
			name:    "null properties (should stay omitted)",
			rawBody: `{"tools":[{"type":"function","function":{"name":"test_tool","parameters":{"type":"object","additionalProperties":false}}}]}`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var req dto.OpenAIChatCompletionReq
			if err := sonic.Unmarshal([]byte(tc.rawBody), &req); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			remarshaled, err := sonic.Marshal(&req)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}

			var roundTripped dto.OpenAIChatCompletionReq
			if err := sonic.Unmarshal(remarshaled, &roundTripped); err != nil {
				t.Fatalf("unmarshal remarshaled error: %v", err)
			}

			if !reflect.DeepEqual(req.Tools, roundTripped.Tools) {
				t.Errorf("TOOLS ROUND-TRIP BROKEN\n  raw:   %#v\n  remarsh: %#v", req.Tools, roundTripped.Tools)
			} else {
				fmt.Printf("  OK: %s\n", tc.rawBody)
			}
		})
	}
}
