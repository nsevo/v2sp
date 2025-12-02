package cmd

import (
	"fmt"
	"os"

	"github.com/nsevo/v2sp/cmd/ui"
	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var systemCommand = &cobra.Command{
	Use:   "system",
	Short: "System management",
	Long:  "Manage system-level configuration (systemd, directories, etc.)",
}

var systemSetupCommand = &cobra.Command{
	Use:   "setup",
	Short: "Setup system configuration",
	Long:  "Create systemd service, directories, and other system configurations",
	Run:   systemSetupHandle,
	Args:  cobra.NoArgs,
}

func init() {
	systemCommand.AddCommand(systemSetupCommand)
	command.AddCommand(systemCommand)
}

func systemSetupHandle(_ *cobra.Command, _ []string) {
	fmt.Print(ui.Header("System Setup", "Configuring system-level settings"))

	steps := []struct {
		Name string
		Func func() error
	}{
		{"Creating directories", createDirectories},
		{"Installing systemd service", installSystemdService},
		{"Enabling auto-start", enableAutoStart},
	}

	for _, step := range steps {
		fmt.Print(ui.DimStyle.Render("  " + ui.IconSpinner1 + " " + step.Name + "..."))
		
		err := step.Func()
		
		// 清除当前行
		fmt.Print("\r\033[K")
		
		if err != nil {
			fmt.Println(ui.Error(step.Name + " failed"))
			fmt.Println(ui.DimStyle.Render("  " + err.Error()))
		} else {
			fmt.Println(ui.Success(step.Name))
		}
	}

	fmt.Println()
	fmt.Println(ui.Divider(60))
	fmt.Println()
	fmt.Println(ui.Success("System setup complete"))
	fmt.Println()
	fmt.Println(ui.DimStyle.Render("  Next steps:"))
	fmt.Println(ui.DimStyle.Render("  1. Generate config: v2sp config init"))
	fmt.Println(ui.DimStyle.Render("  2. Start service: v2sp start"))
	fmt.Println()
}

func createDirectories() error {
	dirs := []string{
		"/etc/v2sp",
		"/etc/v2sp/cert",
		"/usr/local/v2sp",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %v", dir, err)
		}
	}

	return nil
}

func installSystemdService() error {
	serviceContent := `[Unit]
Description=v2sp Service
After=network.target nss-lookup.target
Wants=network.target

[Service]
User=root
Group=root
Type=simple
LimitAS=infinity
LimitRSS=infinity
LimitCORE=infinity
LimitNOFILE=999999
WorkingDirectory=/usr/local/v2sp/
ExecStart=/usr/local/v2sp/v2sp server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
`

	err := os.WriteFile("/etc/systemd/system/v2sp.service", []byte(serviceContent), 0644)
	if err != nil {
		return err
	}

	// Reload systemd
	_, err = exec.RunCommandByShell("systemctl daemon-reload")
	return err
}

func enableAutoStart() error {
	_, err := exec.RunCommandByShell("systemctl enable v2sp")
	return err
}

