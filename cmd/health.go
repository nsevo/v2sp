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
	Short: "Health check",
	Run:   healthHandle,
	Args:  cobra.NoArgs,
}

func init() {
	command.AddCommand(healthCommand)
}

func healthHandle(_ *cobra.Command, _ []string) {
	checks := []HealthCheck{
		{"Service", checkServiceStatus},
		{"Config", checkConfiguration},
		{"Network", checkNetwork},
		{"Disk", checkDiskSpace},
		{"Memory", checkMemoryUsage},
	}

	fmt.Println("Health Check")
	fmt.Println(strings.Repeat("─", 50))

	passed := 0
	failed := 0

	for _, check := range checks {
		result := check.Check()
		
		var status string
		switch result.Status {
		case "pass":
			status = ui.SuccessStyle.Render("✓")
			passed++
		case "warn":
			status = ui.WarningStyle.Render("!")
		case "fail":
			status = ui.ErrorStyle.Render("✗")
			failed++
		}

		fmt.Printf("%s %-12s %s\n", status, check.Name, result.Message)
	}

	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Result: %d passed, %d failed\n", passed, failed)
}

type HealthCheck struct {
	Name  string
	Check func() CheckResult
}

type CheckResult struct {
	Status     string // pass, warn, fail
	Message    string
	Suggestion string
}

func checkServiceStatus() CheckResult {
	running, err := checkRunning()
	if err != nil {
		return CheckResult{Status: "fail", Message: "Cannot check service"}
	}
	if !running {
		return CheckResult{Status: "fail", Message: "Not running"}
	}
	return CheckResult{Status: "pass", Message: "Running"}
}

func checkConfiguration() CheckResult {
	if _, err := os.Stat("/etc/v2sp/config.json"); os.IsNotExist(err) {
		return CheckResult{Status: "fail", Message: "Config not found"}
	}
	return CheckResult{Status: "pass", Message: "OK"}
}

func checkNetwork() CheckResult {
	output, err := exec.RunCommandByShell("curl -s -o /dev/null -w '%{http_code}' --connect-timeout 5 https://www.google.com 2>/dev/null")
	if err != nil || strings.TrimSpace(output) != "200" {
		return CheckResult{Status: "warn", Message: "Limited connectivity"}
	}
	return CheckResult{Status: "pass", Message: "OK"}
}

func checkDiskSpace() CheckResult {
	output, _ := exec.RunCommandByShell("df /etc/v2sp | tail -1 | awk '{print $5}' | sed 's/%//'")
	var usage int
	fmt.Sscanf(strings.TrimSpace(output), "%d", &usage)

	if usage > 90 {
		return CheckResult{Status: "fail", Message: fmt.Sprintf("%d%% (critical)", usage)}
	} else if usage > 80 {
		return CheckResult{Status: "warn", Message: fmt.Sprintf("%d%% (high)", usage)}
	}
	return CheckResult{Status: "pass", Message: fmt.Sprintf("%d%%", usage)}
}

func checkMemoryUsage() CheckResult {
	output, _ := exec.RunCommandByShell("free | grep Mem | awk '{print int($3/$2 * 100)}'")
	var usage int
	fmt.Sscanf(strings.TrimSpace(output), "%d", &usage)

	if usage > 90 {
		return CheckResult{Status: "warn", Message: fmt.Sprintf("%d%% (high)", usage)}
	}
	return CheckResult{Status: "pass", Message: fmt.Sprintf("%d%%", usage)}
}
