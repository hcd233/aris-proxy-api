package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
)

// StepHeader 渲染向导步骤标题，如 "[1/4] Connect to server"
func StepHeader(step, total int, title string) string {
	index := lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf("[%d/%d]", step, total))
	name := lipgloss.NewStyle().Foreground(colorPrimary).Bold(true).Render(title)
	return index + " " + name
}

// SectionTitle 渲染面板节标题，如 "◆ Server"
func SectionTitle(name string) string {
	return lipgloss.NewStyle().Foreground(colorPrimary).Render(constant.ClientUIIconSection + " " + name)
}

// checkRow 渲染一行检查结果：状态图标 + 标签 + 可选 muted 详情
func checkRow(icon string, color lipgloss.TerminalColor, label, detail string) string {
	row := lipgloss.NewStyle().Foreground(color).Render(icon) + " " + label
	if detail != "" {
		row += lipgloss.NewStyle().Foreground(colorMuted).Render(" · " + detail)
	}
	return row
}

// CheckRowOK 渲染成功检查行
func CheckRowOK(label, detail string) string {
	return checkRow(constant.ClientUIIconOK, colorSuccess, label, detail)
}

// CheckRowFail 渲染失败检查行
func CheckRowFail(label, detail string) string {
	return checkRow(constant.ClientUIIconFail, colorError, label, detail)
}

// CheckRowWarn 渲染警告检查行
func CheckRowWarn(label, detail string) string {
	return checkRow(constant.ClientUIIconWarn, colorWarning, label, detail)
}

// CheckRowInfo 渲染中性信息行
func CheckRowInfo(label, detail string) string {
	return checkRow(constant.ClientUIIconInfo, colorMuted, label, detail)
}

// KeyValue 渲染冒号对齐的键值行
func KeyValue(pairs ...[2]string) string {
	width := 0
	for _, pair := range pairs {
		if len(pair[0]) > width {
			width = len(pair[0])
		}
	}
	lines := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		key := lipgloss.NewStyle().Foreground(colorMuted).Render(fmt.Sprintf(constant.ClientUIKeyPaddingFormat, width+1, pair[0]+":"))
		lines = append(lines, key+" "+pair[1])
	}
	return strings.Join(lines, "\n")
}

// SummaryPanel 渲染带圆角边框的总结面板
func SummaryPanel(lines ...string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSuccess).
		Padding(0, 1).
		Render(strings.Join(lines, "\n"))
}
