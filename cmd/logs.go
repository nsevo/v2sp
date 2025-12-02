package cmd

import (
	"fmt"
	"strings"

	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var (
	followLogs bool
	logLines   int
	logLevel   string
	logNode    int
)

var logsCommand = &cobra.Command{
	Use:   "logs",
	Short: "View v2sp logs",
	Long:  "Display and filter v2sp service logs",
	Run:   logsHandle,
	Args:  cobra.NoArgs,
}

func init() {
	logsCommand.Flags().BoolVarP(&followLogs, "follow", "f", false, "Follow log output (like tail -f)")
	logsCommand.Flags().IntVarP(&logLines, "lines", "n", 50, "Number of lines to show")
	logsCommand.Flags().StringVarP(&logLevel, "level", "l", "", "Filter by log level (debug, info, warn, error)")
	logsCommand.Flags().IntVar(&logNode, "node", 0, "Filter by node ID")
	command.AddCommand(logsCommand)
}

func logsHandle(_ *cobra.Command, _ []string) {
	// 构建 journalctl 命令
	args := []string{"-u", "v2sp"}
	
	if followLogs {
		args = append(args, "-f")
	}
	
	args = append(args, "-n", fmt.Sprintf("%d", logLines))
	args = append(args, "--no-pager")

	// 显示头部
	if !followLogs {
		fmt.Print(ui.Header("v2sp Logs", fmt.Sprintf("Last %d lines", logLines)))
	}

	// 如果有过滤条件，显示提示
	if logLevel != "" || logNode > 0 {
		filters := []string{}
		if logLevel != "" {
			filters = append(filters, "Level: "+logLevel)
		}
		if logNode > 0 {
			filters = append(filters, fmt.Sprintf("Node: %d", logNode))
		}
		fmt.Println(ui.Section(
			ui.DimStyle.Render("Filters: " + strings.Join(filters, ", ")),
		))
		fmt.Println()
	}

	// 执行命令
	if followLogs {
		fmt.Println(ui.DimStyle.Render("  Following logs... (Press Ctrl+C to exit)"))
		fmt.Println()
		fmt.Println(ui.Divider(60))
		fmt.Println()
	}

	// 使用管道进行日志格式化
	cmd := "journalctl " + strings.Join(args, " ")
	
	// 如果需要过滤级别
	if logLevel != "" {
		levelUpper := strings.ToUpper(logLevel)
		cmd += " | grep -i " + levelUpper
	}

	// 如果需要过滤节点
	if logNode > 0 {
		cmd += fmt.Sprintf(" | grep 'node.*%d'", logNode)
	}

	// 添加颜色高亮
	cmd += " | sed -e 's/ERROR/" + colorize("ERROR", "red") + "/g'" +
		" | sed -e 's/WARN/" + colorize("WARN", "yellow") + "/g'" +
		" | sed -e 's/INFO/" + colorize("INFO", "cyan") + "/g'" +
		" | sed -e 's/DEBUG/" + colorize("DEBUG", "dim") + "/g'"

	output, err := exec.RunCommandByShell(cmd)
	if err != nil && !followLogs {
		fmt.Println(ui.Error("Failed to read logs"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
		return
	}

	if !followLogs {
		fmt.Println(formatLogs(output))
		fmt.Println()
		fmt.Println(ui.Divider(60))
		fmt.Println()
		fmt.Println(ui.DimStyle.Render("  View live logs: v2sp logs -f"))
		fmt.Println(ui.DimStyle.Render("  Filter by level: v2sp logs --level error"))
		fmt.Println()
	}
}

func formatLogs(logs string) string {
	lines := strings.Split(logs, "\n")
	var formatted []string

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// 简单的日志格式化
		formatted = append(formatted, "  "+line)
	}

	return strings.Join(formatted, "\n")
}

func colorize(text, color string) string {
	colors := map[string]string{
		"red":    "\\033[0;31m",
		"green":  "\\033[0;32m",
		"yellow": "\\033[0;33m",
		"cyan":   "\\033[0;36m",
		"dim":    "\\033[2m",
	}
	reset := "\\033[0m"
	
	if c, ok := colors[color]; ok {
		return c + text + reset
	}
	return text
}

