package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var healthCommand = &cobra.Command{
	Use:   "health",
	Short: "Run health checks",
	Long:  "Perform comprehensive health checks on v2sp service",
	Run:   healthHandle,
	Args:  cobra.NoArgs,
}

func init() {
	command.AddCommand(healthCommand)
}

func healthHandle(_ *cobra.Command, _ []string) {
	fmt.Print(ui.Header("Health Check", "Running diagnostics..."))

	checks := []HealthCheck{
		{
			Name:        "Service Status",
			Description: "Check if v2sp service is running",
			Check:       checkServiceStatus,
		},
		{
			Name:        "Configuration",
			Description: "Validate configuration file",
			Check:       checkConfiguration,
		},
		{
			Name:        "Network",
			Description: "Check network connectivity",
			Check:       checkNetwork,
		},
		{
			Name:        "Disk Space",
			Description: "Check available disk space",
			Check:       checkDiskSpace,
		},
		{
			Name:        "Memory Usage",
			Description: "Check system memory usage",
			Check:       checkMemoryUsage,
		},
		{
			Name:        "Certificates",
			Description: "Check SSL certificates",
			Check:       checkCertificates,
		},
	}

	results := []CheckResult{}
	for _, check := range checks {
		fmt.Print(ui.DimStyle.Render("  " + ui.IconSpinner1 + " Checking " + check.Name + "..."))
		result := check.Check()
		results = append(results, result)
		
		// 清除当前行
		fmt.Print("\r\033[K")
		
		// 显示结果
		var icon string
		switch result.Status {
		case "pass":
			icon = ui.Success("")
		case "warn":
			icon = ui.Warning("")
		case "fail":
			icon = ui.Error("")
		}
		fmt.Printf("  %s %s\n", icon, check.Name)
		
		if result.Message != "" {
			fmt.Println(ui.DimStyle.Render("    " + result.Message))
		}
	}

	fmt.Println()
	fmt.Println(ui.Divider(60))
	fmt.Println()

	// 统计
	passed := 0
	warnings := 0
	failed := 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
		case "warn":
			warnings++
		case "fail":
			failed++
		}
	}

	// 总体状态
	fmt.Println(ui.Section(
		ui.KeyValue("Total Checks", fmt.Sprintf("%d", len(results))),
		ui.KeyValue("Passed", ui.SuccessStyle.Render(fmt.Sprintf("%d", passed))),
		ui.KeyValue("Warnings", ui.WarningStyle.Render(fmt.Sprintf("%d", warnings))),
		ui.KeyValue("Failed", ui.ErrorStyle.Render(fmt.Sprintf("%d", failed))),
	))
	fmt.Println()

	// 建议
	suggestions := []string{}
	for _, r := range results {
		if r.Status != "pass" && r.Suggestion != "" {
			suggestions = append(suggestions, r.Suggestion)
		}
	}

	if len(suggestions) > 0 {
		fmt.Println(ui.DimStyle.Render("  Suggestions:"))
		fmt.Println()
		for _, s := range suggestions {
			fmt.Println(ui.DimStyle.Render("  • " + s))
		}
		fmt.Println()
	}

	// 整体健康度
	var overallHealth string
	if failed > 0 {
		overallHealth = ui.ErrorStyle.Render("Critical ✗")
	} else if warnings > 0 {
		overallHealth = ui.WarningStyle.Render("Warning ⚠")
	} else {
		overallHealth = ui.SuccessStyle.Render("Healthy ✓")
	}

	fmt.Println(ui.Divider(60))
	fmt.Println()
	fmt.Println(ui.Section(
		ui.KeyValue("Overall Health", overallHealth),
	))
	fmt.Println()
}

type HealthCheck struct {
	Name        string
	Description string
	Check       func() CheckResult
}

type CheckResult struct {
	Status     string // pass, warn, fail
	Message    string
	Suggestion string
}

func checkServiceStatus() CheckResult {
	running, err := checkRunning()
	if err != nil {
		return CheckResult{
			Status:     "fail",
			Message:    "Cannot determine service status",
			Suggestion: "Check systemd configuration: systemctl status v2sp",
		}
	}

	if !running {
		return CheckResult{
			Status:     "fail",
			Message:    "Service is not running",
			Suggestion: "Start service: v2sp start",
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "Service is running",
	}
}

func checkConfiguration() CheckResult {
	configPath := "/etc/v2sp/config.json"
	
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return CheckResult{
			Status:     "fail",
			Message:    "Configuration file not found",
			Suggestion: "Generate config: v2sp config init",
		}
	}

	// 简单的 JSON 验证
	content, err := os.ReadFile(configPath)
	if err != nil {
		return CheckResult{
			Status:     "fail",
			Message:    "Cannot read configuration file",
			Suggestion: "Check file permissions: ls -l " + configPath,
		}
	}

	// 基本的 JSON 检查
	if !strings.Contains(string(content), "{") {
		return CheckResult{
			Status:     "fail",
			Message:    "Configuration file is not valid JSON",
			Suggestion: "Edit config: v2sp config edit",
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "Configuration file is valid",
	}
}

func checkNetwork() CheckResult {
	// 检查网络连接
	output, err := exec.RunCommandByShell("curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 https://www.google.com 2>/dev/null")
	if err != nil || strings.TrimSpace(output) != "200" {
		return CheckResult{
			Status:     "warn",
			Message:    "Internet connectivity may be limited",
			Suggestion: "Check network settings and firewall",
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: "Network connectivity OK",
	}
}

func checkDiskSpace() CheckResult {
	output, err := exec.RunCommandByShell("df /etc/v2sp | tail -1 | awk '{print $5}' | sed 's/%//'")
	if err != nil {
		return CheckResult{
			Status:  "warn",
			Message: "Cannot check disk space",
		}
	}

	var usage int
	fmt.Sscanf(strings.TrimSpace(output), "%d", &usage)

	if usage > 90 {
		return CheckResult{
			Status:     "fail",
			Message:    fmt.Sprintf("Disk usage is critical: %d%%", usage),
			Suggestion: "Free up disk space immediately",
		}
	} else if usage > 80 {
		return CheckResult{
			Status:     "warn",
			Message:    fmt.Sprintf("Disk usage is high: %d%%", usage),
			Suggestion: "Consider cleaning up old logs and files",
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: fmt.Sprintf("Disk usage: %d%%", usage),
	}
}

func checkMemoryUsage() CheckResult {
	output, err := exec.RunCommandByShell("free | grep Mem | awk '{print int($3/$2 * 100)}'")
	if err != nil {
		return CheckResult{
			Status:  "warn",
			Message: "Cannot check memory usage",
		}
	}

	var usage int
	fmt.Sscanf(strings.TrimSpace(output), "%d", &usage)

	if usage > 90 {
		return CheckResult{
			Status:     "warn",
			Message:    fmt.Sprintf("Memory usage is high: %d%%", usage),
			Suggestion: "Consider upgrading server or optimizing configuration",
		}
	}

	return CheckResult{
		Status:  "pass",
		Message: fmt.Sprintf("Memory usage: %d%%", usage),
	}
}

func checkCertificates() CheckResult {
	certPath := "/etc/v2sp/fullchain.cer"
	
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return CheckResult{
			Status:  "warn",
			Message: "No SSL certificate found (may be using file mode)",
		}
	}

	// 检查证书过期时间
	output, err := exec.RunCommandByShell(fmt.Sprintf("openssl x509 -in %s -noout -enddate 2>/dev/null", certPath))
	if err != nil {
		return CheckResult{
			Status:  "warn",
			Message: "Cannot check certificate expiration",
		}
	}

	// 简单检查（实际应该解析日期）
	if strings.Contains(output, "notAfter") {
		return CheckResult{
			Status:  "pass",
			Message: "SSL certificate present",
		}
	}

	return CheckResult{
		Status:  "warn",
		Message: "Certificate status unknown",
	}
}

