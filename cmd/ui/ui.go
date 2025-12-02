package ui

import (
	"fmt"
	"strings"
)

const reset = "\033[0m"

type ansiStyle string

func (s ansiStyle) Render(text string) string {
	if text == "" {
		return ""
	}
	if s == "" {
		return text
	}
	return string(s) + text + reset
}

var (
	DimStyle     = ansiStyle("\033[2m")
	SuccessStyle = ansiStyle("\033[0;32m")
	WarningStyle = ansiStyle("\033[0;33m")
	ErrorStyle   = ansiStyle("\033[0;31m")
	InfoStyle    = ansiStyle("\033[0;36m")
)

func Header(title, subtitle string) string {
	var b strings.Builder
	line := Divider(60)
	b.WriteString("\n")
	b.WriteString(line)
	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n")
	if subtitle != "" {
		b.WriteString(subtitle)
		b.WriteString("\n")
	}
	b.WriteString(line)
	b.WriteString("\n\n")
	return b.String()
}

func Section(lines ...string) string {
	return strings.Join(lines, "\n")
}

func Divider(width int) string {
	if width <= 0 {
		width = 60
	}
	return strings.Repeat("─", width)
}

func KeyValue(key, value string) string {
	return fmt.Sprintf("%-12s %s", key, value)
}

func Info(msg string) string {
	return InfoStyle.Render(msg)
}

func Warning(msg string) string {
	return WarningStyle.Render(msg)
}

func Error(msg string) string {
	return ErrorStyle.Render(msg)
}

func Success(msg string) string {
	return SuccessStyle.Render(msg)
}
