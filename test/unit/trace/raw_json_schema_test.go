package trace

import (
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	traceschema "github.com/hcd233/aris-proxy-api/internal/dto/schema"
)

func TestReportTraceEventReqBody_HumaSchemaAcceptsDynamicJSON(t *testing.T) {
	t.Parallel()
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	schema := huma.SchemaFromType(registry, reflect.TypeOf(dto.ReportTraceEventReqBody{}))
	if schema.AdditionalProperties != true {
		t.Fatal("request body must accept future Codex fields")
	}
	for _, name := range []string{"tool_input", "tool_response"} {
		field := schema.Properties[name]
		if field == nil || field.Type != "" || field.ContentEncoding != "" {
			t.Fatalf("schema for %s must be unrestricted JSON: %+v", name, field)
		}
	}
}

func TestReportTraceRecordReq_UsesRawJSON(t *testing.T) {
	t.Parallel()
	field, ok := reflect.TypeOf(dto.ReportTraceRecordReq{}).FieldByName("Payload")
	if !ok || field.Type != reflect.TypeOf(traceschema.RawJSON(nil)) {
		t.Fatalf("Payload must use schema.RawJSON, got %v", field.Type)
	}
}
