package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"

	_ "github.com/nsevo/v2sp/core/xray"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var command = &cobra.Command{
	Use:   "v2sp",
	Short: "Multi-core backend for self-hosted panels",
	Long:  "v2sp - A modern, high-performance proxy backend",
	Run:   defaultCommand,
}

// defaultCommand - 无参数时的行为
func defaultCommand(_ *cobra.Command, _ []string) {
	// 如果是 root 用户或有 sudo，显示交互式菜单
	if os.Geteuid() == 0 {
		p := tea.NewProgram(initialModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			log.WithField("err", err).Error("Failed to start menu")
			os.Exit(1)
		}
	} else {
		// 非 root 用户，显示状态
		showStatus()
	}
}

func Run() {
	err := command.Execute()
	if err != nil {
		log.WithField("err", err).Error("Execute command failed")
	}
}
