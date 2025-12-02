package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

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
	if _, err := exec.LookPath("journalctl"); err != nil {
		fmt.Println("journalctl not found. v2sp logs works on Linux with systemd.")
		return
	}

	// 构建 journalctl 命令 - 按时间正序显示
	args := []string{
		"-u", "v2sp",
		"-n", fmt.Sprintf("%d", logLines),
		"--no-pager",
		"--output=short-iso", // 标准 ISO 时间格式
	}

	if followLogs {
		args = append(args, "-f")
		fmt.Println("Following logs (Ctrl+C to exit)...")
		fmt.Println(strings.Repeat("─", 60))
	} else {
		fmt.Printf("Logs (last %d lines)\n", logLines)
		if logLevel != "" {
			fmt.Printf("Filter: %s\n", logLevel)
		}
		fmt.Println(strings.Repeat("─", 60))
	}

	// 创建命令
	cmd := exec.Command("journalctl", args...)

	// 设置标准输出和错误输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Println(ui.Error("Failed to create pipe: " + err.Error()))
		return
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		fmt.Println(ui.Error("Failed to start journalctl: " + err.Error()))
		return
	}

	// 设置信号处理（Ctrl+C）
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 创建完成通道
	done := make(chan bool)

	// 读取输出
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()

			// 级别过滤
			if logLevel != "" {
				levelUpper := strings.ToUpper(logLevel)
				if !strings.Contains(strings.ToUpper(line), levelUpper) {
					continue
				}
			}

			// 格式化输出
			printLogLine(line)
		}
		done <- true
	}()

	// 等待完成或中断
	select {
	case <-done:
		cmd.Wait()
	case <-sigChan:
		// 收到 Ctrl+C，终止进程
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Println("Interrupted")
	}

	if !followLogs {
		fmt.Println(strings.Repeat("─", 60))
		fmt.Println("Tip: v2sp logs -f (follow) | v2sp logs -l error (filter)")
	}
}

func printLogLine(line string) {
	// 简单的级别高亮
	prefix := "  "

	lineUpper := strings.ToUpper(line)
	if strings.Contains(lineUpper, "ERROR") {
		prefix = ui.ErrorStyle.Render("✗ ")
	} else if strings.Contains(lineUpper, "WARN") {
		prefix = ui.WarningStyle.Render("! ")
	} else if strings.Contains(lineUpper, "INFO") {
		prefix = ui.InfoStyle.Render("• ")
	}

	fmt.Println(prefix + line)
}
