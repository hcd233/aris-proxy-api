// Package input_modality 输入模态枚举契约测试
package input_modality

import (
	"slices"
	"testing"

	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

// TestInputModalitiesOrder 枚举序即规范输出序（models.dev 模态映射与前端 chips 均按此序）
func TestInputModalitiesOrder(t *testing.T) {
	t.Parallel()
	want := []enum.InputModality{"text", "image", "pdf", "video", "audio"}
	if !slices.Equal(enum.InputModalities, want) {
		t.Fatalf("InputModalities = %v", enum.InputModalities)
	}
}
