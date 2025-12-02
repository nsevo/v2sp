package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var (
	watchStatus bool
	simpleMode  bool
)

var statusCommand = &cobra.Command{
	Use:   "status",
	Short: "Show status",
	Run:   statusHandle,
	Args:  cobra.NoArgs,
}

func init() {
	statusCommand.Flags().BoolVarP(&watchStatus, "watch", "w", false, "Watch mode")
	statusCommand.Flags().BoolVarP(&simpleMode, "simple", "s", false, "Simple output")
	command.AddCommand(statusCommand)
}

func statusHandle(_ *cobra.Command, _ []string) {
	if watchStatus {
		watchStatusLoop()
		return
	}

	showStatus()
}

func showStatus() {
	// 检查安装
	if _, err := os.Stat("/usr/local/v2sp/v2sp"); os.IsNotExist(err) {
		fmt.Println("✗ v2sp not installed")
		fmt.Println("  Run: wget -N https://raw.githubusercontent.com/nsevo/v2sp-script/master/install.sh && bash install.sh")
		return
	}

	// 获取状态
	running, _ := checkRunning()
	info := getServiceInfo(running)

	if simpleMode {
		// 极简输出
		if info.Running {
			fmt.Println("running")
		} else {
			fmt.Println("stopped")
		}
		return
	}

	// 标准输出
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("v2sp %s\n", info.Version)
	fmt.Println(strings.Repeat("─", 50))

	// 状态
	if info.Running {
		fmt.Printf("Status:    %s\n", ui.SuccessStyle.Render("● running"))
	} else {
		fmt.Printf("Status:    %s\n", ui.DimStyle.Render("○ stopped"))
	}

	// 运行信息
	if info.Running {
		fmt.Printf("Uptime:    %s\n", info.Uptime)
		fmt.Printf("PID:       %s\n", info.PID)
		fmt.Printf("Memory:    %s\n", info.Memory)
	}

	// 自动启动
	if info.AutoStart {
		fmt.Printf("Boot:      %s\n", ui.SuccessStyle.Render("enabled"))
	} else {
		fmt.Printf("Boot:      %s\n", ui.DimStyle.Render("disabled"))
	}

	// 节点
	if info.Running {
		fmt.Printf("Nodes:     %d active\n", info.NodesActive)
	}

	fmt.Println(strings.Repeat("─", 50))

	// 快捷提示
	if !info.Running {
		fmt.Println("v2sp start | v2sp menu")
	} else {
		fmt.Println("v2sp logs -f | v2sp menu | v2sp restart")
	}
}

func watchStatusLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		// 清屏
		fmt.Print("\033[H\033[2J")
		showStatus()
		fmt.Println("\nPress Ctrl+C to exit")
		<-ticker.C
	}
}

type ServiceInfo struct {
	Running      bool
	Version      string
	Uptime       string
	PID          string
	Memory       string
	AutoStart    bool
	NodesActive  int
}

func getServiceInfo(running bool) *ServiceInfo {
	info := &ServiceInfo{
		Running: running,
		Version: getVersion(),
	}

	if running {
		// PID
		pid, _ := exec.RunCommandByShell("pgrep -f '/usr/local/v2sp/v2sp'")
		info.PID = strings.TrimSpace(pid)

		// Uptime
		if info.PID != "" {
			uptime, _ := exec.RunCommandByShell(fmt.Sprintf("ps -p %s -o etimes=", info.PID))
			if seconds := strings.TrimSpace(uptime); seconds != "" {
				var uptimeSeconds int64
				fmt.Sscanf(seconds, "%d", &uptimeSeconds)
				info.Uptime = ui.FormatUptime(uptimeSeconds)
			}

			// Memory
			mem, _ := exec.RunCommandByShell(fmt.Sprintf("ps -p %s -o rss=", info.PID))
			if memKB := strings.TrimSpace(mem); memKB != "" {
				var memBytes int64
				fmt.Sscanf(memKB, "%d", &memBytes)
				info.Memory = ui.FormatBytes(memBytes * 1024)
			}
		}

		info.NodesActive = 2 // TODO: 从配置读取
	}

	// 自动启动
	info.AutoStart = checkAutoStart()

	return info
}

func getVersion() string {
	output, err := exec.RunCommandByShell("/usr/local/v2sp/v2sp version 2>/dev/null | head -n1")
	if err != nil {
		return "unknown"
	}
	parts := strings.Fields(output)
	if len(parts) >= 2 {
		return parts[1]
	}
	return strings.TrimSpace(output)
}

func checkAutoStart() bool {
	output, _ := exec.RunCommandByShell("systemctl is-enabled v2sp 2>/dev/null")
	return strings.TrimSpace(output) == "enabled"
}
