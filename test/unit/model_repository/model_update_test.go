package model_repository

import (
	"reflect"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	dbmodel "github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"
)

// TestModelUpdateColumnConstants verifies that the column name constants used
// in modelRepository.Update() (in endpoint_repository.go) match the GORM column
// tags on the database model struct. A mismatch causes GORM "invalid field"
// errors (regression: traceID e65fc42f-fb16-4ac0-8ab6-a30dd8a7fe63).
func TestModelUpdateColumnConstantsMatchGORMTags(t *testing.T) {
	modelType := reflect.TypeOf(dbmodel.Model{})

	// Map of Go field names to the constant we expect to match their gorm column tag
	type fieldCheck struct {
		goField  string
		constant string
	}

	checks := []fieldCheck{
		{goField: "Alias", constant: constant.FieldModelAlias},
		{goField: "ModelName", constant: constant.FieldModelModelName},
		{goField: "EndpointID", constant: constant.FieldModelEndpointID},
	}

	for _, c := range checks {
		field, ok := modelType.FieldByName(c.goField)
		if !ok {
			t.Fatalf("field %q not found in dbmodel.Model", c.goField)
		}

		gormTag := field.Tag.Get("gorm")
		expectedColumn := extractColumnFromGormTag(gormTag)
		if expectedColumn == "" {
			t.Fatalf("field %q has no gorm column tag", c.goField)
		}

		if c.constant != expectedColumn {
			t.Errorf("constant mismatch for field %q: constant=%q, gorm column=%q",
				c.goField, c.constant, expectedColumn)
		}
	}
}

var _ = extractColumnFromGormTag // suppress unused lint

func extractColumnFromGormTag(tag string) string {
	const prefix = "column:"
	for _, part := range splitGormTag(tag) {
		if len(part) > len(prefix) && part[:len(prefix)] == prefix {
			return part[len(prefix):]
		}
	}
	return ""
}

func splitGormTag(tag string) []string {
	var result []string
	start := 0
	for i := 0; i < len(tag); i++ {
		if tag[i] == ';' {
			if i > start {
				result = append(result, tag[start:i])
			}
			start = i + 1
		}
	}
	if start < len(tag) {
		result = append(result, tag[start:])
	}
	return result
}
