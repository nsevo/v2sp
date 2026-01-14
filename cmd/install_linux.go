package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/nsevo/v2sp/common/exec"
	"github.com/spf13/cobra"
)

var targetVersion string

var (
	updateCommand = cobra.Command{
		Use:   "update",
		Short: "Update v2sp version",
		Run: func(_ *cobra.Command, _ []string) {
			// This project ships its own installer and stable binary distribution.
			// Keep update simple: download latest linux/amd64 binary and restart systemd service.
			//
			// Override URL via env: V2SP_DOWNLOAD_URL
			url := os.Getenv("V2SP_DOWNLOAD_URL")
			if url == "" {
				url = "https://resources.valtrogen.com/core/v2sp-linux-amd64"
			}

			// Determine installed path
			bin := "/usr/local/bin/v2sp"
			if _, err := os.Stat(bin); err != nil {
				// fallback to /bin/v2sp if someone installed it there
				if _, err2 := os.Stat("/bin/v2sp"); err2 == nil {
					bin = "/bin/v2sp"
				}
			}

			fmt.Println(Warn("Downloading: " + url))
			cmd := fmt.Sprintf(`set -e;
tmp="$(mktemp)";
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$tmp" "%s";
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp" "%s";
else
  echo "curl/wget not found" >&2; exit 1;
fi;
chmod +x "$tmp";
install -m 0755 "$tmp" "%s";
rm -f "$tmp";
systemctl daemon-reload >/dev/null 2>&1 || true;
systemctl restart v2sp.service >/dev/null 2>&1 || systemctl restart v2sp >/dev/null 2>&1 || true;
echo "updated: %s"`,
				url, url, bin, bin)
			out, err := exec.RunCommandByShell(cmd)
			if err != nil {
				fmt.Println(Err("update failed: ", err))
				if strings.TrimSpace(out) != "" {
					fmt.Println(out)
				}
				fmt.Println(Warn("If you run via systemd, check: journalctl -u v2sp -n 50 --no-pager"))
				return
			}
			fmt.Println(Ok("Update completed"))
			if strings.TrimSpace(out) != "" {
				fmt.Println(out)
			}
		},
		Args: cobra.NoArgs,
	}
	uninstallCommand = cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall v2sp",
		Run:   uninstallHandle,
	}
)

func init() {
	updateCommand.PersistentFlags().StringVar(&targetVersion, "version", "", "update target version")
	command.AddCommand(&updateCommand)
	command.AddCommand(&uninstallCommand)
}

func uninstallHandle(_ *cobra.Command, _ []string) {
	var yes string
	fmt.Println(Warn("确定要卸载 v2sp 吗?(Y/n)"))
	fmt.Scan(&yes)
	if strings.ToLower(yes) != "y" {
		fmt.Println("已取消卸载")
		return
	}
	_, err := exec.RunCommandByShell("systemctl stop v2sp&&systemctl disable v2sp")
	if err != nil {
		fmt.Println(Err("exec cmd error: ", err))
		fmt.Println(Err("卸载失败"))
		return
	}
	_ = os.RemoveAll("/etc/systemd/system/v2sp.service")
	// Preserve /etc/v2sp by default (safer). Remove binaries only.
	_ = os.RemoveAll("/usr/local/bin/v2sp")
	_ = os.RemoveAll("/bin/v2sp")
	_, err = exec.RunCommandByShell("systemctl daemon-reload&&systemctl reset-failed")
	if err != nil {
		fmt.Println(Err("exec cmd error: ", err))
		fmt.Println(Err("卸载失败"))
		return
	}
	fmt.Println(Ok("卸载成功"))
}
