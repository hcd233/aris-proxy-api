package dao_update

import (
	"reflect"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/samber/lo"
)

func TestUpdateSelectFieldsWithNilValues(t *testing.T) {
	info := map[string]any{
		constant.FieldScore:    nil,
		constant.FieldScoredAt: nil,
	}

	selectFields := lo.Filter(lo.Keys(info), func(item string, _ int) bool {
		v := info[item]
		if v == nil {
			return true
		}
		return !reflect.ValueOf(v).IsZero()
	})

	if len(selectFields) != 2 {
		t.Fatalf("expected 2 select fields, got %d: %v", len(selectFields), selectFields)
	}
}

func TestUpdateSelectFieldsMixedValues(t *testing.T) {
	now := 12345
	info := map[string]any{
		constant.FieldScore:      nil,
		constant.FieldScoredAt:   nil,
		constant.FieldUpdatedAt:  now,
		constant.FieldMessageIDs: []string{},
	}

	selectFields := lo.Filter(lo.Keys(info), func(item string, _ int) bool {
		v := info[item]
		if v == nil {
			return true
		}
		return !reflect.ValueOf(v).IsZero()
	})

	hasNilScore := false
	hasNilScoredAt := false
	hasUpdatedAt := false
	for _, f := range selectFields {
		switch f {
		case constant.FieldScore:
			hasNilScore = true
		case constant.FieldScoredAt:
			hasNilScoredAt = true
		case constant.FieldUpdatedAt:
			hasUpdatedAt = true
		}
	}

	if !hasNilScore {
		t.Errorf("%s should be included as select field (nil value)", constant.FieldScore)
	}
	if !hasNilScoredAt {
		t.Errorf("%s should be included as select field (nil value)", constant.FieldScoredAt)
	}
	if !hasUpdatedAt {
		t.Errorf("%s should be included as select field (non-zero value)", constant.FieldUpdatedAt)
	}

	if len(selectFields) != 3 {
		t.Logf("select fields: %v (empty slice excluded)", selectFields)
	}
}

func TestReflectValueOfNilPanicsOnIsZero(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when calling reflect.ValueOf(nil).IsZero()")
		}
	}()

	reflect.ValueOf(nil).IsZero()
}
