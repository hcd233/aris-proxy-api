package filter

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    []Filter
		wantErr bool
	}{
		{
			name: "empty expression",
			expr: "",
			want: nil,
		},
		{
			name: "single filter",
			expr: "user:john",
			want: []Filter{{Field: "user", Operator: OpEqual, Value: "john"}},
		},
		{
			name: "multiple filters",
			expr: "user:john model:gpt-4o status:200",
			want: []Filter{
				{Field: "user", Operator: OpEqual, Value: "john"},
				{Field: "model", Operator: OpEqual, Value: "gpt-4o"},
				{Field: "status", Operator: OpEqual, Value: "200"},
			},
		},
		{
			name: "not equal operator",
			expr: "status:!200",
			want: []Filter{{Field: "status", Operator: OpNotEqual, Value: "200"}},
		},
		{
			name: "greater than operator",
			expr: "score:>3",
			want: []Filter{{Field: "score", Operator: OpGreater, Value: "3"}},
		},
		{
			name: "greater than or equal operator",
			expr: "score:>=3",
			want: []Filter{{Field: "score", Operator: OpGTE, Value: "3"}},
		},
		{
			name: "less than operator",
			expr: "score:<3",
			want: []Filter{{Field: "score", Operator: OpLess, Value: "3"}},
		},
		{
			name: "less than or equal operator",
			expr: "score:<=3",
			want: []Filter{{Field: "score", Operator: OpLTE, Value: "3"}},
		},
		{
			name: "quoted value",
			expr: `user:"john doe"`,
			want: []Filter{{Field: "user", Operator: OpEqual, Value: "john doe"}},
		},
		{
			name:    "invalid expression",
			expr:    "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.expr)
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
	strPtr := func(s string) *string { return &s }

	configs := map[string]FieldConfig{
		"user": {
			SQLColumn: "user_name",
			IsFuzzy:   true,
		},
		"model": {
			SQLColumn: "model",
			IsFuzzy:   true,
		},
		"status": {
			SQLColumn: "upstream_status_code",
			IsNumeric: true,
		},
		"score": {
			SQLColumn: "score",
			IsNumeric: true,
			ValueMap: map[string]*string{
				"none": nil, // NULL
			},
		},
		"priority": {
			SQLColumn: "priority",
			IsNumeric: true,
			ValueMap: map[string]*string{
				"high": strPtr("1"),
				"low":  strPtr("3"),
			},
		},
	}

	tests := []struct {
		name     string
		filters  []Filter
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}{
		{
			name:     "empty filters",
			filters:  []Filter{},
			wantSQL:  "",
			wantArgs: nil,
		},
		{
			name:     "user fuzzy match",
			filters:  []Filter{{Field: "user", Operator: OpEqual, Value: "john"}},
			wantSQL:  "user_name LIKE ?",
			wantArgs: []any{"%john%"},
		},
		{
			name:     "user not equal fuzzy",
			filters:  []Filter{{Field: "user", Operator: OpNotEqual, Value: "john"}},
			wantSQL:  "user_name NOT LIKE ?",
			wantArgs: []any{"%john%"},
		},
		{
			name:     "status equal 200",
			filters:  []Filter{{Field: "status", Operator: OpEqual, Value: "200"}},
			wantSQL:  "upstream_status_code = ?",
			wantArgs: []any{"200"},
		},
		{
			name:     "status not equal 200",
			filters:  []Filter{{Field: "status", Operator: OpNotEqual, Value: "200"}},
			wantSQL:  "upstream_status_code != ?",
			wantArgs: []any{"200"},
		},
		{
			name:     "score is null (none)",
			filters:  []Filter{{Field: "score", Operator: OpEqual, Value: "none"}},
			wantSQL:  "score IS NULL",
			wantArgs: nil,
		},
		{
			name:     "score is not null",
			filters:  []Filter{{Field: "score", Operator: OpNotEqual, Value: "none"}},
			wantSQL:  "score IS NOT NULL",
			wantArgs: nil,
		},
		{
			name:     "score greater than",
			filters:  []Filter{{Field: "score", Operator: OpGreater, Value: "3"}},
			wantSQL:  "score > ?",
			wantArgs: []any{"3"},
		},
		{
			name:     "priority mapped value",
			filters:  []Filter{{Field: "priority", Operator: OpEqual, Value: "high"}},
			wantSQL:  "priority = ?",
			wantArgs: []any{"1"},
		},
		{
			name: "combined filters",
			filters: []Filter{
				{Field: "user", Operator: OpEqual, Value: "john"},
				{Field: "status", Operator: OpEqual, Value: "200"},
			},
			wantSQL:  "user_name LIKE ? AND upstream_status_code = ?",
			wantArgs: []any{"%john%", "200"},
		},
		{
			name:    "unknown field",
			filters: []Filter{{Field: "unknown", Operator: OpEqual, Value: "test"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSQL, gotArgs, err := ToSQL(tt.filters, configs)
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
