package filter_test

import (
	"reflect"
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
		{name: "single filter", expr: "user:john", want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"john"}}}},
		{
			name: "multiple filters",
			expr: "user:john model:gpt-4o status:200",
			want: []filter.Filter{
				{Field: "user", Operator: enum.OpEqual, Values: []string{"john"}},
				{Field: "model", Operator: enum.OpEqual, Values: []string{"gpt-4o"}},
				{Field: "status", Operator: enum.OpEqual, Values: []string{"200"}},
			},
		},
		{name: "not equal operator", expr: "status:!200", want: []filter.Filter{{Field: "status", Operator: enum.OpNotEqual, Values: []string{"200"}}}},
		{name: "greater than operator", expr: "score:>3", want: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Values: []string{"3"}}}},
		{name: "greater than or equal operator", expr: "score:>=3", want: []filter.Filter{{Field: "score", Operator: enum.OpGTE, Values: []string{"3"}}}},
		{name: "less than operator", expr: "score:<3", want: []filter.Filter{{Field: "score", Operator: enum.OpLess, Values: []string{"3"}}}},
		{name: "less than or equal operator", expr: "score:<=3", want: []filter.Filter{{Field: "score", Operator: enum.OpLTE, Values: []string{"3"}}}},
		{name: "quoted value", expr: `user:"john doe"`, want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"john doe"}}}},
		{name: "invalid expression", expr: "invalid", wantErr: true},

		// ── multi-value cases ──
		{name: "multi value pipe", expr: "user:alice|bob",
			want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}}}},
		{name: "multi value with not equal", expr: "user:!alice|bob",
			want: []filter.Filter{{Field: "user", Operator: enum.OpNotEqual, Values: []string{"alice", "bob"}}}},
		{name: "multi value with comparison still parses",
			expr: "score:>3|5",
			want: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Values: []string{"3", "5"}}}},
		{name: "quoted value preserves pipe", expr: `user:"alice|bob"`,
			want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice|bob"}}}},
		{name: "trims empty parts", expr: "user:alice||bob|",
			want: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}}}},
		{name: "all empty parts errors out", expr: "user:|", wantErr: true},
		{name: "score unscored-or-value mixed", expr: "score:unscored|3",
			want: []filter.Filter{{Field: "score", Operator: enum.OpEqual, Values: []string{"unscored", "3"}}}},
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
				w := tt.want[i]
				if f.Field != w.Field || f.Operator != w.Operator || len(f.Values) != len(w.Values) {
					t.Errorf("Parse()[%d] = %+v, want %+v", i, f, w)
					continue
				}
				for j := range f.Values {
					if f.Values[j] != w.Values[j] {
						t.Errorf("Parse()[%d].Values[%d] = %q, want %q", i, j, f.Values[j], w.Values[j])
					}
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
			ValueMap: map[string]*string{"unscored": nil},
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
		{name: "user fuzzy match", filters: []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"john"}}}, wantSQL: "user_name LIKE ?", wantArgs: []any{"%john%"}},
		{name: "user not equal fuzzy", filters: []filter.Filter{{Field: "user", Operator: enum.OpNotEqual, Values: []string{"john"}}}, wantSQL: "user_name NOT LIKE ?", wantArgs: []any{"%john%"}},
		{name: "status equal 200", filters: []filter.Filter{{Field: "status", Operator: enum.OpEqual, Values: []string{"200"}}}, wantSQL: "upstream_status_code = ?", wantArgs: []any{"200"}},
		{name: "status not equal 200", filters: []filter.Filter{{Field: "status", Operator: enum.OpNotEqual, Values: []string{"200"}}}, wantSQL: "upstream_status_code != ?", wantArgs: []any{"200"}},
		{name: "score is null (unscored)", filters: []filter.Filter{{Field: "score", Operator: enum.OpEqual, Values: []string{"unscored"}}}, wantSQL: "score IS NULL", wantArgs: nil},
		{name: "score is not null", filters: []filter.Filter{{Field: "score", Operator: enum.OpNotEqual, Values: []string{"unscored"}}}, wantSQL: "score IS NOT NULL", wantArgs: nil},
		{name: "score greater than", filters: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Values: []string{"3"}}}, wantSQL: "score > ?", wantArgs: []any{"3"}},
		{name: "priority mapped value", filters: []filter.Filter{{Field: "priority", Operator: enum.OpEqual, Values: []string{"high"}}}, wantSQL: "priority = ?", wantArgs: []any{"1"}},
		{
			name: "combined filters",
			filters: []filter.Filter{
				{Field: "user", Operator: enum.OpEqual, Values: []string{"john"}},
				{Field: "status", Operator: enum.OpEqual, Values: []string{"200"}},
			},
			wantSQL:  "user_name LIKE ? AND upstream_status_code = ?",
			wantArgs: []any{"%john%", "200"},
		},
		{name: "unknown field", filters: []filter.Filter{{Field: "unknown", Operator: enum.OpEqual, Values: []string{"test"}}}, wantErr: true},

		// ── multi-value cases ──
		{
			name:     "fuzzy multi value OR LIKE",
			filters:  []filter.Filter{{Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}}},
			wantSQL:  "(user_name LIKE ? OR user_name LIKE ?)",
			wantArgs: []any{"%alice%", "%bob%"},
		},
		{
			name:     "fuzzy multi value AND NOT LIKE",
			filters:  []filter.Filter{{Field: "user", Operator: enum.OpNotEqual, Values: []string{"alice", "bob"}}},
			wantSQL:  "(user_name NOT LIKE ? AND user_name NOT LIKE ?)",
			wantArgs: []any{"%alice%", "%bob%"},
		},
		{
			name:     "numeric multi value IN",
			filters:  []filter.Filter{{Field: "status", Operator: enum.OpEqual, Values: []string{"200", "500"}}},
			wantSQL:  "upstream_status_code IN (?)",
			wantArgs: []any{[]string{"200", "500"}},
		},
		{
			name:     "numeric multi value NOT IN",
			filters:  []filter.Filter{{Field: "status", Operator: enum.OpNotEqual, Values: []string{"200", "500"}}},
			wantSQL:  "upstream_status_code NOT IN (?)",
			wantArgs: []any{[]string{"200", "500"}},
		},
		{
			name:     "valuemap unscored mixed with score",
			filters:  []filter.Filter{{Field: "score", Operator: enum.OpEqual, Values: []string{"unscored", "3"}}},
			wantSQL:  "(score IS NULL OR score = ?)",
			wantArgs: []any{"3"},
		},
		{
			name:     "valuemap unscored mixed with score not equal",
			filters:  []filter.Filter{{Field: "score", Operator: enum.OpNotEqual, Values: []string{"unscored", "3"}}},
			wantSQL:  "(score IS NOT NULL AND score != ?)",
			wantArgs: []any{"3"},
		},
		{
			name:    "multi value with comparison rejected",
			filters: []filter.Filter{{Field: "score", Operator: enum.OpGreater, Values: []string{"3", "5"}}},
			wantErr: true,
		},
		{
			name: "combined cross field AND with multi value",
			filters: []filter.Filter{
				{Field: "user", Operator: enum.OpEqual, Values: []string{"alice", "bob"}},
				{Field: "status", Operator: enum.OpEqual, Values: []string{"200", "500"}},
			},
			wantSQL:  "(user_name LIKE ? OR user_name LIKE ?) AND upstream_status_code IN (?)",
			wantArgs: []any{"%alice%", "%bob%", []string{"200", "500"}},
		},
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
				if !reflect.DeepEqual(arg, tt.wantArgs[i]) {
					t.Errorf("ToSQL() args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}
