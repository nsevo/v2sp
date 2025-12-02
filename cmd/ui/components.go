package ui

import (
	"fmt"
	"strings"
	"time"
)

// Spinner 加载动画
type Spinner struct {
	frames []string
	index  int
}

func NewSpinner() *Spinner {
	return &Spinner{
		frames: []string{
			IconSpinner1, IconSpinner2, IconSpinner3,
			IconSpinner4, IconSpinner5, IconSpinner6,
			IconSpinner7, IconSpinner8, IconSpinner9,
			IconSpinner0,
		},
		index: 0,
	}
}

func (s *Spinner) Next() string {
	frame := s.frames[s.index]
	s.index = (s.index + 1) % len(s.frames)
	return InfoStyle.Render(frame)
}

// ProgressBar 进度条
func ProgressBar(current, total int, width int) string {
	if width <= 0 {
		width = 30
	}
	if total <= 0 {
		total = 1
	}

	percentage := float64(current) / float64(total)
	if percentage > 1 {
		percentage = 1
	}

	filled := int(float64(width) * percentage)
	empty := width - filled

	bar := ""
	// 填充部分
	for i := 0; i < filled; i++ {
		bar += "█"
	}
	// 空白部分
	for i := 0; i < empty; i++ {
		bar += "░"
	}

	percent := int(percentage * 100)
	return fmt.Sprintf("[%s] %d%%", InfoStyle.Render(bar), percent)
}

// Table 简单表格
type Table struct {
	headers []string
	rows    [][]string
	widths  []int
}

func NewTable(headers []string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{
		headers: headers,
		rows:    [][]string{},
		widths:  widths,
	}
}

func (t *Table) AddRow(row []string) {
	for i, cell := range row {
		if i < len(t.widths) && len(cell) > t.widths[i] {
			t.widths[i] = len(cell)
		}
	}
	t.rows = append(t.rows, row)
}

func (t *Table) Render() string {
	var sb strings.Builder

	// 渲染表头
	for i, header := range t.headers {
		sb.WriteString(DimStyle.Render(fmt.Sprintf("%-*s", t.widths[i]+2, header)))
	}
	sb.WriteString("\n")

	// 渲染行
	for _, row := range t.rows {
		for i, cell := range row {
			if i < len(t.widths) {
				sb.WriteString(TextStyle.Render(fmt.Sprintf("%-*s", t.widths[i]+2, cell)))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// Panel 带标题的面板
func Panel(title, content string) string {
	var sb strings.Builder
	sb.WriteString(TitleStyle.Render(title))
	sb.WriteString("\n")
	sb.WriteString(Divider(60))
	sb.WriteString("\n\n")
	sb.WriteString(PanelStyle.Render(content))
	sb.WriteString("\n")
	sb.WriteString(Divider(60))
	return sb.String()
}

// FormatUptime 格式化运行时间
func FormatUptime(seconds int64) string {
	duration := time.Duration(seconds) * time.Second
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// FormatMemory 格式化内存
func FormatMemory(bytes uint64) string {
	return FormatBytes(int64(bytes))
}

// Section 创建一个内容区块
func Section(lines ...string) string {
	var sb strings.Builder
	for i, line := range lines {
		sb.WriteString("  ")
		sb.WriteString(line)
		if i < len(lines)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// List 创建列表
func List(items []string, prefix string) string {
	var sb strings.Builder
	for _, item := range items {
		sb.WriteString("  ")
		sb.WriteString(DimStyle.Render(prefix))
		sb.WriteString(" ")
		sb.WriteString(TextStyle.Render(item))
		sb.WriteString("\n")
	}
	return sb.String()
}

// Header 创建页面头部
func Header(title, subtitle string) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(TitleStyle.Render(title))
	sb.WriteString("\n")
	if subtitle != "" {
		sb.WriteString(DimStyle.Render(subtitle))
		sb.WriteString("\n")
	}
	sb.WriteString(Divider(60))
	sb.WriteString("\n\n")
	return sb.String()
}

// Footer 创建页面底部
func Footer(items ...string) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(Divider(60))
	sb.WriteString("\n\n")
	for i, item := range items {
		sb.WriteString("  ")
		sb.WriteString(item)
		if i < len(items)-1 {
			sb.WriteString("  ")
		}
	}
	sb.WriteString("\n\n")
	return sb.String()
}

