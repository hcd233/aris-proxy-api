package model_call_audit

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/dto"
)

// TestSetCompressionStatsDedupStrategies 验证 SetCompressionStats 对传入的
// compression strategies 去重后存储，避免同一策略因多个 tool output 压缩
// 而在 audit 记录中重复出现。
func TestSetCompressionStatsDedupStrategies(t *testing.T) {
	task := &dto.ModelCallAuditTask{}

	task.SetCompressionStats(1000, 400, []string{
		"smart_crusher",
		"smart_crusher",
		"log_compressor",
		"smart_crusher",
	})

	want := []string{"smart_crusher", "log_compressor"}
	got := task.CompressionStrategies

	if len(got) != len(want) {
		t.Fatalf("CompressionStrategies len = %d, want %d (got %v)", len(got), len(want), got)
	}
	for i, s := range want {
		if i >= len(got) || got[i] != s {
			t.Errorf("CompressionStrategies[%d] = %q, want %q (got %v)", i, got[i], s, got)
		}
	}
}

// TestSetCompressionStatsDedupSingleStrategy 验证仅含单一重复策略时去重为 1 项。
func TestSetCompressionStatsDedupSingleStrategy(t *testing.T) {
	task := &dto.ModelCallAuditTask{}

	task.SetCompressionStats(500, 200, []string{
		"smart_crusher",
		"smart_crusher",
		"smart_crusher",
	})

	got := task.CompressionStrategies
	if len(got) != 1 {
		t.Fatalf("CompressionStrategies len = %d, want 1 (got %v)", len(got), got)
	}
	if got[0] != "smart_crusher" {
		t.Errorf("CompressionStrategies[0] = %q, want %q", got[0], "smart_crusher")
	}
}

// TestSetCompressionStatsNilStrategies 验证 nil 策略切片不 panic 且存储为空切片。
func TestSetCompressionStatsNilStrategies(t *testing.T) {
	task := &dto.ModelCallAuditTask{}

	task.SetCompressionStats(100, 50, nil)

	if !task.CompressionEnabled {
		t.Error("CompressionEnabled = false, want true")
	}
	if len(task.CompressionStrategies) != 0 {
		t.Errorf("CompressionStrategies len = %d, want 0", len(task.CompressionStrategies))
	}
}

// TestSetCompressionStatsUniqueStrategies 验证无重复时顺序保持不变。
func TestSetCompressionStatsUniqueStrategies(t *testing.T) {
	task := &dto.ModelCallAuditTask{}

	input := []string{"smart_crusher", "log_compressor", "search_compressor"}
	task.SetCompressionStats(800, 300, input)

	want := []string{"smart_crusher", "log_compressor", "search_compressor"}
	if !slices.Equal(task.CompressionStrategies, want) {
		t.Errorf("CompressionStrategies = %v, want %v", task.CompressionStrategies, want)
	}
}
