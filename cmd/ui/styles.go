package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Claude-style color palette - 低饱和度，专业配色
var (
	// 主色调
	ColorPrimary = lipgloss.Color("#00A6FF") // 柔和的蓝色
	ColorSuccess = lipgloss.Color("#00D787") // 柔和的绿色
	ColorWarning = lipgloss.Color("#FFAF00") // 柔和的黄色
	ColorError   = lipgloss.Color("#FF5F87") // 柔和的红色
	ColorInfo    = lipgloss.Color("#00D7FF") // 柔和的青色

	// 文本色
	ColorText      = lipgloss.Color("#E4E4E4") // 主文本
	ColorTextDim   = lipgloss.Color("#767676") // 次要文本
	ColorTextFaint = lipgloss.Color("#4E4E4E") // 微弱文本

	// 背景色
	ColorBgDark  = lipgloss.Color("#1C1C1C")
	ColorBgLight = lipgloss.Color("#262626")

	// 边框
	ColorBorder = lipgloss.Color("#3A3A3A")
)

// 样式定义
var (
	// 标题样式
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			MarginBottom(0)

	// 分隔线样式
	DividerStyle = lipgloss.NewStyle().
			Foreground(ColorBorder)

	// 普通文本
	TextStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	// 次要文本
	DimStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	// 微弱文本
	FaintStyle = lipgloss.NewStyle().
			Foreground(ColorTextFaint)

	// 成功样式
	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	// 警告样式
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// 错误样式
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	// 信息样式
	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// 标签样式
	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Width(12).
			Align(lipgloss.Right)

	// 值样式
	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			Bold(false)

	// 状态点样式
	StatusDotRunning = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				SetString("●")

	StatusDotStopped = lipgloss.NewStyle().
				Foreground(ColorTextDim).
				SetString("○")

	StatusDotError = lipgloss.NewStyle().
			Foreground(ColorError).
			SetString("●")

	StatusDotWarning = lipgloss.NewStyle().
				Foreground(ColorWarning).
				SetString("●")

	// 快捷键样式
	KeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	KeyDescStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim)

	// 菜单项样式
	MenuItemStyle = lipgloss.NewStyle().
			Foreground(ColorText).
			PaddingLeft(2)

	MenuItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				PaddingLeft(2)

	// 帮助文本样式
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorTextDim).
			Italic(true)

	// 边框样式
	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	// 面板样式
	PanelStyle = lipgloss.NewStyle().
			Padding(0, 2)
)

// 图标
const (
	IconCheck    = "✓"
	IconCross    = "✗"
	IconArrow    = "→"
	IconWarning  = "⚠"
	IconInfo     = "ℹ"
	IconSpinner1 = "⠋"
	IconSpinner2 = "⠙"
	IconSpinner3 = "⠹"
	IconSpinner4 = "⠸"
	IconSpinner5 = "⠼"
	IconSpinner6 = "⠴"
	IconSpinner7 = "⠦"
	IconSpinner8 = "⠧"
	IconSpinner9 = "⠇"
	IconSpinner0 = "⠏"
)

// 辅助函数

// Divider 创建分隔线
func Divider(width int) string {
	if width <= 0 {
		width = 60
	}
	line := ""
	for i := 0; i < width; i++ {
		line += "─"
	}
	return DividerStyle.Render(line)
}

// KeyValue 渲染键值对
func KeyValue(key, value string) string {
	return LabelStyle.Render(key) + "  " + ValueStyle.Render(value)
}

// StatusLine 渲染状态行
func StatusLine(status, text string) string {
	var dot lipgloss.Style
	switch status {
	case "running":
		dot = StatusDotRunning
	case "stopped":
		dot = StatusDotStopped
	case "error":
		dot = StatusDotError
	case "warning":
		dot = StatusDotWarning
	default:
		dot = StatusDotStopped
	}
	return dot.Render() + " " + TextStyle.Render(text)
}

// Key 渲染快捷键
func Key(key, desc string) string {
	return KeyStyle.Render("["+key+"]") + " " + KeyDescStyle.Render(desc)
}

// Success 渲染成功消息
func Success(msg string) string {
	return SuccessStyle.Render(IconCheck) + " " + TextStyle.Render(msg)
}

// Error 渲染错误消息
func Error(msg string) string {
	return ErrorStyle.Render(IconCross) + " " + TextStyle.Render(msg)
}

// Warning 渲染警告消息
func Warning(msg string) string {
	return WarningStyle.Render(IconWarning) + " " + TextStyle.Render(msg)
}

// Info 渲染信息消息
func Info(msg string) string {
	return InfoStyle.Render(IconInfo) + " " + TextStyle.Render(msg)
}

