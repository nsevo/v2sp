package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var (
	ramDebugInterval time.Duration

	ramDebugCommand = &cobra.Command{
		Use:   "ramdebug",
		Short: "Continuously sample v2sp memory/FD to /etc/v2sp/ramdebug.log",
		Long:  "Side-car sampler for a running v2sp process. Writes VmRSS/VmSize/Threads and FD count to /etc/v2sp/ramdebug.log at the given interval. Ctrl+C to stop sampling without affecting v2sp.",
		Example: `
v2sp ramdebug           # sample every 5s (default)
v2sp ramdebug -i 2s     # sample every 2s`,
		Run: ramDebugHandle,
	}
)

func init() {
	ramDebugCommand.Flags().DurationVarP(&ramDebugInterval, "interval", "i", 5*time.Second, "sampling interval")
	command.AddCommand(ramDebugCommand)
}

func ramDebugHandle(_ *cobra.Command, _ []string) {
	pidStr, _ := exec.RunCommandByShell("pidof v2sp")
	pidStr = strings.TrimSpace(pidStr)
	if pidStr == "" {
		fmt.Println(Warn("v2sp is not running; start v2sp first then run ramdebug"))
		return
	}

	pid := strings.Fields(pidStr)[0]
	logPath := "/etc/v2sp/ramdebug.log"
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		fmt.Println(Err("创建日志目录失败: ", err))
		return
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(Err("打开日志文件失败: ", err))
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "=== ramdebug start %s pid=%s interval=%s ===\n", time.Now().Format(time.RFC3339), pid, ramDebugInterval)
	fmt.Printf("Start sampling v2sp (pid=%s) memory to %s; Ctrl+C to stop sampling (v2sp keeps running)\n", pid, logPath)

	ticker := time.NewTicker(ramDebugInterval)
	defer ticker.Stop()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			writeSample(f, pid)
		case <-stopCh:
			writeSample(f, pid)
			fmt.Fprintln(f, "=== ramdebug stop", time.Now().Format(time.RFC3339), "===")
			f.Sync()
			fmt.Println(Ok("Sampling stopped; v2sp was not touched"))
			return
		}
	}
}

func writeSample(f *os.File, pid string) {
	now := time.Now().Format(time.RFC3339)
	status := readStatus(pid)
	fdCmd := fmt.Sprintf("ls /proc/%s/fd 2>/dev/null | wc -l", pid)
	fdOut, _ := exec.RunCommandByShell(fdCmd)
	fdCount := strings.TrimSpace(fdOut)
	if fdCount == "" {
		fdCount = "N/A"
	}
	fmt.Fprintf(f, "[%s] %s fd=%s\n", now, status, fdCount)
}

func readStatus(pid string) string {
	path := fmt.Sprintf("/proc/%s/status", pid)
	file, err := os.Open(path)
	if err != nil {
		return fmt.Sprintf("status=err(%v)", err)
	}
	defer file.Close()

	var vmRSS, vmSize, threads string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			vmRSS = strings.TrimSpace(line[len("VmRSS:"):])
		} else if strings.HasPrefix(line, "VmSize:") {
			vmSize = strings.TrimSpace(line[len("VmSize:"):])
		} else if strings.HasPrefix(line, "Threads:") {
			threads = strings.TrimSpace(line[len("Threads:"):])
		}
		if vmRSS != "" && vmSize != "" && threads != "" {
			break
		}
	}
	if vmRSS == "" && vmSize == "" {
		return "status=N/A"
	}
	return fmt.Sprintf("VmRSS=%s VmSize=%s Threads=%s", vmRSS, vmSize, threads)
}
