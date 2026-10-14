package web_dashboard_layout_test

import (
	"strings"
	"testing"
)

// NavIndicator 只在 layoutKey 变化时测一次激活项落点。若用 getBoundingClientRect
// 读行位置，会读到入场动画的首帧：anthropic 的 `.nav-items > *` 从
// translateY(8px) 起跑（backwards 填充），行随后升回原位，高亮却留在旧坐标，
// 于是首次进入页面时激活态长期偏下 8px（moonshot 的 lunar-focus-in 只缩放，
// 中心不变，所以只在 anthropic 暴露）。位置必须走不受 transform 影响的
// offsetTop 链路。
func TestNavIndicatorMeasuresTransformFreeTop(t *testing.T) {
	t.Parallel()
	source := readWebFile(t, "src", "components", "motion", "nav-indicator.tsx")

	if !strings.Contains(source, "offsetTop") {
		t.Fatal("nav-indicator must derive the active row's top from offsetTop: getBoundingClientRect reports the entrance animation's first frame")
	}
	if strings.Contains(source, "active.getBoundingClientRect") {
		t.Fatal("nav-indicator must not measure the active row with getBoundingClientRect: the sidebar rows' enter animation skews it")
	}
}
