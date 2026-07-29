// Package ui 提供 aris 客户端 TUI 的主题 token 与共享渲染组件。
package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// 语义色（明暗终端自适应）
var (
	colorPrimary = lipgloss.AdaptiveColor{Light: constant.ClientUIColorPrimaryLight, Dark: constant.ClientUIColorPrimaryDark}
	colorSuccess = lipgloss.AdaptiveColor{Light: constant.ClientUIColorSuccessLight, Dark: constant.ClientUIColorSuccessDark}
	colorWarning = lipgloss.AdaptiveColor{Light: constant.ClientUIColorWarningLight, Dark: constant.ClientUIColorWarningDark}
	colorError   = lipgloss.AdaptiveColor{Light: constant.ClientUIColorErrorLight, Dark: constant.ClientUIColorErrorDark}
	colorMuted   = lipgloss.AdaptiveColor{Light: constant.ClientUIColorMutedLight, Dark: constant.ClientUIColorMutedDark}
)

// HuhTheme 返回与 ui 主题对齐的 huh 表单主题
func HuhTheme() *huh.Theme {
	theme := huh.ThemeCharm()
	theme.Focused.Title = theme.Focused.Title.Foreground(colorPrimary)
	theme.Focused.NoteTitle = theme.Focused.NoteTitle.Foreground(colorPrimary)
	theme.Focused.SelectSelector = theme.Focused.SelectSelector.Foreground(colorPrimary)
	theme.Focused.FocusedButton = theme.Focused.FocusedButton.Background(colorPrimary)
	theme.Focused.ErrorIndicator = theme.Focused.ErrorIndicator.Foreground(colorError)
	theme.Focused.ErrorMessage = theme.Focused.ErrorMessage.Foreground(colorError)
	return theme
}
