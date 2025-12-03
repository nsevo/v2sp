package cmd

import (
	"fmt"

	"github.com/nsevo/v2sp/common/version"
	"github.com/spf13/cobra"
)

var (
	intro = "Multi-core backend for self-hosted panels"
)

var versionCommand = cobra.Command{
	Use:   "version",
	Short: "Print version info",
	Run: func(_ *cobra.Command, _ []string) {
		showVersion()
	},
}

func init() {
	command.AddCommand(&versionCommand)
}

func showVersion() {
	fmt.Printf("%s %s\n", version.Codename, version.Version)
	fmt.Println(intro)
	//fmt.Printf("Supported cores: %s\n", strings.Join(vCore.RegisteredCore(), ", "))
	// Warning
	//fmt.Println(Warn("This version need V2board version >= 1.7.0."))
	//fmt.Println(Warn("The version have many changed for config, please check your config file"))
}
