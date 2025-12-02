package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var (
	watchStatus bool
)

var statusCommand = &cobra.Command{
	Use:   "status",
	Short: "Show v2sp status",
	Long:  "Display detailed status information about v2sp service",
	Run:   statusHandle,
	Args:  cobra.NoArgs,
}

func init() {
	statusCommand.Flags().BoolVarP(&watchStatus, "watch", "w", false, "Watch mode (refresh every 2s)")
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
	// 获取服务状态
	running, err := checkRunning()
	if err != nil {
		fmt.Println(ui.Error("Failed to check service status"))
		fmt.Println(ui.DimStyle.Render("  " + err.Error()))
		return
	}

	// 检查是否安装
	if _, err := os.Stat("/usr/local/v2sp/v2sp"); os.IsNotExist(err) {
		fmt.Println(ui.Header("v2sp", ""))
		fmt.Println(ui.Error("v2sp is not installed"))
		fmt.Println()
		fmt.Println(ui.DimStyle.Render("  Install with: curl -fsSL https://get.v2sp.io | bash"))
		fmt.Println()
		return
	}

	// 获取详细信息
	info := getServiceInfo(running)

	// 渲染状态页面
	renderStatus(info)
}

func watchStatusLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		// 清屏
		fmt.Print("\033[H\033[2J")

		showStatus()

		fmt.Println()
		fmt.Println(ui.DimStyle.Render("  Press Ctrl+C to exit"))

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
	ConfigPath   string
	LogPath      string
	NodesActive  int
	NodesError   int
}

func getServiceInfo(running bool) *ServiceInfo {
	info := &ServiceInfo{
		Running:    running,
		Version:    getVersion(),
		ConfigPath: "/etc/v2sp/config.json",
		LogPath:    "/etc/v2sp/error.log",
	}

	if running {
		// 获取 PID
		pid, _ := exec.RunCommandByShell("pgrep -f '/usr/local/v2sp/v2sp'")
		info.PID = strings.TrimSpace(pid)

		// 获取运行时间
		if info.PID != "" {
			uptime, _ := exec.RunCommandByShell(fmt.Sprintf("ps -p %s -o etimes=", info.PID))
			if seconds := strings.TrimSpace(uptime); seconds != "" {
				var uptimeSeconds int64
				fmt.Sscanf(seconds, "%d", &uptimeSeconds)
				info.Uptime = ui.FormatUptime(uptimeSeconds)
			}

			// 获取内存使用
			mem, _ := exec.RunCommandByShell(fmt.Sprintf("ps -p %s -o rss=", info.PID))
			if memKB := strings.TrimSpace(mem); memKB != "" {
				var memBytes int64
				fmt.Sscanf(memKB, "%d", &memBytes)
				info.Memory = ui.FormatBytes(memBytes * 1024)
			}
		}

		// 检查节点状态（简化版，实际应该从配置文件读取）
		info.NodesActive = 2 // TODO: 从实际配置读取
		info.NodesError = 0
	}

	// 检查自动启动
	if checkAutoStart() {
		info.AutoStart = true
	}

	return info
}

func getVersion() string {
	output, err := exec.RunCommandByShell("/usr/local/v2sp/v2sp version 2>/dev/null | head -n1")
	if err != nil {
		return "unknown"
	}
	// 提取版本号
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

func renderStatus(info *ServiceInfo) {
	// 页面头部
	fmt.Print(ui.Header("v2sp", ""))

	// 服务状态
	var statusText string
	if info.Running {
		statusText = ui.StatusLine("running", "Running")
	} else {
		statusText = ui.StatusLine("stopped", "Stopped")
	}

	var autoStartText string
	if info.AutoStart {
		autoStartText = ui.SuccessStyle.Render("Enabled")
	} else {
		autoStartText = ui.DimStyle.Render("Disabled")
	}

	// 基本信息
	lines := []string{
		fmt.Sprintf("%s  %s  %s",
			statusText,
			ui.DimStyle.Render("•"),
			ui.KeyValue("Version", info.Version),
		),
	}

	if info.Running && info.Uptime != "" {
		lines = append(lines, "")
		lines = append(lines,
			ui.KeyValue("Uptime", info.Uptime),
			ui.KeyValue("PID", info.PID),
			ui.KeyValue("Memory", info.Memory),
		)
	}

	lines = append(lines, "")
	lines = append(lines, ui.KeyValue("Auto-start", autoStartText))

	fmt.Println(ui.Section(lines...))
	fmt.Println()

	// 节点信息
	if info.Running {
		fmt.Println(ui.Section(
			ui.DimStyle.Render("Nodes"),
			"",
			fmt.Sprintf("  %s  %d active, %d error",
				ui.StatusDotRunning.Render(),
				info.NodesActive,
				info.NodesError,
			),
		))
		fmt.Println()
	}

	// 系统信息
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	fmt.Println(ui.Section(
		ui.DimStyle.Render("System"),
		"",
		ui.KeyValue("Go Version", runtime.Version()),
		ui.KeyValue("OS/Arch", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)),
		ui.KeyValue("CPUs", fmt.Sprintf("%d", runtime.NumCPU())),
	))

	// 页面底部 - 快捷操作提示
	fmt.Print(ui.Footer(
		ui.Key("R", "Restart"),
		ui.Key("L", "Logs"),
		ui.Key("E", "Edit config"),
		ui.Key("M", "Menu"),
	))

	// 额外提示
	if !info.Running {
		fmt.Println(ui.DimStyle.Render("  Start service: v2sp start"))
	} else {
		fmt.Println(ui.DimStyle.Render("  View logs: v2sp logs -f"))
	}
	fmt.Println()
}

