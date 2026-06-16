// Package filter 验证通用 filter 包对 JSONB 数组字段的条件生成。
package filter

import (
	"strings"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
	commonfilter "github.com/hcd233/aris-proxy-api/internal/common/filter"
)

func TestJSONBArrayCondition_SingleEqual(t *testing.T) {
	t.Parallel()
	criteria := &commonfilter.FilterCriteria{
		Filters: []commonfilter.Filter{
			{Field: "model", Operator: enum.OpEqual, Values: []string{"gpt-4o"}},
		},
		FieldConfigs: map[string]commonfilter.FieldConfig{
			"model": {SQLColumn: "models", IsJSONBArray: true},
		},
	}
	sql, args, err := commonfilter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "models::jsonb @> jsonb_build_array(?)"
	if sql != want {
		t.Errorf("sql mismatch\nwant: %q\ngot:  %q", want, sql)
	}
	if len(args) != 1 || args[0] != "gpt-4o" {
		t.Errorf("args mismatch, want [gpt-4o], got %v", args)
	}
}

func TestJSONBArrayCondition_MultiEqual(t *testing.T) {
	t.Parallel()
	criteria := &commonfilter.FilterCriteria{
		Filters: []commonfilter.Filter{
			{Field: "model", Operator: enum.OpEqual, Values: []string{"gpt-4o", "claude-3-5-sonnet"}},
		},
		FieldConfigs: map[string]commonfilter.FieldConfig{
			"model": {SQLColumn: "models", IsJSONBArray: true},
		},
	}
	sql, args, err := commonfilter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "models::jsonb @> jsonb_build_array(?)") {
		t.Errorf("sql should contain jsonb_build_array condition, got %q", sql)
	}
	if !strings.Contains(sql, " OR ") {
		t.Errorf("multi-value equal should use OR, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args length want 2, got %d", len(args))
	}
}

func TestJSONBArrayCondition_SingleNotEqual(t *testing.T) {
	t.Parallel()
	criteria := &commonfilter.FilterCriteria{
		Filters: []commonfilter.Filter{
			{Field: "model", Operator: enum.OpNotEqual, Values: []string{"gpt-4o"}},
		},
		FieldConfigs: map[string]commonfilter.FieldConfig{
			"model": {SQLColumn: "models", IsJSONBArray: true},
		},
	}
	sql, args, err := commonfilter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "NOT models::jsonb @> jsonb_build_array(?)"
	if sql != want {
		t.Errorf("sql mismatch\nwant: %q\ngot:  %q", want, sql)
	}
	if len(args) != 1 || args[0] != "gpt-4o" {
		t.Errorf("args mismatch, want [gpt-4o], got %v", args)
	}
}

func TestJSONBArrayCondition_MultiNotEqual(t *testing.T) {
	t.Parallel()
	criteria := &commonfilter.FilterCriteria{
		Filters: []commonfilter.Filter{
			{Field: "model", Operator: enum.OpNotEqual, Values: []string{"gpt-4o", "claude-3-5-sonnet"}},
		},
		FieldConfigs: map[string]commonfilter.FieldConfig{
			"model": {SQLColumn: "models", IsJSONBArray: true},
		},
	}
	sql, args, err := commonfilter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "NOT models::jsonb @> jsonb_build_array(?)") {
		t.Errorf("sql should contain NOT condition, got %q", sql)
	}
	if !strings.Contains(sql, " AND ") {
		t.Errorf("multi-value not-equal should use AND, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args length want 2, got %d", len(args))
	}
}

func TestJSONBArrayCondition_UnsupportedComparison(t *testing.T) {
	t.Parallel()
	criteria := &commonfilter.FilterCriteria{
		Filters: []commonfilter.Filter{
			{Field: "model", Operator: enum.OpGreater, Values: []string{"gpt-4o"}},
		},
		FieldConfigs: map[string]commonfilter.FieldConfig{
			"model": {SQLColumn: "models", IsJSONBArray: true},
		},
	}
	_, _, err := commonfilter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err == nil {
		t.Fatal("expected error for unsupported operator, got nil")
	}
}

func TestJSONBArrayCondition_CombinedWithOtherField(t *testing.T) {
	t.Parallel()
	criteria := &commonfilter.FilterCriteria{
		Filters: []commonfilter.Filter{
			{Field: "score", Operator: enum.OpEqual, Values: []string{"5"}},
			{Field: "model", Operator: enum.OpEqual, Values: []string{"gpt-4o"}},
		},
		FieldConfigs: map[string]commonfilter.FieldConfig{
			"score": {SQLColumn: "score"},
			"model": {SQLColumn: "models", IsJSONBArray: true},
		},
	}
	sql, args, err := commonfilter.ToSQL(criteria.Filters, criteria.FieldConfigs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(sql, "score = ?") {
		t.Errorf("sql should contain score condition, got %q", sql)
	}
	if !strings.Contains(sql, "models::jsonb @> jsonb_build_array(?)") {
		t.Errorf("sql should contain model condition, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args length want 2, got %d", len(args))
	}
}
