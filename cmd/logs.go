package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/spf13/cobra"
)

var (
	followLogs bool
	logLines   int
	logLevel   string
)

var logsCommand = &cobra.Command{
	Use:   "logs",
	Short: "View logs",
	Long:  "View v2sp service logs",
	Run:   logsHandle,
	Args:  cobra.NoArgs,
}

func init() {
	logsCommand.Flags().BoolVarP(&followLogs, "follow", "f", false, "Follow log output")
	logsCommand.Flags().IntVarP(&logLines, "lines", "n", 50, "Number of lines")
	logsCommand.Flags().StringVarP(&logLevel, "level", "l", "", "Filter by level (error, warn, info, debug)")
	command.AddCommand(logsCommand)
}

func logsHandle(_ *cobra.Command, _ []string) {
	// 构建 journalctl 命令
	args := []string{"-u", "v2sp", "-n", fmt.Sprintf("%d", logLines), "--no-pager"}
	
	if followLogs {
		args = append(args, "-f")
	}

	// 级别过滤
	var grepArgs []string
	if logLevel != "" {
		grepArgs = []string{"grep", "-i", "--line-buffered", strings.ToUpper(logLevel)}
	}

	// 显示简单头部
	if !followLogs {
		fmt.Printf("Logs (last %d lines)\n", logLines)
		if logLevel != "" {
			fmt.Printf("Filter: %s\n", logLevel)
		}
		fmt.Println(strings.Repeat("─", 60))
	}

	// 执行命令
	cmd := exec.Command("journalctl", args...)
	
	if len(grepArgs) > 0 && followLogs {
		// 使用管道过滤
		grep := exec.Command(grepArgs[0], grepArgs[1:]...)
		
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Println(ui.Error("Failed to create pipe: " + err.Error()))
			return
		}
		
		grep.Stdin = pipe
		grep.Stdout = os.Stdout
		grep.Stderr = os.Stderr
		
		if err := cmd.Start(); err != nil {
			fmt.Println(ui.Error("Failed to start journalctl: " + err.Error()))
			return
		}
		
		if err := grep.Start(); err != nil {
			fmt.Println(ui.Error("Failed to start grep: " + err.Error()))
			return
		}
		
		cmd.Wait()
		grep.Wait()
	} else {
		// 直接输出
		output, err := cmd.Output()
		if err != nil {
			fmt.Println(ui.Error("Failed to read logs"))
			return
		}
		
		// 格式化输出
		formatAndPrint(string(output), logLevel)
		
		if !followLogs {
			fmt.Println(strings.Repeat("─", 60))
			fmt.Println("Tip: v2sp logs -f (follow) | v2sp logs -l error (filter)")
		}
	}
}

func formatAndPrint(logs, levelFilter string) {
	scanner := bufio.NewScanner(strings.NewReader(logs))
	
	for scanner.Scan() {
		line := scanner.Text()
		
		// 级别过滤
		if levelFilter != "" {
			if !strings.Contains(strings.ToUpper(line), strings.ToUpper(levelFilter)) {
				continue
			}
		}
		
		// 简单高亮
		if strings.Contains(line, "ERROR") || strings.Contains(line, "error") {
			fmt.Print(ui.ErrorStyle.Render("• "))
		} else if strings.Contains(line, "WARN") || strings.Contains(line, "warning") {
			fmt.Print(ui.WarningStyle.Render("• "))
		} else if strings.Contains(line, "INFO") || strings.Contains(line, "info") {
			fmt.Print(ui.DimStyle.Render("• "))
		} else {
			fmt.Print("  ")
		}
		
		fmt.Println(line)
	}
}
