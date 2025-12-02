package cmd

import (
	"fmt"
	"os"

	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var (
	logCommand = &cobra.Command{
		Use:   "log",
		Short: "View or clean v2sp logs",
		Run:   logTailHandle,
	}

	logCleanCommand = &cobra.Command{
		Use:   "clean",
		Short: "Clear v2sp logs (journal + error.log)",
		Run:   logCleanHandle,
		Args:  cobra.NoArgs,
	}
)

func init() {
	logCommand.AddCommand(logCleanCommand)
	command.AddCommand(logCommand)
}

func logTailHandle(_ *cobra.Command, _ []string) {
	exec.RunCommandStd("journalctl", "-u", "v2sp.service", "-e", "--no-pager", "-f")
}

func logCleanHandle(_ *cobra.Command, _ []string) {
	logFiles := []string{
		"/etc/v2sp/error.log",
		"/etc/v2sp/access.log",
	}

	for _, file := range logFiles {
		if file == "" {
			continue
		}
		if _, err := os.Stat(file); err == nil {
			if err := os.Truncate(file, 0); err != nil {
				fmt.Printf("Failed to truncate %s: %v\n", file, err)
			} else {
				fmt.Printf("Cleared %s\n", file)
			}
		}
	}

	_, err := exec.RunCommandByShell("journalctl -u v2sp --rotate >/dev/null 2>&1 && journalctl -u v2sp --vacuum-time=1s >/dev/null 2>&1")
	if err != nil {
		fmt.Println("journalctl cleanup skipped or failed:", err)
	} else {
		fmt.Println("Cleared systemd journal for v2sp")
	}

	fmt.Println("Log cleanup completed")
}
