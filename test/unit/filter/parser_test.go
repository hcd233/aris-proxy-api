package filter_test

import (
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	"github.com/hcd233/aris-proxy-api/internal/common/filter"
)

func strPtr(s string) *string { return &s }

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expr    string
		want    []filter.Filter
		wantErr bool
	}{
		{name: "empty expression", expr: "", want: nil},
		{name: "single filter", expr: "user:john", want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Value: "john"}}},
		{
			name: "multiple filters",
			expr: "user:john model:gpt-4o status:200",
			want: []filter.Filter{
				{Field: "user", Operator: enum.OpEqual, Value: "john"},
				{Field: "model", Operator: enum.OpEqual, Value: "gpt-4o"},
				{Field: "status", Operator: enum.OpEqual, Value: "200"},
			},
		},
		{name: "not equal operator", expr: "status:!200", want: []filter.Filter{{Field: "status", Operator: enum.OpNotEqual, Value: "200"}}},
		{name: "greater than operator", expr: "score:>3", want: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Value: "3"}}},
		{name: "greater than or equal operator", expr: "score:>=3", want: []filter.Filter{{Field: "score", Operator: enum.OpGTE, Value: "3"}}},
		{name: "less than operator", expr: "score:<3", want: []filter.Filter{{Field: "score", Operator: enum.OpLess, Value: "3"}}},
		{name: "less than or equal operator", expr: "score:<=3", want: []filter.Filter{{Field: "score", Operator: enum.OpLTE, Value: "3"}}},
		{name: "quoted value", expr: `user:"john doe"`, want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Value: "john doe"}}},
		{name: "invalid expression", expr: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := filter.Parse(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("Parse() got %d filters, want %d", len(got), len(tt.want))
				return
			}
			for i, f := range got {
				if f != tt.want[i] {
					t.Errorf("Parse()[%d] = %v, want %v", i, f, tt.want[i])
				}
			}
		})
	}
}

func TestToSQL(t *testing.T) {
	t.Parallel()

	configs := map[string]filter.FieldConfig{
		"user":   {SQLColumn: "user_name", IsFuzzy: true},
		"model":  {SQLColumn: "model", IsFuzzy: true},
		"status": {SQLColumn: "upstream_status_code", IsNumeric: true},
		"score": {SQLColumn: "score", IsNumeric: true,
			ValueMap: map[string]*string{"none": nil},
		},
		"priority": {SQLColumn: "priority", IsNumeric: true,
			ValueMap: map[string]*string{"high": strPtr("1"), "low": strPtr("3")},
		},
	}

	tests := []struct {
		name     string
		filters  []filter.Filter
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}{
		{name: "empty filters", filters: []filter.Filter{}, wantSQL: "", wantArgs: nil},
		{name: "user fuzzy match", filters: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Value: "john"}}, wantSQL: "user_name LIKE ?", wantArgs: []any{"%john%"}},
		{name: "user not equal fuzzy", filters: []filter.Filter{{Field: "user", Operator: enum.OpNotEqual, Value: "john"}}, wantSQL: "user_name NOT LIKE ?", wantArgs: []any{"%john%"}},
		{name: "status equal 200", filters: []filter.Filter{{Field: "status", Operator: enum.OpEqual, Value: "200"}}, wantSQL: "upstream_status_code = ?", wantArgs: []any{"200"}},
		{name: "status not equal 200", filters: []filter.Filter{{Field: "status", Operator: enum.OpNotEqual, Value: "200"}}, wantSQL: "upstream_status_code != ?", wantArgs: []any{"200"}},
		{name: "score is null (none)", filters: []filter.Filter{{Field: "score", Operator: enum.OpEqual, Value: "none"}}, wantSQL: "score IS NULL", wantArgs: nil},
		{name: "score is not null", filters: []filter.Filter{{Field: "score", Operator: enum.OpNotEqual, Value: "none"}}, wantSQL: "score IS NOT NULL", wantArgs: nil},
		{name: "score greater than", filters: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Value: "3"}}, wantSQL: "score > ?", wantArgs: []any{"3"}},
		{name: "priority mapped value", filters: []filter.Filter{{Field: "priority", Operator: enum.OpEqual, Value: "high"}}, wantSQL: "priority = ?", wantArgs: []any{"1"}},
		{
			name: "combined filters",
			filters: []filter.Filter{
				{Field: "user", Operator: enum.OpEqual, Value: "john"},
				{Field: "status", Operator: enum.OpEqual, Value: "200"},
			},
			wantSQL:  "user_name LIKE ? AND upstream_status_code = ?",
			wantArgs: []any{"%john%", "200"},
		},
		{name: "unknown field", filters: []filter.Filter{{Field: "unknown", Operator: enum.OpEqual, Value: "test"}}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotSQL, gotArgs, err := filter.ToSQL(tt.filters, configs)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotSQL != tt.wantSQL {
				t.Errorf("ToSQL() SQL = %v, want %v", gotSQL, tt.wantSQL)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("ToSQL() args len = %d, want %d", len(gotArgs), len(tt.wantArgs))
				return
			}
			for i, arg := range gotArgs {
				if arg != tt.wantArgs[i] {
					t.Errorf("ToSQL() args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}
