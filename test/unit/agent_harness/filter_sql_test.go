package agent_harness

import (
	"errors"
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/application/session/query"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

func TestBuildHarnessFilterCondition(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		filter      filter.Filter
		wantISNull  bool
		wantArgs    []any
		wantISNullO bool
	}{
		{
			name:     "specific harness",
			filter:   filter.Filter{Field: "harness", Operator: enum.OpEqual, Values: []string{"ZCode"}},
			wantArgs: []any{"ZCode"},
		},
		{
			name:       "other matches null resolution",
			filter:     filter.Filter{Field: "harness", Operator: enum.OpEqual, Values: []string{"Other"}},
			wantISNull: true,
		},
		{
			name:       "other is case insensitive",
			filter:     filter.Filter{Field: "harness", Operator: enum.OpEqual, Values: []string{"other"}},
			wantISNull: true,
		},
		{
			name:        "negated harness includes other",
			filter:      filter.Filter{Field: "harness", Operator: enum.OpNotEqual, Values: []string{"ZCode"}},
			wantArgs:    []any{"ZCode"},
			wantISNullO: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			sqlText, args, err := query.BuildHarnessFilterCondition(c.filter)
			if err != nil {
				t.Fatalf("BuildHarnessFilterCondition returned error: %v", err)
			}
			if !strings.Contains(sqlText, "SELECT r.name FROM agent_harness_rules") {
				t.Errorf("condition must resolve harness through the rules table, got: %s", sqlText)
			}
			if !strings.Contains(sqlText, "LIKE r.pattern || '%'") {
				t.Errorf("condition must match lower(ua) against rule prefix, got: %s", sqlText)
			}
			if c.wantISNull && !strings.HasSuffix(sqlText, "IS NULL") {
				t.Errorf("Other must compile to IS NULL, got: %s", sqlText)
			}
			if c.wantISNullO && !strings.Contains(sqlText, "IS NULL OR") {
				t.Errorf("negation must include unmapped rows, got: %s", sqlText)
			}
			if len(args) != len(c.wantArgs) {
				t.Fatalf("args = %v, want %v", args, c.wantArgs)
			}
			for i := range args {
				if args[i] != c.wantArgs[i] {
					t.Errorf("args[%d] = %v, want %v", i, args[i], c.wantArgs[i])
				}
			}
		})
	}
}

func TestBuildHarnessFilterConditionRejectsUnsupportedOperators(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		filter filter.Filter
	}{
		{"comparison", filter.Filter{Field: "harness", Operator: enum.OpGreater, Values: []string{"1"}}},
		{"multi value", filter.Filter{Field: "harness", Operator: enum.OpEqual, Values: []string{"A", "B"}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if _, _, err := query.BuildHarnessFilterCondition(c.filter); !errors.Is(err, ierr.ErrValidation) {
				t.Errorf("err = %v, want ErrValidation", err)
			}
		})
	}
}
