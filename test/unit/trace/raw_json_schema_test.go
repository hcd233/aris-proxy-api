package trace

import (
	"reflect"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hcd233/aris-proxy-api/internal/dto"
	traceschema "github.com/hcd233/aris-proxy-api/internal/dto/schema"
)

func TestReportTraceEventReqBody_HumaSchema(t *testing.T) {
	t.Parallel()
	registry := huma.NewMapRegistry("#/components/schemas/", huma.DefaultSchemaNamer)
	schema := huma.SchemaFromType(registry, reflect.TypeOf(dto.ReportTraceEventReqBody{}))
	if schema.Properties["records"] == nil {
		t.Fatal("request body must expose records")
	}
	agent := schema.Properties["agent"]
	if agent == nil {
		t.Fatal("request body must expose agent")
	}
	found := false
	for _, v := range agent.Enum {
		if v == "claude" {
			found = true
		}
	}
	if !found {
		t.Fatalf("agent enum must contain claude: %+v", agent.Enum)
	}
}

func TestReportTraceRecordReq_UsesRawJSON(t *testing.T) {
	t.Parallel()
	field, ok := reflect.TypeOf(dto.ReportTraceRecordReq{}).FieldByName("Payload")
	if !ok || field.Type != reflect.TypeOf(traceschema.RawJSON(nil)) {
		t.Fatalf("Payload must use schema.RawJSON, got %v", field.Type)
	}
}
